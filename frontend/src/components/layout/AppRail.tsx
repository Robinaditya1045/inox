import React, { useState } from 'react';
import { NavLink } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { useRoom } from '../../hooks/useRoom';
import { Avatar } from '../common/Avatar';
import { SettingsShell } from '../profile/SettingsShell';
import { Compass, LogOut } from 'lucide-react';
import styles from './AppRail.module.css';

export const AppRail: React.FC = () => {
  const { user, logout } = useAuth();
  const { invitations } = useRoom();
  
  const [isProfileOpen, setIsProfileOpen] = useState(false);

  const hasPendingInvites = invitations.length > 0;

  return (
    <>
      <nav className={styles.rail} aria-label="Main navigation">
        {/* Home / Lobby */}
        <NavLink
          to="/"
          end
          className={({ isActive }) =>
            `${styles.railItem} ${isActive ? styles.railItemActive : ''}`
          }
          title="Room Lobby"
          aria-label="Room Lobby"
        >
          <Compass size={18} />
          {hasPendingInvites && (
            <span
              style={{
                position: 'absolute',
                top: 4,
                right: 4,
                width: 7,
                height: 7,
                borderRadius: 'var(--radius-full)',
                background: 'var(--color-danger)',
                border: '1px solid var(--color-canvas)',
              }}
              aria-hidden="true"
            />
          )}
        </NavLink>

        <div className={styles.railDivider} />

        <div className={styles.railSpacer} />

        {/* Logout */}
        <button
          className={styles.railItem}
          onClick={logout}
          title="Sign Out"
          aria-label="Sign Out"
        >
          <LogOut size={16} />
        </button>

        {/* User Avatar */}
        <button
          className={styles.railItem}
          onClick={() => setIsProfileOpen(true)}
          title="Profile & Settings"
          aria-label="Profile & Settings"
          style={{ position: 'relative', overflow: 'visible' }}
        >
          <Avatar
            src={user?.avatar_url}
            username={user?.username || 'U'}
            size="sm"
            status="online"
          />
        </button>
      </nav>

      <SettingsShell
        isOpen={isProfileOpen}
        onClose={() => setIsProfileOpen(false)}
      />
    </>
  );
};
