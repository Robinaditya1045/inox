import { useState, useEffect, useCallback } from 'react';
import { useRoom } from './useRoom';
import { useWS } from './useWS';
import { usePermissions } from './usePermissions';
import { roomService, DEFAULT_PERMISSIONS } from '../services/room/room.service';
import type { RoomMember, RoomRole } from '../types/room';
import { logger } from '../utils/logger';

export interface UsePresenceReturn {
  members: RoomMember[];
  isLoadingMembers: boolean;
  kickMember: (userId: string) => Promise<void>;
  updateRole: (userId: string, newRole: RoomRole) => Promise<void>;
  canModerate: (targetMember: RoomMember) => boolean;
}

export const usePresence = (): UsePresenceReturn => {
  const { activeRoom } = useRoom();
  const { subscribe } = useWS();
  const permissions = usePermissions();

  const [members, setMembers] = useState<RoomMember[]>(activeRoom?.members || []);
  const [isLoadingMembers, setIsLoadingMembers] = useState<boolean>(false);

  // Sync initial member list from room state
  useEffect(() => {
    setMembers(activeRoom?.members || []);
  }, [activeRoom?.members]);

  // Subscribe to JOIN_ROOM and LEAVE_ROOM events to keep presence list real-time
  useEffect(() => {
    if (!activeRoom?.id) return;

    const unsubJoin = subscribe('JOIN_ROOM', (msg) => {
      if (msg.sender_id && msg.sender_name) {
        logger.info('usePresence: Peer joined', { userId: msg.sender_id, username: msg.sender_name });
        setMembers((prev) => {
          if (prev.some((m) => m.user_id === msg.sender_id)) return prev;
          const newMember: RoomMember = {
            user_id: msg.sender_id!,
            username: msg.sender_name!,
            role: 'guest',
            permissions: DEFAULT_PERMISSIONS['guest'],
            joined_at: new Date().toISOString(),
          };
          return [...prev, newMember];
        });
      }
    });

    const unsubLeave = subscribe('LEAVE_ROOM', (msg) => {
      if (msg.sender_id) {
        logger.info('usePresence: Peer left', { userId: msg.sender_id });
        setMembers((prev) => prev.filter((m) => m.user_id !== msg.sender_id));
      }
    });

    return () => {
      unsubJoin();
      unsubLeave();
    };
  }, [activeRoom?.id, subscribe]);

  const kickMember = useCallback(async (userId: string) => {
    if (!activeRoom?.id || !permissions.can_kick_users) {
      logger.warn('usePresence: Permission denied or no active room to kick member');
      return;
    }

    try {
      setIsLoadingMembers(true);
      await roomService.kickMember(activeRoom.id, userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
      logger.info('usePresence: Successfully kicked member', { userId });
    } catch (err) {
      logger.error('usePresence: Failed to kick member', { err, userId });
      throw err;
    } finally {
      setIsLoadingMembers(false);
    }
  }, [activeRoom?.id, permissions.can_kick_users]);

  const updateRole = useCallback(async (userId: string, newRole: RoomRole) => {
    if (!activeRoom?.id || !permissions.can_manage_roles) {
      logger.warn('usePresence: Permission denied to update role');
      return;
    }

    try {
      setIsLoadingMembers(true);
      await roomService.updateMemberRole(activeRoom.id, userId, newRole);
      setMembers((prev) =>
        prev.map((m) => (m.user_id === userId ? { ...m, role: newRole, permissions: DEFAULT_PERMISSIONS[newRole] } : m))
      );
      logger.info('usePresence: Updated member role', { userId, newRole });
    } catch (err) {
      logger.error('usePresence: Failed to update role', { err, userId, newRole });
      throw err;
    } finally {
      setIsLoadingMembers(false);
    }
  }, [activeRoom?.id, permissions.can_manage_roles]);

  const canModerate = useCallback((targetMember: RoomMember): boolean => {
    if (!permissions.can_manage_roles && !permissions.can_kick_users) return false;
    if (permissions.isOwner) return targetMember.role !== 'owner';
    if (permissions.isModerator) return targetMember.role === 'member' || targetMember.role === 'guest';
    return false;
  }, [permissions]);

  return {
    members,
    isLoadingMembers,
    kickMember,
    updateRole,
    canModerate,
  };
};
