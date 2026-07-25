import React, { useState } from 'react';
import { useAuth } from '../../hooks/useAuth';
import { Button } from '../common/Button';
import { Input } from '../common/Input';
import { X, Image as ImageIcon, Camera } from 'lucide-react';
import { apiClient as api } from '../../api/client';

interface UserAvatarSettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const UserAvatarSettingsModal: React.FC<UserAvatarSettingsModalProps> = ({ isOpen, onClose }) => {
  const { user, mutateUser } = useAuth();
  const [avatarUrl, setAvatarUrl] = useState(user?.avatar_url || '');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!avatarUrl.trim()) return;

    setIsLoading(true);
    setError(null);

    try {
      await api.put('/users/profile/avatar', { avatar_url: avatarUrl });
      
      // Update local context
      if (mutateUser && user) {
        mutateUser({ ...user, avatar_url: avatarUrl });
      }
      
      onClose();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to update avatar');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.7)',
        backdropFilter: 'blur(10px)',
        zIndex: 1000,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className="glass-panel"
        style={{
          width: '100%',
          maxWidth: '480px',
          background: 'var(--color-bg-surface)',
          borderRadius: '24px',
          padding: '32px',
          boxShadow: '0 24px 48px -12px rgba(0, 0, 0, 0.5), 0 0 0 1px var(--color-border-glass)',
          display: 'flex',
          flexDirection: 'column',
          gap: '24px',
          position: 'relative',
        }}
      >
        <button
          onClick={onClose}
          style={{
            position: 'absolute',
            top: '24px',
            right: '24px',
            background: 'none',
            border: 'none',
            color: 'var(--color-text-muted)',
            cursor: 'pointer',
            padding: '4px',
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            transition: 'all 0.2s',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.color = 'var(--color-text-primary)';
            e.currentTarget.style.background = 'rgba(255, 255, 255, 0.1)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.color = 'var(--color-text-muted)';
            e.currentTarget.style.background = 'none';
          }}
        >
          <X size={20} />
        </button>

        <div>
          <h2 style={{ fontSize: '1.5rem', fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: '8px' }}>
            Profile Settings
          </h2>
          <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.95rem' }}>
            Personalize your account with a custom avatar.
          </p>
        </div>

        <div style={{ display: 'flex', justifyContent: 'center', margin: '16px 0' }}>
          <div style={{ position: 'relative' }}>
            {avatarUrl ? (
              <img
                src={avatarUrl}
                alt="Avatar Preview"
                style={{
                  width: '120px',
                  height: '120px',
                  borderRadius: '50%',
                  objectFit: 'cover',
                  border: '4px solid var(--color-bg-surface-hover)',
                  boxShadow: '0 8px 16px rgba(0, 0, 0, 0.2)',
                }}
                onError={(e) => {
                  (e.target as HTMLImageElement).src = 'https://via.placeholder.com/120?text=Error';
                }}
              />
            ) : (
              <div
                style={{
                  width: '120px',
                  height: '120px',
                  borderRadius: '50%',
                  background: 'var(--color-bg-surface-hover)',
                  border: '4px solid var(--color-border-glass)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'var(--color-text-muted)',
                  fontSize: '3rem',
                  fontWeight: 700,
                  boxShadow: '0 8px 16px rgba(0, 0, 0, 0.2)',
                }}
              >
                {user?.username?.[0]?.toUpperCase() || 'U'}
              </div>
            )}
            <div
              style={{
                position: 'absolute',
                bottom: '0',
                right: '0',
                background: 'var(--color-accent-purple)',
                color: 'white',
                padding: '8px',
                borderRadius: '50%',
                display: 'flex',
                boxShadow: '0 4px 8px rgba(170, 59, 255, 0.4)',
              }}
            >
              <Camera size={16} />
            </div>
          </div>
        </div>

        {error && (
          <div style={{ padding: '12px', background: 'rgba(244, 63, 94, 0.1)', color: 'var(--color-accent-rose)', borderRadius: '8px', fontSize: '0.9rem', textAlign: 'center' }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <Input
            label="Avatar Image URL"
            type="url"
            placeholder="https://example.com/my-avatar.png"
            value={avatarUrl}
            onChange={(e) => setAvatarUrl(e.target.value)}
            icon={<ImageIcon size={18} />}
          />
          
          <div style={{ display: 'flex', gap: '12px', marginTop: '8px' }}>
            <Button
              type="button"
              variant="secondary"
              onClick={onClose}
              style={{ flex: 1 }}
              disabled={isLoading}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="primary"
              style={{ flex: 1 }}
              isLoading={isLoading}
            >
              Save Avatar
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
