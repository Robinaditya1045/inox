import React, {
  useState,
  useEffect,
  useCallback,
  useRef,
  type ReactNode,
} from "react";
import { PlayerSyncContext } from "../contexts/playerSync.context";
import { useRoomSocket } from "../hooks/useRoomSocket";
import { usePermissions } from "../hooks/usePermissions";
import { useRoom } from "../hooks/useRoom";
import type { WSPlaybackPayload } from "../types/ws";
import { logger } from "../utils/logger";

interface PlayerSyncProviderProps {
  children: ReactNode;
}

const DEFAULT_STREAM_URL = "https://media.w3.org/2010/05/bunny/movie.mp4";

/**
 * Owns playback state for the active room. This MUST be a single shared instance:
 * when it was a plain hook, RoomPage and WatchPartyPlayer each held their own copy,
 * so changing media from the library updated the header but never the player —
 * the player only moved if the server happened to echo CHANGE_MEDIA back.
 */
export const PlayerSyncProvider: React.FC<PlayerSyncProviderProps> = ({
  children,
}) => {
  const { activeRoom } = useRoom();
  const roomId = activeRoom?.id;

  const [mediaUrl, setMediaUrlState] = useState<string>(
    activeRoom?.current_media_url || DEFAULT_STREAM_URL,
  );
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [currentTime, setCurrentTime] = useState<number>(0);
  const [lastSyncTimestamp, setLastSyncTimestamp] = useState<number>(
    Date.now(),
  );

  // Prevents echo loops when the video element is driven by a remote WebSocket event
  const isRemoteUpdateRef = useRef<boolean>(false);

  const { send, subscribe } = useRoomSocket();
  const permissions = usePermissions();

  // Reset playback whenever the room changes so room A's position never leaks into room B
  useEffect(() => {
    setMediaUrlState(activeRoom?.current_media_url || DEFAULT_STREAM_URL);
    setCurrentTime(0);
    setIsPlaying(false);
    setLastSyncTimestamp(Date.now());
  }, [roomId, activeRoom?.current_media_url]);

  const clearRemoteFlag = useCallback(() => {
    isRemoteUpdateRef.current = false;
  }, []);

  useEffect(() => {
    const unsubPlay = subscribe("PLAY", (msg) => {
      const payload = msg.payload as WSPlaybackPayload;
      if (payload && typeof payload.media_time_seconds === "number") {
        logger.debug("PlayerSync: Remote PLAY received", {
          time: payload.media_time_seconds,
          sender: msg.sender_name,
        });
        isRemoteUpdateRef.current = true;

        const latencySec = msg.timestamp
          ? Math.max(0, (Date.now() - msg.timestamp) / 1000)
          : 0;
        const targetTime =
          payload.media_time_seconds + (latencySec < 5 ? latencySec : 0);

        setCurrentTime(targetTime);
        setIsPlaying(true);
        setLastSyncTimestamp(Date.now());
      }
    });

    const unsubPause = subscribe("PAUSE", (msg) => {
      const payload = msg.payload as WSPlaybackPayload;
      if (payload && typeof payload.media_time_seconds === "number") {
        logger.debug("PlayerSync: Remote PAUSE received", {
          time: payload.media_time_seconds,
          sender: msg.sender_name,
        });
        isRemoteUpdateRef.current = true;
        setCurrentTime(payload.media_time_seconds);
        setIsPlaying(false);
        setLastSyncTimestamp(Date.now());
      }
    });

    const unsubSeek = subscribe("SEEK", (msg) => {
      const payload = msg.payload as WSPlaybackPayload;
      if (payload && typeof payload.media_time_seconds === "number") {
        logger.debug("PlayerSync: Remote SEEK received", {
          time: payload.media_time_seconds,
          sender: msg.sender_name,
        });
        isRemoteUpdateRef.current = true;
        setCurrentTime(payload.media_time_seconds);
        setLastSyncTimestamp(Date.now());
      }
    });

    const unsubChangeMedia = subscribe("CHANGE_MEDIA", (msg) => {
      const payload = msg.payload as WSPlaybackPayload;
      if (payload && payload.media_url) {
        logger.debug("PlayerSync: Remote CHANGE_MEDIA received", {
          url: payload.media_url,
          sender: msg.sender_name,
        });
        isRemoteUpdateRef.current = true;
        setMediaUrlState(payload.media_url);
        setCurrentTime(0);
        setIsPlaying(false);
        setLastSyncTimestamp(Date.now());
      }
    });

    const unsubSyncPlayback = subscribe("SYNC_PLAYBACK", (msg) => {
      const payload = msg.payload as WSPlaybackPayload;
      if (payload) {
        logger.info(
          "PlayerSync: Authoritative SYNC_PLAYBACK received on join",
          { ...payload },
        );
        isRemoteUpdateRef.current = true;
        if (payload.media_url) {
          setMediaUrlState(payload.media_url);
        }

        let targetTime = payload.media_time_seconds || 0;
        if (payload.is_playing && payload.last_updated) {
          const elapsedSec = Math.max(
            0,
            (Date.now() - payload.last_updated) / 1000,
          );
          targetTime += elapsedSec;
        }

        setCurrentTime(targetTime);
        setIsPlaying(!!payload.is_playing);
        setLastSyncTimestamp(Date.now());
      }
    });

    return () => {
      unsubPlay();
      unsubPause();
      unsubSeek();
      unsubChangeMedia();
      unsubSyncPlayback();
    };
  }, [subscribe]);

  const setMediaUrl = useCallback(
    (url: string) => {
      if (!permissions.can_control_playback) {
        logger.warn("PlayerSync: Permission denied to change media stream URL");
        return;
      }
      // Apply locally first — playback must not depend on the server echoing the event back.
      setMediaUrlState(url);
      setCurrentTime(0);
      setIsPlaying(false);
      setLastSyncTimestamp(Date.now());
      send<WSPlaybackPayload>("CHANGE_MEDIA", {
        media_url: url,
        media_time_seconds: 0,
        is_playing: false,
      });
      logger.info("PlayerSync: Media stream URL changed and broadcasted", {
        url,
      });
    },
    [permissions.can_control_playback, send],
  );

  const play = useCallback(
    (time: number) => {
      if (!permissions.can_control_playback) {
        logger.warn("PlayerSync: Permission denied to emit PLAY");
        return;
      }
      setIsPlaying(true);
      setCurrentTime(time);
      send<WSPlaybackPayload>("PLAY", { media_time_seconds: time });
      logger.debug("PlayerSync: Emitted PLAY", { time });
    },
    [permissions.can_control_playback, send],
  );

  const pause = useCallback(
    (time: number) => {
      if (!permissions.can_control_playback) {
        logger.warn("PlayerSync: Permission denied to emit PAUSE");
        return;
      }
      setIsPlaying(false);
      setCurrentTime(time);
      send<WSPlaybackPayload>("PAUSE", { media_time_seconds: time });
      logger.debug("PlayerSync: Emitted PAUSE", { time });
    },
    [permissions.can_control_playback, send],
  );

  const seek = useCallback(
    (time: number) => {
      if (!permissions.can_control_playback) {
        logger.warn("PlayerSync: Permission denied to emit SEEK");
        return;
      }
      setCurrentTime(time);
      send<WSPlaybackPayload>("SEEK", { media_time_seconds: time });
      logger.debug("PlayerSync: Emitted SEEK", { time });
    },
    [permissions.can_control_playback, send],
  );

  const notifyLocalProgress = useCallback((time: number) => {
    if (!isRemoteUpdateRef.current) {
      setCurrentTime(time);
    }
  }, []);

  const value = {
    mediaUrl,
    isPlaying,
    currentTime,
    lastSyncTimestamp,
    setMediaUrl,
    play,
    pause,
    seek,
    notifyLocalProgress,
    clearRemoteFlag,
  };

  return (
    <PlayerSyncContext.Provider value={value}>
      {children}
    </PlayerSyncContext.Provider>
  );
};
