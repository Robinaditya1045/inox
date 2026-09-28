import React, { type ReactNode } from "react";
import { AppRail } from "./AppRail";
import { HomeSidebar } from "./HomeSidebar";
import styles from "./AppLayout.module.css";

interface AppLayoutProps {
  children: ReactNode;
  /** Pass 'room' to suppress the HomeSidebar (room has its own sidebar) */
  variant?: "home" | "room";
}

export const AppLayout: React.FC<AppLayoutProps> = ({
  children,
  variant = "home",
}) => {
  return (
    <div
      className={`${styles.shell} ${variant === "room" ? styles.shellRoom : ""}`}
    >
      <a href="#main-content" className="skip-link">
        Skip to content
      </a>

      <div className={styles.railArea}>
        <AppRail variant={variant} />
      </div>

      {variant === "home" && (
        <div className={styles.sidebarArea}>
          <HomeSidebar />
        </div>
      )}

      <main id="main-content" tabIndex={-1} className={styles.workspaceArea}>
        {children}
      </main>
    </div>
  );
};
