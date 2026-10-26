import React, { useId } from "react";
import { History, Play, Radio, RotateCcw } from "lucide-react";
import { Button } from "../common/Button";
import { MediaPoster } from "../library/MediaPoster";
import type { RoomArrival } from "../../contexts/playerSync.context";
import type { MediaAsset } from "../../types/media";
import { describeMedia } from "../../utils/roomActivity";
import { formatDuration } from "../../utils/mediaAsset";
import styles from "./ContinueWatching.module.css";

type LeftOn = NonNullable<RoomArrival["leftOn"]>;

interface ContinueWatchingProps {
  leftOn: LeftOn;
  /** The library, to name and picture what the room was left on */
  assets: MediaAsset[];
  /** Plays the room from `time` */
  onResume: (time: number) => void;
}

// Under this far in, nobody has really started it; this close to the end (the
// credits, or the last few percent of a short clip), they have finished it.
const STARTED_AFTER_SEC = 5;
const FINISHED_WITHIN_SEC = 20;
const FINISHED_WITHIN_SHARE = 0.05;

/** What a room was left on, offered above the library to pick back up. */
export const ContinueWatching: React.FC<ContinueWatchingProps> = ({
  leftOn,
  assets,
  onResume,
}) => {
  const titleId = useId();
  const preview = describeMedia(leftOn.mediaUrl, assets, leftOn.kind);
  const duration = preview.durationSeconds;
  const started = !preview.isLive && leftOn.time >= STARTED_AFTER_SEC;
  const finished =
    started &&
    !!duration &&
    duration - leftOn.time <=
      Math.min(FINISHED_WITHIN_SEC, duration * FINISHED_WITHIN_SHARE);
  const progress =
    started && duration ? Math.min(1, leftOn.time / duration) : null;

  const eyebrow = preview.isLive
    ? "Last on in this room"
    : started && !finished
      ? "Continue watching"
      : "Last played in this room";

  let meta: string | undefined;
  if (preview.isLive) meta = "Live channel";
  else if (finished) meta = "Finished";
  else if (started)
    meta = duration
      ? `Left at ${formatDuration(leftOn.time)} of ${formatDuration(duration)}`
      : `Left at ${formatDuration(leftOn.time)}`;
  else meta = formatDuration(duration) || preview.source;

  const action = preview.isLive
    ? { label: "Watch live", icon: <Radio size={14} />, time: 0 }
    : finished
      ? { label: "Watch again", icon: <RotateCcw size={14} />, time: 0 }
      : started
        ? {
            label: `Resume from ${formatDuration(leftOn.time)}`,
            icon: <Play size={14} fill="currentColor" />,
            time: leftOn.time,
          }
        : {
            label: "Play",
            icon: <Play size={14} fill="currentColor" />,
            time: 0,
          };

  return (
    <section className={styles.card} aria-labelledby={titleId}>
      <MediaPoster
        src={preview.thumbnailUrl}
        kind={preview.isLive ? "live" : "vod"}
        className={styles.poster}
      >
        {progress !== null && (
          <span className={styles.track} aria-hidden="true">
            <span
              className={styles.fill}
              style={{ width: `${progress * 100}%` }}
            />
          </span>
        )}
      </MediaPoster>
      <div className={styles.info}>
        <span className={styles.eyebrow}>
          <History size={12} aria-hidden="true" />
          {eyebrow}
        </span>
        <h3 id={titleId} className={styles.title} title={preview.title}>
          {preview.title}
        </h3>
        {meta && <p className={styles.meta}>{meta}</p>}
        <div className={styles.actions}>
          <Button
            variant="primary"
            icon={action.icon}
            onClick={() => onResume(action.time)}
          >
            {action.label}
          </Button>
        </div>
      </div>
    </section>
  );
};
