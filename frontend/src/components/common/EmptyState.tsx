import React, { type ReactNode } from "react";
import styles from "./EmptyState.module.css";

interface EmptyStateProps {
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  /** Buttons that get the user out of the empty state */
  action?: ReactNode;
  tone?: "default" | "accent" | "danger";
  /** Dashed card around it, for empty regions inside a page */
  card?: boolean;
  compact?: boolean;
  className?: string;
  /** Heading level for the title; defaults to a paragraph */
  as?: "h1" | "h2" | "h3" | "p";
}

/** Shared empty / error / "nothing here yet" layout. */
export const EmptyState: React.FC<EmptyStateProps> = ({
  icon,
  title,
  description,
  action,
  tone = "default",
  card = false,
  compact = false,
  className = "",
  as: TitleTag = "p",
}) => {
  const cls = [
    styles.empty,
    card ? styles.card : "",
    compact ? styles.compact : "",
    tone !== "default" ? styles[tone] : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={cls}>
      {icon && (
        <div className={styles.icon} aria-hidden="true">
          {icon}
        </div>
      )}
      <div className={styles.text}>
        <TitleTag className={styles.title}>{title}</TitleTag>
        {description && <p className={styles.description}>{description}</p>}
      </div>
      {action && <div className={styles.actions}>{action}</div>}
    </div>
  );
};
