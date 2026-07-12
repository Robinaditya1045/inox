import React, { useState } from 'react';
import { useMediaLibrary } from '../../hooks/useMediaLibrary';
import { Modal } from '../common/Modal';
import { Input } from '../common/Input';
import { Button } from '../common/Button';
import { Spinner } from '../common/Spinner';
import type { MediaAsset } from '../../types/media';
import { normalizeMediaUrl } from '../../utils/mediaUrl';
import {
  Film,
  Link2,
  Check,
  RefreshCw,
  Play,
  Clock,
  Layers,
  AlertCircle,
} from 'lucide-react';

interface MediaLibraryPickerProps {
  isOpen: boolean;
  onClose: () => void;
  currentUrl: string;
  onSelectUrl: (url: string) => void;
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds === 0) return '—';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function getPlayUrl(asset: MediaAsset): string {
  // Prefer HLS master for ABR, fall back to source
  return normalizeMediaUrl(asset.hls_master_url || asset.source_url);
}

function getRenditionBadge(asset: MediaAsset): string | null {
  if (!asset.renditions || asset.renditions.length === 0) return null;
  const maxRes = Math.max(...asset.renditions.map((r) => parseInt(r.resolution) || 0));
  return maxRes > 0 ? `${maxRes}p` : null;
}

export const MediaLibraryPicker: React.FC<MediaLibraryPickerProps> = ({
  isOpen,
  onClose,
  currentUrl,
  onSelectUrl,
}) => {
  const { assets, isLoading, error, refresh } = useMediaLibrary();
  const [customUrl, setCustomUrl] = useState('');
  const [selected, setSelected] = useState<string>(currentUrl);
  const [tab, setTab] = useState<'library' | 'url'>('library');

  const handleConfirm = () => {
    const target = tab === 'url' ? normalizeMediaUrl(customUrl.trim()) : normalizeMediaUrl(selected);
    if (target) {
      onSelectUrl(target);
      onClose();
    }
  };

  const handleSelectAsset = (asset: MediaAsset) => {
    setSelected(getPlayUrl(asset));
    setTab('library');
  };

  const tabStyle = (active: boolean): React.CSSProperties => ({
    flex: 1,
    padding: '9px 12px',
    background: active ? 'rgba(170,59,255,0.15)' : 'transparent',
    borderBottom: `2px solid ${active ? 'var(--color-accent-purple)' : 'transparent'}`,
    color: active ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
    fontSize: '0.82rem',
    fontWeight: 600,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '6px',
    cursor: 'pointer',
    transition: 'all 0.15s',
    border: 'none',
    borderBottomStyle: 'solid',
    borderBottomWidth: '2px',
    borderBottomColor: active ? 'var(--color-accent-purple)' : 'transparent',
    background2: active ? 'rgba(170,59,255,0.15)' : 'transparent',
  });

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Media Library" maxWidth="560px">
      {/* Tab Bar */}
      <div
        style={{
          display: 'flex',
          borderBottom: '1px solid var(--color-border-glass)',
          marginBottom: '16px',
          background: 'rgba(5,7,10,0.4)',
          borderRadius: '8px 8px 0 0',
          overflow: 'hidden',
        }}
      >
        <button
          onClick={() => setTab('library')}
          style={{
            flex: 1,
            padding: '10px',
            background: tab === 'library' ? 'rgba(170,59,255,0.15)' : 'transparent',
            borderBottom: `2px solid ${tab === 'library' ? 'var(--color-accent-purple)' : 'transparent'}`,
            color: tab === 'library' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
            fontSize: '0.82rem',
            fontWeight: 600,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '6px',
            cursor: 'pointer',
            border: 'none',
          }}
        >
          <Layers size={14} />
          Uploaded Library
          {assets.length > 0 && (
            <span
              style={{
                padding: '1px 6px',
                borderRadius: '8px',
                background: 'rgba(170,59,255,0.2)',
                color: 'var(--color-accent-purple)',
                fontSize: '0.7rem',
                fontWeight: 700,
              }}
            >
              {assets.length}
            </span>
          )}
        </button>
        <button
          onClick={() => setTab('url')}
          style={{
            flex: 1,
            padding: '10px',
            background: tab === 'url' ? 'rgba(170,59,255,0.15)' : 'transparent',
            borderBottom: `2px solid ${tab === 'url' ? 'var(--color-accent-purple)' : 'transparent'}`,
            color: tab === 'url' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
            fontSize: '0.82rem',
            fontWeight: 600,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '6px',
            cursor: 'pointer',
            border: 'none',
          }}
        >
          <Link2 size={14} />
          Custom URL
        </button>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', minHeight: '320px' }}>
        {tab === 'library' ? (
          <>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.07em' }}>
                Transcoded Assets (ABR Ready)
              </span>
              <button
                onClick={refresh}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  color: 'var(--color-text-secondary)',
                  fontSize: '0.75rem',
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  padding: '4px 8px',
                  borderRadius: '6px',
                  transition: 'all 0.15s',
                }}
              >
                <RefreshCw size={13} />
                Refresh
              </button>
            </div>

            {isLoading ? (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 1, gap: '12px', height: '200px' }}>
                <Spinner size={24} color="var(--color-accent-purple)" />
                <span style={{ color: 'var(--color-text-secondary)', fontSize: '0.85rem' }}>Loading library...</span>
              </div>
            ) : error ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', flex: 1, gap: '10px', height: '200px', textAlign: 'center' }}>
                <AlertCircle size={28} color="var(--color-accent-rose)" />
                <span style={{ color: 'var(--color-text-secondary)', fontSize: '0.85rem' }}>{error}</span>
                <Button variant="ghost" size="sm" onClick={refresh}>Retry</Button>
              </div>
            ) : assets.length === 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', flex: 1, gap: '10px', height: '200px', textAlign: 'center', opacity: 0.6 }}>
                <Film size={32} color="var(--color-text-muted)" />
                <span style={{ color: 'var(--color-text-secondary)', fontSize: '0.88rem' }}>
                  No ready media in library yet. Upload and transcode assets via the admin portal.
                </span>
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '300px', overflowY: 'auto', paddingRight: '2px' }}>
                {assets.map((asset) => {
                  const playUrl = getPlayUrl(asset);
                  const isSelected = selected === playUrl;
                  const qualityBadge = getRenditionBadge(asset);
                  const hasHLS = !!asset.hls_master_url;

                  return (
                    <div
                      key={asset.id}
                      onClick={() => handleSelectAsset(asset)}
                      style={{
                        padding: '10px 12px',
                        borderRadius: '8px',
                        background: isSelected ? 'rgba(170,59,255,0.12)' : 'var(--color-bg-surface)',
                        border: `1px solid ${isSelected ? 'var(--color-accent-purple)' : 'var(--color-border-glass)'}`,
                        display: 'flex',
                        alignItems: 'center',
                        gap: '10px',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                      onMouseEnter={(e) => {
                        if (!isSelected) {
                          e.currentTarget.style.borderColor = 'var(--color-border-hover)';
                          e.currentTarget.style.background = 'var(--color-bg-surface-hover)';
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (!isSelected) {
                          e.currentTarget.style.borderColor = 'var(--color-border-glass)';
                          e.currentTarget.style.background = 'var(--color-bg-surface)';
                        }
                      }}
                    >
                      {/* Thumbnail / Icon */}
                      <div
                        style={{
                          width: '40px',
                          height: '40px',
                          borderRadius: '6px',
                          background: isSelected ? 'rgba(170,59,255,0.3)' : 'var(--color-bg-surface-hover)',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          flexShrink: 0,
                          overflow: 'hidden',
                        }}
                      >
                        {asset.thumbnail_url ? (
                          <img
                            src={asset.thumbnail_url}
                            alt={asset.title}
                            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                          />
                        ) : (
                          <Film size={18} color={isSelected ? 'var(--color-accent-purple)' : 'var(--color-text-muted)'} />
                        )}
                      </div>

                      {/* Info */}
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                          <span style={{ fontWeight: 600, fontSize: '0.88rem', color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '200px' }}>
                            {asset.title}
                          </span>
                          {hasHLS && (
                            <span style={{ padding: '1px 5px', borderRadius: '4px', background: 'rgba(16,185,129,0.15)', color: 'var(--color-accent-emerald)', fontSize: '0.66rem', fontWeight: 700, flexShrink: 0 }}>
                              HLS·ABR
                            </span>
                          )}
                          {qualityBadge && (
                            <span style={{ padding: '1px 5px', borderRadius: '4px', background: 'rgba(170,59,255,0.15)', color: 'var(--color-accent-purple)', fontSize: '0.66rem', fontWeight: 700, flexShrink: 0 }}>
                              {qualityBadge}
                            </span>
                          )}
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '2px' }}>
                          {asset.duration_seconds > 0 && (
                            <span style={{ fontSize: '0.72rem', color: 'var(--color-text-muted)', display: 'flex', alignItems: 'center', gap: '3px' }}>
                              <Clock size={10} />
                              {formatDuration(asset.duration_seconds)}
                            </span>
                          )}
                          {asset.renditions && asset.renditions.length > 0 && (
                            <span style={{ fontSize: '0.72rem', color: 'var(--color-text-muted)' }}>
                              {asset.renditions.length} renditions
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Selected Check or Play Icon */}
                      <div style={{ flexShrink: 0 }}>
                        {isSelected ? (
                          <div style={{ width: '22px', height: '22px', borderRadius: '50%', background: 'var(--color-accent-purple)', color: '#FFF', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                            <Check size={13} />
                          </div>
                        ) : (
                          <Play size={16} color="var(--color-text-muted)" />
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.07em' }}>
              Custom Video URL (MP4 / WebM / HLS .m3u8)
            </span>
            <Input
              type="url"
              placeholder="https://example.com/video.mp4 or playlist.m3u8"
              value={customUrl}
              onChange={(e) => setCustomUrl(e.target.value)}
              icon={<Link2 size={16} />}
            />
            <p style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', lineHeight: 1.5 }}>
              Paste any direct video URL. HLS manifests (.m3u8) will use Hls.js for adaptive bitrate streaming.
            </p>
          </div>
        )}
      </div>

      {/* Action Bar */}
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '16px', paddingTop: '14px', borderTop: '1px solid var(--color-border-glass)' }}>
        <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
        <Button
          type="button"
          variant="primary"
          disabled={tab === 'library' ? !selected : !customUrl.trim()}
          onClick={handleConfirm}
        >
          Load Stream
        </Button>
      </div>
    </Modal>
  );
};
