import React, { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { useRoom } from '../../hooks/useRoom';
import { Button } from '../common/Button';
import { CreateRoomModal } from '../room/CreateRoomModal';
import styles from './Sidebar.module.css';
import { Tv, Plus, LogOut, Compass, Lock, Globe, Sparkles, Bell, Check, X, PanelLeftClose, PanelLeftOpen } from 'lucide-react';

export const Sidebar: React.FC = () => {
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isCollapsed, setIsCollapsed] = useState(false);
  const { user, logout, isLoading: isAuthLoading } = useAuth();
  const { rooms, activeRoom, leaveRoom, invitations, acceptInvitation, declineInvitation } = useRoom();
  const navigate = useNavigate();

  const handleLeaveCurrentRoom = async () => {
    await leaveRoom();
    navigate('/');
  };

  return (
    <>
      <aside className={`glass-panel ${styles.sidebar} ${isCollapsed ? styles.sidebarCollapsed : ''}`}>
        {/* Brand Header */}
        <div className={styles.brandHeader}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', overflow: 'hidden' }}>
            <div className={styles.brandLogo}>
              <Sparkles size={20} />
            </div>
            {!isCollapsed && <span className={styles.brandText}>Inox</span>}
          </div>
          <button
            onClick={() => setIsCollapsed(!isCollapsed)}
            className={styles.collapseButton}
            aria-label={isCollapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
            title={isCollapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
          >
            {isCollapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
          </button>
        </div>

        {/* Navigation */}
        <div className={styles.navSection}>
          <NavLink
            to="/"
            end
            className={({ isActive }) => `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`}
            title="Room Lobby"
          >
            <Compass size={18} color="var(--color-accent-cyan)" style={{ flexShrink: 0 }} />
            {!isCollapsed && <span>Room Lobby</span>}
          </NavLink>
        </div>

        {/* Pending Invites Section */}
        {invitations.length > 0 && (
          <div style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: '8px', borderBottom: '1px solid var(--color-border-glass)', background: 'rgba(168, 85, 247, 0.08)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: '0.75rem', fontWeight: 700, color: 'var(--color-accent-purple)', textTransform: 'uppercase', letterSpacing: '0.08em', display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Bell size={14} /> Pending Invites ({invitations.length})
              </span>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '180px', overflowY: 'auto' }}>
              {invitations.map((inv) => (
                <div key={inv.id} style={{ padding: '8px 10px', borderRadius: '8px', background: 'var(--color-bg-surface)', border: '1px solid var(--color-border-glass)', display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>{inv.room_name || 'Private Room'}</span>
                    <span style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)' }}>from @{inv.inviter_name || 'user'}</span>
                  </div>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <button
                      onClick={async () => {
                        try {
                          const joined = await acceptInvitation(inv.id);
                          navigate(`/room/${joined.id}`);
                        } catch (err) {
                          // handled in provider
                        }
                      }}
                      style={{ flex: 1, padding: '4px', borderRadius: '6px', background: 'var(--color-accent-emerald)', color: '#FFF', border: 'none', fontSize: '0.75rem', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}
                    >
                      <Check size={12} /> Accept
                    </button>
                    <button
                      onClick={() => declineInvitation(inv.id)}
                      style={{ flex: 1, padding: '4px', borderRadius: '6px', background: 'var(--color-bg-surface-hover)', color: 'var(--color-text-secondary)', border: '1px solid var(--color-border-glass)', fontSize: '0.75rem', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}
                    >
                      <X size={12} /> Decline
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Room List Section */}
        <div className={styles.roomListSection}>
          <div className={styles.sectionHeader}>
            {!isCollapsed && <span>Available Rooms ({rooms.length})</span>}
            <button
              onClick={() => setIsCreateModalOpen(true)}
              style={{
                color: 'var(--color-accent-purple)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: '4px',
                borderRadius: '6px',
                transition: 'all var(--transition-fast)',
                background: 'none',
                border: 'none',
                cursor: 'pointer',
              }}
              title="Create Room"
            >
              <Plus size={18} />
            </button>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            {rooms.length === 0 && !isCollapsed && (
              <div style={{ padding: '20px 10px', textAlign: 'center', color: 'var(--color-text-muted)', fontSize: '0.85rem' }}>
                No active rooms found. Start a new watch party!
              </div>
            )}

            {rooms.map((room) => {
              const isCurrentRoom = activeRoom?.id === room.id;
              return (
                <NavLink
                  key={room.id}
                  to={`/room/${room.id}`}
                  className={({ isActive }) => `${styles.navLink} ${isActive || isCurrentRoom ? styles.navLinkActive : ''}`}
                  title={room.name}
                >
                  <Tv size={16} color={isCurrentRoom ? 'var(--color-accent-purple)' : 'var(--color-text-muted)'} style={{ flexShrink: 0 }} />
                  {!isCollapsed && (
                    <>
                      <span style={{ flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {room.name}
                      </span>
                      {room.is_private ? <Lock size={14} color="var(--color-text-muted)" style={{ flexShrink: 0 }} /> : <Globe size={14} color="var(--color-text-muted)" style={{ flexShrink: 0 }} />}
                    </>
                  )}
                </NavLink>
              );
            })}
          </div>
        </div>

        {/* Active Room Indicator Footer */}
        {activeRoom && !isCollapsed && (
          <div style={{ padding: '12px 16px', background: 'rgba(170, 59, 255, 0.1)', borderTop: '1px solid var(--color-border-glass)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <span style={{ fontSize: '0.75rem', color: 'var(--color-accent-purple)', fontWeight: 600 }}>ACTIVE ROOM</span>
              <span style={{ fontSize: '0.85rem', color: 'var(--color-text-primary)', fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {activeRoom.name}
              </span>
            </div>
            <Button variant="danger" size="sm" onClick={handleLeaveCurrentRoom}>
              Leave
            </Button>
          </div>
        )}

        {/* User Profile Footer */}
        <div className={styles.userFooter}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', overflow: 'hidden' }}>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '50%',
                background: 'var(--color-bg-surface-hover)',
                border: '1px solid var(--color-border-hover)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontWeight: 700,
                fontSize: '0.9rem',
                color: 'var(--color-accent-cyan)',
                flexShrink: 0,
              }}
            >
              {user?.username?.[0]?.toUpperCase() || 'U'}
            </div>
            {!isCollapsed && (
              <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
                <span style={{ fontWeight: 600, fontSize: '0.9rem', color: 'var(--color-text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {user?.username}
                </span>
                <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {user?.email}
                </span>
              </div>
            )}
          </div>

          <button
            onClick={logout}
            disabled={isAuthLoading}
            style={{
              padding: '8px',
              borderRadius: '8px',
              color: 'var(--color-text-secondary)',
              transition: 'all var(--transition-fast)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = 'var(--color-bg-surface-hover)';
              e.currentTarget.style.color = 'var(--color-accent-rose)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = 'transparent';
              e.currentTarget.style.color = 'var(--color-text-secondary)';
            }}
            title="Sign Out"
          >
            <LogOut size={18} />
          </button>
        </div>
      </aside>

      <CreateRoomModal isOpen={isCreateModalOpen} onClose={() => setIsCreateModalOpen(false)} />
    </>
  );
};
