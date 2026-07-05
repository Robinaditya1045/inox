import React, { useState } from 'react';
import { useAuth } from '../hooks/useAuth';
import { useRoom } from '../hooks/useRoom';
import { Button } from '../components/common/Button';
import { CreateRoomModal } from '../components/room/CreateRoomModal';
import { Plus, Users, Tv, Radio, Lock, Globe, ArrowRight } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

export const DashboardPage: React.FC = () => {
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const { user } = useAuth();
  const { rooms, invitations, acceptInvitation, declineInvitation, isLoadingRoom } = useRoom();
  const navigate = useNavigate();

  return (
    <main style={{ padding: '40px', maxWidth: '1200px', width: '100%', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '36px' }}>
      {/* Welcome Banner */}
      <div
        className="glass-panel"
        style={{
          padding: '40px',
          borderRadius: '20px',
          background: 'radial-gradient(circle at 90% 20%, rgba(170, 59, 255, 0.2) 0%, rgba(17, 22, 34, 0.8) 70%)',
          border: '1px solid var(--color-border-hover)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '24px',
          flexWrap: 'wrap',
          boxShadow: '0 10px 30px rgba(0, 0, 0, 0.5), 0 0 30px rgba(170, 59, 255, 0.1)',
        }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', maxWidth: '620px' }}>
          <h1 style={{ fontFamily: 'var(--font-display)', fontSize: '2.4rem', fontWeight: 700, lineHeight: 1.2 }}>
            Welcome to <span style={{ color: 'var(--color-accent-purple)' }}>Inox</span>, {user?.username}!
          </h1>
          <p style={{ color: 'var(--color-text-secondary)', fontSize: '1.05rem', lineHeight: 1.6 }}>
            The high-concurrency real-time watch party and voice platform. Select an active room from the sidebar, or create a new synchronized room to begin.
          </p>
        </div>

        <Button variant="primary" size="lg" icon={<Plus size={20} />} onClick={() => setIsCreateModalOpen(true)}>
          Create Watch Room
        </Button>
      </div>

      {/* Pending Invitations Section */}
      {invitations.length > 0 && (
        <section style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.4rem', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '10px', color: 'var(--color-accent-purple)' }}>
              <Lock size={20} />
              <span>Pending Private Room Invites</span>
              <span style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', fontWeight: 400 }}>({invitations.length})</span>
            </h2>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '16px' }}>
            {invitations.map((inv) => (
              <div
                key={inv.id}
                className="glass-panel"
                style={{
                  padding: '20px',
                  borderRadius: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '14px',
                  background: 'rgba(168, 85, 247, 0.1)',
                  border: '1px solid rgba(168, 85, 247, 0.3)',
                  boxShadow: '0 8px 24px rgba(168, 85, 247, 0.15)',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <span style={{ fontSize: '1.1rem', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                      {inv.room_name || 'Private Watch Room'}
                    </span>
                    <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
                      Invited by <strong style={{ color: 'var(--color-accent-cyan)' }}>@{inv.inviter_name || 'Member'}</strong>
                    </span>
                  </div>
                  <div style={{ padding: '4px 8px', borderRadius: '6px', background: 'rgba(168, 85, 247, 0.2)', color: 'var(--color-accent-purple)', fontSize: '0.75rem', fontWeight: 700 }}>
                    PRIVATE
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '10px', marginTop: '4px' }}>
                  <Button
                    variant="primary"
                    size="sm"
                    style={{ flex: 1, background: 'var(--color-accent-emerald)', borderColor: 'var(--color-accent-emerald)' }}
                    onClick={async () => {
                      try {
                        const joined = await acceptInvitation(inv.id);
                        navigate(`/room/${joined.id}`);
                      } catch (err) {
                        // handled in provider
                      }
                    }}
                  >
                    Accept & Join
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    style={{ flex: 1 }}
                    onClick={() => declineInvitation(inv.id)}
                  >
                    Decline
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Active Watch Rooms Section */}
      <section style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.5rem', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '10px' }}>
            <Tv size={22} color="var(--color-accent-cyan)" />
            <span>Active Rooms</span>
            <span style={{ fontSize: '0.9rem', color: 'var(--color-text-muted)', fontWeight: 400 }}>({rooms.length})</span>
          </h2>
        </div>

        {rooms.length === 0 ? (
          <div
            className="glass-panel"
            style={{
              padding: '48px 24px',
              borderRadius: '16px',
              textAlign: 'center',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: '16px',
              border: '1px dashed var(--color-border-hover)',
            }}
          >
            <div style={{ width: '56px', height: '56px', borderRadius: '50%', background: 'var(--color-bg-surface)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-text-muted)' }}>
              <Tv size={28} />
            </div>
            <div>
              <h3 style={{ fontSize: '1.2rem', fontWeight: 600 }}>No Watch Parties Live Right Now</h3>
              <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.95rem', marginTop: '4px' }}>
                Be the first to launch a room and invite your friends to watch together!
              </p>
            </div>
            <Button variant="secondary" size="md" icon={<Plus size={18} />} onClick={() => setIsCreateModalOpen(true)}>
              Start New Room
            </Button>
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: '20px' }}>
            {rooms.map((room) => (
              <div
                key={room.id}
                className="glass-panel"
                style={{
                  padding: '24px',
                  borderRadius: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '16px',
                  transition: 'all var(--transition-normal)',
                  cursor: 'pointer',
                  border: '1px solid var(--color-border-glass)',
                }}
                onClick={() => navigate(`/room/${room.id}`)}
                onMouseEnter={(e) => {
                  e.currentTarget.style.transform = 'translateY(-3px)';
                  e.currentTarget.style.borderColor = 'var(--color-accent-border)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.transform = 'translateY(0)';
                  e.currentTarget.style.borderColor = 'var(--color-border-glass)';
                }}
              >
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '12px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <div style={{ width: '44px', height: '44px', borderRadius: '12px', background: 'rgba(170, 59, 255, 0.15)', color: 'var(--color-accent-purple)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <Tv size={22} />
                    </div>
                    <div>
                      <h3 style={{ fontSize: '1.15rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>{room.name}</h3>
                      <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>ID: {room.id.slice(0, 8)}...</span>
                    </div>
                  </div>

                  <div style={{ padding: '4px 8px', borderRadius: '6px', background: 'var(--color-bg-surface)', border: '1px solid var(--color-border-glass)', display: 'flex', alignItems: 'center', gap: '4px', fontSize: '0.75rem', color: 'var(--color-text-secondary)' }}>
                    {room.is_private ? <Lock size={12} color="var(--color-accent-rose)" /> : <Globe size={12} color="var(--color-accent-cyan)" />}
                    <span>{room.is_private ? 'Private' : 'Public'}</span>
                  </div>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--color-border-glass)', paddingTop: '14px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
                    <Users size={16} />
                    <span>{room.members?.length || 1} watching</span>
                  </div>

                  <Button variant="ghost" size="sm" icon={<ArrowRight size={16} />} isLoading={isLoadingRoom}>
                    Join Room
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Feature Architecture Overview */}
      <section style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '24px' }}>
        <div className="glass-panel" style={{ padding: '24px', borderRadius: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ width: '44px', height: '44px', borderRadius: '12px', background: 'rgba(170, 59, 255, 0.15)', color: 'var(--color-accent-purple)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Tv size={24} />
          </div>
          <h3 style={{ fontSize: '1.2rem', fontWeight: 600 }}>Synchronized Watch Parties</h3>
          <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.95rem', lineHeight: 1.5 }}>
            16:9 media playback synced down to the millisecond across all members using WebSocket play/pause/seek events.
          </p>
        </div>

        <div className="glass-panel" style={{ padding: '24px', borderRadius: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ width: '44px', height: '44px', borderRadius: '12px', background: 'rgba(0, 240, 255, 0.15)', color: 'var(--color-accent-cyan)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Radio size={24} />
          </div>
          <h3 style={{ fontSize: '1.2rem', fontWeight: 600 }}>Pion SFU Voice Chat</h3>
          <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.95rem', lineHeight: 1.5 }}>
            Low-latency WebRTC audio and screen sharing powered by our Go Pion Selective Forwarding Unit.
          </p>
        </div>

        <div className="glass-panel" style={{ padding: '24px', borderRadius: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ width: '44px', height: '44px', borderRadius: '12px', background: 'rgba(16, 185, 129, 0.15)', color: 'var(--color-accent-emerald)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Users size={24} />
          </div>
          <h3 style={{ fontSize: '1.2rem', fontWeight: 600 }}>RBAC Governance</h3>
          <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.95rem', lineHeight: 1.5 }}>
            Granular permission flags for Owners, Moderators, Members, and Guests enforcing UI visibility and moderation.
          </p>
        </div>
      </section>

      <CreateRoomModal isOpen={isCreateModalOpen} onClose={() => setIsCreateModalOpen(false)} />
    </main>
  );
};
