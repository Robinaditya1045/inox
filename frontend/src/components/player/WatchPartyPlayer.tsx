import React, { useRef, useEffect, useState, useCallback } from "react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import Hls, { type LoaderConfig } from "hls.js";
import { usePlayerSync } from "../../hooks/usePlayerSync";
import { usePermissions } from "../../hooks/usePermissions";
import { useRoomSocket } from "../../hooks/useRoomSocket";
import { normalizeMediaUrl } from "../../utils/mediaUrl";
import {
  DRIFT_CHECK_INTERVAL_MS,
  LEADER_REPORT_INTERVAL_MS,
  clampToSeekable,
  driftAction,
  isLiveMediaUrl,
  liveEdgeTime,
  roomLocalTime,
  secondsBehindTarget,
  toStreamPosition,
} from "../../utils/liveSync";
import { logger } from "../../utils/logger";
import { PlayerScrubber } from "./PlayerScrubber";
import { Spinner } from "../common/Spinner";
import menuStyles from "../common/Menu.module.css";
import {
  Play,
  Pause,
  Volume2,
  VolumeX,
  Maximize2,
  Minimize2,
  Lock,
  Wifi,
  WifiOff,
  Layers,
  ChevronUp,
  AlertCircle,
  Radio,
  Check,
  Film,
} from "lucide-react";
import styles from "./WatchPartyPlayer.module.css";

interface WatchPartyPlayerProps {
  onOpenLibrary?: () => void;
}

// Backoff for restarting a stream once hls.js has given up on it. A fatal network
// error means its own retries are already spent; live upstreams drop out routinely,
// so the player keeps trying for as long as the viewer stays.
const RETRY_BASE_MS = 2_000;
const RETRY_MAX_MS = 30_000;

// A live stream is resumed in place when it fails, which keeps the picture. Only if
// that keeps failing is the master playlist reloaded: that rebuilds the media
// pipeline -- the screen goes black -- and the one thing it fixes is an upstream the
// server has re-resolved since this player loaded it.
const LIVE_RELOAD_AFTER_ATTEMPTS = 3;

// How hard hls.js tries each request of a live stream. The defaults are tuned for
// on-demand video, where a segment is worth waiting for: they retry one for half a
// minute. A live segment that has not arrived within a second or two is worthless --
// the broadcast has moved on -- and waiting for it only guarantees a stall, so it
// is dropped quickly and hls.js plays past the gap. Playlists are the lifeline and
// get the patience instead.
const LIVE_FRAG_POLICY: { default: LoaderConfig } = {
  default: {
    maxTimeToFirstByteMs: 8_000,
    maxLoadTimeMs: 20_000,
    timeoutRetry: { maxNumRetry: 1, retryDelayMs: 0, maxRetryDelayMs: 0 },
    errorRetry: { maxNumRetry: 2, retryDelayMs: 500, maxRetryDelayMs: 1_000 },
  },
};
const LIVE_PLAYLIST_POLICY: { default: LoaderConfig } = {
  default: {
    maxTimeToFirstByteMs: 10_000,
    maxLoadTimeMs: 20_000,
    timeoutRetry: { maxNumRetry: 2, retryDelayMs: 0, maxRetryDelayMs: 0 },
    errorRetry: { maxNumRetry: 4, retryDelayMs: 1_000, maxRetryDelayMs: 4_000 },
  },
};

export const WatchPartyPlayer: React.FC<WatchPartyPlayerProps> = ({
  onOpenLibrary,
}) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const hlsRef = useRef<Hls | null>(null);

  const [isPlayingLocal, setIsPlayingLocal] = useState(false);
  const [duration, setDuration] = useState(0);
  const [progress, setProgress] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [currentLevel, setCurrentLevel] = useState(-1); // -1 = auto ABR
  const [levels, setLevels] = useState<
    { index: number; height: number; bitrate: number }[]
  >([]);
  const [isBuffering, setIsBuffering] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [behindEdge, setBehindEdge] = useState(0);
  const controlsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Set while the element is buffering. The leader reports it so the server can keep
  // extrapolating instead of accepting a frozen playhead, and followers use it to
  // skip drift correction on readings taken mid-rebuffer.
  const stalledRef = useRef(false);

  const {
    mediaUrl,
    isPlaying: isPlayingRemote,
    currentTime: remoteTime,
    lastSyncTimestamp,
    play: emitPlay,
    pause: emitPause,
    seek: emitSeek,
    notifyLocalProgress,
    clearRemoteFlag,
    kind,
    leaderName,
    isLeader,
    liveAnchor,
    liveStatus,
    reportLivePosition,
    declineLiveLeadership,
  } = usePlayerSync();

  const isLive = kind === "live";

  const permissions = usePermissions();
  const { isConnected } = useRoomSocket();

  // Attach Hls.js or native video whenever mediaUrl changes
  useEffect(() => {
    const video = videoRef.current;
    const effectiveUrl = normalizeMediaUrl(mediaUrl);
    if (!video || !effectiveUrl) return;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    // Destroy any previous Hls instance
    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }

    // Tear the old source down explicitly. Without this, switching between a direct
    // MP4 and an HLS master can leave the previous src attached and the new one never
    // starts. hls.js only clears src on detach when it owns the object URL.
    video.removeAttribute("src");
    video.load();

    // Reset per-source state before the new source attaches
    setProgress(0);
    setDuration(0);
    setIsPlayingLocal(false);
    setLevels([]);
    setCurrentLevel(-1);
    setLoadError(null);

    const isHLS =
      effectiveUrl.includes(".m3u8") ||
      effectiveUrl.includes("/hls/") ||
      effectiveUrl.includes("hls_master");

    // Read from the URL being attached rather than from `isLive`, which is server
    // state that lands a beat after the URL does.
    const streamIsLive = isLiveMediaUrl(effectiveUrl);

    if (isHLS && Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        // ABR config — start conservative, ramp up fast
        abrEwmaDefaultEstimate: 1_000_000,
        startLevel: -1, // auto
        // Live tuning. Every viewer targets the same distance behind the edge, so a
        // room starts out roughly together before leader corrections even arrive,
        // and back buffer is kept short because nobody can seek backwards anyway.
        ...(streamIsLive
          ? {
              liveSyncDurationCount: 3,
              liveMaxLatencyDurationCount: 10,
              backBufferLength: 30,
              fragLoadPolicy: LIVE_FRAG_POLICY,
              playlistLoadPolicy: LIVE_PLAYLIST_POLICY,
              // The master playlist is the one live route behind RequireAuth, and a
              // bare hls.js request to the API's origin carries no session, so it
              // 401s. Send it the way apiClient does. Every URI inside carries its
              // own playback token, so those requests are left alone.
              xhrSetup: (xhr: XMLHttpRequest, url: string) => {
                if (!isLiveMediaUrl(url)) return;
                xhr.open("GET", url, true);
                xhr.withCredentials = true;
                const sessionId = localStorage.getItem("inox_session_id");
                if (sessionId) {
                  xhr.setRequestHeader("Authorization", `Bearer ${sessionId}`);
                  xhr.setRequestHeader("X-Session-ID", sessionId);
                }
              },
            }
          : {}),
      });
      hlsRef.current = hls;

      // startLoad() only resumes a stream whose manifest has loaded; before that
      // there is nothing to resume, and the manifest has to be requested again.
      let retryDelayMs = RETRY_BASE_MS;
      let recovering = false;
      let attempts = 0;
      const scheduleRetry = () => {
        recovering = true;
        if (retryTimer) return;
        retryTimer = setTimeout(() => {
          retryTimer = undefined;
          if (hlsRef.current !== hls) return;
          attempts += 1;
          const reload =
            hls.levels.length === 0 ||
            (streamIsLive && attempts >= LIVE_RELOAD_AFTER_ATTEMPTS);
          if (reload) {
            hls.loadSource(effectiveUrl);
          } else {
            hls.startLoad();
          }
        }, retryDelayMs);
        retryDelayMs = Math.min(retryDelayMs * 2, RETRY_MAX_MS);
      };
      const clearRetry = () => {
        recovering = false;
        attempts = 0;
        retryDelayMs = RETRY_BASE_MS;
        setLoadError(null);
      };

      // Media is flowing again. Without this, the error notice outlives the
      // outage and sits over a stream that is playing fine.
      hls.on(Hls.Events.FRAG_BUFFERED, () => {
        if (recovering) clearRetry();
      });

      hls.on(Hls.Events.MANIFEST_PARSED, (_, data) => {
        const parsedLevels = data.levels.map((l, i) => ({
          index: i,
          height: l.height || 0,
          bitrate: l.bitrate || 0,
        }));
        parsedLevels.sort((a, b) => b.height - a.height);
        setLevels(parsedLevels);
        setCurrentLevel(-1);
        clearRetry();
      });

      hls.on(Hls.Events.LEVEL_SWITCHED, (_, data) => {
        setCurrentLevel(data.level);
      });

      // Without this, a failed manifest or segment fetch fails silently and the
      // player just sits on a black frame with no indication of why.
      hls.on(Hls.Events.ERROR, (_, data) => {
        if (!data.fatal) {
          logger.debug("Player: non-fatal HLS error", {
            type: data.type,
            details: data.details,
          });
          return;
        }
        logger.error("Player: fatal HLS error", {
          type: data.type,
          details: data.details,
          url: effectiveUrl,
        });
        if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
          setLoadError("Could not load this stream. Retrying…");
          scheduleRetry();
        } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
          setLoadError("Playback error. Recovering…");
          hls.recoverMediaError();
        } else {
          setLoadError("This media could not be played.");
          hls.destroy();
          hlsRef.current = null;
        }
      });

      hls.loadSource(effectiveUrl);
      hls.attachMedia(video);
    } else {
      // Safari native HLS, or a direct MP4 / WebM
      video.src = effectiveUrl;
      video.load();
    }

    return () => {
      clearTimeout(retryTimer);
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
    };
  }, [mediaUrl]);

  // Sync from remote WebSocket events — triggered by lastSyncTimestamp changing.
  //
  // VOD only. remoteTime is an offset from the start of a file, which on a live
  // stream would land this client somewhere unrelated (usually outside seekable,
  // where hls.js quietly clamps); live rooms are driven by the drift effect below.
  useEffect(() => {
    const video = videoRef.current;
    if (!video || !lastSyncTimestamp || isLive) return;

    if (Math.abs(video.currentTime - remoteTime) > 0.5) {
      video.currentTime = remoteTime;
      setProgress(remoteTime);
    }

    if (isPlayingRemote && video.paused) {
      video.play().catch(() => {});
      setIsPlayingLocal(true);
    } else if (!isPlayingRemote && !video.paused) {
      video.pause();
      setIsPlayingLocal(false);
    }

    const timer = setTimeout(() => clearRemoteFlag(), 300);
    return () => clearTimeout(timer);
  }, [lastSyncTimestamp, clearRemoteFlag, isLive]);

  const handleTimeUpdate = () => {
    const video = videoRef.current;
    if (!video) return;
    setProgress(video.currentTime);
    if (isLive) {
      setBehindEdge(secondsBehindTarget(video, hlsRef.current));
      return;
    }
    notifyLocalProgress(video.currentTime);
  };

  const handleWaiting = () => {
    stalledRef.current = true;
    setIsBuffering(true);
  };

  const handlePlaying = () => {
    stalledRef.current = false;
    setIsBuffering(false);
  };

  // A film that plays to its end stops on its own, without anyone pressing pause,
  // so nothing would tell the server; the room would stay "playing" (and listed
  // under the lobby's Now Streaming) until everyone left. Anyone who could have
  // paused says so; the PAUSE is idempotent if several of them do.
  const handleEnded = () => {
    setIsPlayingLocal(false);
    const video = videoRef.current;
    if (!isLive && permissions.can_control_playback && video) {
      emitPause(video.currentTime);
    }
  };

  const handleLoadedMetadata = () => {
    const video = videoRef.current;
    if (!video) return;
    setDuration(video.duration);
    // A live stream has no meaningful duration or start offset to restore.
    if (isLive) return;
    // Apply any pending remote sync on initial load
    if (remoteTime > 0 && Math.abs(video.currentTime - remoteTime) > 0.5) {
      video.currentTime = remoteTime;
      setProgress(remoteTime);
      if (isPlayingRemote && video.paused) {
        video.play().catch(() => {});
        setIsPlayingLocal(true);
      }
    }
  };

  // Hold the anchor in a ref so the drift interval below stays stable. Depending on
  // liveAnchor directly would tear down and restart the timer on every leader report.
  const liveAnchorRef = useRef(liveAnchor);
  useEffect(() => {
    liveAnchorRef.current = liveAnchor;
  }, [liveAnchor]);

  // Leader: publish this client's playhead, in the stream's coordinates.
  useEffect(() => {
    if (!isLive || !isLeader) return;

    // If nobody else in the room can lead either, the server keeps this client in the
    // role; declining once per term is enough to say so.
    let declined = false;

    const publish = () => {
      const video = videoRef.current;
      if (!video) return;

      const hls = hlsRef.current;
      if (!hls) {
        // Safari and iOS play HLS natively, so there is no fragment list here and no
        // way to express a position in the stream's coordinates. Holding the role
        // silently would leave the room uncorrected forever, so give it back.
        if (!declined) {
          logger.warn(
            "Player: cannot lead live sync without hls.js, declining",
          );
          declined = true;
          declineLiveLeadership();
        }
        return;
      }

      const position = toStreamPosition(hls, video.currentTime);
      if (!position) return;

      // While buffering, this playhead has stopped but the broadcast has not.
      // Flagging it lets the server keep extrapolating rather than dragging every
      // follower backwards and then forwards again on recovery.
      const stalled =
        stalledRef.current || video.readyState < 3 || video.paused;
      reportLivePosition(position, stalled);
    };

    publish(); // don't make a new leader's room wait a full interval
    const timer = setInterval(publish, LEADER_REPORT_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [isLive, isLeader, reportLivePosition, declineLiveLeadership]);

  // Follower: measure drift against the room and close it by playback rate.
  //
  // Seeking a live stream flushes the buffer, so a seek is reserved for gaps too wide
  // to close smoothly; see driftAction for the ladder.
  useEffect(() => {
    if (!isLive || isLeader) return;

    const correct = () => {
      const video = videoRef.current;
      const hls = hlsRef.current;
      const anchor = liveAnchorRef.current;
      if (!video || !hls || !anchor) return;

      // A reading taken while buffering, paused, or in a throttled background tab is
      // noise; acting on it sends the player chasing its own tail.
      if (video.paused || stalledRef.current || video.readyState < 3) return;
      if (typeof document !== "undefined" && document.hidden) return;

      const target = roomLocalTime(
        hls,
        anchor.position,
        anchor.receivedAt,
        anchor.ageMs,
      );

      if (target === null) {
        // The room's segment has aged out of this client's window. Snap to the edge
        // rather than retrying against a position that will never come back.
        const edge = liveEdgeTime(video);
        if (edge !== null && Math.abs(edge - video.currentTime) > 1) {
          logger.warn("Player: fell outside the live window, snapping to edge");
          video.currentTime = edge;
          video.playbackRate = 1;
        }
        return;
      }

      const action = driftAction(target - video.currentTime);
      if (action.seek) {
        logger.debug("Player: live drift too wide, seeking", {
          drift: target - video.currentTime,
        });
        video.currentTime = clampToSeekable(video, target);
        video.playbackRate = 1;
        return;
      }
      if (video.playbackRate !== action.rate) {
        video.playbackRate = action.rate;
      }
    };

    const timer = setInterval(correct, DRIFT_CHECK_INTERVAL_MS);
    // Captured now, because by cleanup time the ref may already point elsewhere and
    // the element we actually sped up would be left running off-speed forever.
    const corrected = videoRef.current;
    return () => {
      clearInterval(timer);
      if (corrected) corrected.playbackRate = 1;
    };
  }, [isLive, isLeader]);

  const handlePlayClick = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;

    // Live streams have no room-wide pause: the broadcast keeps running whatever
    // anyone does. This stops playback for one viewer only, and the drift correction
    // pulls them back to the room as soon as they resume.
    if (isLive) {
      if (video.paused) {
        video.play().catch(() => {});
        setIsPlayingLocal(true);
      } else {
        video.pause();
        setIsPlayingLocal(false);
      }
      return;
    }

    if (!permissions.can_control_playback) return;

    if (video.paused) {
      video.play().catch(() => {});
      setIsPlayingLocal(true);
      emitPlay(video.currentTime);
    } else {
      video.pause();
      setIsPlayingLocal(false);
      emitPause(video.currentTime);
    }
  }, [permissions.can_control_playback, emitPlay, emitPause, isLive]);

  const handleSeek = useCallback(
    (newTime: number) => {
      if (!permissions.can_control_playback) return;
      const video = videoRef.current;
      if (video) video.currentTime = newTime;
      setProgress(newTime);
      emitSeek(newTime);
    },
    [permissions.can_control_playback, emitSeek],
  );

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVol = parseFloat(e.target.value);
    setVolume(newVol);
    setIsMuted(newVol === 0);
    if (videoRef.current) {
      videoRef.current.volume = newVol;
      videoRef.current.muted = newVol === 0;
    }
  };

  const toggleMute = useCallback(() => {
    if (!videoRef.current) return;
    const nextMuted = !isMuted;
    setIsMuted(nextMuted);
    videoRef.current.muted = nextMuted;
    if (!nextMuted && volume === 0) {
      setVolume(0.5);
      videoRef.current.volume = 0.5;
    }
  }, [isMuted, volume]);

  const toggleFullscreen = useCallback(() => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen().catch(() => {});
      setIsFullscreen(true);
    } else {
      document.exitFullscreen().catch(() => {});
      setIsFullscreen(false);
    }
  }, []);

  const handleMouseMove = useCallback(() => {
    setShowControls(true);
    if (controlsTimeoutRef.current) clearTimeout(controlsTimeoutRef.current);
    controlsTimeoutRef.current = setTimeout(() => {
      if (isPlayingLocal) setShowControls(false);
    }, 3000);
  }, [isPlayingLocal]);

  // Keyboard accessibility shortcuts inside focused player
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore key events when typing inside inputs or textareas
      const target = e.target as HTMLElement;
      if (
        target &&
        (target.tagName === "INPUT" || target.tagName === "TEXTAREA")
      )
        return;

      if (e.code === "Space") {
        e.preventDefault();
        handlePlayClick();
      } else if (e.code === "KeyM") {
        e.preventDefault();
        toggleMute();
      } else if (e.code === "KeyF") {
        e.preventDefault();
        toggleFullscreen();
      }
    };

    const container = containerRef.current;
    if (!container) return;
    container.addEventListener("keydown", handleKeyDown);
    return () => container.removeEventListener("keydown", handleKeyDown);
  }, [handlePlayClick, toggleMute, toggleFullscreen]);

  const setQualityLevel = (level: number) => {
    if (hlsRef.current) {
      hlsRef.current.currentLevel = level;
      if (level !== -1) {
        hlsRef.current.nextLoadLevel = level;
      }
      setCurrentLevel(level);
    }
  };

  const currentQualityLabel =
    currentLevel === -1 || levels.length === 0
      ? "Auto"
      : `${levels.find((l) => l.index === currentLevel)?.height ?? "?"}p`;

  const controlsVisible = showControls || !isPlayingLocal;
  const canControl = permissions.can_control_playback;
  const hasMedia = !!mediaUrl;
  const volumePct = (isMuted ? 0 : volume) * 100;

  return (
    <div
      ref={containerRef}
      tabIndex={0}
      onMouseMove={handleMouseMove}
      role="region"
      aria-label="Inox WatchParty Video Player"
      className={styles.container}
      data-controls={controlsVisible ? "shown" : "hidden"}
    >
      {/* Top Status Bar */}
      <div className={`${styles.topBar} ${styles.chrome}`}>
        <div className={styles.cluster}>
          <div
            className={`${styles.statusIndicator} ${isConnected ? styles.statusSynced : styles.statusOffline}`}
          >
            {isConnected ? <Wifi size={11} /> : <WifiOff size={11} />}
            <span>{isConnected ? "Synced" : "Offline"}</span>
          </div>
          {levels.length > 0 && (
            <div className={styles.abrIndicator}>
              ABR · {currentQualityLabel}
            </div>
          )}
        </div>

        {canControl && onOpenLibrary && (
          <button
            type="button"
            onClick={onOpenLibrary}
            aria-label="Open Media Library"
            className={styles.libraryBtn}
          >
            <Layers size={14} aria-hidden="true" />
            <span>Library</span>
          </button>
        )}
      </div>

      {/* Video Element */}
      <video
        ref={videoRef}
        onTimeUpdate={handleTimeUpdate}
        onLoadedMetadata={handleLoadedMetadata}
        onWaiting={handleWaiting}
        onPlaying={handlePlaying}
        onStalled={handleWaiting}
        onCanPlay={handlePlaying}
        onPlay={() => setIsPlayingLocal(true)}
        onPause={() => setIsPlayingLocal(false)}
        onEnded={handleEnded}
        className={`${styles.video} ${canControl || isLive ? styles.videoClickable : ""}`}
        onClick={handlePlayClick}
        playsInline
      />

      {/* Centre overlay: one state at a time, error first */}
      {!hasMedia ? (
        <div className={styles.emptyState}>
          <span className={styles.emptyIcon} aria-hidden="true">
            <Film size={26} />
          </span>
          <p className={styles.emptyTitle}>Nothing is playing yet</p>
          <p className={styles.emptyBody}>
            {canControl && onOpenLibrary
              ? "Pick something from the library to start the party."
              : "The host will pick something to watch."}
          </p>
          {canControl && onOpenLibrary && (
            <button
              type="button"
              className={styles.emptyBtn}
              onClick={onOpenLibrary}
            >
              <Layers size={15} aria-hidden="true" /> Open Library
            </button>
          )}
        </div>
      ) : loadError ? (
        <div role="status" aria-live="polite" className={styles.loadError}>
          <AlertCircle size={14} aria-hidden="true" />
          <span>{loadError}</span>
        </div>
      ) : isBuffering && isPlayingLocal ? (
        <div
          className={styles.centerOverlay}
          role="status"
          aria-label="Buffering"
        >
          <span className={styles.bufferRing}>
            <Spinner size={34} color="var(--color-on-media)" label={null} />
          </span>
        </div>
      ) : !isPlayingLocal ? (
        canControl || isLive ? (
          <button
            type="button"
            className={styles.bigPlay}
            onClick={handlePlayClick}
            aria-label="Play Video"
            tabIndex={-1}
          >
            <Play size={30} fill="currentColor" aria-hidden="true" />
          </button>
        ) : (
          <div className={styles.centerOverlay}>
            <span className={styles.pausedPill}>
              <Pause size={13} fill="currentColor" aria-hidden="true" />
              Paused by the host
            </span>
          </div>
        )
      ) : null}

      {/* Bottom Controls */}
      {hasMedia && (
        <div className={`${styles.bottomBar} ${styles.chrome}`}>
          {/* Transport row. A live stream has no fixed timeline to scrub, so it
              reports where the room is instead of offering a seek target. */}
          {isLive ? (
            <div className={styles.liveBar}>
              <span
                className={`${styles.livePill} ${behindEdge > 8 ? styles.livePillBehind : ""}`}
              >
                <Radio size={10} aria-hidden="true" />
                LIVE
              </span>
              {behindEdge > 1 && (
                <span className={styles.liveMeta}>
                  behind by {behindEdge.toFixed(1)}s
                </span>
              )}
              {liveStatus && liveStatus.status !== "live" && (
                <span
                  className={styles.liveNotice}
                  role="status"
                  aria-live="polite"
                >
                  <AlertCircle size={11} aria-hidden="true" />
                  {liveStatus.message || "The broadcast was interrupted."}
                </span>
              )}
              {leaderName && (
                <span className={styles.liveLeader}>
                  {isLeader
                    ? "You are setting the pace"
                    : `Following ${leaderName}`}
                </span>
              )}
            </div>
          ) : (
            <PlayerScrubber
              duration={duration}
              progress={progress}
              canControl={canControl}
              onSeek={handleSeek}
            />
          )}

          {/* Action Row */}
          <div className={styles.actionRow}>
            <div className={styles.cluster}>
              {/* Play/Pause */}
              <button
                type="button"
                onClick={handlePlayClick}
                disabled={!canControl && !isLive}
                aria-label={isPlayingLocal ? "Pause Video" : "Play Video"}
                title={
                  !canControl && !isLive
                    ? "Only people who can control playback can do this"
                    : undefined
                }
                className={`${styles.playBtn} ${canControl || isLive ? styles.playBtnCanControl : styles.playBtnDisabled}`}
              >
                {!canControl && !isLive ? (
                  <Lock size={15} aria-hidden="true" />
                ) : isPlayingLocal ? (
                  <Pause size={16} fill="currentColor" aria-hidden="true" />
                ) : (
                  <Play
                    size={16}
                    fill="currentColor"
                    className={styles.playGlyph}
                    aria-hidden="true"
                  />
                )}
              </button>

              {/* Volume */}
              <div className={styles.volume}>
                <button
                  type="button"
                  onClick={toggleMute}
                  aria-label={
                    isMuted || volume === 0 ? "Unmute Audio" : "Mute Audio"
                  }
                  className={`${styles.iconBtn} ${isMuted || volume === 0 ? styles.iconBtnDanger : ""}`}
                >
                  {isMuted || volume === 0 ? (
                    <VolumeX size={18} aria-hidden="true" />
                  ) : (
                    <Volume2 size={18} aria-hidden="true" />
                  )}
                </button>
                <input
                  type="range"
                  min={0}
                  max={1}
                  step="0.05"
                  value={isMuted ? 0 : volume}
                  onChange={handleVolumeChange}
                  aria-label="Volume Slider"
                  aria-valuetext={`${Math.round(volumePct)}%`}
                  className={styles.volumeSlider}
                  style={{ "--fill": `${volumePct}%` } as React.CSSProperties}
                />
              </div>
            </div>

            <div className={styles.cluster}>
              {/* Quality Selector. Not portalled: in fullscreen only the player's
                  own subtree is visible, so the menu has to live inside it. */}
              {levels.length > 0 && (
                <DropdownMenu.Root modal={false}>
                  <DropdownMenu.Trigger asChild>
                    <button
                      type="button"
                      aria-label="Video Quality Settings"
                      className={styles.qualityBtn}
                    >
                      <ChevronUp
                        size={12}
                        aria-hidden="true"
                        className={styles.qualityChevron}
                      />
                      {currentQualityLabel}
                    </button>
                  </DropdownMenu.Trigger>
                  <DropdownMenu.Content
                    className={`${menuStyles.content} ${styles.qualityMenu}`}
                    side="top"
                    align="end"
                    sideOffset={8}
                  >
                    <DropdownMenu.Label className={menuStyles.label}>
                      Quality
                    </DropdownMenu.Label>
                    <DropdownMenu.Item
                      className={menuStyles.item}
                      onSelect={() => setQualityLevel(-1)}
                    >
                      <span className={menuStyles.itemLabel}>Auto ABR</span>
                      {currentLevel === -1 && (
                        <span className={menuStyles.itemCheck}>
                          <Check size={14} />
                        </span>
                      )}
                    </DropdownMenu.Item>
                    {levels.map((l) => (
                      <DropdownMenu.Item
                        key={l.index}
                        className={menuStyles.item}
                        onSelect={() => setQualityLevel(l.index)}
                      >
                        <span className={menuStyles.itemLabel}>
                          {l.height ? `${l.height}p` : `Level ${l.index}`}
                          <span className={styles.bitrate}>
                            {" "}
                            · {Math.round(l.bitrate / 1000)}k
                          </span>
                        </span>
                        {currentLevel === l.index && (
                          <span className={menuStyles.itemCheck}>
                            <Check size={14} />
                          </span>
                        )}
                      </DropdownMenu.Item>
                    ))}
                  </DropdownMenu.Content>
                </DropdownMenu.Root>
              )}

              {/* Fullscreen */}
              <button
                type="button"
                onClick={toggleFullscreen}
                aria-label={
                  isFullscreen ? "Exit Fullscreen" : "Enter Fullscreen"
                }
                className={styles.iconBtn}
              >
                {isFullscreen ? (
                  <Minimize2 size={18} aria-hidden="true" />
                ) : (
                  <Maximize2 size={18} aria-hidden="true" />
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
