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
import { useToast } from "../hooks/useToast";
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
  WSVoiceInviteReplyPayload,
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

/** Remote media held per publisher, so a microphone, camera and screen share
 *  from the same person land on the same tile. */
type PeerMedia = Partial<Record<RemoteTrackKind, MediaStream>>;

/** How long a voice invite stays on the screen of the person invited. */
const VOICE_INVITE_TTL_MS = 25_000;
/**
 * How long the inviter's button stays "Invited". Longer than the invite lives,
 * so an answer always finds it, and than the server's 15s cooldown between
 * invites to the same person, so the button never offers one it would drop.
 */
const VOICE_INVITE_PENDING_MS = 30_000;

/** Why getUserMedia refused the camera, in words a person can act on. */
function describeCameraError(err: unknown): string {
  const name = err instanceof DOMException ? err.name : "";
  switch (name) {
    case "NotAllowedError":
      return "Camera access is blocked. Allow it in your browser's site settings and try again.";
    case "NotFoundError":
    case "OverconstrainedError":
      return "No camera was found on this device.";
    case "NotReadableError":
      return "Your camera is in use by another app.";
    default:
      return "Something went wrong starting your camera.";
  }
}

export const RTCProvider: React.FC<RTCProviderProps> = ({ children }) => {
  const [connectionState, setConnectionState] =
    useState<RTCConnectionState>("disconnected");
  const [isAudioMuted, setIsAudioMuted] = useState<boolean>(false);
  const [isDeafened, setIsDeafened] = useState<boolean>(false);
  const [isCameraOn, setIsCameraOn] = useState<boolean>(false);
  const [localCameraStream, setLocalCameraStream] =
    useState<MediaStream | null>(null);
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
  const [invitedToVoice, setInvitedToVoice] = useState<ReadonlySet<string>>(
    () => new Set(),
  );

  const { send, subscribe } = useRoomSocket();
  const permissions = usePermissions();
  const { activeRoom } = useRoom();
  const { user } = useAuth();
  const { toast, dismiss } = useToast();
  const roomId = activeRoom?.id;
  const isConnectedRef = useRef(false);
  // Set while the browser is asking for the camera, so a second click does not
  // publish a second one.
  const isCameraStartingRef = useRef(false);
  // Invites this client sent, by invitee, until VOICE_INVITE_PENDING_MS is up.
  // Only these people's replies are shown, and each only once.
  const sentInvitesRef = useRef(
    new Map<
      string,
      { timer: ReturnType<typeof setTimeout>; answered: boolean }
    >(),
  );
  // The invite toast on screen from each inviter, so a second invite replaces
  // the first and joining voice clears them all.
  const inviteToastsRef = useRef(new Map<string, number>());

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
    setIsCameraOn(false);
    setIsScreenSharing(false);
    setRemoteMedia(new Map());
    setLocalCameraStream(null);
    setLocalScreenStream(null);
    setLocalAudioStream(null);
  }, []);

  const dismissInviteToasts = useCallback(() => {
    inviteToastsRef.current.forEach((id) => dismiss(id));
    inviteToastsRef.current.clear();
  }, [dismiss]);

  // Leaving the room leaves the call. Without the explicit hang-up the SFU keeps
  // the peer until its websocket drops, and everyone else keeps a dead tile.
  // Invites belong to the room too, in both directions.
  useEffect(() => {
    const sentInvites = sentInvitesRef.current;
    return () => {
      if (isConnectedRef.current) {
        teardown(true);
      }
      setRoster([]);
      sentInvites.forEach(({ timer }) => clearTimeout(timer));
      sentInvites.clear();
      setInvitedToVoice(new Set());
      dismissInviteToasts();
    };
  }, [roomId, teardown, dismissInviteToasts]);

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
        onSignal: (type, sdp, streams) => {
          sendRef.current(type === "offer" ? "SFU_OFFER" : "SFU_ANSWER", {
            sdp,
            type,
            streams,
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
      // A camera or share started while the microphone prompt was still open
      // went down with the connection.
      setIsCameraOn(false);
      setLocalCameraStream(null);
      setIsScreenSharing(false);
      setLocalScreenStream(null);
      logger.error("RTCProvider: Failed to connect audio", { err });
    }
  }, [roomId, permissions.can_stream_audio, teardown]);

  const disconnectAudio = useCallback(() => {
    teardown(true);
    logger.info("RTCProvider: Disconnected voice channel");
  }, [teardown]);

  // Read by invite toasts, which outlive the render that showed them.
  const connectAudioRef = useRef(connectAudio);
  const canStreamAudioRef = useRef(permissions.can_stream_audio);
  useEffect(() => {
    connectAudioRef.current = connectAudio;
    canStreamAudioRef.current = permissions.can_stream_audio;
  });

  const inviteToVoice = useCallback((userId: string) => {
    // The server only relays invites from someone already in the call.
    if (connectionStateRef.current !== "connected") return;
    if (sentInvitesRef.current.has(userId)) return;

    sendRef.current("VOICE_INVITE", undefined, userId);
    const timer = setTimeout(() => {
      sentInvitesRef.current.delete(userId);
      setInvitedToVoice((prev) => {
        const next = new Set(prev);
        next.delete(userId);
        return next;
      });
    }, VOICE_INVITE_PENDING_MS);
    sentInvitesRef.current.set(userId, { timer, answered: false });
    setInvitedToVoice((prev) => new Set(prev).add(userId));
  }, []);

  // Invites to voice, both ways. Each is addressed to one person by the
  // server, so anything arriving here is meant for this user.
  useEffect(() => {
    if (!roomId) return;

    const unsubInvite = subscribe("VOICE_INVITE", (msg) => {
      const inviterId = msg.sender_id;
      if (!inviterId || (msg.room_id && msg.room_id !== roomId)) return;
      // Nothing to accept for someone already in the call, or not allowed in.
      if (isConnectedRef.current || !canStreamAudioRef.current) return;

      const reply = (accepted: boolean) => {
        const payload: WSVoiceInviteReplyPayload = { accepted };
        sendRef.current("VOICE_INVITE_REPLY", payload, inviterId);
      };
      const previous = inviteToastsRef.current.get(inviterId);
      if (previous !== undefined) dismiss(previous);

      const id = toast({
        title: `${msg.sender_name || "Someone"} invited you to voice`,
        description: "Join them in the voice channel?",
        duration: VOICE_INVITE_TTL_MS,
        actions: [
          {
            label: "Accept",
            variant: "primary",
            onClick: () => {
              inviteToastsRef.current.delete(inviterId);
              // The click is the gesture the microphone prompt and audio
              // playback need, so the call starts from here and not later.
              void connectAudioRef.current().then(() => {
                // A refused microphone leaves the call unjoined; the voice
                // panel says why, and the inviter hears nothing either way.
                if (isConnectedRef.current) reply(true);
              });
            },
          },
          {
            label: "Reject",
            onClick: () => {
              inviteToastsRef.current.delete(inviterId);
              reply(false);
            },
          },
        ],
      });
      inviteToastsRef.current.set(inviterId, id);
    });

    const unsubReply = subscribe("VOICE_INVITE_REPLY", (msg) => {
      const inviteeId = msg.sender_id;
      const sent = inviteeId && sentInvitesRef.current.get(inviteeId);
      // Another tab's invite, or an answer already shown.
      if (!sent || sent.answered) return;
      sent.answered = true;

      const name = msg.sender_name || "They";
      const payload = msg.payload as WSVoiceInviteReplyPayload | undefined;
      toast(
        payload?.accepted
          ? { tone: "success", title: `${name} is joining voice` }
          : { title: `${name} declined your voice invite` },
      );
    });

    return () => {
      unsubInvite();
      unsubReply();
    };
  }, [roomId, subscribe, toast, dismiss]);

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

  const toggleCamera = useCallback(async () => {
    if (!permissions.can_stream_video) {
      logger.warn("RTCProvider: Permission denied for camera");
      return;
    }
    if (!isConnectedRef.current || isCameraStartingRef.current) return;

    if (isCameraOn) {
      rtcService.stopCamera();
      setIsCameraOn(false);
      setLocalCameraStream(null);
      return;
    }

    isCameraStartingRef.current = true;
    try {
      const stream = await rtcService.startCamera();
      if (!stream) return;
      setLocalCameraStream(stream);
      setIsCameraOn(true);

      // A camera can go without the button being pressed: unplugged, or its
      // permission revoked mid-call. Either has to unwind the same state.
      stream.getVideoTracks()[0].addEventListener("ended", () => {
        rtcService.stopCamera();
        setIsCameraOn(false);
        setLocalCameraStream(null);
      });
    } catch (err) {
      logger.error("RTCProvider: Failed to start camera", { err });
      toast({
        title: "Couldn't turn on your camera",
        description: describeCameraError(err),
        tone: "danger",
      });
    } finally {
      isCameraStartingRef.current = false;
    }
  }, [permissions.can_stream_video, isCameraOn, toast]);

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

  // Once in voice -- by accepting one invite, the voice button, or from another
  // tab, which the roster shows -- the remaining invites have nothing to offer.
  const isSelfInRoster = roster.some((peer) => peer.user_id === user?.id);
  useEffect(() => {
    if (inCall || isSelfInRoster) dismissInviteToasts();
  }, [inCall, isSelfInRoster, dismissInviteToasts]);

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
        isCameraOn: isLocal ? isCameraOn : entry.is_camera_on,
        isScreenSharing: isLocal ? isScreenSharing : entry.is_screen_sharing,
        audioStream: isLocal ? (localAudioStream ?? undefined) : media?.mic,
        cameraStream: isLocal
          ? (localCameraStream ?? undefined)
          : media?.camera,
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
        isCameraOn,
        isScreenSharing,
        audioStream: localAudioStream ?? undefined,
        cameraStream: localCameraStream ?? undefined,
        screenStream: localScreenStream ?? undefined,
      });
    }

    return list;
  }, [
    roster,
    remoteMedia,
    user,
    isAudioMuted,
    isCameraOn,
    isScreenSharing,
    localAudioStream,
    localCameraStream,
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
    isCameraOn,
    isScreenSharing,
    peers,
    speakingIds,
    localScreenStream,
    invitedToVoice,
    inviteToVoice,
    connectAudio,
    disconnectAudio,
    toggleMute,
    toggleDeafen,
    toggleCamera,
    toggleScreenShare,
  };

  return <RTCContext.Provider value={value}>{children}</RTCContext.Provider>;
};
