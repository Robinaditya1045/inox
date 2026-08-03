import React, { useRef, useEffect, useState, useCallback } from 'react';
import Hls from 'hls.js';
import { usePlayerSync } from '../../hooks/usePlayerSync';
import { usePermissions } from '../../hooks/usePermissions';
import { useRoomSocket } from '../../hooks/useRoomSocket';
import { normalizeMediaUrl } from '../../utils/mediaUrl';
import { PlayerScrubber } from "./PlayerScrubber";
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
} from 'lucide-react';
import styles from './WatchPartyPlayer.module.css';

interface WatchPartyPlayerProps {
  onOpenLibrary?: () => void;
}

export const WatchPartyPlayer: React.FC<WatchPartyPlayerProps> = ({ onOpenLibrary }) => {
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
  const [levels, setLevels] = useState<{ index: number; height: number; bitrate: number }[]>([]);
  const [showQuality, setShowQuality] = useState(false);
  const controlsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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
  } = usePlayerSync();

  const permissions = usePermissions();
  const { isConnected } = useRoomSocket();

  // Attach Hls.js or native video whenever mediaUrl changes
  useEffect(() => {
    const video = videoRef.current;
    const effectiveUrl = normalizeMediaUrl(mediaUrl);
    if (!video || !effectiveUrl) return;

    // Destroy any previous Hls instance
    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }

    const isHLS =
      effectiveUrl.includes('.m3u8') ||
      effectiveUrl.includes('/hls/') ||
      effectiveUrl.includes('hls_master');

    if (isHLS && Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        // ABR config — start conservative, ramp up fast
        abrEwmaDefaultEstimate: 1_000_000,
        startLevel: -1, // auto
      });
      hls.loadSource(effectiveUrl);
      hls.attachMedia(video);

      hls.on(Hls.Events.MANIFEST_PARSED, (_, data) => {
        const parsedLevels = data.levels.map((l, i) => ({
          index: i,
          height: l.height || 0,
          bitrate: l.bitrate || 0,
        }));
        parsedLevels.sort((a, b) => b.height - a.height);
        setLevels(parsedLevels);
        setCurrentLevel(-1);
      });

      hls.on(Hls.Events.LEVEL_SWITCHED, (_, data) => {
        setCurrentLevel(data.level);
      });

      hlsRef.current = hls;
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Safari native HLS
      video.src = effectiveUrl;
    } else {
      // Direct MP4 / WebM
      video.src = effectiveUrl;
    }

    // Reset playback state on source change
    setProgress(0);
    setDuration(0);
    setIsPlayingLocal(false);

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
    };
  }, [mediaUrl]);

  // Sync from remote WebSocket events — triggered by lastSyncTimestamp changing
  useEffect(() => {
    const video = videoRef.current;
    if (!video || !lastSyncTimestamp) return;

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
  }, [lastSyncTimestamp, clearRemoteFlag]);

  const handleTimeUpdate = () => {
    const video = videoRef.current;
    if (!video) return;
    setProgress(video.currentTime);
    notifyLocalProgress(video.currentTime);
  };

  const handleLoadedMetadata = () => {
    const video = videoRef.current;
    if (!video) return;
    setDuration(video.duration);
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

  const handlePlayClick = useCallback(() => {
    if (!permissions.can_control_playback) return;
    const video = videoRef.current;
    if (!video) return;

    if (video.paused) {
      video.play().catch(() => {});
      setIsPlayingLocal(true);
      emitPlay(video.currentTime);
    } else {
      video.pause();
      setIsPlayingLocal(false);
      emitPause(video.currentTime);
    }
  }, [permissions.can_control_playback, emitPlay, emitPause]);

  const handleSeek = useCallback(
    (newTime: number) => {
      if (!permissions.can_control_playback) return;
      const video = videoRef.current;
      if (video) video.currentTime = newTime;
      setProgress(newTime);
      emitSeek(newTime);
    },
    [permissions.can_control_playback, emitSeek]
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
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return;

      if (e.code === 'Space') {
        e.preventDefault();
        handlePlayClick();
      } else if (e.code === 'KeyM') {
        e.preventDefault();
        toggleMute();
      } else if (e.code === 'KeyF') {
        e.preventDefault();
        toggleFullscreen();
      }
    };

    const container = containerRef.current;
    if (!container) return;
    container.addEventListener('keydown', handleKeyDown);
    return () => container.removeEventListener('keydown', handleKeyDown);
  }, [handlePlayClick, toggleMute, toggleFullscreen]);

  const setQualityLevel = (level: number) => {
    if (hlsRef.current) {
      hlsRef.current.currentLevel = level;
      if (level !== -1) {
        hlsRef.current.nextLoadLevel = level;
      }
      setCurrentLevel(level);
    }
    setShowQuality(false);
  };

  const currentQualityLabel =
    currentLevel === -1 || levels.length === 0
      ? 'Auto'
      : `${levels.find((l) => l.index === currentLevel)?.height ?? '?'}p`;

  return (
    <div
      ref={containerRef}
      tabIndex={0}
      onMouseMove={handleMouseMove}
      role="region"
      aria-label="Inox WatchParty Video Player"
      className={styles.container}
    >
      {/* Top Status Bar */}
      <div
        className={styles.topBar}
        style={{
          opacity: showControls || !isPlayingLocal ? 1 : 0,
          pointerEvents: showControls || !isPlayingLocal ? 'auto' : 'none',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
          <div className={`${styles.statusIndicator} ${isConnected ? styles.statusSynced : styles.statusOffline}`}>
            {isConnected ? <Wifi size={11} /> : <WifiOff size={11} />}
            <span>{isConnected ? 'SYNCED' : 'OFFLINE'}</span>
          </div>
          {levels.length > 0 && (
            <div className={styles.abrIndicator}>
              ABR · {currentQualityLabel}
            </div>
          )}
        </div>

        {permissions.can_control_playback && onOpenLibrary && (
          <button
            onClick={onOpenLibrary}
            aria-label="Open Media Library"
            className={styles.libraryBtn}
          >
            <Layers size={13} />
            <span>Library</span>
          </button>
        )}
      </div>

      {/* Video Element */}
      <video
        ref={videoRef}
        onTimeUpdate={handleTimeUpdate}
        onLoadedMetadata={handleLoadedMetadata}
        onPlay={() => setIsPlayingLocal(true)}
        onPause={() => setIsPlayingLocal(false)}
        className={styles.video}
        style={{ cursor: permissions.can_control_playback ? 'pointer' : 'default' }}
        onClick={handlePlayClick}
        playsInline
      />

      {/* Bottom Controls */}
      <div
        className={styles.bottomBar}
        style={{
          opacity: showControls || !isPlayingLocal ? 1 : 0,
          transform: showControls || !isPlayingLocal ? 'translateY(0)' : 'translateY(6px)',
          pointerEvents: showControls || !isPlayingLocal ? 'auto' : 'none',
        }}
      >
        {/* Progress Scrubber (Memoized Child) */}
        <PlayerScrubber
          duration={duration}
          progress={progress}
          canControl={permissions.can_control_playback}
          onSeek={handleSeek}
        />

        {/* Action Row */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
            {/* Play/Pause */}
            <button
              onClick={handlePlayClick}
              disabled={!permissions.can_control_playback}
              aria-label={isPlayingLocal ? 'Pause Video' : 'Play Video'}
              className={`${styles.playBtn} ${permissions.can_control_playback ? styles.playBtnCanControl : styles.playBtnDisabled}`}
            >
              {!permissions.can_control_playback ? (
                <Lock size={15} />
              ) : isPlayingLocal ? (
                <Pause size={16} fill="currentColor" />
              ) : (
                <Play size={16} fill="currentColor" style={{ marginLeft: 2 }} />
              )}
            </button>

            {/* Volume */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <button
                onClick={toggleMute}
                aria-label={isMuted || volume === 0 ? 'Unmute Audio' : 'Mute Audio'}
                className={`${styles.iconBtn} ${isMuted || volume === 0 ? styles.iconBtnDanger : ''}`}
              >
                {isMuted || volume === 0 ? <VolumeX size={16} /> : <Volume2 size={16} />}
              </button>
              <input
                type="range"
                min={0}
                max={1}
                step="0.05"
                value={isMuted ? 0 : volume}
                onChange={handleVolumeChange}
                aria-label="Volume Slider"
                className={styles.volumeSlider}
                style={{
                  background: `linear-gradient(to right, var(--color-accent) 0%, var(--color-accent) ${(isMuted ? 0 : volume) * 100}%, var(--color-border-default) ${(isMuted ? 0 : volume) * 100}%, var(--color-border-default) 100%)`,
                }}
              />
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            {/* Quality Selector */}
            {levels.length > 0 && (
              <div style={{ position: 'relative' }}>
                <button
                  onClick={() => setShowQuality((p) => !p)}
                  aria-label="Video Quality Settings"
                  className={styles.qualityBtn}
                >
                  <ChevronUp size={12} />
                  {currentQualityLabel}
                </button>

                {showQuality && (
                  <div className={styles.qualityMenu}>
                    <button
                      onClick={() => setQualityLevel(-1)}
                      className={`${styles.qualityMenuItem} ${currentLevel === -1 ? styles.qualityMenuItemActive : ''}`}
                    >
                      Auto ABR
                    </button>
                    {levels.map((l) => (
                      <button
                        key={l.index}
                        onClick={() => setQualityLevel(l.index)}
                        className={`${styles.qualityMenuItem} ${currentLevel === l.index ? styles.qualityMenuItemActive : ''}`}
                      >
                        {l.height ? `${l.height}p` : `Level ${l.index}`} · {Math.round(l.bitrate / 1000)}k
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Fullscreen */}
            <button
              onClick={toggleFullscreen}
              aria-label={isFullscreen ? 'Exit Fullscreen' : 'Enter Fullscreen'}
              className={styles.iconBtn}
            >
              {isFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

