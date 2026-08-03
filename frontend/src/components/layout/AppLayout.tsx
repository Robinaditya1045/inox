import React, { type ReactNode } from 'react';
import { AppRail } from './AppRail';
import { HomeSidebar } from './HomeSidebar';
import styles from './AppLayout.module.css';

interface AppLayoutProps {
  children: ReactNode;
  /** Pass 'room' to suppress the HomeSidebar (room has its own sidebar) */
  variant?: 'home' | 'room';
}

export const AppLayout: React.FC<AppLayoutProps> = ({ children, variant = 'home' }) => {
  return (
    <div className={`${styles.shell} ${variant === 'room' ? styles.shellRoom : ''}`}>
      <div className={styles.railArea}>
        <AppRail />
      </div>

      {variant === 'home' && (
        <div className={styles.sidebarArea}>
          <HomeSidebar />
        </div>
      )}

      <main className={styles.workspaceArea}>
        {children}
      </main>
    </div>
  );
};
