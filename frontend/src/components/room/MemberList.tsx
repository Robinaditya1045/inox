import React, { useState } from 'react';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { usePresence } from '../../hooks/usePresence';
import { useAuth } from '../../hooks/useAuth';
import type { RoomRole } from '../../types/room';
import { Shield, UserMinus, Crown, ShieldAlert, UserCheck, MoreVertical, UserPlus, Check, AlertCircle } from 'lucide-react';
import { useRoom } from '../../hooks/useRoom';
import styles from './MemberList.module.css';

export const MemberList: React.FC = () => {
  const { members, isLoadingMembers, kickMember, updateRole, canModerate } = usePresence();
  const { user } = useAuth();
  const { permissions, inviteUser, activeRoom } = useRoom();
  
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
          color: 'var(--color-accent)',
          bg: 'var(--color-accent-subtle)',
          border: 'var(--color-accent-border)',
          icon: <Crown size={12} color="currentColor" />,
        };
      case 'moderator':
        return {
          label: 'Mod',
          color: 'var(--color-accent-cyan)',
          bg: 'rgba(6, 182, 212, 0.15)',
          border: 'rgba(6, 182, 212, 0.4)',
          icon: <ShieldAlert size={12} color="currentColor" />,
        };
      case 'member':
        return {
          label: 'Member',
          color: 'var(--color-success)',
          bg: 'var(--color-success-subtle)',
          border: 'var(--color-success-border)',
          icon: <UserCheck size={12} color="currentColor" />,
        };
      default:
        return {
          label: 'Guest',
          color: 'var(--color-text-secondary)',
          bg: 'var(--color-surface-3)',
          border: 'var(--color-border-default)',
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
    } catch {
      // Error handled in hook
    }
  };

  const handleKick = async (targetId: string) => {
    try {
      await kickMember(targetId);
    } catch {
      // Error handled in hook
    }
  };

  return (
    <div className={styles.container}>
      {/* Invite Section for Private Rooms */}
      {(permissions?.can_invite_users || activeRoom?.is_private) && (
        <div className={styles.inviteSection}>
          <div className={styles.inviteHeader}>
            <UserPlus size={14} className={styles.inviteIcon} />
            <span>Invite to Room</span>
          </div>
          <form onSubmit={handleInvite} className={styles.inviteForm}>
            <input
              type="text"
              placeholder="Username..."
              value={inviteUsername}
              onChange={(e) => setInviteUsername(e.target.value)}
              disabled={isInviting}
              className={styles.inviteInput}
            />
            <button
              type="submit"
              disabled={isInviting || !inviteUsername.trim()}
              className={styles.inviteBtn}
            >
              {isInviting ? '...' : 'Invite'}
            </button>
          </form>
          {inviteSuccess && (
            <span className={styles.inviteSuccess}>
              <Check size={12} /> {inviteSuccess}
            </span>
          )}
          {inviteError && (
            <span className={styles.inviteError}>
              <AlertCircle size={12} /> {inviteError}
            </span>
          )}
        </div>
      )}

      <div className={styles.header}>
        <span className={styles.headerTitle}>
          Active Participants ({members.length})
        </span>
        {isLoadingMembers && <span className={styles.loadingText}>Updating...</span>}
      </div>

      <div className={styles.list}>
        {members.map((member) => {
          const badge = getRoleBadge(member.role);
          const isSelf = member.user_id === user?.id;
          const showAdminControls = !isSelf && canModerate(member);

          return (
            <div key={member.user_id} className={styles.memberItem}>
              <div className={styles.memberInfo}>
                {/* Avatar with Online Pulse */}
                <div className={styles.avatarWrapper}>
                  <div className={`${styles.avatar} ${isSelf ? styles.avatarSelf : styles.avatarOther}`}>
                    {getInitials(member.username)}
                  </div>
                  <div className={styles.onlineBadge} />
                </div>

                {/* Name and Role */}
                <div className={styles.memberDetails}>
                  <span className={styles.memberName}>
                    {member.username} {isSelf && <span className={styles.selfLabel}>(You)</span>}
                  </span>
                  <div
                    className={styles.roleBadge}
                    style={{ background: badge.bg, border: `1px solid ${badge.border}`, color: badge.color }}
                  >
                    {badge.icon}
                    <span>{badge.label}</span>
                  </div>
                </div>
              </div>

              {/* Moderation Actions Menu */}
              {showAdminControls && (
                <DropdownMenu.Root>
                  <DropdownMenu.Trigger asChild>
                    <button className={styles.actionBtn} aria-label="Manage member">
                      <MoreVertical size={16} />
                    </button>
                  </DropdownMenu.Trigger>

                  <DropdownMenu.Portal>
                    <DropdownMenu.Content className={styles.dropdownContent} sideOffset={4} align="end">
                      <DropdownMenu.Label className={styles.dropdownLabel}>
                        Change Role
                      </DropdownMenu.Label>
                      
                      <DropdownMenu.Item
                        className={styles.dropdownItem}
                        onClick={() => handleRoleChange(member.user_id, 'moderator')}
                        style={{ color: 'var(--color-accent-cyan)' }}
                      >
                        <ShieldAlert size={14} /> Promote to Mod
                      </DropdownMenu.Item>
                      
                      <DropdownMenu.Item
                        className={styles.dropdownItem}
                        onClick={() => handleRoleChange(member.user_id, 'member')}
                        style={{ color: 'var(--color-success)' }}
                      >
                        <UserCheck size={14} /> Set as Member
                      </DropdownMenu.Item>
                      
                      <DropdownMenu.Item
                        className={styles.dropdownItem}
                        onClick={() => handleRoleChange(member.user_id, 'guest')}
                      >
                        <Shield size={14} /> Demote to Guest
                      </DropdownMenu.Item>
                      
                      <DropdownMenu.Separator className={styles.dropdownSeparator} />
                      
                      <DropdownMenu.Item
                        className={`${styles.dropdownItem} ${styles.dropdownItemDanger}`}
                        onClick={() => handleKick(member.user_id)}
                      >
                        <UserMinus size={14} /> Kick from Room
                      </DropdownMenu.Item>
                    </DropdownMenu.Content>
                  </DropdownMenu.Portal>
                </DropdownMenu.Root>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
