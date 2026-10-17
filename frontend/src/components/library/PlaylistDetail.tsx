import React, { useState } from "react";
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Film,
  Pencil,
  Play,
  Trash2,
  X,
} from "lucide-react";
import { MediaPoster } from "./MediaPoster";
import { Button } from "../common/Button";
import { IconButton } from "../common/IconButton";
import { EmptyState } from "../common/EmptyState";
import type { Playlist } from "../../types/media";
import {
  formatDuration,
  formatTotalDuration,
  getPlayUrl,
  playlistDuration,
} from "../../utils/mediaAsset";
import { stagger } from "../../utils/motion";
import styles from "./PlaylistDetail.module.css";

interface PlaylistDetailProps {
  playlist: Playlist;
  /** What the room is playing now, to mark it in the list */
  currentUrl: string;
  onBack: () => void;
  /** Plays the playlist in the room, from the item at startIndex */
  onPlay: (startIndex: number) => void;
  onEdit: () => void;
  onDelete: () => void;
  onRemoveItem: (itemId: string) => void;
  onMoveItem: (itemId: string, by: -1 | 1) => void;
  onBrowseLibrary: () => void;
}

export const PlaylistDetail: React.FC<PlaylistDetailProps> = ({
  playlist,
  currentUrl,
  onBack,
  onPlay,
  onEdit,
  onDelete,
  onRemoveItem,
  onMoveItem,
  onBrowseLibrary,
}) => {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const count = playlist.items.length;
  const playable = playlist.items.filter((i) => i.media.status === "ready");
  const total = formatTotalDuration(playlistDuration(playlist));

  return (
    <div className={styles.detail}>
      <button type="button" className={styles.back} onClick={onBack}>
        <ArrowLeft size={14} aria-hidden="true" />
        All playlists
      </button>
      <div className={styles.hero}>
        <MediaPoster
          src={playlist.items[0]?.media.thumbnail_url}
          className={styles.cover}
        />
        <div className={styles.summary}>
          <h3 className={styles.name}>{playlist.name}</h3>
          {playlist.description && (
            <p className={styles.description}>{playlist.description}</p>
          )}
          <p className={styles.meta}>
            {count === 1 ? "1 video" : `${count} videos`}
            {total && <> · {total}</>}
          </p>

          {confirmDelete ? (
            <div className={styles.actions} role="group" aria-live="polite">
              <span className={styles.confirmText}>Delete this playlist?</span>
              <Button
                variant="danger"
                size="sm"
                icon={<Trash2 size={13} />}
                onClick={onDelete}
              >
                Delete
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setConfirmDelete(false)}
              >
                Keep it
              </Button>
            </div>
          ) : (
            <div className={styles.actions}>
              <Button
                size="sm"
                icon={<Play size={13} fill="currentColor" />}
                disabled={playable.length === 0}
                onClick={() => onPlay(0)}
              >
                Play all
              </Button>
              <IconButton
                label="Edit name and description"
                size="sm"
                variant="ghost"
                onClick={onEdit}
              >
                <Pencil size={15} />
              </IconButton>
              <IconButton
                label="Delete playlist"
                size="sm"
                variant="ghost"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 size={15} />
              </IconButton>
            </div>
          )}
        </div>
      </div>

      {count === 0 ? (
        <EmptyState
          compact
          icon={<Film size={20} />}
          title="Nothing in this playlist yet"
          description="Use the save button on any video in the library to add it here."
          action={
            <Button variant="secondary" size="sm" onClick={onBrowseLibrary}>
              Browse the library
            </Button>
          }
        />
      ) : (
        <ol className={styles.items} aria-label={`Videos in ${playlist.name}`}>
          {playlist.items.map((item, i) => {
            const media = item.media;
            const ready = media.status === "ready";
            const isCurrent = ready && getPlayUrl(media) === currentUrl;
            return (
              <li
                key={item.id}
                className={styles.item}
                style={stagger(i, 12)}
                data-current={isCurrent || undefined}
              >
                <span className={styles.position} aria-hidden="true">
                  {i + 1}
                </span>
                <button
                  type="button"
                  className={styles.itemMain}
                  disabled={!ready}
                  onClick={() => onPlay(i)}
                  aria-label={`Play from ${media.title}`}
                >
                  <MediaPoster
                    src={media.thumbnail_url}
                    className={styles.thumb}
                  >
                    <span className={styles.thumbPlay} aria-hidden="true">
                      <Play size={16} fill="currentColor" />
                    </span>
                  </MediaPoster>
                  <span className={styles.itemText}>
                    <span className={styles.itemTitle}>{media.title}</span>
                    <span className={styles.itemMeta}>
                      {isCurrent
                        ? "Now playing"
                        : !ready
                          ? "Unavailable while it processes"
                          : formatDuration(media.duration_seconds) || "Video"}
                    </span>
                  </span>
                </button>
                <span className={styles.itemActions}>
                  <IconButton
                    label="Move up"
                    size="sm"
                    variant="ghost"
                    disabled={i === 0}
                    onClick={() => onMoveItem(item.id, -1)}
                  >
                    <ArrowUp size={15} />
                  </IconButton>
                  <IconButton
                    label="Move down"
                    size="sm"
                    variant="ghost"
                    disabled={i === count - 1}
                    onClick={() => onMoveItem(item.id, 1)}
                  >
                    <ArrowDown size={15} />
                  </IconButton>
                  <IconButton
                    label="Remove from playlist"
                    size="sm"
                    variant="ghost"
                    onClick={() => onRemoveItem(item.id)}
                  >
                    <X size={15} />
                  </IconButton>
                </span>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
};
