import React, { useState } from 'react';
import { Modal } from '../common/Modal';
import { Input } from '../common/Input';
import { Button } from '../common/Button';
import { Film, Link2, Sparkles, Check } from 'lucide-react';

interface MediaUrlModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentUrl: string;
  onSelectUrl: (url: string) => void;
}

const PRESET_STREAMS = [
  {
    title: 'Big Buck Bunny (1080p HD Full Movie)',
    category: 'Animation Demo',
    url: 'https://media.w3.org/2010/05/bunny/movie.mp4',
    duration: '9:56',
  },
  {
    title: 'Sintel (Sci-Fi Fantasy Trailer)',
    category: 'Sci-Fi Short',
    url: 'https://media.w3.org/2010/05/sintel/trailer.mp4',
    duration: '0:52',
  },
  {
    title: 'W3C Standard Test Video',
    category: 'Action Promo',
    url: 'https://media.w3.org/2010/05/video/movie_300.mp4',
    duration: '0:15',
  },
  {
    title: 'MDN Nature Flower (CC0 Sample)',
    category: 'Nature Short',
    url: 'https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4',
    duration: '0:05',
  },
];

export const MediaUrlModal: React.FC<MediaUrlModalProps> = ({
  isOpen,
  onClose,
  currentUrl,
  onSelectUrl,
}) => {
  const [customUrl, setCustomUrl] = useState('');
  const [selectedPreset, setSelectedPreset] = useState<string>(currentUrl);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const target = customUrl.trim() || selectedPreset;
    if (target) {
      onSelectUrl(target);
      onClose();
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Select Watch Party Media Stream" maxWidth="580px">
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        {/* Presets Section */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em', display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Sparkles size={14} color="var(--color-accent-purple)" />
            <span>Curated High-Definition Streams</span>
          </span>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '240px', overflowY: 'auto' }}>
            {PRESET_STREAMS.map((preset) => {
              const isSelected = selectedPreset === preset.url && !customUrl.trim();
              return (
                <div
                  key={preset.url}
                  onClick={() => {
                    setSelectedPreset(preset.url);
                    setCustomUrl('');
                  }}
                  style={{
                    padding: '12px 16px',
                    borderRadius: '10px',
                    background: isSelected ? 'rgba(170, 59, 255, 0.15)' : 'var(--color-bg-surface)',
                    border: `1px solid ${isSelected ? 'var(--color-accent-purple)' : 'var(--color-border-glass)'}`,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    cursor: 'pointer',
                    transition: 'all var(--transition-fast)',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <div
                      style={{
                        width: '36px',
                        height: '36px',
                        borderRadius: '8px',
                        background: isSelected ? 'var(--color-accent-purple)' : 'var(--color-bg-surface-hover)',
                        color: '#FFF',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      <Film size={18} />
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column' }}>
                      <span style={{ fontWeight: 600, fontSize: '0.95rem', color: 'var(--color-text-primary)' }}>
                        {preset.title}
                      </span>
                      <span style={{ fontSize: '0.75rem', color: 'var(--color-text-secondary)' }}>
                        {preset.category} • {preset.duration}
                      </span>
                    </div>
                  </div>

                  {isSelected && (
                    <div style={{ width: '24px', height: '24px', borderRadius: '50%', background: 'var(--color-accent-purple)', color: '#FFF', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <Check size={14} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>

        {/* Custom URL Input */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', borderTop: '1px solid var(--color-border-glass)', paddingTop: '16px' }}>
          <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em', display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Link2 size={14} color="var(--color-accent-cyan)" />
            <span>Or Enter Custom Video URL (MP4 / WebM / HLS)</span>
          </span>
          <Input
            type="url"
            placeholder="https://example.com/video.mp4"
            value={customUrl}
            onChange={(e) => setCustomUrl(e.target.value)}
            icon={<Link2 size={18} />}
          />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary">
            Load Stream
          </Button>
        </div>
      </form>
    </Modal>
  );
};
