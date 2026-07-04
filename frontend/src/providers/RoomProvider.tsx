import React, { useState, useEffect, useCallback, type ReactNode } from 'react';
import { RoomContext } from '../contexts/room.context';
import { roomService, DEFAULT_PERMISSIONS } from '../services/room/room.service';
import type { Room, CreateRoomRequest, RoomRole } from '../types';
import { useAuth } from '../hooks/useAuth';
import { logger } from '../utils/logger';
import { APIError } from '../api/client';

interface RoomProviderProps {
  children: ReactNode;
}

export const RoomProvider: React.FC<RoomProviderProps> = ({ children }) => {
  const [activeRoom, setActiveRoom] = useState<Room | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [isLoadingRoom, setIsLoadingRoom] = useState<boolean>(false);
  const [roomError, setRoomError] = useState<string | null>(null);

  const { user } = useAuth();

  const clearRoomError = useCallback(() => {
    setRoomError(null);
  }, []);

  const refreshRooms = useCallback(async () => {
    if (!user) {
      setRooms([]);
      return;
    }
    try {
      const fetchedRooms = await roomService.listRooms();
      setRooms(fetchedRooms);
    } catch (err) {
      logger.warn('RoomProvider: Failed to refresh room list', { err });
    }
  }, [user]);

  useEffect(() => {
    refreshRooms();
  }, [refreshRooms]);

  // Compute current user's role and permission matrix whenever activeRoom or user changes
  const { userRole, permissions } = React.useMemo(() => {
    if (!activeRoom || !user) {
      return { userRole: null, permissions: null };
    }

    if (activeRoom.owner_id === user.id) {
      return {
        userRole: 'owner' as RoomRole,
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

    // Default fallback if not found in member list yet
    return {
      userRole: 'member' as RoomRole,
      permissions: DEFAULT_PERMISSIONS.member,
    };
  }, [activeRoom, user]);

  const createRoom = useCallback(async (data: CreateRoomRequest): Promise<Room> => {
    setIsLoadingRoom(true);
    setRoomError(null);
    try {
      const newRoom = await roomService.createRoom(data);
      setActiveRoom(newRoom);
      await refreshRooms();
      logger.info('RoomProvider: Room created and joined', { roomId: newRoom.id });
      return newRoom;
    } catch (err) {
      const msg = err instanceof APIError ? err.message : 'Failed to create room.';
      setRoomError(msg);
      logger.error('RoomProvider: Create room error', { err });
      throw err;
    } finally {
      setIsLoadingRoom(false);
    }
  }, [refreshRooms]);

  const joinRoom = useCallback(async (roomId: string): Promise<Room> => {
    setIsLoadingRoom(true);
    setRoomError(null);
    try {
      const joinedRoom = await roomService.joinRoom(roomId);
      setActiveRoom(joinedRoom);
      logger.info('RoomProvider: Successfully joined room', { roomId });
      return joinedRoom;
    } catch (err) {
      const msg = err instanceof APIError ? err.message : 'Failed to join room.';
      setRoomError(msg);
      logger.error('RoomProvider: Join room error', { err });
      throw err;
    } finally {
      setIsLoadingRoom(false);
    }
  }, []);

  const leaveRoom = useCallback(async () => {
    if (!activeRoom || !user) return;
    setIsLoadingRoom(true);
    try {
      await roomService.kickMember(activeRoom.id, user.id);
      logger.info('RoomProvider: Left room successfully', { roomId: activeRoom.id });
    } catch (err) {
      logger.warn('RoomProvider: Error while leaving room', { err });
    } finally {
      setActiveRoom(null);
      setIsLoadingRoom(false);
      await refreshRooms();
    }
  }, [activeRoom, user, refreshRooms]);

  const updateMemberRole = useCallback(async (userId: string, role: RoomRole) => {
    if (!activeRoom) return;
    try {
      await roomService.updateMemberRole(activeRoom.id, userId, role);
      // Refresh room details
      const updated = await roomService.getRoom(activeRoom.id);
      setActiveRoom(updated);
    } catch (err) {
      const msg = err instanceof APIError ? err.message : 'Failed to update role.';
      setRoomError(msg);
      throw err;
    }
  }, [activeRoom]);

  const kickMember = useCallback(async (userId: string) => {
    if (!activeRoom) return;
    try {
      await roomService.kickMember(activeRoom.id, userId);
      const updated = await roomService.getRoom(activeRoom.id);
      setActiveRoom(updated);
    } catch (err) {
      const msg = err instanceof APIError ? err.message : 'Failed to kick member.';
      setRoomError(msg);
      throw err;
    }
  }, [activeRoom]);

  const value = {
    activeRoom,
    rooms,
    userRole,
    permissions,
    isLoadingRoom,
    roomError,
    createRoom,
    joinRoom,
    leaveRoom,
    refreshRooms,
    updateMemberRole,
    kickMember,
    clearRoomError,
  };

  return <RoomContext.Provider value={value}>{children}</RoomContext.Provider>;
};
