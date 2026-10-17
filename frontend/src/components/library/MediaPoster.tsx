import React, { useState } from "react";
import { Radio } from "lucide-react";
import { Logo } from "../common/Logo";
import { normalizeMediaUrl } from "../../utils/mediaUrl";
import styles from "./MediaPoster.module.css";

interface MediaPosterProps {
  /** Poster image: uploaded, or a frame the server captured from the video */
  src?: string | null;
  /** Live channels get the same default with a broadcast glyph instead of the mark */
  kind?: "vod" | "live";
  className?: string;
  /** Badges and buttons laid over the picture */
  children?: React.ReactNode;
}

/**
 * A 16:9 poster for a library item. Anything without a poster of its own —
 * live channels, external links, videos whose frame could not be captured, or
 * an image that fails to load — gets one shared default, so the grid reads as
 * a single set rather than a scatter of differently broken tiles.
 */
export const MediaPoster: React.FC<MediaPosterProps> = ({
  src,
  kind = "vod",
  className = "",
  children,
}) => {
  const url = normalizeMediaUrl(src);
  // Keyed on the URL, so a replaced poster gets its own chance to load.
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const showImage = !!url && failedUrl !== url;

  return (
    <span className={`${styles.poster} ${className}`}>
      {showImage ? (
        <img
          className={styles.image}
          src={url}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailedUrl(url)}
        />
      ) : (
        <span className={styles.fallback} aria-hidden="true">
          <span className={styles.fallbackGlyph}>
            {kind === "live" ? (
              <Radio size={26} strokeWidth={1.75} />
            ) : (
              <Logo size={34} glyphOnly />
            )}
          </span>
        </span>
      )}
      {children}
    </span>
  );
};
