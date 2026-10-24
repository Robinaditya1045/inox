import { createContext } from "react";
import type {
  Room,
  CreateRoomRequest,
  RoomRole,
  RoomPermissions,
  RoomInvitation,
  WSPresenceUser,
} from "../types";

export interface RoomContextValue {
  activeRoom: Room | null;
  /**
   * Everyone connected to the active room right now, from the server's PRESENCE
   * list. Null until it arrives.
   */
  onlineUsers: WSPresenceUser[] | null;
  /** Re-read the active room's member list (roles, joins, departures). */
  refreshMembers: () => Promise<void>;
  rooms: Room[];
  invitations: RoomInvitation[];
  userRole: RoomRole | null;
  permissions: RoomPermissions | null;
  /** Join/create/accept in flight. Not the room list: see hasLoadedRooms. */
  isLoadingRoom: boolean;
  /** The room list has been fetched at least once for the current user. */
  hasLoadedRooms: boolean;
  roomError: string | null;
  createRoom: (data: CreateRoomRequest) => Promise<Room>;
  joinRoom: (roomId: string) => Promise<Room>;
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
