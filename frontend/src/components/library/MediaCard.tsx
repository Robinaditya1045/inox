import React from "react";
import { Check, Play } from "lucide-react";
import { MediaPoster } from "./MediaPoster";
import type { MediaAsset } from "../../types/media";
import { formatDuration } from "../../utils/mediaAsset";
import { stagger } from "../../utils/motion";
import styles from "./MediaCard.module.css";

interface MediaCardProps {
  asset: MediaAsset;
  index: number;
  isSelected: boolean;
  /** What the room is playing right now */
  isNowPlaying?: boolean;
  disabled?: boolean;
  /** Small chips over the poster's top-left corner (quality, live status) */
  badges?: React.ReactNode;
  /** One line under the title */
  meta?: React.ReactNode;
  /** Buttons over the poster's top-right corner, shown on hover and focus */
  actions?: React.ReactNode;
  onSelect: () => void;
  onPlay: () => void;
}

/**
 * A library tile in the YouTube / Netflix mould: the poster carries the running
 * time and a play affordance, with the title and a line of detail below. It is
 * a radio in the grid's radiogroup — a click selects, a double-click or the
 * play button plays straight away.
 */
export const MediaCard: React.FC<MediaCardProps> = ({
  asset,
  index,
  isSelected,
  isNowPlaying,
  disabled,
  badges,
  meta,
  actions,
  onSelect,
  onPlay,
}) => {
  const duration = formatDuration(asset.duration_seconds);

  return (
    <div
      className={styles.card}
      style={stagger(index, 16)}
      data-selected={isSelected || undefined}
      data-disabled={disabled || undefined}
    >
      <button
        type="button"
        role="radio"
        aria-checked={isSelected}
        aria-disabled={disabled || undefined}
        className={styles.hit}
        onClick={() => !disabled && onSelect()}
        onDoubleClick={() => !disabled && onPlay()}
      >
        <MediaPoster
          src={asset.thumbnail_url}
          kind={asset.kind}
          className={styles.poster}
        >
          <span className={styles.scrim} aria-hidden="true" />
          {badges && <span className={styles.badges}>{badges}</span>}
          {isNowPlaying && (
            <span className={styles.nowPlaying}>
              <span className={styles.eq} aria-hidden="true">
                <i />
                <i />
                <i />
              </span>
              Now playing
            </span>
          )}
          {duration && !isNowPlaying && (
            <span className={styles.duration}>{duration}</span>
          )}
          {isSelected && (
            <span className={styles.check} aria-hidden="true">
              <Check size={13} strokeWidth={3} />
            </span>
          )}
        </MediaPoster>

        <span className={styles.body}>
          <span className={styles.title}>{asset.title}</span>
          {meta && <span className={styles.meta}>{meta}</span>}
        </span>
      </button>

      {/* Over the radio rather than inside it: a button cannot contain another.
          Kept out of the tab order: from the keyboard, selecting the card and
          using the dialog's Play button does the same. */}
      {!disabled && (
        <span className={styles.playLayer}>
          <button
            type="button"
            className={styles.play}
            onClick={onPlay}
            aria-label={`Play ${asset.title} now`}
            tabIndex={-1}
          >
            <Play size={20} fill="currentColor" aria-hidden="true" />
          </button>
        </span>
      )}

      {actions && <div className={styles.actions}>{actions}</div>}
    </div>
  );
};
