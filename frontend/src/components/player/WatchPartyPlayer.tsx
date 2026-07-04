import React, { useRef, useEffect, useState } from 'react';
import { usePlayerSync } from '../../hooks/usePlayerSync';
import { usePermissions } from '../../hooks/usePermissions';
import { useWS } from '../../hooks/useWS';
import { Button } from '../common/Button';
import { MediaUrlModal } from './MediaUrlModal';
import {
  Play,
  Pause,
  Volume2,
  VolumeX,
  Maximize2,
  Minimize2,
  Film,
  Lock,
  Wifi,
  WifiOff,
  Sparkles,
} from 'lucide-react';

export const WatchPartyPlayer: React.FC = () => {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const [isPlayingLocal, setIsPlayingLocal] = useState(false);
  const [duration, setDuration] = useState(0);
  const [progress, setProgress] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isUrlModalOpen, setIsUrlModalOpen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const controlsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const {
    mediaUrl,
    isPlaying: isPlayingRemote,
    currentTime: remoteTime,
    isRemoteUpdate,
    setMediaUrl,
    play: emitPlay,
    pause: emitPause,
    seek: emitSeek,
    notifyLocalProgress,
    clearRemoteFlag,
  } = usePlayerSync();

  const permissions = usePermissions();
  const { isConnected } = useWS();

  // Handle remote video sync from WebSocket events
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    if (isRemoteUpdate) {
      if (Math.abs(video.currentTime - remoteTime) > 0.5) {
        video.currentTime = remoteTime;
        setProgress(remoteTime);
      }

      if (isPlayingRemote && video.paused) {
        video.play().catch(() => {
          // Autoplay block or user interaction required
        });
        setIsPlayingLocal(true);
      } else if (!isPlayingRemote && !video.paused) {
        video.pause();
        setIsPlayingLocal(false);
      }

      // Clear remote update flag after DOM reconciliation
      const timer = setTimeout(() => {
        clearRemoteFlag();
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [remoteTime, isPlayingRemote, isRemoteUpdate, clearRemoteFlag]);

  // Video DOM Event Handlers
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
    if (video) {
      video.currentTime = newTime;
    }
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

  const handleMouseMove = () => {
    setShowControls(true);
    if (controlsTimeoutRef.current) {
      clearTimeout(controlsTimeoutRef.current);
    }
    controlsTimeoutRef.current = setTimeout(() => {
      if (isPlayingLocal) {
        setShowControls(false);
      }
    }, 3000);
  };

  const formatTime = (seconds: number): string => {
    if (isNaN(seconds)) return '00:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  return (
    <div
      ref={containerRef}
      onMouseMove={handleMouseMove}
      style={{
        width: '100%',
        height: '100%',
        borderRadius: '16px',
        overflow: 'hidden',
        background: '#05070A',
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        border: '1px solid var(--color-border-hover)',
        boxShadow: '0 0 40px rgba(0, 0, 0, 0.8)',
      }}
    >
      {/* Top Status Overlay Bar */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          padding: '16px 24px',
          background: 'linear-gradient(180deg, rgba(5, 7, 10, 0.85) 0%, transparent 100%)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          zIndex: 10,
          opacity: showControls || !isPlayingLocal ? 1 : 0,
          transition: 'opacity var(--transition-normal)',
          pointerEvents: showControls || !isPlayingLocal ? 'auto' : 'none',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <div
            style={{
              padding: '6px 12px',
              borderRadius: '20px',
              background: isConnected ? 'rgba(16, 185, 129, 0.15)' : 'rgba(244, 63, 94, 0.15)',
              border: `1px solid ${isConnected ? 'rgba(16, 185, 129, 0.4)' : 'rgba(244, 63, 94, 0.4)'}`,
              color: isConnected ? 'var(--color-accent-emerald)' : 'var(--color-accent-rose)',
              fontSize: '0.75rem',
              fontWeight: 700,
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            {isConnected ? <Wifi size={14} /> : <WifiOff size={14} />}
            <span>{isConnected ? 'WS: SYNCED' : 'WS: OFFLINE'}</span>
          </div>

          <div
            style={{
              padding: '6px 12px',
              borderRadius: '20px',
              background: 'var(--color-bg-surface)',
              border: '1px solid var(--color-border-glass)',
              fontSize: '0.75rem',
              color: 'var(--color-text-secondary)',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            <Sparkles size={14} color="var(--color-accent-purple)" />
            <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '240px' }}>
              Stream: {mediaUrl.split('/').pop() || 'HD Media'}
            </span>
          </div>
        </div>

        <Button
          variant="secondary"
          size="sm"
          icon={<Film size={16} />}
          onClick={() => setIsUrlModalOpen(true)}
          disabled={!permissions.can_control_playback}
        >
          Change Stream
        </Button>
      </div>

      {/* Main Video Element */}
      <video
        ref={videoRef}
        src={mediaUrl}
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

      {/* Bottom Glass Control Bar */}
      <div
        className="glass-panel-heavy"
        style={{
          position: 'absolute',
          bottom: '16px',
          left: '16px',
          right: '16px',
          padding: '16px 20px',
          borderRadius: '16px',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
          zIndex: 10,
          opacity: showControls || !isPlayingLocal ? 1 : 0,
          transform: showControls || !isPlayingLocal ? 'translateY(0)' : 'translateY(10px)',
          transition: 'all var(--transition-normal)',
          pointerEvents: showControls || !isPlayingLocal ? 'auto' : 'none',
          boxShadow: '0 10px 30px rgba(0, 0, 0, 0.8), 0 0 20px rgba(170, 59, 255, 0.15)',
          border: '1px solid var(--color-border-hover)',
        }}
      >
        {/* Scrubber / Progress Bar */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', width: '100%' }}>
          <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-text-secondary)', minWidth: '42px', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
            {formatTime(progress)}
          </span>

          <div style={{ flex: 1, position: 'relative', display: 'flex', alignItems: 'center' }}>
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
                height: '6px',
                borderRadius: '3px',
                background: `linear-gradient(to right, var(--color-accent-purple) 0%, var(--color-accent-purple) ${(progress / (duration || 1)) * 100}%, rgba(255, 255, 255, 0.15) ${(progress / (duration || 1)) * 100}%, rgba(255, 255, 255, 0.15) 100%)`,
                appearance: 'none',
                cursor: permissions.can_control_playback ? 'pointer' : 'not-allowed',
                outline: 'none',
              }}
            />
          </div>

          <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-text-muted)', minWidth: '42px', fontVariantNumeric: 'tabular-nums' }}>
            {formatTime(duration)}
          </span>
        </div>

        {/* Action Buttons Row */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
            {/* Play/Pause Button */}
            <button
              onClick={handlePlayClick}
              disabled={!permissions.can_control_playback}
              style={{
                width: '42px',
                height: '42px',
                borderRadius: '12px',
                background: permissions.can_control_playback
                  ? 'linear-gradient(135deg, var(--color-accent-purple), #8B5CF6)'
                  : 'var(--color-bg-surface)',
                color: '#FFF',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid rgba(255, 255, 255, 0.2)',
                boxShadow: permissions.can_control_playback ? '0 0 15px var(--color-accent-purple-glow)' : 'none',
                cursor: permissions.can_control_playback ? 'pointer' : 'not-allowed',
                transition: 'all var(--transition-fast)',
              }}
              title={permissions.can_control_playback ? (isPlayingLocal ? 'Pause' : 'Play') : 'Playback control restricted by RBAC'}
            >
              {!permissions.can_control_playback ? (
                <Lock size={18} color="var(--color-text-muted)" />
              ) : isPlayingLocal ? (
                <Pause size={20} fill="#FFF" />
              ) : (
                <Play size={20} fill="#FFF" style={{ marginLeft: '2px' }} />
              )}
            </button>

            {/* Volume Control */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <button
                onClick={toggleMute}
                style={{
                  color: isMuted || volume === 0 ? 'var(--color-accent-rose)' : 'var(--color-text-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  padding: '6px',
                  borderRadius: '8px',
                }}
              >
                {isMuted || volume === 0 ? <VolumeX size={20} /> : <Volume2 size={20} />}
              </button>
              <input
                type="range"
                min={0}
                max={1}
                step="0.05"
                value={isMuted ? 0 : volume}
                onChange={handleVolumeChange}
                style={{
                  width: '80px',
                  height: '4px',
                  borderRadius: '2px',
                  background: `linear-gradient(to right, var(--color-accent-cyan) 0%, var(--color-accent-cyan) ${(isMuted ? 0 : volume) * 100}%, rgba(255, 255, 255, 0.15) ${(isMuted ? 0 : volume) * 100}%, rgba(255, 255, 255, 0.15) 100%)`,
                  appearance: 'none',
                  cursor: 'pointer',
                  outline: 'none',
                }}
              />
            </div>

            {/* RBAC Notice for Guests */}
            {!permissions.can_control_playback && (
              <span style={{ fontSize: '0.75rem', color: 'var(--color-accent-rose)', display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 10px', borderRadius: '6px', background: 'rgba(244, 63, 94, 0.1)' }}>
                <Lock size={12} />
                <span>RBAC: View-Only Mode</span>
              </span>
            )}
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <button
              onClick={toggleFullscreen}
              style={{
                padding: '8px',
                borderRadius: '8px',
                color: 'var(--color-text-secondary)',
                transition: 'all var(--transition-fast)',
                display: 'flex',
                alignItems: 'center',
              }}
              title="Fullscreen"
            >
              {isFullscreen ? <Minimize2 size={20} /> : <Maximize2 size={20} />}
            </button>
          </div>
        </div>
      </div>

      <MediaUrlModal
        isOpen={isUrlModalOpen}
        onClose={() => setIsUrlModalOpen(false)}
        currentUrl={mediaUrl}
        onSelectUrl={setMediaUrl}
      />
    </div>
  );
};
