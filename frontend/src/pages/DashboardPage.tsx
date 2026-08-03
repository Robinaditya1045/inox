import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { useRoom } from '../hooks/useRoom';
import { Button } from '../components/common/Button';
import { Badge } from '../components/common/Badge';
import { CreateRoomModal } from '../components/room/CreateRoomModal';
import { Spinner } from '../components/common/Spinner';
import {
  Tv,
  Plus,
  Check,
  X,
  Bell,
  Globe,
  Lock,
  Users,
} from 'lucide-react';

export const DashboardPage: React.FC = () => {
  const { user } = useAuth();
  const { rooms, invitations, acceptInvitation, declineInvitation, isLoadingRoom } = useRoom();
  const navigate = useNavigate();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  return (
    <>
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
          overflow: 'auto',
          padding: 'var(--space-8)',
          gap: 'var(--space-8)',
        }}
      >
        {/* Page heading */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <h1
              style={{
                fontSize: 'var(--text-heading)',
                fontWeight: 700,
                color: 'var(--color-text-primary)',
              }}
            >
              Activity
            </h1>
            <p style={{ fontSize: 'var(--text-compact)', color: 'var(--color-text-muted)', marginTop: 2 }}>
              Hello, {user?.username}
            </p>
          </div>
          <Button
            variant="primary"
            size="sm"
            icon={<Plus size={14} />}
            onClick={() => setIsCreateOpen(true)}
          >
            New Room
          </Button>
        </div>

        {/* Pending Invitations */}
        {invitations.length > 0 && (
          <section>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 'var(--space-2)',
                marginBottom: 'var(--space-3)',
              }}
            >
              <Bell size={14} style={{ color: 'var(--color-text-muted)' }} />
              <h2
                style={{
                  fontSize: 'var(--text-label)',
                  fontWeight: 700,
                  color: 'var(--color-text-muted)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.07em',
                }}
              >
                Pending Invitations
              </h2>
              <Badge variant="danger">{invitations.length}</Badge>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
              {invitations.map((inv) => (
                <div
                  key={inv.id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: 'var(--space-3) var(--space-4)',
                    borderRadius: 'var(--radius-lg)',
                    background: 'var(--color-surface-1)',
                    border: '1px solid var(--color-border-default)',
                    gap: 'var(--space-4)',
                  }}
                >
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2, overflow: 'hidden' }}>
                    <span style={{ fontSize: 'var(--text-compact)', fontWeight: 600, color: 'var(--color-text-primary)' }}>
                      {inv.room_name || 'Private Room'}
                    </span>
                    <span style={{ fontSize: 'var(--text-meta)', color: 'var(--color-text-muted)' }}>
                      Invited by @{inv.inviter_name || 'someone'}
                    </span>
                  </div>

                  <div style={{ display: 'flex', gap: 'var(--space-2)', flexShrink: 0 }}>
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={<X size={12} />}
                      onClick={() => declineInvitation(inv.id)}
                    >
                      Decline
                    </Button>
                    <Button
                      variant="primary"
                      size="sm"
                      icon={<Check size={12} />}
                      onClick={async () => {
                        try {
                          const joined = await acceptInvitation(inv.id);
                          navigate(`/room/${joined.id}`);
                        } catch { /* ignore */ }
                      }}
                    >
                      Accept
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* Active Rooms */}
        <section style={{ flex: 1 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-2)',
              marginBottom: 'var(--space-3)',
            }}
          >
            <Tv size={14} style={{ color: 'var(--color-text-muted)' }} />
            <h2
              style={{
                fontSize: 'var(--text-label)',
                fontWeight: 700,
                color: 'var(--color-text-muted)',
                textTransform: 'uppercase',
                letterSpacing: '0.07em',
              }}
            >
              Active Rooms
            </h2>
          </div>

          {isLoadingRoom ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)', padding: 'var(--space-4)' }}>
              <Spinner size={16} />
              <span style={{ fontSize: 'var(--text-compact)', color: 'var(--color-text-muted)' }}>
                Loading rooms...
              </span>
            </div>
          ) : rooms.length === 0 ? (
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 'var(--space-4)',
                padding: 'var(--space-12) var(--space-8)',
                textAlign: 'center',
                borderRadius: 'var(--radius-xl)',
                border: '1px dashed var(--color-border-default)',
                background: 'var(--color-surface-1)',
              }}
            >
              <Tv size={28} style={{ color: 'var(--color-text-muted)', opacity: 0.5 }} />
              <div>
                <p style={{ fontSize: 'var(--text-compact)', fontWeight: 600, color: 'var(--color-text-secondary)' }}>
                  No rooms are active right now
                </p>
                <p style={{ fontSize: 'var(--text-meta)', color: 'var(--color-text-muted)', marginTop: 4 }}>
                  Start a watch party and invite your friends
                </p>
              </div>
              <Button
                variant="primary"
                size="sm"
                icon={<Plus size={14} />}
                onClick={() => setIsCreateOpen(true)}
              >
                Create Room
              </Button>
            </div>
          ) : (
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
                gap: 'var(--space-2)',
              }}
            >
              {rooms.map((room) => {
                const memberCount = room.members?.length ?? 0;
                return (
                  <div
                    key={room.id}
                    onClick={() => navigate(`/room/${room.id}`)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => e.key === 'Enter' && navigate(`/room/${room.id}`)}
                    aria-label={`Join ${room.name}`}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 'var(--space-3)',
                      padding: 'var(--space-3) var(--space-4)',
                      borderRadius: 'var(--radius-lg)',
                      background: 'var(--color-surface-1)',
                      border: '1px solid var(--color-border-default)',
                      cursor: 'pointer',
                      transition: 'background-color var(--transition-fast), border-color var(--transition-fast)',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = 'var(--color-surface-2)';
                      e.currentTarget.style.borderColor = 'var(--color-border-strong)';
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'var(--color-surface-1)';
                      e.currentTarget.style.borderColor = 'var(--color-border-default)';
                    }}
                  >
                    {/* Live dot */}
                    <div
                      style={{
                        width: 8,
                        height: 8,
                        borderRadius: 'var(--radius-full)',
                        background: memberCount > 0 ? 'var(--color-live)' : 'var(--color-offline)',
                        flexShrink: 0,
                        boxShadow: memberCount > 0 ? '0 0 6px var(--color-live)' : 'none',
                      }}
                      aria-hidden="true"
                    />

                    {/* Name */}
                    <span
                      style={{
                        flex: 1,
                        fontSize: 'var(--text-compact)',
                        fontWeight: 600,
                        color: 'var(--color-text-primary)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {room.name}
                    </span>

                    {/* Member count */}
                    {memberCount > 0 && (
                      <span
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 3,
                          fontSize: 'var(--text-meta)',
                          color: 'var(--color-text-muted)',
                          flexShrink: 0,
                        }}
                      >
                        <Users size={11} />
                        {memberCount}
                      </span>
                    )}

                    {/* Privacy */}
                    {room.is_private ? (
                      <Lock size={12} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} />
                    ) : (
                      <Globe size={12} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} />
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </section>
      </div>

      <CreateRoomModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} />
    </>
  );
};
