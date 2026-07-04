import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Modal } from '../common/Modal';
import { Input } from '../common/Input';
import { Button } from '../common/Button';
import { useRoom } from '../../hooks/useRoom';
import { Tv, Lock, Globe, AlertCircle } from 'lucide-react';

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
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {(roomError || validationError) && (
          <div
            style={{
              padding: '12px 16px',
              background: 'rgba(244, 63, 94, 0.1)',
              border: '1px solid rgba(244, 63, 94, 0.3)',
              borderRadius: '8px',
              color: 'var(--color-accent-rose)',
              fontSize: '0.875rem',
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
            }}
          >
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
        <div
          style={{
            padding: '16px',
            borderRadius: '10px',
            background: 'var(--color-bg-surface)',
            border: '1px solid var(--color-border-glass)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            cursor: 'pointer',
          }}
          onClick={() => setIsPrivate(!isPrivate)}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '8px',
                background: isPrivate ? 'rgba(244, 63, 94, 0.15)' : 'rgba(0, 240, 255, 0.15)',
                color: isPrivate ? 'var(--color-accent-rose)' : 'var(--color-accent-cyan)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {isPrivate ? <Lock size={18} /> : <Globe size={18} />}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
              <span style={{ fontWeight: 600, fontSize: '0.95rem', color: 'var(--color-text-primary)' }}>
                {isPrivate ? 'Private Room' : 'Public Room'}
              </span>
              <span style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)' }}>
                {isPrivate
                  ? 'Only invited members with direct link can join'
                  : 'Visible in lobby for anyone to join'}
              </span>
            </div>
          </div>

          <div
            style={{
              width: '44px',
              height: '24px',
              borderRadius: '12px',
              background: isPrivate ? 'var(--color-accent-purple)' : 'var(--color-bg-surface-hover)',
              border: '1px solid var(--color-border-hover)',
              position: 'relative',
              transition: 'all var(--transition-fast)',
            }}
          >
            <div
              style={{
                width: '18px',
                height: '18px',
                borderRadius: '50%',
                background: '#FFF',
                position: 'absolute',
                top: '2px',
                left: isPrivate ? '22px' : '2px',
                transition: 'all var(--transition-fast)',
                boxShadow: '0 2px 4px rgba(0, 0, 0, 0.4)',
              }}
            />
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px', marginTop: '8px' }}>
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
