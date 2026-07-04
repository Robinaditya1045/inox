import { createContext } from 'react';
import type { Room, CreateRoomRequest, RoomRole, RoomPermissions } from '../types';

export interface RoomContextValue {
  activeRoom: Room | null;
  rooms: Room[];
  userRole: RoomRole | null;
  permissions: RoomPermissions | null;
  isLoadingRoom: boolean;
  roomError: string | null;
  createRoom: (data: CreateRoomRequest) => Promise<Room>;
  joinRoom: (roomId: string) => Promise<Room>;
  leaveRoom: () => Promise<void>;
  refreshRooms: () => Promise<void>;
  updateMemberRole: (userId: string, role: RoomRole) => Promise<void>;
  kickMember: (userId: string) => Promise<void>;
  clearRoomError: () => void;
}

export const RoomContext = createContext<RoomContextValue | null>(null);
