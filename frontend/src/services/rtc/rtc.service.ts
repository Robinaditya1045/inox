import { logger } from '../../utils/logger';

export type TrackHandler = (track: MediaStreamTrack, stream: MediaStream, peerId: string) => void;
export type IceCandidateHandler = (candidate: RTCIceCandidate) => void;

class RTCService {
  private pc: RTCPeerConnection | null = null;
  private localAudioStream: MediaStream | null = null;
  private localScreenStream: MediaStream | null = null;
  private onTrackCallback: TrackHandler | null = null;
  private onIceCandidateCallback: IceCandidateHandler | null = null;

  public setCallbacks(onTrack: TrackHandler, onIceCandidate: IceCandidateHandler): void {
    this.onTrackCallback = onTrack;
    this.onIceCandidateCallback = onIceCandidate;
  }

  public async initialize(): Promise<RTCPeerConnection> {
    if (this.pc) {
      this.close();
    }

    const config: RTCConfiguration = {
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' },
      ],
    };

    this.pc = new RTCPeerConnection(config);

    this.pc.onicecandidate = (event) => {
      if (event.candidate && this.onIceCandidateCallback) {
        this.onIceCandidateCallback(event.candidate);
      }
    };

    this.pc.ontrack = (event) => {
      if (event.streams && event.streams[0] && this.onTrackCallback) {
        const stream = event.streams[0];
        const peerId = stream.id || `remote-${Date.now()}`;
        logger.info('RTCService: Received remote track', { kind: event.track.kind, peerId });
        this.onTrackCallback(event.track, stream, peerId);
      }
    };

    this.pc.onconnectionstatechange = () => {
      logger.info('RTCService: Connection state changed', { state: this.pc?.connectionState });
    };

    return this.pc;
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

      logger.info('RTCService: Acquired local audio stream');
      return this.localAudioStream;
    } catch (err) {
      logger.error('RTCService: Failed to acquire local audio', { err });
      throw err;
    }
  }

  public async startScreenShare(): Promise<MediaStream> {
    try {
      this.localScreenStream = await navigator.mediaDevices.getDisplayMedia({
        video: {
          frameRate: { ideal: 30, max: 60 },
          width: { ideal: 1920, max: 3840 },
          height: { ideal: 1080, max: 2160 },
        },
        audio: false,
      });

      if (this.pc && this.localScreenStream) {
        this.localScreenStream.getVideoTracks().forEach((track) => {
          this.pc?.addTrack(track, this.localScreenStream!);
        });
      }

      logger.info('RTCService: Started screen sharing');
      return this.localScreenStream;
    } catch (err) {
      logger.error('RTCService: Failed to get display media', { err });
      throw err;
    }
  }

  public stopScreenShare(): void {
    if (this.localScreenStream) {
      this.localScreenStream.getTracks().forEach((track) => {
        track.stop();
        if (this.pc) {
          const sender = this.pc.getSenders().find((s) => s.track === track);
          if (sender) {
            this.pc.removeTrack(sender);
          }
        }
      });
      this.localScreenStream = null;
      logger.info('RTCService: Stopped screen sharing');
    }
  }

  public setAudioMuted(muted: boolean): void {
    if (this.localAudioStream) {
      this.localAudioStream.getAudioTracks().forEach((track) => {
        track.enabled = !muted;
      });
    }
  }

  public async createOffer(): Promise<RTCSessionDescriptionInit> {
    if (!this.pc) throw new Error('RTC connection not initialized');
    const offer = await this.pc.createOffer({
      offerToReceiveAudio: true,
      offerToReceiveVideo: true,
    });
    await this.pc.setLocalDescription(offer);
    return offer;
  }

  public async handleAnswer(sdp: string): Promise<void> {
    if (!this.pc) return;
    if (this.pc.signalingState !== 'have-local-offer') {
      logger.warn('RTCService: Ignoring SFU_ANSWER because signalingState is not have-local-offer', { state: this.pc.signalingState });
      return;
    }
    await this.pc.setRemoteDescription(new RTCSessionDescription({ type: 'answer', sdp }));
  }

  public async handleIceCandidate(candidate: string, sdpMid?: string, sdpMLineIndex?: number): Promise<void> {
    if (!this.pc) return;
    try {
      await this.pc.addIceCandidate(new RTCIceCandidate({
        candidate,
        sdpMid: sdpMid || undefined,
        sdpMLineIndex: sdpMLineIndex ?? undefined,
      }));
    } catch (err) {
      logger.error('RTCService: Failed to add ICE candidate', { err });
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
    if (this.pc) {
      this.pc.close();
      this.pc = null;
    }
    logger.info('RTCService: Closed WebRTC connection');
  }
}

export const rtcService = new RTCService();
