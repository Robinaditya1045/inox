import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Modal } from '../common/Modal';
import { TextField as Input } from "../common/TextField";
import { Button } from '../common/Button';
import { useRoom } from '../../hooks/useRoom';
import { Tv, Lock, Globe, AlertCircle } from 'lucide-react';
import styles from './CreateRoomModal.module.css';

interface CreateRoomModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const CreateRoomModal: React.FC<CreateRoomModalProps> = ({ isOpen, onClose }) => {
  const [name, setName] = useState('');
  const [isPrivate, setIsPrivate] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  const { createRoom, isLoadingRoom, roomError, clearRoomError } = useRoom();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearRoomError();
    setValidationError(null);

    if (!name.trim()) {
      setValidationError('Room name is required.');
      return;
    }

    try {
      const newRoom = await createRoom({ name: name.trim(), is_private: isPrivate });
      onClose();
      setName('');
      setIsPrivate(false);
      navigate(`/room/${newRoom.id}`);
    } catch {
      // Error handled by room context
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Create Watch Party Room">
      <form onSubmit={handleSubmit} className={styles.form}>
        {(roomError || validationError) && (
          <div className={styles.errorBox}>
            <AlertCircle size={18} style={{ flexShrink: 0 }} />
            <span>{validationError || roomError}</span>
          </div>
        )}

        <Input
          label="Room Name"
          type="text"
          placeholder="e.g. Cyberpunk Anime Night"
          value={name}
          onChange={(e) => setName(e.target.value)}
          icon={<Tv size={18} />}
          required
        />

        {/* Privacy Toggle */}
        <button
          type="button"
          role="switch"
          aria-checked={isPrivate}
          className={styles.privacyBtn}
          onClick={() => setIsPrivate(!isPrivate)}
        >
          <div className={styles.privacyContent}>
            <div className={`${styles.privacyIcon} ${isPrivate ? styles.privacyIconPrivate : styles.privacyIconPublic}`}>
              {isPrivate ? <Lock size={18} /> : <Globe size={18} />}
            </div>
            <div className={styles.privacyText}>
              <span className={styles.privacyTitle}>
                {isPrivate ? 'Private Room' : 'Public Room'}
              </span>
              <span className={styles.privacyDesc}>
                {isPrivate
                  ? 'Only invited members with direct link can join'
                  : 'Visible in lobby for anyone to join'}
              </span>
            </div>
          </div>

          <div className={`${styles.switchTrack} ${isPrivate ? styles.switchTrackPrivate : ''}`}>
            <div className={`${styles.switchThumb} ${isPrivate ? styles.switchThumbPrivate : ''}`} />
          </div>
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
