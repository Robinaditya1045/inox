import { useState, useCallback, useMemo } from "react";
import { useRoom } from "./useRoom";
import { useAuth } from "./useAuth";
import { usePermissions } from "./usePermissions";
import { roomService } from "../services/room/room.service";
import type { RoomMember, RoomRole } from "../types/room";
import { logger } from "../utils/logger";

export interface PresenceMember extends RoomMember {
  /** Connected to the room right now, in any tab. */
  isOnline: boolean;
}

export interface UsePresenceReturn {
  /** Everyone with membership of the room, online or not. */
  members: PresenceMember[];
  onlineCount: number;
  isLoadingMembers: boolean;
  kickMember: (userId: string) => Promise<void>;
  updateRole: (userId: string, newRole: RoomRole) => Promise<void>;
  canModerate: (targetMember: RoomMember) => boolean;
}

export const usePresence = (): UsePresenceReturn => {
  const { activeRoom, onlineUsers, refreshMembers } = useRoom();
  const { user } = useAuth();
  const permissions = usePermissions();

  const [isLoadingMembers, setIsLoadingMembers] = useState<boolean>(false);
  const roomId = activeRoom?.id;

  // The roster is the room's membership; the server's presence list only says
  // which of those people are connected. Someone who joined since the roster
  // was loaded shows up once RoomProvider has re-read it.
  const members = useMemo<PresenceMember[]>(() => {
    const onlineIds = new Set(onlineUsers?.map((u) => u.user_id));
    return (activeRoom?.members ?? []).map((member) => ({
      ...member,
      // You are here even before the server's first list arrives.
      isOnline: onlineIds.has(member.user_id) || member.user_id === user?.id,
    }));
  }, [activeRoom?.members, onlineUsers, user?.id]);

  const onlineCount = members.filter((m) => m.isOnline).length;

  const kickMember = useCallback(
    async (userId: string) => {
      if (!roomId || !permissions.can_kick_users) {
        logger.warn(
          "usePresence: Permission denied or no active room to kick member",
        );
        return;
      }

      try {
        setIsLoadingMembers(true);
        await roomService.kickMember(roomId, userId);
        await refreshMembers();
        logger.info("usePresence: Successfully kicked member", { userId });
      } catch (err) {
        logger.error("usePresence: Failed to kick member", { err, userId });
        throw err;
      } finally {
        setIsLoadingMembers(false);
      }
    },
    [roomId, permissions.can_kick_users, refreshMembers],
  );

  const updateRole = useCallback(
    async (userId: string, newRole: RoomRole) => {
      if (!roomId || !permissions.can_manage_roles) {
        logger.warn("usePresence: Permission denied to update role");
        return;
      }

      try {
        setIsLoadingMembers(true);
        await roomService.updateMemberRole(roomId, userId, newRole);
        await refreshMembers();
        logger.info("usePresence: Updated member role", { userId, newRole });
      } catch (err) {
        logger.error("usePresence: Failed to update role", {
          err,
          userId,
          newRole,
        });
        throw err;
      } finally {
        setIsLoadingMembers(false);
      }
    },
    [roomId, permissions.can_manage_roles, refreshMembers],
  );

  const canModerate = useCallback(
    (targetMember: RoomMember): boolean => {
      if (!permissions.can_manage_roles && !permissions.can_kick_users)
        return false;
      if (permissions.isOwner) return targetMember.role !== "owner";
      if (permissions.isModerator)
        return targetMember.role === "member" || targetMember.role === "guest";
      return false;
    },
    [permissions],
  );

  return {
    members,
    onlineCount,
    isLoadingMembers,
    kickMember,
    updateRole,
    canModerate,
  };
};
