import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import { Avatar } from "../common/Avatar";
import { EmptyState } from "../common/EmptyState";
import { useAuth } from "../../hooks/useAuth";
import { useRoom } from "../../hooks/useRoom";
import { useToast } from "../../hooks/useToast";
import { apiClient as api, APIError } from "../../api/client";
import { tabIds } from "../../utils/tabs";
import {
  User,
  Bell,
  Monitor,
  Shield,
  Image as ImageIcon,
  Camera,
  Check,
  LogOut,
} from "lucide-react";
import styles from "./SettingsShell.module.css";

interface SettingsShellProps {
  isOpen: boolean;
  onClose: () => void;
}

type SettingsTab = "profile" | "appearance" | "notifications" | "privacy";

const TAB_PREFIX = "settings";

const ProfileSettingsTab: React.FC = () => {
  const { user, mutateUser } = useAuth();
  const [avatarUrl, setAvatarUrl] = useState(user?.avatar_url || "");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!avatarUrl.trim() || avatarUrl === user?.avatar_url) return;

    setIsLoading(true);
    setError(null);
    setSaved(false);

    try {
      await api.put("/users/profile/avatar", { avatar_url: avatarUrl });
      if (mutateUser && user) {
        mutateUser({ ...user, avatar_url: avatarUrl });
      }
      setSaved(true);
    } catch (err) {
      setError(
        err instanceof APIError && err.message
          ? err.message
          : "Failed to update avatar",
      );
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <div className={styles.profileCard}>
        <div className={styles.avatarPreview}>
          <Avatar
            src={avatarUrl || undefined}
            username={user?.username || "U"}
            size="xl"
          />
          <span className={styles.cameraBadge} aria-hidden="true">
            <Camera size={13} />
          </span>
        </div>
        <div className={styles.identity}>
          <span className={styles.identityName}>{user?.username}</span>
          <span className={styles.identityMeta}>{user?.email}</span>
        </div>
      </div>

      <form onSubmit={handleSubmit} className={styles.form}>
        {error && (
          <div className={styles.errorBox} role="alert">
            {error}
          </div>
        )}

        <TextField
          label="Avatar image URL"
          type="url"
          name="avatar-url"
          inputMode="url"
          autoComplete="off"
          spellCheck={false}
          placeholder="https://example.com/avatar.png"
          value={avatarUrl}
          onChange={(e) => {
            setAvatarUrl(e.target.value);
            setSaved(false);
          }}
          icon={<ImageIcon size={16} />}
          helperText="A square image of at least 128×128 works best."
        />

        <div className={styles.formActions}>
          <Button
            type="submit"
            variant="primary"
            disabled={!avatarUrl.trim() || avatarUrl === user?.avatar_url}
            isLoading={isLoading}
          >
            Save changes
          </Button>
          {saved && (
            <span className={styles.saved} role="status">
              <Check size={14} aria-hidden="true" /> Saved
            </span>
          )}
        </div>
      </form>
    </>
  );
};

export const SettingsShell: React.FC<SettingsShellProps> = ({
  isOpen,
  onClose,
}) => {
  const [activeTab, setActiveTab] = React.useState<SettingsTab>("profile");
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const { logout } = useAuth();
  const { disconnectFromRoom } = useRoom();
  const { toast } = useToast();
  const navigate = useNavigate();

  const tabs: { id: SettingsTab; label: string; icon: React.ReactNode }[] = [
    { id: "profile", label: "My Account", icon: <User size={16} /> },
    { id: "appearance", label: "Appearance", icon: <Monitor size={16} /> },
    { id: "notifications", label: "Notifications", icon: <Bell size={16} /> },
    { id: "privacy", label: "Privacy & Safety", icon: <Shield size={16} /> },
  ];
  const active = tabs.find((t) => t.id === activeTab) ?? tabs[0];
  const ids = tabIds(TAB_PREFIX, active.id);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    try {
      // Hang up and close the room socket before the session goes: otherwise
      // the microphone keeps publishing to the room after logout.
      disconnectFromRoom();
      await logout();
      onClose();
      navigate("/login", { replace: true });
    } catch {
      toast({ tone: "danger", title: "Couldn't log you out. Try again." });
    } finally {
      setIsLoggingOut(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Settings"
      maxWidth="800px"
      flush
    >
      <div className={styles.layout}>
        <nav className={styles.sidebar} aria-label="Settings sections">
          <div
            className={styles.sidebarSection}
            role="tablist"
            aria-orientation="vertical"
            aria-label="User settings"
          >
            <span className={styles.sectionTitle}>User Settings</span>
            {tabs.map((tab) => {
              const tabIdSet = tabIds(TAB_PREFIX, tab.id);
              const selected = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  id={tabIdSet.tab}
                  aria-selected={selected}
                  aria-controls={tabIdSet.panel}
                  onClick={() => setActiveTab(tab.id)}
                  className={styles.tabBtn}
                >
                  <span aria-hidden="true">{tab.icon}</span>
                  <span>{tab.label}</span>
                </button>
              );
            })}
          </div>

          <div className={styles.logout}>
            <button
              type="button"
              className={`${styles.tabBtn} ${styles.logoutBtn}`}
              onClick={handleLogout}
              disabled={isLoggingOut}
            >
              <LogOut size={16} aria-hidden="true" />
              <span>{isLoggingOut ? "Logging out…" : "Log out"}</span>
            </button>
          </div>
        </nav>

        <div
          className={styles.content}
          role="tabpanel"
          id={ids.panel}
          aria-labelledby={ids.tab}
        >
          <div key={active.id} className={styles.panel}>
            <h2 className={styles.contentTitle}>{active.label}</h2>

            {activeTab === "profile" && <ProfileSettingsTab />}

            {activeTab === "appearance" && (
              <EmptyState
                card
                compact
                icon={<Monitor size={20} />}
                title="Appearance settings are coming soon"
                description="Inox uses a dark theme tuned for watching video."
              />
            )}

            {activeTab === "notifications" && (
              <EmptyState
                card
                compact
                icon={<Bell size={20} />}
                title="Notification settings are coming soon"
                description="Invites and friend requests show up in the lobby for now."
              />
            )}

            {activeTab === "privacy" && (
              <EmptyState
                card
                compact
                icon={<Shield size={20} />}
                title="Privacy settings are coming soon"
                description="Private rooms are already invite-only."
              />
            )}
          </div>
        </div>
      </div>
    </Modal>
  );
};
