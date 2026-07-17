import React from 'react';

interface PlayerScrubberProps {
  duration: number;
  progress: number;
  canControl: boolean;
  onSeek: (time: number) => void;
}

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

export const PlayerScrubber: React.FC<PlayerScrubberProps> = React.memo(
  ({ duration, progress, canControl, onSeek }) => {
    const pct = duration > 0 ? (progress / duration) * 100 : 0;

    const handleScrubberChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      if (!canControl) return;
      const newTime = parseFloat(e.target.value);
      onSeek(newTime);
    };

    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <span
          style={{
            fontSize: '0.72rem',
            fontWeight: 600,
            color: 'var(--color-text-secondary)',
            minWidth: '36px',
            textAlign: 'right',
            fontVariantNumeric: 'tabular-nums',
          }}
        >
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
            disabled={!canControl}
            aria-label="Video Playback Scrubber"
            aria-valuemin={0}
            aria-valuemax={duration || 100}
            aria-valuenow={progress}
            aria-valuetext={`${formatTime(progress)} of ${formatTime(duration)}`}
            style={{
              width: '100%',
              height: '4px',
              borderRadius: '2px',
              background: `linear-gradient(to right, var(--color-accent-purple) 0%, var(--color-accent-purple) ${pct}%, rgba(255,255,255,0.15) ${pct}%, rgba(255,255,255,0.15) 100%)`,
              appearance: 'none',
              cursor: canControl ? 'pointer' : 'not-allowed',
              outline: 'none',
            }}
          />
        </div>
        <span
          style={{
            fontSize: '0.72rem',
            fontWeight: 600,
            color: 'var(--color-text-muted)',
            minWidth: '36px',
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {formatTime(duration)}
        </span>
      </div>
    );
  }
);

PlayerScrubber.displayName = 'PlayerScrubber';
