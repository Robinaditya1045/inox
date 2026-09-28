import React from "react";
import styles from "./Badge.module.css";

export type BadgeVariant =
  "default" | "accent" | "success" | "warning" | "danger" | "live";

interface BadgeProps {
  children: React.ReactNode;
  variant?: BadgeVariant;
  /** Springs in when mounted — for counts that appear while you watch */
  pop?: boolean;
  className?: string;
}

export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = "default",
  pop = false,
  className = "",
}) => {
  return (
    <span
      className={[
        styles.badge,
        styles[variant],
        pop ? styles.pop : "",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {children}
    </span>
  );
};
