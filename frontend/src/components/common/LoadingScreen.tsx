import React from "react";
import { Logo } from "./Logo";
import { Spinner } from "./Spinner";
import styles from "./LoadingScreen.module.css";

interface LoadingScreenProps {
  label?: string;
  /** Fill the parent instead of the whole viewport */
  inline?: boolean;
}

export const LoadingScreen: React.FC<LoadingScreenProps> = ({
  label = "Loading…",
  inline = false,
}) => (
  <div
    className={`${styles.screen} ${inline ? styles.inline : ""}`}
    role="status"
    aria-live="polite"
  >
    {!inline && (
      <span className={styles.mark}>
        <Logo size={48} />
      </span>
    )}
    <span className={styles.status}>
      <Spinner size={16} label={null} />
      {label}
    </span>
  </div>
);
