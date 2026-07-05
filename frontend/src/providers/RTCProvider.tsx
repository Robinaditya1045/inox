import React, { useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import { RTCContext } from '../contexts/rtc.context';
import { useWS } from '../hooks/useWS';
import { usePermissions } from '../hooks/usePermissions';
import { useRoom } from '../hooks/useRoom';
import { rtcService } from '../services/rtc/rtc.service';
import type { WSSfuAnswerPayload, WSSfuIcePayload, WSSfuOfferPayload } from '../types/ws';
import type { RTCConnectionState } from '../types/rtc';
import { logger } from '../utils/logger';

interface RTCProviderProps {
  children: ReactNode;
}

export const RTCProvider: React.FC<RTCProviderProps> = ({ children }) => {
  const [connectionState, setConnectionState] = useState<RTCConnectionState>('disconnected');
  const [isAudioMuted, setIsAudioMuted] = useState<boolean>(false);
  const [isDeafened, setIsDeafened] = useState<boolean>(false);
  const [isScreenSharing, setIsScreenSharing] = useState<boolean>(false);
  const [remoteStreams, setRemoteStreams] = useState<Map<string, MediaStream>>(new Map());
  const [localScreenStream, setLocalScreenStream] = useState<MediaStream | null>(null);

  const { send, subscribe } = useWS();
  const permissions = usePermissions();
  const { activeRoom } = useRoom();
  const roomId = activeRoom?.id;
  const isConnectedRef = useRef(false);

  // Clean up WebRTC on room switch or unmount
  useEffect(() => {
    return () => {
      if (isConnectedRef.current) {
        rtcService.close();
        isConnectedRef.current = false;
        setConnectionState('disconnected');
        setRemoteStreams(new Map());
        setLocalScreenStream(null);
      }
    };
  }, [roomId]);

  // Subscribe to SFU_ANSWER and SFU_ICE_CANDIDATE from Hub
  useEffect(() => {
    if (!roomId) return;

    const unsubAnswer = subscribe('SFU_ANSWER', async (msg) => {
      const payload = msg.payload as WSSfuAnswerPayload;
      if (payload && payload.sdp) {
        logger.debug('RTCProvider: Received SFU_ANSWER');
        await rtcService.handleAnswer(payload.sdp);
        setConnectionState('connected');
      }
    });

    const unsubIce = subscribe('SFU_ICE_CANDIDATE', async (msg) => {
      const payload = msg.payload as WSSfuIcePayload;
      if (payload && payload.candidate) {
        await rtcService.handleIceCandidate(payload.candidate, payload.sdpMid, payload.sdpMLineIndex);
      }
    });

    return () => {
      unsubAnswer();
      unsubIce();
    };
  }, [roomId, subscribe]);

  const connectAudio = useCallback(async () => {
    if (!roomId || !permissions.can_stream_audio) {
      logger.warn('RTCProvider: Cannot connect audio - no room or permission denied');
      return;
    }

    try {
      setConnectionState('connecting');

      // Wire callbacks
      rtcService.setCallbacks(
        (_track, stream, peerId) => {
          setRemoteStreams((prev) => {
            const next = new Map(prev);
            next.set(peerId, stream);
            return next;
          });
        },
        (candidate) => {
          const icePayload: WSSfuIcePayload = {
            candidate: candidate.candidate,
            sdpMid: candidate.sdpMid || undefined,
            sdpMLineIndex: candidate.sdpMLineIndex ?? undefined,
          };
          send('SFU_ICE_CANDIDATE', icePayload);
        }
      );

      await rtcService.initialize();
      await rtcService.startLocalAudio();

      const offer = await rtcService.createOffer();
      const offerPayload: WSSfuOfferPayload = {
        sdp: offer.sdp || '',
        type: 'offer',
      };

      send('SFU_OFFER', offerPayload);
      isConnectedRef.current = true;
      logger.info('RTCProvider: Sent SFU_OFFER to backend');
    } catch (err) {
      setConnectionState('failed');
      logger.error('RTCProvider: Failed to connect audio', { err });
    }
  }, [roomId, permissions.can_stream_audio, send]);

  const disconnectAudio = useCallback(() => {
    rtcService.close();
    setConnectionState('disconnected');
    setIsAudioMuted(false);
    setIsScreenSharing(false);
    setRemoteStreams(new Map());
    setLocalScreenStream(null);
    isConnectedRef.current = false;
    logger.info('RTCProvider: Disconnected voice channel');
  }, []);

  const toggleMute = useCallback(() => {
    if (connectionState !== 'connected') return;
    const nextMuted = !isAudioMuted;
    rtcService.setAudioMuted(nextMuted);
    setIsAudioMuted(nextMuted);
  }, [connectionState, isAudioMuted]);

  const toggleDeafen = useCallback(() => {
    if (connectionState !== 'connected') return;
    setIsDeafened(!isDeafened);
  }, [connectionState, isDeafened]);

  const toggleScreenShare = useCallback(async () => {
    if (!permissions.can_share_screen) {
      logger.warn('RTCProvider: Permission denied for screen sharing');
      return;
    }

    if (isScreenSharing) {
      rtcService.stopScreenShare();
      setIsScreenSharing(false);
      setLocalScreenStream(null);
      // Renegotiate SDP offer without screen track
      if (connectionState === 'connected') {
        const offer = await rtcService.createOffer();
        send('SFU_OFFER', { sdp: offer.sdp || '', type: 'offer' });
      }
    } else {
      try {
        const stream = await rtcService.startScreenShare();
        setLocalScreenStream(stream);
        setIsScreenSharing(true);

        // Listen for user ending screen share via browser UI
        stream.getVideoTracks()[0].onended = () => {
          setIsScreenSharing(false);
          setLocalScreenStream(null);
          rtcService.stopScreenShare();
        };

        // Renegotiate SDP offer with new screen track
        if (connectionState === 'connected') {
          const offer = await rtcService.createOffer();
          send('SFU_OFFER', { sdp: offer.sdp || '', type: 'offer' });
        }
      } catch (err) {
        logger.error('RTCProvider: Screen share failed or cancelled', { err });
      }
    }
  }, [permissions.can_share_screen, isScreenSharing, connectionState, send]);

  const value = {
    connectionState,
    isAudioMuted,
    isDeafened,
    isScreenSharing,
    remoteStreams,
    localScreenStream,
    connectAudio,
    disconnectAudio,
    toggleMute,
    toggleDeafen,
    toggleScreenShare,
  };

  return <RTCContext.Provider value={value}>{children}</RTCContext.Provider>;
};
