import React, { useState } from 'react';
import { usePresence } from '../../hooks/usePresence';
import { useAuth } from '../../hooks/useAuth';
import type { RoomRole } from '../../types/room';
import { Shield, UserMinus, Crown, ShieldAlert, UserCheck, MoreVertical, UserPlus, Check, X, AlertCircle } from 'lucide-react';
import { useRoom } from '../../hooks/useRoom';

export const MemberList: React.FC = () => {
  const { members, isLoadingMembers, kickMember, updateRole, canModerate } = usePresence();
  const { user } = useAuth();
  const { permissions, inviteUser, activeRoom } = useRoom();
  
  const [activeMenuId, setActiveMenuId] = useState<string | null>(null);
  const [inviteUsername, setInviteUsername] = useState('');
  const [isInviting, setIsInviting] = useState(false);
  const [inviteSuccess, setInviteSuccess] = useState<string | null>(null);
  const [inviteError, setInviteError] = useState<string | null>(null);

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inviteUsername.trim()) return;
    setIsInviting(true);
    setInviteError(null);
    setInviteSuccess(null);
    try {
      await inviteUser(inviteUsername.trim());
      setInviteSuccess(`Invited @${inviteUsername.trim()}`);
      setInviteUsername('');
      setTimeout(() => setInviteSuccess(null), 4000);
    } catch (err: any) {
      setInviteError(err?.message || 'Failed to invite user');
    } finally {
      setIsInviting(false);
    }
  };

  const getRoleBadge = (role: RoomRole) => {
    switch (role) {
      case 'owner':
        return {
          label: 'Owner',
          color: '#A855F7',
          bg: 'rgba(168, 85, 247, 0.15)',
          border: 'rgba(168, 85, 247, 0.4)',
          icon: <Crown size={12} color="#A855F7" />,
        };
      case 'moderator':
        return {
          label: 'Mod',
          color: '#06B6D4',
          bg: 'rgba(6, 182, 212, 0.15)',
          border: 'rgba(6, 182, 212, 0.4)',
          icon: <ShieldAlert size={12} color="#06B6D4" />,
        };
      case 'member':
        return {
          label: 'Member',
          color: '#10B981',
          bg: 'rgba(16, 185, 129, 0.15)',
          border: 'rgba(16, 185, 129, 0.4)',
          icon: <UserCheck size={12} color="#10B981" />,
        };
      default:
        return {
          label: 'Guest',
          color: '#94A3B8',
          bg: 'rgba(148, 163, 184, 0.15)',
          border: 'rgba(148, 163, 184, 0.4)',
          icon: null,
        };
    }
  };

  const getInitials = (name: string): string => {
    return name
      .split(' ')
      .map((part) => part[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  const handleRoleChange = async (targetId: string, newRole: RoomRole) => {
    try {
      await updateRole(targetId, newRole);
      setActiveMenuId(null);
    } catch {
      // Error handled in hook
    }
  };

  const handleKick = async (targetId: string) => {
    try {
      await kickMember(targetId);
      setActiveMenuId(null);
    } catch {
      // Error handled in hook
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        width: '100%',
        padding: '16px',
        overflowY: 'auto',
        gap: '12px',
      }}
    >
      {/* Invite Section for Private Rooms */}
      {(permissions?.can_invite_users || activeRoom?.is_private) && (
        <div
          style={{
            padding: '12px',
            borderRadius: '12px',
            background: 'var(--color-bg-surface)',
            border: '1px solid var(--color-border-glass)',
            display: 'flex',
            flexDirection: 'column',
            gap: '8px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>
            <UserPlus size={14} color="var(--color-accent-purple)" />
            <span>Invite to Room</span>
          </div>
          <form onSubmit={handleInvite} style={{ display: 'flex', gap: '6px' }}>
            <input
              type="text"
              placeholder="Username..."
              value={inviteUsername}
              onChange={(e) => setInviteUsername(e.target.value)}
              disabled={isInviting}
              style={{
                flex: 1,
                padding: '6px 10px',
                borderRadius: '8px',
                background: 'var(--color-bg-surface-hover)',
                border: '1px solid var(--color-border-glass)',
                color: 'var(--color-text-primary)',
                fontSize: '0.8rem',
                outline: 'none',
              }}
            />
            <button
              type="submit"
              disabled={isInviting || !inviteUsername.trim()}
              style={{
                padding: '6px 12px',
                borderRadius: '8px',
                background: 'var(--color-accent-purple)',
                color: '#FFF',
                border: 'none',
                fontSize: '0.8rem',
                fontWeight: 600,
                cursor: isInviting || !inviteUsername.trim() ? 'not-allowed' : 'pointer',
                opacity: isInviting || !inviteUsername.trim() ? 0.6 : 1,
              }}
            >
              {isInviting ? '...' : 'Invite'}
            </button>
          </form>
          {inviteSuccess && (
            <span style={{ fontSize: '0.75rem', color: 'var(--color-accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Check size={12} /> {inviteSuccess}
            </span>
          )}
          {inviteError && (
            <span style={{ fontSize: '0.75rem', color: 'var(--color-accent-rose)', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <AlertCircle size={12} /> {inviteError}
            </span>
          )}
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingBottom: '8px', borderBottom: '1px solid var(--color-border-glass)' }}>
        <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          Active Participants ({members.length})
        </span>
        {isLoadingMembers && <span style={{ fontSize: '0.75rem', color: 'var(--color-accent-purple)' }}>Updating...</span>}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {members.map((member) => {
          const badge = getRoleBadge(member.role);
          const isSelf = member.user_id === user?.id;
          const showAdminControls = !isSelf && canModerate(member);

          return (
            <div
              key={member.user_id}
              style={{
                padding: '10px 12px',
                borderRadius: '12px',
                background: 'var(--color-bg-surface)',
                border: '1px solid var(--color-border-glass)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                position: 'relative',
                transition: 'all var(--transition-fast)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                {/* Avatar with Online Pulse */}
                <div style={{ position: 'relative' }}>
                  <div
                    style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: '10px',
                      background: isSelf ? 'var(--color-accent-purple)' : 'var(--color-bg-surface-hover)',
                      color: '#FFF',
                      fontSize: '0.8rem',
                      fontWeight: 700,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    {getInitials(member.username)}
                  </div>
                  <div
                    style={{
                      position: 'absolute',
                      bottom: '-2px',
                      right: '-2px',
                      width: '10px',
                      height: '10px',
                      borderRadius: '50%',
                      background: 'var(--color-accent-emerald)',
                      border: '2px solid var(--color-bg-surface)',
                      boxShadow: '0 0 8px var(--color-accent-emerald)',
                    }}
                  />
                </div>

                {/* Name and Role */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                  <span style={{ fontSize: '0.9rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>
                    {member.username} {isSelf && <span style={{ color: 'var(--color-text-muted)', fontWeight: 400 }}>(You)</span>}
                  </span>
                  <div
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '4px',
                      padding: '2px 8px',
                      borderRadius: '10px',
                      background: badge.bg,
                      border: `1px solid ${badge.border}`,
                      color: badge.color,
                      fontSize: '0.68rem',
                      fontWeight: 700,
                      width: 'fit-content',
                    }}
                  >
                    {badge.icon}
                    <span>{badge.label}</span>
                  </div>
                </div>
              </div>

              {/* Moderation Actions Menu Toggle */}
              {showAdminControls && (
                <div>
                  <button
                    onClick={() => setActiveMenuId(activeMenuId === member.user_id ? null : member.user_id)}
                    style={{
                      padding: '6px',
                      borderRadius: '8px',
                      color: 'var(--color-text-secondary)',
                      background: activeMenuId === member.user_id ? 'var(--color-bg-surface-hover)' : 'transparent',
                      transition: 'all var(--transition-fast)',
                      display: 'flex',
                      alignItems: 'center',
                    }}
                  >
                    <MoreVertical size={16} />
                  </button>

                  {/* Dropdown Menu */}
                  {activeMenuId === member.user_id && (
                    <div
                      className="glass-panel-heavy"
                      style={{
                        position: 'absolute',
                        right: '12px',
                        top: '46px',
                        width: '180px',
                        padding: '6px',
                        borderRadius: '12px',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: '4px',
                        zIndex: 20,
                        boxShadow: '0 10px 25px rgba(0, 0, 0, 0.8)',
                        border: '1px solid var(--color-border-hover)',
                      }}
                    >
                      <span style={{ fontSize: '0.68rem', fontWeight: 600, color: 'var(--color-text-muted)', padding: '4px 8px', textTransform: 'uppercase' }}>
                        Change Role
                      </span>
                      <button
                        onClick={() => handleRoleChange(member.user_id, 'moderator')}
                        style={{ padding: '6px 8px', borderRadius: '6px', textAlign: 'left', fontSize: '0.8rem', color: '#06B6D4', display: 'flex', alignItems: 'center', gap: '8px', background: 'transparent' }}
                      >
                        <ShieldAlert size={14} /> Promote to Mod
                      </button>
                      <button
                        onClick={() => handleRoleChange(member.user_id, 'member')}
                        style={{ padding: '6px 8px', borderRadius: '6px', textAlign: 'left', fontSize: '0.8rem', color: '#10B981', display: 'flex', alignItems: 'center', gap: '8px', background: 'transparent' }}
                      >
                        <UserCheck size={14} /> Set as Member
                      </button>
                      <button
                        onClick={() => handleRoleChange(member.user_id, 'guest')}
                        style={{ padding: '6px 8px', borderRadius: '6px', textAlign: 'left', fontSize: '0.8rem', color: '#94A3B8', display: 'flex', alignItems: 'center', gap: '8px', background: 'transparent' }}
                      >
                        <Shield size={14} /> Demote to Guest
                      </button>
                      <div style={{ height: '1px', background: 'var(--color-border-glass)', margin: '4px 0' }} />
                      <button
                        onClick={() => handleKick(member.user_id)}
                        style={{ padding: '6px 8px', borderRadius: '6px', textAlign: 'left', fontSize: '0.8rem', color: 'var(--color-accent-rose)', display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(244, 63, 94, 0.1)' }}
                      >
                        <UserMinus size={14} /> Kick from Room
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
