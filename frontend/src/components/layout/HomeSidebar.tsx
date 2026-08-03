import React, { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useRoom } from '../../hooks/useRoom';
import { CreateRoomModal } from '../room/CreateRoomModal';
import { Badge } from '../common/Badge';
import {
  Plus,
  Lock,
  Globe,
  Bell,
  Check,
  X,
  Tv,
  Users,
} from 'lucide-react';
import styles from './HomeSidebar.module.css';

export const HomeSidebar: React.FC = () => {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const {
    rooms,
    activeRoom,
    invitations,
    acceptInvitation,
    declineInvitation,
    isLoadingRoom,
  } = useRoom();
  const navigate = useNavigate();

  return (
    <>
      <aside className={styles.sidebar} aria-label="Rooms & Invitations">
        {/* Header */}
        <div className={styles.header}>
          <span className={styles.headerTitle}>Rooms</span>
          <button
            className={styles.addButton}
            onClick={() => setIsCreateOpen(true)}
            title="Create Room"
            aria-label="Create Room"
          >
            <Plus size={16} />
          </button>
        </div>

        {/* Pending Invitations */}
        {invitations.length > 0 && (
          <div className={`${styles.section} ${styles.inviteSection}`}>
            <div className={styles.sectionLabel}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Bell size={12} />
                Invites
              </span>
              <Badge variant="danger">{invitations.length}</Badge>
            </div>

            {invitations.map((inv) => (
              <div
                key={inv.id}
                style={{
                  padding: '8px var(--space-2)',
                  borderRadius: 'var(--radius-lg)',
                  background: 'var(--color-surface-2)',
                  border: '1px solid var(--color-border-default)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 'var(--space-2)',
                }}
              >
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <span style={{ fontSize: 'var(--text-compact)', fontWeight: 600, color: 'var(--color-text-primary)' }}>
                    {inv.room_name || 'Private Room'}
                  </span>
                  <span style={{ fontSize: 'var(--text-label)', color: 'var(--color-text-muted)' }}>
                    from @{inv.inviter_name || 'someone'}
                  </span>
                </div>
                <div style={{ display: 'flex', gap: 'var(--space-1)' }}>
                  <button
                    onClick={async () => {
                      try {
                        const joined = await acceptInvitation(inv.id);
                        navigate(`/room/${joined.id}`);
                      } catch { /* ignore */ }
                    }}
                    style={{
                      flex: 1,
                      padding: '4px',
                      borderRadius: 'var(--radius-md)',
                      background: 'var(--color-success-subtle)',
                      color: 'var(--color-success)',
                      border: '1px solid var(--color-success-border)',
                      fontSize: 'var(--text-label)',
                      fontWeight: 600,
                      cursor: 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: 4,
                      transition: 'background-color var(--transition-fast)',
                    }}
                  >
                    <Check size={11} /> Accept
                  </button>
                  <button
                    onClick={() => declineInvitation(inv.id)}
                    style={{
                      flex: 1,
                      padding: '4px',
                      borderRadius: 'var(--radius-md)',
                      background: 'transparent',
                      color: 'var(--color-text-muted)',
                      border: '1px solid var(--color-border-default)',
                      fontSize: 'var(--text-label)',
                      fontWeight: 600,
                      cursor: 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: 4,
                    }}
                  >
                    <X size={11} /> Decline
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Room List */}
        <div className={styles.scrollList}>
          <div className={styles.sectionLabel}>
            <span>Active Rooms ({rooms.length})</span>
          </div>

          {rooms.length === 0 && !isLoadingRoom && (
            <div className={styles.emptyState}>
              No rooms are live.
              <br />
              <button
                onClick={() => setIsCreateOpen(true)}
                style={{
                  marginTop: 'var(--space-2)',
                  color: 'var(--color-accent)',
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  fontSize: 'var(--text-compact)',
                  fontWeight: 600,
                }}
              >
                Create one →
              </button>
            </div>
          )}

          {rooms.map((room) => {
            const isActive = activeRoom?.id === room.id;
            const memberCount = room.members?.length ?? 0;

            return (
              <NavLink
                key={room.id}
                to={`/room/${room.id}`}
                title={room.name}
                style={({ isActive: navActive }) => ({
                  display: 'flex',
                  alignItems: 'center',
                  gap: 'var(--space-2)',
                  padding: '6px var(--space-2)',
                  borderRadius: 'var(--radius-lg)',
                  textDecoration: 'none',
                  color: navActive || isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                  background: navActive || isActive ? 'var(--color-surface-2)' : 'transparent',
                  fontWeight: navActive || isActive ? 600 : 400,
                  fontSize: 'var(--text-compact)',
                  transition: 'background-color var(--transition-fast), color var(--transition-fast)',
                })}
              >
                <Tv
                  size={14}
                  style={{
                    color: isActive ? 'var(--color-accent)' : 'var(--color-text-muted)',
                    flexShrink: 0,
                  }}
                />
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {room.name}
                </span>
                {memberCount > 0 && (
                  <span style={{ display: 'flex', alignItems: 'center', gap: 2, color: 'var(--color-text-muted)', fontSize: 'var(--text-label)', flexShrink: 0 }}>
                    <Users size={10} />
                    {memberCount}
                  </span>
                )}
                {room.is_private ? (
                  <Lock size={11} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} />
                ) : (
                  <Globe size={11} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} />
                )}
              </NavLink>
            );
          })}
        </div>
      </aside>

      <CreateRoomModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} />
    </>
  );
};
