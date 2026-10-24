import React, {
  useState,
  useEffect,
  useCallback,
  useRef,
  type ReactNode,
} from "react";
import { RoomContext } from "../contexts/room.context";
import {
  roomService,
  DEFAULT_PERMISSIONS,
} from "../services/room/room.service";
import type {
  Room,
  CreateRoomRequest,
  RoomRole,
  RoomInvitation,
  WSPresencePayload,
  WSPresenceUser,
} from "../types";
import { useAuth } from "../hooks/useAuth";
import { logger } from "../utils/logger";
import { APIError } from "../api/client";
import { wsService } from "../services/ws/ws.service";

interface RoomProviderProps {
  children: ReactNode;
}

interface RoomPresence {
  roomId: string;
  users: WSPresenceUser[];
}

// Joins and leaves tend to come in bursts (a reload is a leave and a join), so
// the member list is fetched once things settle rather than once per change.
const MEMBER_REFRESH_DELAY_MS = 500;

export const RoomProvider: React.FC<RoomProviderProps> = ({ children }) => {
  const [activeRoom, setActiveRoom] = useState<Room | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [invitations, setInvitations] = useState<RoomInvitation[]>([]);
  const [isLoadingRoom, setIsLoadingRoom] = useState<boolean>(false);
  const [roomError, setRoomError] = useState<string | null>(null);
  // False until the first room list for this user has come back (or failed), so
  // the lobby can show a skeleton instead of a premature "no rooms" state.
  const [hasLoadedRooms, setHasLoadedRooms] = useState<boolean>(false);

  const { user } = useAuth();

  // Guards against out-of-order joins: a slow join for room A must not overwrite
  // a later, faster join for room B.
  const joinSeqRef = useRef(0);

  // Who is connected to a room right now, from the server's PRESENCE list. It
  // lives here rather than in the member list because the server only sends it
  // when someone arrives or leaves, so a component mounted afterwards would
  // never hear it.
  const [presence, setPresence] = useState<RoomPresence | null>(null);
  const activeRoomRef = useRef(activeRoom);
  useEffect(() => {
    activeRoomRef.current = activeRoom;
  }, [activeRoom]);

  // Re-reads the active room's member list. The result is dropped if the user
  // has moved to another room in the meantime.
  const refreshMembers = useCallback(async () => {
    const roomId = activeRoomRef.current?.id;
    if (!roomId) return;
    const updated = await roomService.getRoom(roomId);
    setActiveRoom((current) =>
      current?.id === updated.id ? updated : current,
    );
  }, []);

  useEffect(() => {
    let previous: RoomPresence | null = null;
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;

    // Other people's membership changes reach this client only as presence:
    // someone new joined, or someone left (perhaps the room itself, which should
    // take them off the list rather than leave them showing offline).
    const scheduleRefresh = () => {
      if (refreshTimer) clearTimeout(refreshTimer);
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        refreshMembers().catch((err) =>
          logger.warn("RoomProvider: Failed to refresh members", { err }),
        );
      }, MEMBER_REFRESH_DELAY_MS);
    };

    const unsubscribe = wsService.on("PRESENCE", (msg) => {
      const users = (msg.payload as WSPresencePayload | undefined)?.users;
      if (!msg.room_id || !users) return;
      const next = { roomId: msg.room_id, users };
      const prior = previous?.roomId === next.roomId ? previous : null;
      previous = next;
      setPresence(next);

      const room = activeRoomRef.current;
      if (!room || room.id !== next.roomId) return;
      const memberIds = new Set(room.members?.map((m) => m.user_id));
      const onlineIds = new Set(users.map((u) => u.user_id));
      const arrived = users.some((u) => !memberIds.has(u.user_id));
      const departed = !!prior?.users.some((u) => !onlineIds.has(u.user_id));
      if (arrived || departed) scheduleRefresh();
    });

    return () => {
      unsubscribe();
      if (refreshTimer) clearTimeout(refreshTimer);
    };
  }, [refreshMembers]);

  // Null until the server has said who is here, so callers can tell "nobody
  // else is online" from "not known yet".
  const onlineUsers =
    presence && activeRoom && presence.roomId === activeRoom.id
      ? presence.users
      : null;

  const clearRoomError = useCallback(() => {
    setRoomError(null);
  }, []);

  const refreshRooms = useCallback(async () => {
    if (!user) {
      setRooms([]);
      setHasLoadedRooms(false);
      return;
    }
    try {
      const fetchedRooms = await roomService.listRooms();
      setRooms(fetchedRooms);
    } catch (err) {
      logger.warn("RoomProvider: Failed to refresh room list", { err });
    } finally {
      setHasLoadedRooms(true);
    }
  }, [user]);

  const refreshInvitations = useCallback(async () => {
    if (!user) {
      setInvitations([]);
      return;
    }
    try {
      const fetched = await roomService.getPendingInvitations();
      setInvitations(fetched);
    } catch (err) {
      logger.warn("RoomProvider: Failed to refresh invitations", { err });
    }
  }, [user]);

  useEffect(() => {
    refreshRooms();
    refreshInvitations();
  }, [refreshRooms, refreshInvitations]);

  // Compute current user's role and permission matrix whenever activeRoom or user changes
  const { userRole, permissions } = React.useMemo(() => {
    if (!activeRoom || !user) {
      return { userRole: null, permissions: null };
    }

    if (activeRoom.owner_id === user.id) {
      return {
        userRole: "owner" as RoomRole,
        permissions: DEFAULT_PERMISSIONS.owner,
      };
    }

    const member = activeRoom.members?.find((m) => m.user_id === user.id);
    if (member) {
      return {
        userRole: member.role,
        permissions: member.permissions || DEFAULT_PERMISSIONS[member.role],
      };
    }

    // Not in the member list: fall back to guest, not member. Granting full member
    // permissions here silently hid non-membership.
    return {
      userRole: "guest" as RoomRole,
      permissions: DEFAULT_PERMISSIONS.guest,
    };
  }, [activeRoom, user]);

  const createRoom = useCallback(
    async (data: CreateRoomRequest): Promise<Room> => {
      setIsLoadingRoom(true);
      setRoomError(null);
      try {
        const newRoom = await roomService.createRoom(data);
        setActiveRoom(newRoom);
        await refreshRooms();
        logger.info("RoomProvider: Room created and joined", {
          roomId: newRoom.id,
        });
        return newRoom;
      } catch (err) {
        const msg =
          err instanceof APIError ? err.message : "Failed to create room.";
        setRoomError(msg);
        logger.error("RoomProvider: Create room error", { err });
        throw err;
      } finally {
        setIsLoadingRoom(false);
      }
    },
    [refreshRooms],
  );

  const joinRoom = useCallback(async (roomId: string): Promise<Room> => {
    const seq = ++joinSeqRef.current;
    setIsLoadingRoom(true);
    setRoomError(null);
    try {
      const joinedRoom = await roomService.joinRoom(roomId);
      if (seq !== joinSeqRef.current) {
        logger.info("RoomProvider: Discarding superseded join result", {
          roomId,
        });
        return joinedRoom;
      }
      setActiveRoom(joinedRoom);
      logger.info("RoomProvider: Successfully joined room", { roomId });
      return joinedRoom;
    } catch (err) {
      const msg =
        err instanceof APIError ? err.message : "Failed to join room.";
      if (seq === joinSeqRef.current) {
        setRoomError(msg);
      }
      logger.error("RoomProvider: Join room error", { err });
      throw err;
    } finally {
      if (seq === joinSeqRef.current) {
        setIsLoadingRoom(false);
      }
    }
  }, []);

  const leaveRoom = useCallback(async () => {
    // Always tear local state down, even if there is nothing to tell the server about —
    // otherwise a stale activeRoom keeps the lobby thinking you are still in a room.
    if (!activeRoom || !user) {
      joinSeqRef.current++;
      wsService.disconnect();
      setActiveRoom(null);
      return;
    }
    joinSeqRef.current++;
    setIsLoadingRoom(true);
    // Hang up before telling the server, which closes every connection the
    // user still has to the room with a "you left" notice meant for their
    // other tabs, not this one.
    wsService.disconnect();
    try {
      await roomService.leaveRoom(activeRoom.id);
      logger.info("RoomProvider: Left room successfully", {
        roomId: activeRoom.id,
      });
    } catch (err) {
      logger.warn("RoomProvider: Error while leaving room", { err });
    } finally {
      setActiveRoom(null);
      setIsLoadingRoom(false);
      await refreshRooms();
    }
  }, [activeRoom, user, refreshRooms]);

  // Drops this client's connection to the room without leaving it: membership is
  // kept, but the socket closes and clearing activeRoom tears down the call. Used
  // on logout, where leaving would wrongly give up the user's rooms.
  const disconnectFromRoom = useCallback(() => {
    joinSeqRef.current++;
    wsService.disconnect();
    setActiveRoom(null);
  }, []);

  const updateMemberRole = useCallback(
    async (userId: string, role: RoomRole) => {
      if (!activeRoom) return;
      try {
        await roomService.updateMemberRole(activeRoom.id, userId, role);
        // Refresh room details
        const updated = await roomService.getRoom(activeRoom.id);
        setActiveRoom(updated);
      } catch (err) {
        const msg =
          err instanceof APIError ? err.message : "Failed to update role.";
        setRoomError(msg);
        throw err;
      }
    },
    [activeRoom],
  );

  const kickMember = useCallback(
    async (userId: string) => {
      if (!activeRoom) return;
      try {
        await roomService.kickMember(activeRoom.id, userId);
        const updated = await roomService.getRoom(activeRoom.id);
        setActiveRoom(updated);
      } catch (err) {
        const msg =
          err instanceof APIError ? err.message : "Failed to kick member.";
        setRoomError(msg);
        throw err;
      }
    },
    [activeRoom],
  );

  const inviteUser = useCallback(
    async (username: string): Promise<RoomInvitation> => {
      if (!activeRoom) throw new Error("No active room");
      try {
        const inv = await roomService.inviteUser(activeRoom.id, username);
        logger.info("RoomProvider: Invited user successfully", { username });
        return inv;
      } catch (err) {
        const msg =
          err instanceof APIError ? err.message : "Failed to invite user.";
        setRoomError(msg);
        throw err;
      }
    },
    [activeRoom],
  );

  const acceptInvitation = useCallback(
    async (invId: string): Promise<Room> => {
      setIsLoadingRoom(true);
      setRoomError(null);
      try {
        const { room } = await roomService.acceptInvitation(invId);
        setActiveRoom(room);
        await refreshRooms();
        await refreshInvitations();
        logger.info("RoomProvider: Accepted invitation and joined room", {
          roomId: room.id,
        });
        return room;
      } catch (err) {
        const msg =
          err instanceof APIError
            ? err.message
            : "Failed to accept invitation.";
        setRoomError(msg);
        throw err;
      } finally {
        setIsLoadingRoom(false);
      }
    },
    [refreshRooms, refreshInvitations],
  );

  const declineInvitation = useCallback(
    async (invId: string): Promise<void> => {
      try {
        await roomService.declineInvitation(invId);
        await refreshInvitations();
        logger.info("RoomProvider: Declined invitation", { invId });
      } catch (err) {
        const msg =
          err instanceof APIError
            ? err.message
            : "Failed to decline invitation.";
        setRoomError(msg);
        throw err;
      }
    },
    [refreshInvitations],
  );

  const value = {
    activeRoom,
    onlineUsers,
    refreshMembers,
    rooms,
    invitations,
    userRole,
    permissions,
    isLoadingRoom,
    hasLoadedRooms,
    roomError,
    createRoom,
    joinRoom,
    leaveRoom,
    disconnectFromRoom,
    refreshRooms,
    updateMemberRole,
    kickMember,
    inviteUser,
    acceptInvitation,
    declineInvitation,
    refreshInvitations,
    clearRoomError,
  };

  return <RoomContext.Provider value={value}>{children}</RoomContext.Provider>;
};
