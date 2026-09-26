import type { RemoteTrackKind } from "../../types/rtc";
import { logger } from "../../utils/logger";

export interface RemoteTrackEvent {
  userId: string;
  kind: RemoteTrackKind;
  stream: MediaStream;
  track: MediaStreamTrack;
}

export interface RTCServiceCallbacks {
  /** An SDP this client produced and must send to the SFU. */
  onSignal: (type: "offer" | "answer", sdp: string) => void;
  onIceCandidate: (candidate: RTCIceCandidate) => void;
  onRemoteTrack: (event: RemoteTrackEvent) => void;
  onRemoteTrackEnded: (userId: string, kind: RemoteTrackKind) => void;
  onConnectionStateChange: (state: RTCPeerConnectionState) => void;
}

/**
 * Splits the identity the SFU gives every forwarded track ("mic.<userId>").
 *
 * Browsers name their own tracks with random UUIDs, so a forwarded track can only
 * be attributed to a person if the SFU renames it — which it does, in both halves
 * of the msid. Anything that does not parse is not one of ours.
 */
function parseTrackName(
  name: string,
): { kind: RemoteTrackKind; userId: string } | null {
  const separator = name.indexOf(".");
  if (separator <= 0) return null;

  const kind = name.slice(0, separator);
  const userId = name.slice(separator + 1);
  if (!userId) return null;
  if (kind !== "mic" && kind !== "screen") return null;

  return { kind, userId };
}

class RTCService {
  private pc: RTCPeerConnection | null = null;
  private localAudioStream: MediaStream | null = null;
  private localScreenStream: MediaStream | null = null;
  private screenSender: RTCRtpSender | null = null;
  private callbacks: RTCServiceCallbacks | null = null;

  /** Candidates that arrived before there was a remote description to attach them to. */
  private pendingCandidates: RTCIceCandidateInit[] = [];

  /** A local change that could not be offered yet because an exchange was open. */
  private negotiationQueued = false;

  /**
   * Signaling runs one step at a time. Offers, answers and candidates arrive in
   * bursts over the same socket, and applying two of them concurrently leaves the
   * connection in a state neither side expects.
   */
  private chain: Promise<void> = Promise.resolve();

  private enqueue(operation: () => Promise<void>): Promise<void> {
    this.chain = this.chain
      .catch(() => {})
      .then(operation)
      .catch((err) => {
        logger.error("RTCService: Signaling step failed", { err });
      });
    return this.chain;
  }

  public async initialize(
    callbacks: RTCServiceCallbacks,
  ): Promise<RTCPeerConnection> {
    this.close();
    this.callbacks = callbacks;

    const config: RTCConfiguration = {
      iceServers: [
        { urls: "stun:stun.l.google.com:19302" },
        { urls: "stun:stun1.l.google.com:19302" },
      ],
    };

    const pc = new RTCPeerConnection(config);
    this.pc = pc;

    pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.callbacks?.onIceCandidate(event.candidate);
      }
    };

    // Either side may need to renegotiate: this client when it adds or drops a
    // screen share, the SFU whenever someone else joins, leaves or starts sharing.
    // This handler covers our half; handleOffer covers theirs.
    pc.onnegotiationneeded = () => this.negotiate();

    pc.ontrack = (event) => this.handleRemoteTrack(event);

    pc.onconnectionstatechange = () => {
      const state = pc.connectionState;
      logger.info("RTCService: Connection state changed", { state });
      this.callbacks?.onConnectionStateChange(state);
    };

    pc.oniceconnectionstatechange = () => {
      // A network change (wifi to cellular, VPN, laptop waking) invalidates the
      // candidate pair rather than the session. Restarting ICE recovers the call
      // without tearing the room's media down and rebuilding it.
      if (pc.iceConnectionState === "failed") {
        logger.warn("RTCService: ICE failed, restarting");
        pc.restartIce();
      }
    };

    return pc;
  }

  /**
   * Offers this client's current track set, or remembers to once the exchange in
   * flight is over. A dropped renegotiation means media that never starts, so it
   * is deferred rather than skipped.
   */
  private negotiate(): void {
    void this.enqueue(async () => {
      if (!this.pc) return;
      if (this.pc.signalingState !== "stable") {
        this.negotiationQueued = true;
        return;
      }
      this.negotiationQueued = false;
      await this.pc.setLocalDescription();
      if (this.pc.localDescription) {
        logger.debug("RTCService: Offering updated track set to SFU");
        this.callbacks?.onSignal("offer", this.pc.localDescription.sdp);
      }
    });
  }

  /** Runs a renegotiation that was deferred while an exchange was open. */
  private resumeQueuedNegotiation(): void {
    if (!this.negotiationQueued) return;
    this.negotiationQueued = false;
    this.negotiate();
  }

  private handleRemoteTrack(event: RTCTrackEvent): void {
    const track = event.track;
    const stream = event.streams[0] ?? new MediaStream([track]);
    const identity = parseTrackName(stream.id) ?? parseTrackName(track.id);

    if (!identity) {
      logger.warn("RTCService: Ignoring unidentified remote track", {
        streamId: stream.id,
        trackId: track.id,
        kind: track.kind,
      });
      return;
    }

    logger.info("RTCService: Received remote track", identity);
    this.callbacks?.onRemoteTrack({ ...identity, stream, track });

    const drop = () => {
      this.callbacks?.onRemoteTrackEnded(identity.userId, identity.kind);
    };
    track.addEventListener("ended", drop);
    stream.addEventListener("removetrack", (e) => {
      if (e.track.id === track.id) drop();
    });
  }

  public async startLocalAudio(): Promise<MediaStream> {
    try {
      this.localAudioStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: false,
      });

      if (this.pc && this.localAudioStream) {
        this.localAudioStream.getAudioTracks().forEach((track) => {
          this.pc?.addTrack(track, this.localAudioStream!);
        });
      }

      logger.info("RTCService: Acquired local audio stream");
      return this.localAudioStream;
    } catch (err) {
      logger.error("RTCService: Failed to acquire local audio", { err });
      throw err;
    }
  }

  public getLocalAudioStream(): MediaStream | null {
    return this.localAudioStream;
  }

  public async startScreenShare(): Promise<MediaStream> {
    if (!this.pc) throw new Error("RTC connection not initialized");

    const stream = await navigator.mediaDevices.getDisplayMedia({
      video: {
        frameRate: { ideal: 30, max: 60 },
        width: { ideal: 1920, max: 3840 },
        height: { ideal: 1080, max: 2160 },
      },
      audio: false,
    });

    const [videoTrack] = stream.getVideoTracks();
    if (!videoTrack) {
      stream.getTracks().forEach((t) => t.stop());
      throw new Error("Screen share produced no video track");
    }

    this.localScreenStream = stream;
    // Adding the track fires negotiationneeded, which publishes it to the SFU.
    this.screenSender = this.pc.addTrack(videoTrack, stream);
    logger.info("RTCService: Started screen sharing");
    return stream;
  }

  public stopScreenShare(): void {
    if (this.screenSender && this.pc) {
      // Removing the sender — not just stopping the track — is what tells the SFU
      // the share is over, via the renegotiation it triggers.
      try {
        this.pc.removeTrack(this.screenSender);
      } catch (err) {
        logger.warn("RTCService: Failed to detach screen sender", { err });
      }
    }
    this.screenSender = null;

    if (this.localScreenStream) {
      this.localScreenStream.getTracks().forEach((track) => track.stop());
      this.localScreenStream = null;
      logger.info("RTCService: Stopped screen sharing");
    }
  }

  public setAudioMuted(muted: boolean): void {
    if (this.localAudioStream) {
      this.localAudioStream.getAudioTracks().forEach((track) => {
        track.enabled = !muted;
      });
    }
  }

  /**
   * Applies an offer from the SFU and answers it.
   *
   * This client is the polite peer: if it had an offer of its own in flight when
   * this one arrived, setRemoteDescription rolls that offer back implicitly and
   * negotiationneeded fires again afterwards, so its change is not lost.
   */
  public handleOffer(sdp: string): Promise<void> {
    return this.enqueue(async () => {
      if (!this.pc) return;
      await this.pc.setRemoteDescription({ type: "offer", sdp });
      await this.flushCandidates();
      await this.pc.setLocalDescription();
      if (this.pc.localDescription) {
        this.callbacks?.onSignal("answer", this.pc.localDescription.sdp);
      }
      // Answering rolls back any offer of ours that collided with this one.
      this.resumeQueuedNegotiation();
    });
  }

  public handleAnswer(sdp: string): Promise<void> {
    return this.enqueue(async () => {
      if (!this.pc) return;
      if (this.pc.signalingState !== "have-local-offer") {
        // An answer to an offer this client already rolled back. Applying it would
        // undo the exchange that replaced it.
        logger.debug("RTCService: Ignoring answer outside an open exchange", {
          state: this.pc.signalingState,
        });
        return;
      }
      await this.pc.setRemoteDescription({ type: "answer", sdp });
      await this.flushCandidates();
      this.resumeQueuedNegotiation();
    });
  }

  public handleIceCandidate(
    candidate: string,
    sdpMid?: string,
    sdpMLineIndex?: number,
  ): Promise<void> {
    return this.enqueue(async () => {
      if (!this.pc) return;
      const init: RTCIceCandidateInit = {
        candidate,
        sdpMid: sdpMid ?? undefined,
        sdpMLineIndex: sdpMLineIndex ?? undefined,
      };

      // Candidates routinely beat the description they belong to; holding them is
      // cheaper than losing them, which would leave the call on relay-or-nothing.
      if (!this.pc.remoteDescription) {
        this.pendingCandidates.push(init);
        return;
      }
      await this.addCandidate(init);
    });
  }

  private async flushCandidates(): Promise<void> {
    const queued = this.pendingCandidates;
    this.pendingCandidates = [];
    for (const candidate of queued) {
      await this.addCandidate(candidate);
    }
  }

  private async addCandidate(init: RTCIceCandidateInit): Promise<void> {
    try {
      await this.pc?.addIceCandidate(new RTCIceCandidate(init));
    } catch (err) {
      logger.warn("RTCService: Failed to add ICE candidate", { err });
    }
  }

  public close(): void {
    if (this.localAudioStream) {
      this.localAudioStream.getTracks().forEach((t) => t.stop());
      this.localAudioStream = null;
    }
    if (this.localScreenStream) {
      this.localScreenStream.getTracks().forEach((t) => t.stop());
      this.localScreenStream = null;
    }
    this.screenSender = null;
    this.pendingCandidates = [];
    this.negotiationQueued = false;
    this.callbacks = null;
    if (this.pc) {
      this.pc.onicecandidate = null;
      this.pc.ontrack = null;
      this.pc.onnegotiationneeded = null;
      this.pc.onconnectionstatechange = null;
      this.pc.oniceconnectionstatechange = null;
      this.pc.close();
      this.pc = null;
      logger.info("RTCService: Closed WebRTC connection");
    }
  }
}

export const rtcService = new RTCService();
