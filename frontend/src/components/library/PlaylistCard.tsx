import React from "react";
import { ListVideo, Plus } from "lucide-react";
import { MediaPoster } from "./MediaPoster";
import type { Playlist } from "../../types/media";
import { formatTotalDuration, playlistDuration } from "../../utils/mediaAsset";
import { stagger } from "../../utils/motion";
import styles from "./PlaylistCard.module.css";

interface PlaylistCardProps {
  playlist: Playlist;
  index: number;
  onOpen: () => void;
}

const countLabel = (n: number) => (n === 1 ? "1 video" : `${n} videos`);

/** A playlist tile: its first video's poster on a stack, with the count over it. */
export const PlaylistCard: React.FC<PlaylistCardProps> = ({
  playlist,
  index,
  onOpen,
}) => {
  const first = playlist.items[0]?.media;
  const total = formatTotalDuration(playlistDuration(playlist));

  return (
    <button
      type="button"
      className={styles.card}
      style={stagger(index, 16)}
      onClick={onOpen}
    >
      <span className={styles.stack}>
        <MediaPoster src={first?.thumbnail_url} className={styles.poster}>
          <span className={styles.count}>
            <ListVideo size={14} aria-hidden="true" />
            {countLabel(playlist.items.length)}
          </span>
        </MediaPoster>
      </span>
      <span className={styles.body}>
        <span className={styles.title}>{playlist.name}</span>
        <span className={styles.meta}>
          {playlist.items.length === 0
            ? "Empty"
            : total
              ? `${countLabel(playlist.items.length)} · ${total}`
              : countLabel(playlist.items.length)}
        </span>
      </span>
    </button>
  );
};

/** The tile that starts a new playlist, sized to sit in the same grid. */
export const NewPlaylistCard: React.FC<{
  index: number;
  onClick: () => void;
}> = ({ index, onClick }) => (
  <button
    type="button"
    className={`${styles.card} ${styles.newCard}`}
    style={stagger(index, 16)}
    onClick={onClick}
  >
    <span className={styles.newTile}>
      <span className={styles.newIcon} aria-hidden="true">
        <Plus size={20} />
      </span>
    </span>
    <span className={styles.body}>
      <span className={styles.title}>New playlist</span>
      <span className={styles.meta}>Line up videos to play in order</span>
    </span>
  </button>
);
