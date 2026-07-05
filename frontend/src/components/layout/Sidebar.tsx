import React, { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { useRoom } from '../../hooks/useRoom';
import { Button } from '../common/Button';
import { CreateRoomModal } from '../room/CreateRoomModal';
import { Tv, Plus, LogOut, Compass, Lock, Globe, Sparkles, Bell, Check, X } from 'lucide-react';

export const Sidebar: React.FC = () => {
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const { user, logout, isLoading: isAuthLoading } = useAuth();
  const { rooms, activeRoom, leaveRoom, invitations, acceptInvitation, declineInvitation } = useRoom();
  const navigate = useNavigate();

  const handleLeaveCurrentRoom = async () => {
    await leaveRoom();
    navigate('/');
  };

  return (
    <>
      <aside
        className="glass-panel"
        style={{
          width: 'var(--sidebar-left-width)',
          height: '100vh',
          display: 'flex',
          flexDirection: 'column',
          borderRight: '1px solid var(--color-border-glass)',
          flexShrink: 0,
          position: 'sticky',
          top: 0,
        }}
      >
        {/* Brand Header */}
        <div
          style={{
            height: 'var(--header-height)',
            padding: '0 20px',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
            borderBottom: '1px solid var(--color-border-glass)',
          }}
        >
          <div
            style={{
              width: '36px',
              height: '36px',
              borderRadius: '10px',
              background: 'linear-gradient(135deg, var(--color-accent-purple), var(--color-accent-cyan))',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#FFF',
              boxShadow: '0 0 15px rgba(170, 59, 255, 0.4)',
            }}
          >
            <Sparkles size={20} />
          </div>
          <span style={{ fontFamily: 'var(--font-display)', fontSize: '1.4rem', fontWeight: 700, color: 'var(--color-text-primary)' }}>
            Inox
          </span>
        </div>

        {/* Navigation */}
        <div style={{ padding: '20px 16px', display: 'flex', flexDirection: 'column', gap: '8px', borderBottom: '1px solid var(--color-border-glass)' }}>
          <NavLink
            to="/"
            end
            style={({ isActive }) => ({
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '10px 14px',
              borderRadius: '8px',
              color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
              background: isActive ? 'var(--color-bg-surface-hover)' : 'transparent',
              border: `1px solid ${isActive ? 'var(--color-border-hover)' : 'transparent'}`,
              fontWeight: isActive ? 600 : 500,
              fontSize: '0.95rem',
              transition: 'all var(--transition-fast)',
              textDecoration: 'none',
            })}
          >
            <Compass size={18} color="var(--color-accent-cyan)" />
            <span>Room Lobby</span>
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
        <div style={{ flex: 1, padding: '20px 16px', display: 'flex', flexDirection: 'column', gap: '12px', overflowY: 'auto' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 4px' }}>
            <span style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
              Available Rooms ({rooms.length})
            </span>
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
              }}
              title="Create Room"
            >
              <Plus size={18} />
            </button>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            {rooms.length === 0 && (
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
                  style={({ isActive }) => ({
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '10px 12px',
                    borderRadius: '8px',
                    color: isActive || isCurrentRoom ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                    background: isActive || isCurrentRoom ? 'rgba(170, 59, 255, 0.15)' : 'transparent',
                    border: `1px solid ${isActive || isCurrentRoom ? 'var(--color-accent-border)' : 'transparent'}`,
                    fontWeight: 500,
                    fontSize: '0.9rem',
                    transition: 'all var(--transition-fast)',
                    textDecoration: 'none',
                  })}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', overflow: 'hidden' }}>
                    <Tv size={16} color={isCurrentRoom ? 'var(--color-accent-purple)' : 'var(--color-text-muted)'} />
                    <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {room.name}
                    </span>
                  </div>
                  {room.is_private ? <Lock size={14} color="var(--color-text-muted)" /> : <Globe size={14} color="var(--color-text-muted)" />}
                </NavLink>
              );
            })}
          </div>
        </div>

        {/* Active Room Indicator Footer */}
        {activeRoom && (
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
        <div
          style={{
            padding: '16px',
            borderTop: '1px solid var(--color-border-glass)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            background: 'var(--color-bg-surface)',
          }}
        >
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
            <div style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <span style={{ fontWeight: 600, fontSize: '0.9rem', color: 'var(--color-text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {user?.username}
              </span>
              <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {user?.email}
              </span>
            </div>
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
