import React, { type ReactNode } from "react";
import styles from "./PageHeader.module.css";

interface PageHeaderProps {
  icon?: ReactNode;
  title: string;
  /** Sits after the title (e.g. tabs); drops to its own row on phones */
  children?: ReactNode;
  actions?: ReactNode;
}

/** The 48px bar at the top of every workspace page, matching the room header. */
export const PageHeader: React.FC<PageHeaderProps> = ({
  icon,
  title,
  children,
  actions,
}) => (
  <header className={`${styles.header} ${children ? styles.hasMiddle : ""}`}>
    <div className={styles.titleGroup}>
      {icon && (
        <span className={styles.icon} aria-hidden="true">
          {icon}
        </span>
      )}
      <h1 className={styles.title}>{title}</h1>
    </div>
    {children && (
      <>
        <span className={styles.rule} aria-hidden="true" />
        <div className={styles.middle}>{children}</div>
      </>
    )}
    {!children && <div className={styles.middle} />}
    {actions && <div className={styles.actions}>{actions}</div>}
  </header>
);
