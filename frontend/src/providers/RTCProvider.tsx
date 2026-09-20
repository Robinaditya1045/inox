import React, {
  useState,
  useEffect,
  useCallback,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import { RTCContext } from "../contexts/rtc.context";
import { useRoomSocket } from "../hooks/useRoomSocket";
import { usePermissions } from "../hooks/usePermissions";
import { useRoom } from "../hooks/useRoom";
import { useAuth } from "../hooks/useAuth";
import {
  useSpeakingPeers,
  type SpeakingSource,
} from "../hooks/useSpeakingPeers";
import { rtcService } from "../services/rtc/rtc.service";
import type {
  WSSfuAnswerPayload,
  WSSfuIcePayload,
  WSSfuOfferPayload,
  WSSfuPeer,
  WSSfuPeersPayload,
  WSSfuStatePayload,
} from "../types/ws";
import type {
  RemoteTrackKind,
  RTCConnectionState,
  VoicePeer,
} from "../types/rtc";
import { logger } from "../utils/logger";

interface RTCProviderProps {
  children: ReactNode;
}

/** Remote media held per publisher, so a screen share and a microphone from the
 *  same person land on the same tile. */
type PeerMedia = Partial<Record<RemoteTrackKind, MediaStream>>;

export const RTCProvider: React.FC<RTCProviderProps> = ({ children }) => {
  const [connectionState, setConnectionState] =
    useState<RTCConnectionState>("disconnected");
  const [isAudioMuted, setIsAudioMuted] = useState<boolean>(false);
  const [isDeafened, setIsDeafened] = useState<boolean>(false);
  const [isScreenSharing, setIsScreenSharing] = useState<boolean>(false);
  const [localScreenStream, setLocalScreenStream] =
    useState<MediaStream | null>(null);
  const [localAudioStream, setLocalAudioStream] = useState<MediaStream | null>(
    null,
  );
  const [roster, setRoster] = useState<WSSfuPeer[]>([]);
  const [remoteMedia, setRemoteMedia] = useState<Map<string, PeerMedia>>(
    () => new Map(),
  );

  const { send, subscribe } = useRoomSocket();
  const permissions = usePermissions();
  const { activeRoom } = useRoom();
  const { user } = useAuth();
  const roomId = activeRoom?.id;
  const isConnectedRef = useRef(false);

  // The socket is shared, so a stale send() closure would post signaling into the
  // wrong room after a switch. Keeping it in a ref lets the peer-connection
  // callbacks stay stable without capturing one.
  const sendRef = useRef(send);
  // Read by connectAudio, which must not capture a state value that is a render
  // behind when deciding whether there is a live call to keep.
  const connectionStateRef = useRef<RTCConnectionState>("disconnected");
  useEffect(() => {
    sendRef.current = send;
    connectionStateRef.current = connectionState;
  });

  const teardown = useCallback((notifyServer: boolean) => {
    if (notifyServer && isConnectedRef.current) {
      sendRef.current("SFU_LEAVE", {});
    }
    rtcService.close();
    isConnectedRef.current = false;
    setConnectionState("disconnected");
    setIsAudioMuted(false);
    setIsScreenSharing(false);
    setRemoteMedia(new Map());
    setLocalScreenStream(null);
    setLocalAudioStream(null);
  }, []);

  // Leaving the room leaves the call. Without the explicit hang-up the SFU keeps
  // the peer until its websocket drops, and everyone else keeps a dead tile.
  useEffect(() => {
    return () => {
      if (isConnectedRef.current) {
        teardown(true);
      }
      setRoster([]);
    };
  }, [roomId, teardown]);

  // Signaling from the SFU.
  useEffect(() => {
    if (!roomId) return;

    const unsubAnswer = subscribe("SFU_ANSWER", async (msg) => {
      const payload = msg.payload as WSSfuAnswerPayload;
      if (payload?.sdp) {
        await rtcService.handleAnswer(payload.sdp);
      }
    });

    // The SFU offers whenever the track set changes on its side: someone joined,
    // left, or started sharing. Answering these is what makes other people
    // audible after the call is already up.
    const unsubOffer = subscribe("SFU_OFFER", async (msg) => {
      const payload = msg.payload as WSSfuOfferPayload;
      if (payload?.sdp) {
        logger.debug("RTCProvider: Received SFU renegotiation offer");
        await rtcService.handleOffer(payload.sdp);
      }
    });

    const unsubIce = subscribe("SFU_ICE_CANDIDATE", async (msg) => {
      const payload = msg.payload as WSSfuIcePayload;
      if (payload?.candidate) {
        await rtcService.handleIceCandidate(
          payload.candidate,
          payload.sdpMid,
          payload.sdpMLineIndex,
        );
      }
    });

    const unsubPeers = subscribe("SFU_PEERS", (msg) => {
      const payload = msg.payload as WSSfuPeersPayload;
      const next = payload?.peers ?? [];
      setRoster(next);

      // Drop media belonging to people who have left, so that someone who
      // rejoins is not shown the frozen last frame of their previous share.
      const present = new Set(next.map((peer) => peer.user_id));
      setRemoteMedia((prev) => {
        const stale = [...prev.keys()].filter((id) => !present.has(id));
        if (stale.length === 0) return prev;
        const pruned = new Map(prev);
        stale.forEach((id) => pruned.delete(id));
        return pruned;
      });
    });

    return () => {
      unsubAnswer();
      unsubOffer();
      unsubIce();
      unsubPeers();
    };
  }, [roomId, subscribe]);

  const connectAudio = useCallback(async () => {
    if (!roomId || !permissions.can_stream_audio) {
      logger.warn(
        "RTCProvider: Cannot connect audio - no room or permission denied",
      );
      return;
    }
    if (isConnectedRef.current) {
      // Already in the call, unless the connection died: reconnecting then means
      // replacing the dead peer connection rather than doing nothing.
      if (connectionStateRef.current !== "failed") return;
      teardown(true);
    }

    try {
      setConnectionState("connecting");
      isConnectedRef.current = true;

      await rtcService.initialize({
        onSignal: (type, sdp) => {
          sendRef.current(type === "offer" ? "SFU_OFFER" : "SFU_ANSWER", {
            sdp,
            type,
          });
        },
        onIceCandidate: (candidate) => {
          const icePayload: WSSfuIcePayload = {
            candidate: candidate.candidate,
            sdpMid: candidate.sdpMid || undefined,
            sdpMLineIndex: candidate.sdpMLineIndex ?? undefined,
          };
          sendRef.current("SFU_ICE_CANDIDATE", icePayload);
        },
        onRemoteTrack: ({ userId, kind, stream }) => {
          setRemoteMedia((prev) => {
            const next = new Map(prev);
            next.set(userId, { ...next.get(userId), [kind]: stream });
            return next;
          });
        },
        onRemoteTrackEnded: (userId, kind) => {
          setRemoteMedia((prev) => {
            const current = prev.get(userId);
            if (!current?.[kind]) return prev;
            const next = new Map(prev);
            const remaining: PeerMedia = { ...current };
            delete remaining[kind];
            if (Object.keys(remaining).length === 0) {
              next.delete(userId);
            } else {
              next.set(userId, remaining);
            }
            return next;
          });
        },
        onConnectionStateChange: (state) => {
          switch (state) {
            case "connected":
              setConnectionState("connected");
              break;
            case "failed":
              setConnectionState("failed");
              break;
            case "closed":
              setConnectionState((prev) =>
                prev === "failed" ? prev : "disconnected",
              );
              break;
            default:
              // "disconnected" is usually a blip the ICE restart recovers from, so
              // it reads as reconnecting rather than as a dropped call.
              setConnectionState((prev) =>
                prev === "connected" && state === "disconnected"
                  ? "connecting"
                  : prev === "failed"
                    ? prev
                    : "connecting",
              );
          }
        },
      });

      const micStream = await rtcService.startLocalAudio();
      setLocalAudioStream(micStream);
      // Adding the microphone fires negotiationneeded, which sends the opening
      // offer through onSignal above.
      logger.info("RTCProvider: Joining voice channel");
    } catch (err) {
      // A refused microphone or a failed handshake must not leave a half-built
      // connection behind: the next attempt has to start from nothing.
      rtcService.close();
      isConnectedRef.current = false;
      setConnectionState("failed");
      logger.error("RTCProvider: Failed to connect audio", { err });
    }
  }, [roomId, permissions.can_stream_audio, teardown]);

  const disconnectAudio = useCallback(() => {
    teardown(true);
    logger.info("RTCProvider: Disconnected voice channel");
  }, [teardown]);

  const toggleMute = useCallback(() => {
    if (!isConnectedRef.current) return;
    const nextMuted = !isAudioMuted;
    rtcService.setAudioMuted(nextMuted);
    setIsAudioMuted(nextMuted);
    // A muted microphone keeps sending silence, so the server has to be told
    // before anyone else's roster can show it.
    const payload: WSSfuStatePayload = { is_muted: nextMuted };
    sendRef.current("SFU_STATE", payload);
  }, [isAudioMuted]);

  const toggleDeafen = useCallback(() => {
    if (!isConnectedRef.current) return;
    setIsDeafened((prev) => !prev);
  }, []);

  const toggleScreenShare = useCallback(async () => {
    if (!permissions.can_share_screen) {
      logger.warn("RTCProvider: Permission denied for screen sharing");
      return;
    }
    if (!isConnectedRef.current) return;

    if (isScreenSharing) {
      rtcService.stopScreenShare();
      setIsScreenSharing(false);
      setLocalScreenStream(null);
      return;
    }

    try {
      const stream = await rtcService.startScreenShare();
      setLocalScreenStream(stream);
      setIsScreenSharing(true);

      // Ending the share from the browser's own "Stop sharing" bar has to unwind
      // the same state the button does.
      stream.getVideoTracks()[0].addEventListener("ended", () => {
        rtcService.stopScreenShare();
        setIsScreenSharing(false);
        setLocalScreenStream(null);
      });
    } catch (err) {
      logger.error("RTCProvider: Screen share failed or cancelled", { err });
    }
  }, [permissions.can_share_screen, isScreenSharing]);

  const inCall =
    connectionState === "connected" || connectionState === "connecting";

  const peers = useMemo<VoicePeer[]>(() => {
    const list = roster.map<VoicePeer>((entry) => {
      const isLocal = entry.user_id === user?.id;
      const media = remoteMedia.get(entry.user_id);
      return {
        userId: entry.user_id,
        username: entry.username,
        isLocal,
        // Your own controls answer immediately; the server's echo of them is a
        // round trip behind, and a mic button that lags is a mic button nobody
        // trusts.
        isMuted: isLocal ? isAudioMuted : entry.is_muted,
        isScreenSharing: isLocal ? isScreenSharing : entry.is_screen_sharing,
        audioStream: isLocal ? (localAudioStream ?? undefined) : media?.mic,
        screenStream: isLocal
          ? (localScreenStream ?? undefined)
          : media?.screen,
      };
    });

    // Until the roster lands, you are still in your own call.
    if (inCall && user && !list.some((peer) => peer.isLocal)) {
      list.unshift({
        userId: user.id,
        username: user.username,
        isLocal: true,
        isMuted: isAudioMuted,
        isScreenSharing,
        audioStream: localAudioStream ?? undefined,
        screenStream: localScreenStream ?? undefined,
      });
    }

    return list;
  }, [
    roster,
    remoteMedia,
    user,
    isAudioMuted,
    isScreenSharing,
    localAudioStream,
    localScreenStream,
    inCall,
  ]);

  const speakingSources = useMemo<SpeakingSource[]>(
    () =>
      peers.flatMap((peer) =>
        peer.audioStream ? [{ id: peer.userId, stream: peer.audioStream }] : [],
      ),
    [peers],
  );
  const speakingIds = useSpeakingPeers(speakingSources);

  const value = {
    connectionState,
    isAudioMuted,
    isDeafened,
    isScreenSharing,
    peers,
    speakingIds,
    localScreenStream,
    connectAudio,
    disconnectAudio,
    toggleMute,
    toggleDeafen,
    toggleScreenShare,
  };

  return <RTCContext.Provider value={value}>{children}</RTCContext.Provider>;
};
