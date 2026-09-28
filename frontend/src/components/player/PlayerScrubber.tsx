import React, { type CSSProperties } from "react";
import styles from "./PlayerScrubber.module.css";

interface PlayerScrubberProps {
  duration: number;
  progress: number;
  canControl: boolean;
  onSeek: (time: number) => void;
}

const formatTime = (seconds: number): string => {
  if (isNaN(seconds) || !isFinite(seconds)) return "00:00";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  }
  return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
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
      <div className={styles.scrubber}>
        <span className={`${styles.time} ${styles.timeCurrent}`}>
          {formatTime(progress)}
        </span>
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
          className={styles.range}
          style={{ "--fill": `${pct}%` } as CSSProperties}
        />
        <span className={`${styles.time} ${styles.timeTotal}`}>
          {formatTime(duration)}
        </span>
      </div>
    );
  },
);

PlayerScrubber.displayName = "PlayerScrubber";
