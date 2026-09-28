import React, { type CSSProperties, useState } from "react";
import { hueFor } from "../../utils/avatarHue";
import styles from "./Avatar.module.css";

export type AvatarSize = "xs" | "sm" | "md" | "lg" | "xl";
export type AvatarStatus = "online" | "offline" | "none";

interface AvatarProps {
  /** Image URL — if absent, initials are derived from username */
  src?: string | null;
  username: string;
  size?: AvatarSize;
  /** 'square' renders with rounded-rectangle, default is circle */
  shape?: "circle" | "square";
  /** Show a presence dot */
  status?: AvatarStatus;
  className?: string;
}

function getInitials(name: string): string {
  return name
    .split(" ")
    .map((p) => p[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

export const Avatar: React.FC<AvatarProps> = ({
  src,
  username,
  size = "md",
  shape = "circle",
  status = "none",
  className = "",
}) => {
  // A broken avatar URL falls back to initials instead of a broken image icon.
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const showImage = !!src && failedSrc !== src;

  const cls = [
    styles.avatar,
    styles[size],
    shape === "square" ? styles.square : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={cls}
      style={{ "--avatar-bg": hueFor(username) } as CSSProperties}
      aria-hidden="true"
    >
      {showImage ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailedSrc(src)}
        />
      ) : (
        <span>{getInitials(username)}</span>
      )}
      {status !== "none" && (
        <span
          className={`${styles.statusDot} ${status === "online" ? styles.online : styles.offline}`}
        />
      )}
    </div>
  );
};
