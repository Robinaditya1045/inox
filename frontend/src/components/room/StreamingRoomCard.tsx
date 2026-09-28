import React, { type CSSProperties, useState } from "react";
import { Link } from "react-router-dom";
import { Film, Lock, Play, Radio, Users } from "lucide-react";
import type { Room } from "../../types/room";
import type { MediaPreview } from "../../utils/roomActivity";
import { viewersOf } from "../../utils/roomActivity";
import { hueFor } from "../../utils/avatarHue";
import { stagger } from "../../utils/motion";
import { Avatar } from "../common/Avatar";
import styles from "./StreamingRoomCard.module.css";

interface StreamingRoomCardProps {
  room: Room;
  preview: MediaPreview;
  index: number;
}

/** Lobby card for a room that is playing something right now. */
export const StreamingRoomCard: React.FC<StreamingRoomCardProps> = ({
  room,
  preview,
  index,
}) => {
  // Artwork that fails to load falls back to the placeholder, not a broken image.
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const showImage =
    !!preview.thumbnailUrl && failedSrc !== preview.thumbnailUrl;
  const viewers = viewersOf(room);

  return (
    <Link
      to={`/room/${room.id}`}
      className={styles.card}
      style={
        { ...stagger(index), "--room-hue": hueFor(room.name) } as CSSProperties
      }
      aria-label={`Join ${room.name}, ${preview.isLive ? "live" : "now playing"}: ${preview.title}, ${viewers} watching`}
    >
      <div className={styles.preview}>
        {showImage ? (
          <img
            className={styles.image}
            src={preview.thumbnailUrl}
            alt=""
            width={320}
            height={180}
            loading="lazy"
            decoding="async"
            onError={() => setFailedSrc(preview.thumbnailUrl ?? null)}
          />
        ) : (
          <div className={styles.placeholder} aria-hidden="true">
            <span className={styles.placeholderIcon}>
              {preview.isLive ? <Radio size={34} /> : <Film size={34} />}
            </span>
          </div>
        )}

        <div className={styles.chips} aria-hidden="true">
          <span
            className={`${styles.status} ${preview.isLive ? styles.statusLive : styles.statusPlaying}`}
          >
            <span className={styles.eq}>
              <span />
              <span />
              <span />
            </span>
            {preview.isLive ? "Live" : "Playing"}
          </span>
          <span className={styles.viewers}>
            <Users size={12} />
            {viewers} watching
          </span>
        </div>

        <span className={styles.playHint} aria-hidden="true">
          <Play size={20} fill="currentColor" />
        </span>
      </div>

      <div className={styles.body}>
        <Avatar username={room.name} shape="square" size="md" />
        <div className={styles.text}>
          <span className={styles.mediaTitle} title={preview.title}>
            {preview.title}
          </span>
          <span className={styles.roomLine}>
            <span className={styles.roomName}>{room.name}</span>
            {room.is_private && <Lock size={11} aria-label="Private" />}
            {preview.source && <span>· {preview.source}</span>}
          </span>
        </div>
      </div>
    </Link>
  );
};
