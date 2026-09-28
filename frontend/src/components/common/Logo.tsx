import React from "react";
import styles from "./Logo.module.css";

interface LogoProps {
  size?: number;
  withWordmark?: boolean;
  /** Just the white glyph in currentColor, for placing on a coloured tile */
  glyphOnly?: boolean;
  className?: string;
}

const PLAY =
  "M12.4 10.3c0-1.02 1.1-1.66 1.99-1.15l8.46 4.89a1.33 1.33 0 0 1 0 2.3l-8.46 4.89c-.89.51-1.99-.13-1.99-1.15V10.3Z";

/** The Inox mark: a play button over a shared timeline. */
export const Logo: React.FC<LogoProps> = ({
  size = 32,
  withWordmark = false,
  glyphOnly = false,
  className = "",
}) => (
  <span className={`${styles.logo} ${className}`}>
    <svg
      className={styles.mark}
      width={size}
      height={size}
      viewBox="0 0 32 32"
      aria-hidden="true"
      focusable="false"
    >
      {!glyphOnly && (
        <rect className={styles.markBg} width="32" height="32" rx="9" />
      )}
      <path d={PLAY} fill={glyphOnly ? "currentColor" : "#fff"} />
      <path
        d="M8.5 23.5h15"
        stroke={glyphOnly ? "currentColor" : "#fff"}
        strokeOpacity=".55"
        strokeWidth="2"
        strokeLinecap="round"
      />
    </svg>
    {withWordmark ? (
      <span className={styles.wordmark} translate="no">
        Inox
      </span>
    ) : (
      <span className="sr-only">Inox</span>
    )}
  </span>
);
