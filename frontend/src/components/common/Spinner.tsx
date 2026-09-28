import React, { type CSSProperties } from "react";
import styles from "./Spinner.module.css";

interface SpinnerProps {
  size?: number;
  /** Defaults to the accent text colour. Pass any CSS colour, e.g. "currentColor". */
  color?: string;
  className?: string;
  /** Accessible name. Pass null when a surrounding element already announces
   *  the busy state (a loading button, a labelled status line). */
  label?: string | null;
}

export const Spinner: React.FC<SpinnerProps> = ({
  size = 24,
  color = "var(--color-accent-text)",
  className = "",
  label = "Loading",
}) => {
  const style = {
    width: size,
    height: size,
    "--spinner-color": color,
  } as CSSProperties;

  return (
    <span
      className={`${styles.spinner} ${className}`}
      style={style}
      {...(label === null
        ? { "aria-hidden": true }
        : { role: "status", "aria-label": label })}
    />
  );
};
