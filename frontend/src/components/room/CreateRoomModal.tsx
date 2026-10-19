import React, { useId, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import { useRoom } from "../../hooks/useRoom";
import { Tv, Lock, Globe, AlertCircle, KeyRound } from "lucide-react";
import styles from "./CreateRoomModal.module.css";

interface CreateRoomModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const CreateRoomModal: React.FC<CreateRoomModalProps> = ({
  isOpen,
  onClose,
}) => {
  const [name, setName] = useState("");
  const [isPrivate, setIsPrivate] = useState(false);
  const [password, setPassword] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const privacyDescId = useId();

  const { createRoom, isLoadingRoom, roomError, clearRoomError } = useRoom();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearRoomError();
    setValidationError(null);

    if (!name.trim()) {
      setValidationError("Give your room a name.");
      return;
    }

    try {
      const newRoom = await createRoom({
        name: name.trim(),
        is_private: isPrivate,
        password: password.trim() || undefined,
      });
      onClose();
      setName("");
      setIsPrivate(false);
      setPassword("");
      navigate(`/room/${newRoom.id}`);
    } catch {
      // Error handled by room context
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Create a room"
      description="Pick a name. You can invite people once you're inside."
    >
      <form onSubmit={handleSubmit} className={styles.form} noValidate>
        {roomError && (
          <div className={styles.errorBox} role="alert">
            <AlertCircle size={16} aria-hidden="true" />
            <span>{roomError}</span>
          </div>
        )}

        <TextField
          label="Room name"
          type="text"
          name="room-name"
          autoComplete="off"
          placeholder="Friday night sci-fi…"
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            if (validationError) setValidationError(null);
          }}
          icon={<Tv size={16} />}
          error={validationError}
          maxLength={80}
          required
        />

        {/* Privacy Toggle */}
        <button
          type="button"
          role="switch"
          aria-checked={isPrivate}
          // The visible title follows the state; the switch's own name stays
          // "Private room" so on/off keeps meaning the same thing to a screen reader.
          aria-label="Private room"
          aria-describedby={privacyDescId}
          className={styles.privacyBtn}
          onClick={() => setIsPrivate(!isPrivate)}
        >
          <span className={styles.privacyContent}>
            <span
              className={`${styles.privacyIcon} ${isPrivate ? styles.privacyIconPrivate : styles.privacyIconPublic}`}
              aria-hidden="true"
            >
              <span key={String(isPrivate)} className={styles.privacyIconGlyph}>
                {isPrivate ? <Lock size={17} /> : <Globe size={17} />}
              </span>
            </span>
            <span className={styles.privacyText}>
              <span className={styles.privacyTitle}>
                {isPrivate ? "Private room" : "Public room"}
              </span>
              <span id={privacyDescId} className={styles.privacyDesc}>
                {isPrivate
                  ? "Hidden from the lobby. People join by invitation, or with the password below."
                  : "Anyone in the lobby can see and join it."}
              </span>
            </span>
          </span>

          <span
            className={`${styles.switchTrack} ${isPrivate ? styles.switchTrackOn : ""}`}
            aria-hidden="true"
          >
            <span className={styles.switchThumb} />
          </span>
        </button>

        <TextField
          label="Room password"
          type="password"
          name="room-password"
          autoComplete="new-password"
          placeholder="Optional"
          value={password}
          onChange={(e) => {
            setPassword(e.target.value);
            if (validationError) setValidationError(null);
          }}
          icon={<KeyRound size={16} />}
          helperText={
            isPrivate && !password.trim()
              ? "Without one, a private room can only be joined by invitation."
              : "With a password set, the room's link alone won't let anyone in."
          }
          revealable
          maxLength={128}
        />

        <div className={styles.footer}>
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" isLoading={isLoadingRoom}>
            Create & Join
          </Button>
        </div>
      </form>
    </Modal>
  );
};
