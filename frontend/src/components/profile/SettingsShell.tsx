import React, { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import { Avatar } from "../common/Avatar";
import { EmptyState } from "../common/EmptyState";
import { Spinner } from "../common/Spinner";
import { useAuth } from "../../hooks/useAuth";
import { useRoom } from "../../hooks/useRoom";
import { useToast } from "../../hooks/useToast";
import { APIError } from "../../api/client";
import { authService } from "../../services/auth/auth.service";
import { cropAvatar, MAX_AVATAR_SOURCE_BYTES } from "../../utils/cropAvatar";
import { tabIds } from "../../utils/tabs";
import {
  User,
  Bell,
  Monitor,
  Shield,
  Image as ImageIcon,
  Camera,
  LogOut,
  Trash2,
  Upload,
} from "lucide-react";
import styles from "./SettingsShell.module.css";

interface SettingsShellProps {
  isOpen: boolean;
  onClose: () => void;
}

type SettingsTab = "profile" | "appearance" | "notifications" | "privacy";

const TAB_PREFIX = "settings";

const AVATAR_ACCEPT = "image/png,image/jpeg,image/webp,image/gif";

type AvatarAction = "upload" | "link" | "remove";

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof APIError && err.message ? err.message : fallback;
}

const ProfileSettingsTab: React.FC = () => {
  const { user, mutateUser } = useAuth();
  const { toast } = useToast();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [linkUrl, setLinkUrl] = useState("");
  const [pending, setPending] = useState<AvatarAction | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const setAvatar = (avatarUrl: string | undefined) => {
    if (user) mutateUser({ ...user, avatar_url: avatarUrl });
  };

  const uploadFile = async (file: File) => {
    if (pending) return;
    setError(null);
    if (!file.type.startsWith("image/")) {
      setError("Choose a PNG, JPEG, WebP or GIF image.");
      return;
    }
    if (file.size > MAX_AVATAR_SOURCE_BYTES) {
      setError("That image is over 20 MB. Choose a smaller one.");
      return;
    }

    setPending("upload");
    try {
      let image: Blob;
      try {
        image = await cropAvatar(file);
      } catch {
        setError("That file couldn't be read as an image.");
        return;
      }
      setAvatar(await authService.uploadAvatar(image));
      toast({ tone: "success", title: "Avatar updated" });
    } catch (err) {
      setError(errorMessage(err, "Failed to upload avatar"));
    } finally {
      setPending(null);
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    // Reset so picking the same file again still fires a change.
    e.target.value = "";
    if (file) void uploadFile(file);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files[0];
    if (file) void uploadFile(file);
  };

  const handleRemove = async () => {
    setPending("remove");
    setError(null);
    try {
      await authService.removeAvatar();
      setAvatar(undefined);
      toast({ title: "Avatar removed" });
    } catch (err) {
      setError(errorMessage(err, "Failed to remove avatar"));
    } finally {
      setPending(null);
    }
  };

  const handleLinkSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const link = linkUrl.trim();
    if (!link || link === user?.avatar_url) return;

    setPending("link");
    setError(null);
    try {
      await authService.setAvatarLink(link);
      setAvatar(link);
      setLinkUrl("");
      toast({ tone: "success", title: "Avatar updated" });
    } catch (err) {
      setError(errorMessage(err, "Failed to update avatar"));
    } finally {
      setPending(null);
    }
  };

  return (
    <>
      <div className={styles.profileCard}>
        <button
          type="button"
          className={styles.avatarPreview}
          data-dragging={isDragging || undefined}
          aria-label="Upload a new avatar"
          aria-busy={pending === "upload" || undefined}
          disabled={pending !== null}
          onClick={() => fileInputRef.current?.click()}
          onDragOver={(e) => {
            e.preventDefault();
            setIsDragging(true);
          }}
          onDragLeave={() => setIsDragging(false)}
          onDrop={handleDrop}
        >
          <Avatar
            src={user?.avatar_url}
            username={user?.username || "U"}
            size="xl"
          />
          {pending === "upload" && (
            <span className={styles.avatarBusy}>
              <Spinner size={22} color="currentColor" label={null} />
            </span>
          )}
          <span className={styles.cameraBadge} aria-hidden="true">
            <Camera size={13} />
          </span>
        </button>
        <div className={styles.identity}>
          <span className={styles.identityName}>{user?.username}</span>
          <span className={styles.identityMeta}>{user?.email}</span>
          <div className={styles.avatarActions}>
            <Button
              size="sm"
              variant="secondary"
              icon={<Upload size={14} />}
              isLoading={pending === "upload"}
              disabled={pending !== null}
              onClick={() => fileInputRef.current?.click()}
            >
              Upload image
            </Button>
            {user?.avatar_url && (
              <Button
                size="sm"
                variant="ghost"
                icon={<Trash2 size={14} />}
                isLoading={pending === "remove"}
                disabled={pending !== null}
                onClick={handleRemove}
              >
                Remove
              </Button>
            )}
          </div>
        </div>
        <input
          ref={fileInputRef}
          type="file"
          accept={AVATAR_ACCEPT}
          className={styles.fileInput}
          tabIndex={-1}
          aria-hidden="true"
          onChange={handleFileChange}
        />
      </div>

      <form onSubmit={handleLinkSubmit} className={styles.form}>
        {error && (
          <div className={styles.errorBox} role="alert">
            {error}
          </div>
        )}

        <TextField
          label="Or use an image link"
          type="url"
          name="avatar-url"
          inputMode="url"
          autoComplete="off"
          spellCheck={false}
          placeholder="https://example.com/avatar.png"
          value={linkUrl}
          onChange={(e) => setLinkUrl(e.target.value)}
          icon={<ImageIcon size={16} />}
          helperText="Uploads are cropped to a square. Linked images should be square already."
        />

        <div className={styles.formActions}>
          <Button
            type="submit"
            variant="primary"
            disabled={
              !linkUrl.trim() ||
              linkUrl.trim() === user?.avatar_url ||
              (pending !== null && pending !== "link")
            }
            isLoading={pending === "link"}
          >
            Use link
          </Button>
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
