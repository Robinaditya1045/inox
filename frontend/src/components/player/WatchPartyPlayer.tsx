import React, { useRef, useEffect, useState, useCallback } from 'react';
import Hls from 'hls.js';
import { usePlayerSync } from '../../hooks/usePlayerSync';
import { usePermissions } from '../../hooks/usePermissions';
import { useWS } from '../../hooks/useWS';
import { normalizeMediaUrl } from '../../utils/mediaUrl';
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
    isRemoteUpdate,
    play: emitPlay,
    pause: emitPause,
    seek: emitSeek,
    notifyLocalProgress,
    clearRemoteFlag,
  } = usePlayerSync();

  const permissions = usePermissions();
  const { isConnected } = useWS();

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

  // Sync from remote WebSocket events
  useEffect(() => {
    const video = videoRef.current;
    if (!video || !isRemoteUpdate) return;

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
  }, [remoteTime, isPlayingRemote, isRemoteUpdate, clearRemoteFlag]);

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
    if (isRemoteUpdate && Math.abs(video.currentTime - remoteTime) > 0.5) {
      video.currentTime = remoteTime;
      setProgress(remoteTime);
      if (isPlayingRemote && video.paused) {
        video.play().catch(() => {});
        setIsPlayingLocal(true);
      }
    }
  };

  const handlePlayClick = () => {
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
  };

  const handleScrubberChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!permissions.can_control_playback) return;
    const newTime = parseFloat(e.target.value);
    const video = videoRef.current;
    if (video) video.currentTime = newTime;
    setProgress(newTime);
    emitSeek(newTime);
  };

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVol = parseFloat(e.target.value);
    setVolume(newVol);
    setIsMuted(newVol === 0);
    if (videoRef.current) {
      videoRef.current.volume = newVol;
      videoRef.current.muted = newVol === 0;
    }
  };

  const toggleMute = () => {
    if (!videoRef.current) return;
    const nextMuted = !isMuted;
    setIsMuted(nextMuted);
    videoRef.current.muted = nextMuted;
    if (!nextMuted && volume === 0) {
      setVolume(0.5);
      videoRef.current.volume = 0.5;
    }
  };

  const toggleFullscreen = () => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen().catch(() => {});
      setIsFullscreen(true);
    } else {
      document.exitFullscreen().catch(() => {});
      setIsFullscreen(false);
    }
  };

  const handleMouseMove = useCallback(() => {
    setShowControls(true);
    if (controlsTimeoutRef.current) clearTimeout(controlsTimeoutRef.current);
    controlsTimeoutRef.current = setTimeout(() => {
      if (isPlayingLocal) setShowControls(false);
    }, 3000);
  }, [isPlayingLocal]);

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

  const formatTime = (seconds: number): string => {
    if (isNaN(seconds) || !isFinite(seconds)) return '00:00';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) {
      return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    }
    return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  };

  const pct = duration > 0 ? (progress / duration) * 100 : 0;
  const currentQualityLabel =
    currentLevel === -1 || levels.length === 0
      ? 'Auto'
      : `${levels.find((l) => l.index === currentLevel)?.height ?? '?'}p`;

  return (
    <div
      ref={containerRef}
      onMouseMove={handleMouseMove}
      style={{
        width: '100%',
        height: '100%',
        borderRadius: '12px',
        overflow: 'hidden',
        background: '#05070A',
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        border: '1px solid var(--color-border-hover)',
      }}
    >
      {/* Top Status Bar */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          padding: '10px 14px',
          background: 'linear-gradient(180deg, rgba(5,7,10,0.9) 0%, transparent 100%)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          zIndex: 10,
          opacity: showControls || !isPlayingLocal ? 1 : 0,
          transition: 'opacity 0.3s',
          pointerEvents: showControls || !isPlayingLocal ? 'auto' : 'none',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <div
            style={{
              padding: '3px 8px',
              borderRadius: '12px',
              background: isConnected ? 'rgba(16,185,129,0.15)' : 'rgba(244,63,94,0.15)',
              border: `1px solid ${isConnected ? 'rgba(16,185,129,0.4)' : 'rgba(244,63,94,0.4)'}`,
              color: isConnected ? 'var(--color-accent-emerald)' : 'var(--color-accent-rose)',
              fontSize: '0.7rem',
              fontWeight: 700,
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
            }}
          >
            {isConnected ? <Wifi size={11} /> : <WifiOff size={11} />}
            <span>{isConnected ? 'SYNCED' : 'OFFLINE'}</span>
          </div>
          {levels.length > 0 && (
            <div
              style={{
                padding: '3px 8px',
                borderRadius: '12px',
                background: 'rgba(170,59,255,0.15)',
                border: '1px solid rgba(170,59,255,0.4)',
                color: 'var(--color-accent-purple)',
                fontSize: '0.7rem',
                fontWeight: 700,
              }}
            >
              ABR · {currentQualityLabel}
            </div>
          )}
        </div>

        {permissions.can_control_playback && onOpenLibrary && (
          <button
            onClick={onOpenLibrary}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '4px 10px',
              borderRadius: '8px',
              background: 'rgba(170,59,255,0.2)',
              border: '1px solid rgba(170,59,255,0.5)',
              color: 'var(--color-accent-purple)',
              fontSize: '0.75rem',
              fontWeight: 600,
              cursor: 'pointer',
              transition: 'all 0.15s',
            }}
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
        style={{
          width: '100%',
          height: '100%',
          objectFit: 'contain',
          cursor: permissions.can_control_playback ? 'pointer' : 'default',
        }}
        onClick={handlePlayClick}
        playsInline
      />

      {/* Bottom Controls */}
      <div
        style={{
          position: 'absolute',
          bottom: '10px',
          left: '10px',
          right: '10px',
          padding: '10px 14px',
          borderRadius: '10px',
          background: 'rgba(5,7,10,0.85)',
          backdropFilter: 'blur(12px)',
          display: 'flex',
          flexDirection: 'column',
          gap: '8px',
          zIndex: 10,
          opacity: showControls || !isPlayingLocal ? 1 : 0,
          transform: showControls || !isPlayingLocal ? 'translateY(0)' : 'translateY(6px)',
          transition: 'all 0.3s',
          pointerEvents: showControls || !isPlayingLocal ? 'auto' : 'none',
          border: '1px solid var(--color-border-hover)',
        }}
      >
        {/* Progress Scrubber */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ fontSize: '0.72rem', fontWeight: 600, color: 'var(--color-text-secondary)', minWidth: '36px', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
            {formatTime(progress)}
          </span>
          <div style={{ flex: 1, position: 'relative', height: '16px', display: 'flex', alignItems: 'center' }}>
            <input
              type="range"
              min={0}
              max={duration || 100}
              step="0.1"
              value={progress}
              onChange={handleScrubberChange}
              disabled={!permissions.can_control_playback}
              style={{
                width: '100%',
                height: '4px',
                borderRadius: '2px',
                background: `linear-gradient(to right, var(--color-accent-purple) 0%, var(--color-accent-purple) ${pct}%, rgba(255,255,255,0.15) ${pct}%, rgba(255,255,255,0.15) 100%)`,
                appearance: 'none',
                cursor: permissions.can_control_playback ? 'pointer' : 'not-allowed',
                outline: 'none',
              }}
            />
          </div>
          <span style={{ fontSize: '0.72rem', fontWeight: 600, color: 'var(--color-text-muted)', minWidth: '36px', fontVariantNumeric: 'tabular-nums' }}>
            {formatTime(duration)}
          </span>
        </div>

        {/* Action Row */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            {/* Play/Pause */}
            <button
              onClick={handlePlayClick}
              disabled={!permissions.can_control_playback}
              style={{
                width: '34px',
                height: '34px',
                borderRadius: '8px',
                background: permissions.can_control_playback
                  ? 'linear-gradient(135deg, var(--color-accent-purple), #8B5CF6)'
                  : 'var(--color-bg-surface)',
                color: '#FFF',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: 'none',
                cursor: permissions.can_control_playback ? 'pointer' : 'not-allowed',
                transition: 'all 0.15s',
                flexShrink: 0,
              }}
            >
              {!permissions.can_control_playback ? (
                <Lock size={15} color="var(--color-text-muted)" />
              ) : isPlayingLocal ? (
                <Pause size={16} fill="#FFF" />
              ) : (
                <Play size={16} fill="#FFF" style={{ marginLeft: '2px' }} />
              )}
            </button>

            {/* Volume */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <button
                onClick={toggleMute}
                style={{
                  color: isMuted || volume === 0 ? 'var(--color-accent-rose)' : 'var(--color-text-secondary)',
                  display: 'flex',
                  alignItems: 'center',
                  padding: '4px',
                  borderRadius: '6px',
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                }}
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
                style={{
                  width: '64px',
                  height: '3px',
                  borderRadius: '2px',
                  background: `linear-gradient(to right, var(--color-accent-cyan) 0%, var(--color-accent-cyan) ${(isMuted ? 0 : volume) * 100}%, rgba(255,255,255,0.15) ${(isMuted ? 0 : volume) * 100}%, rgba(255,255,255,0.15) 100%)`,
                  appearance: 'none',
                  cursor: 'pointer',
                  outline: 'none',
                }}
              />
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            {/* Quality Selector */}
            {levels.length > 0 && (
              <div style={{ position: 'relative' }}>
                <button
                  onClick={() => setShowQuality((p) => !p)}
                  style={{
                    padding: '4px 10px',
                    borderRadius: '6px',
                    background: 'var(--color-bg-surface)',
                    border: '1px solid var(--color-border-glass)',
                    color: 'var(--color-text-secondary)',
                    fontSize: '0.72rem',
                    fontWeight: 600,
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                  }}
                >
                  <ChevronUp size={12} />
                  {currentQualityLabel}
                </button>

                {showQuality && (
                  <div
                    style={{
                      position: 'absolute',
                      bottom: 'calc(100% + 6px)',
                      right: 0,
                      background: 'rgba(5,7,10,0.95)',
                      border: '1px solid var(--color-border-hover)',
                      borderRadius: '8px',
                      overflow: 'hidden',
                      minWidth: '90px',
                      backdropFilter: 'blur(12px)',
                      zIndex: 20,
                    }}
                  >
                    <button
                      onClick={() => setQualityLevel(-1)}
                      style={{
                        width: '100%',
                        padding: '7px 12px',
                        background: currentLevel === -1 ? 'rgba(170,59,255,0.2)' : 'transparent',
                        color: currentLevel === -1 ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                        fontSize: '0.75rem',
                        fontWeight: 600,
                        textAlign: 'left',
                        border: 'none',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      Auto ABR
                    </button>
                    {levels.map((l) => (
                      <button
                        key={l.index}
                        onClick={() => setQualityLevel(l.index)}
                        style={{
                          width: '100%',
                          padding: '7px 12px',
                          background: currentLevel === l.index ? 'rgba(170,59,255,0.2)' : 'transparent',
                          color: currentLevel === l.index ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          textAlign: 'left',
                          border: 'none',
                          cursor: 'pointer',
                          transition: 'all 0.15s',
                        }}
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
              style={{
                padding: '5px',
                borderRadius: '6px',
                color: 'var(--color-text-secondary)',
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
              }}
            >
              {isFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
