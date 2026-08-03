import React, { useState } from 'react';
import { Modal } from '../common/Modal';
import { TextField as Input } from '../common/TextField';
import { Button } from '../common/Button';
import { useAuth } from '../../hooks/useAuth';
import { apiClient as api } from '../../api/client';
import { User, Bell, Monitor, Shield, Image as ImageIcon, Camera } from 'lucide-react';
import styles from './SettingsShell.module.css';
import profileStyles from './ProfileSettingsTab.module.css';

interface SettingsShellProps {
  isOpen: boolean;
  onClose: () => void;
}

type SettingsTab = 'profile' | 'appearance' | 'notifications' | 'privacy';

const ProfileSettingsTab: React.FC = () => {
  const { user, mutateUser } = useAuth();
  const [avatarUrl, setAvatarUrl] = useState(user?.avatar_url || '');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!avatarUrl.trim() || avatarUrl === user?.avatar_url) return;

    setIsLoading(true);
    setError(null);

    try {
      await api.put('/users/profile/avatar', { avatar_url: avatarUrl });
      if (mutateUser && user) {
        mutateUser({ ...user, avatar_url: avatarUrl });
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to update avatar');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className={profileStyles.profileTab}>
      <div className={profileStyles.avatarSection}>
        <div className={profileStyles.avatarPreviewContainer}>
          <div className={profileStyles.avatarWrapper}>
            {avatarUrl ? (
              <img
                src={avatarUrl}
                alt="Avatar Preview"
                className={profileStyles.avatarImage}
                onError={(e) => {
                  (e.target as HTMLImageElement).src = 'https://via.placeholder.com/120?text=Error';
                }}
              />
            ) : (
              <div className={profileStyles.avatarPlaceholder}>
                {user?.username?.[0]?.toUpperCase() || 'U'}
              </div>
            )}
            <div className={profileStyles.cameraIconWrapper}>
              <Camera size={16} />
            </div>
          </div>
        </div>
      </div>

      <div className={profileStyles.formSection}>
        <h3 className={profileStyles.sectionHeading}>Avatar Settings</h3>
        {error && (
          <div className={profileStyles.errorBox}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className={profileStyles.form}>
          <Input
            label="Avatar Image URL"
            type="url"
            placeholder="https://example.com/my-avatar.png"
            value={avatarUrl}
            onChange={(e) => setAvatarUrl(e.target.value)}
            icon={<ImageIcon size={18} />}
          />
          
          <div className={profileStyles.actions}>
            <Button
              type="submit"
              variant="primary"
              disabled={isLoading || avatarUrl === user?.avatar_url}
              isLoading={isLoading}
            >
              Save Changes
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};

export const SettingsShell: React.FC<SettingsShellProps> = ({ isOpen, onClose }) => {
  const [activeTab, setActiveTab] = React.useState<SettingsTab>('profile');

  const tabs: { id: SettingsTab; label: string; icon: React.ReactNode }[] = [
    { id: 'profile', label: 'My Account', icon: <User size={16} /> },
    { id: 'appearance', label: 'Appearance', icon: <Monitor size={16} /> },
    { id: 'notifications', label: 'Notifications', icon: <Bell size={16} /> },
    { id: 'privacy', label: 'Privacy & Safety', icon: <Shield size={16} /> },
  ];

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Settings" maxWidth="800px">
      <div className={styles.layout}>
        {/* Sidebar */}
        <div className={styles.sidebar}>
          <div className={styles.sidebarSection}>
            <span className={styles.sectionTitle}>User Settings</span>
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`${styles.tabBtn} ${activeTab === tab.id ? styles.tabBtnActive : ''}`}
              >
                {tab.icon}
                <span>{tab.label}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Main Content Area */}
        <div className={styles.content}>
          <h2 className={styles.contentTitle}>
            {tabs.find((t) => t.id === activeTab)?.label}
          </h2>
          
          <div className={styles.settingsGroup}>
            {activeTab === 'profile' && (
              <ProfileSettingsTab />
            )}
            
            {activeTab === 'appearance' && (
              <div className={styles.placeholder}>
                <Monitor size={32} className={styles.placeholderIcon} />
                <p>Appearance settings will go here.</p>
              </div>
            )}

            {activeTab === 'notifications' && (
              <div className={styles.placeholder}>
                <Bell size={32} className={styles.placeholderIcon} />
                <p>Notification settings will go here.</p>
              </div>
            )}

            {activeTab === 'privacy' && (
              <div className={styles.placeholder}>
                <Shield size={32} className={styles.placeholderIcon} />
                <p>Privacy settings will go here.</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </Modal>
  );
};
