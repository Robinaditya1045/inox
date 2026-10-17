import React, { useEffect, useRef, useState } from "react";
import {
  ChevronDown,
  ListVideo,
  SkipBack,
  SkipForward,
  X,
} from "lucide-react";
import { usePlayerSync } from "../../hooks/usePlayerSync";
import { usePermissions } from "../../hooks/usePermissions";
import { IconButton } from "../common/IconButton";
import { MediaPoster } from "../library/MediaPoster";
import { formatDuration } from "../../utils/mediaAsset";
import styles from "./PlaybackQueueBar.module.css";

/**
 * The playlist the room is playing through: where it is, what is next, and —
 * for members who control playback — skipping and stopping. Expands into a
 * strip of every video in the queue, like a series' episode row.
 */
export const PlaybackQueueBar: React.FC = () => {
  const { queue, mediaUrl, advanceQueue, clearQueue } = usePlayerSync();
  const { can_control_playback: canControl } = usePermissions();
  const [isOpen, setIsOpen] = useState(false);
  const currentRef = useRef<HTMLLIElement | null>(null);

  // Keep the current video in view in the strip as the room moves on.
  const index = queue?.index ?? -1;
  useEffect(() => {
    if (isOpen) {
      currentRef.current?.scrollIntoView({
        block: "nearest",
        inline: "center",
        behavior: "smooth",
      });
    }
  }, [isOpen, index]);

  if (!queue) return null;

  const total = queue.items.length;
  // Off the queue's current item, the room is between the server's two
  // messages; nothing here is actionable for that instant.
  const onTrack = queue.items[queue.index]?.media_url === mediaUrl;
  const next = queue.items[queue.index + 1];

  return (
    <section className={styles.bar} aria-label="Play queue">
      <div className={styles.row}>
        <button
          type="button"
          className={styles.summary}
          onClick={() => setIsOpen((o) => !o)}
          aria-expanded={isOpen}
        >
          <span className={styles.icon} aria-hidden="true">
            <ListVideo size={16} />
          </span>
          <span className={styles.text}>
            <span className={styles.title}>
              {queue.title || "Playlist"}
              <span className={styles.position}>
                {queue.index + 1} / {total}
              </span>
            </span>
            <span className={styles.next}>
              {next ? (
                <>
                  Up next: <strong>{next.title}</strong>
                </>
              ) : (
                "Last video in the playlist"
              )}
            </span>
          </span>
          <ChevronDown
            size={16}
            className={styles.chevron}
            data-open={isOpen || undefined}
            aria-hidden="true"
          />
        </button>

        {canControl && (
          <div className={styles.controls}>
            <IconButton
              label="Previous video"
              size="sm"
              variant="ghost"
              disabled={!onTrack || queue.index === 0}
              onClick={() => advanceQueue(queue.index - 1)}
            >
              <SkipBack size={15} />
            </IconButton>
            <IconButton
              label="Next video"
              size="sm"
              variant="ghost"
              disabled={!onTrack || !next}
              onClick={() => advanceQueue()}
            >
              <SkipForward size={15} />
            </IconButton>
            <IconButton
              label="Stop playing the playlist"
              tooltip="Stop the playlist (the current video keeps playing)"
              size="sm"
              variant="ghost"
              onClick={clearQueue}
            >
              <X size={15} />
            </IconButton>
          </div>
        )}
      </div>

      {isOpen && (
        <ol className={styles.strip}>
          {queue.items.map((item, i) => {
            const isCurrent = i === queue.index;
            const duration = formatDuration(item.duration_seconds);
            return (
              <li
                key={`${i}-${item.media_url}`}
                ref={isCurrent ? currentRef : undefined}
                className={styles.stripItem}
                data-current={isCurrent || undefined}
                data-played={i < queue.index || undefined}
              >
                <button
                  type="button"
                  className={styles.stripBtn}
                  disabled={!canControl || isCurrent || !onTrack}
                  onClick={() => advanceQueue(i)}
                  aria-current={isCurrent || undefined}
                  aria-label={`${i + 1}. ${item.title}${isCurrent ? " (playing)" : ""}`}
                >
                  <MediaPoster
                    src={item.thumbnail_url}
                    className={styles.stripPoster}
                  >
                    <span className={styles.stripIndex}>{i + 1}</span>
                    {duration && (
                      <span className={styles.stripDuration}>{duration}</span>
                    )}
                  </MediaPoster>
                  <span className={styles.stripTitle}>{item.title}</span>
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
};
