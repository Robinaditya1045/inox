import { createContext } from "react";
import type {
  Room,
  CreateRoomRequest,
  RoomRole,
  RoomPermissions,
  RoomInvitation,
  RoomJoinRefusal,
} from "../types";

export interface RoomContextValue {
  activeRoom: Room | null;
  rooms: Room[];
  invitations: RoomInvitation[];
  userRole: RoomRole | null;
  permissions: RoomPermissions | null;
  /** Join/create/accept in flight. Not the room list: see hasLoadedRooms. */
  isLoadingRoom: boolean;
  /** The room list has been fetched at least once for the current user. */
  hasLoadedRooms: boolean;
  roomError: string | null;
  /** Set alongside roomError when a join was refused for a reason the UI can act
   *  on, so it can prompt for a password instead of only reporting the failure. */
  roomErrorCode: RoomJoinRefusal | null;
  createRoom: (data: CreateRoomRequest) => Promise<Room>;
  joinRoom: (roomId: string, password?: string) => Promise<Room>;
  leaveRoom: () => Promise<void>;
  /** Close the room connection and hang up, keeping membership (for logout). */
  disconnectFromRoom: () => void;
  refreshRooms: () => Promise<void>;
  updateMemberRole: (userId: string, role: RoomRole) => Promise<void>;
  kickMember: (userId: string) => Promise<void>;
  inviteUser: (username: string) => Promise<RoomInvitation>;
  acceptInvitation: (invId: string) => Promise<Room>;
  declineInvitation: (invId: string) => Promise<void>;
  refreshInvitations: () => Promise<void>;
  clearRoomError: () => void;
}

export const RoomContext = createContext<RoomContextValue | null>(null);
