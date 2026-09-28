import React, { useId, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import { useRoom } from "../../hooks/useRoom";
import { Tv, Lock, Globe, AlertCircle } from "lucide-react";
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
      });
      onClose();
      setName("");
      setIsPrivate(false);
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
                  ? "Hidden from the lobby. People join by invite or with the room link."
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
