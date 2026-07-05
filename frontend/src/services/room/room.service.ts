import { apiClient } from '../../api/client';
import type { Room, CreateRoomRequest, RoomRole, RoomPermissions, RoomInvitation } from '../../types';
import { logger } from '../../utils/logger';

export const DEFAULT_PERMISSIONS: Record<RoomRole, RoomPermissions> = {
  owner: {
    can_control_playback: true,
    can_stream_audio: true,
    can_stream_video: true,
    can_share_screen: true,
    can_send_messages: true,
    can_invite_users: true,
    can_kick_users: true,
    can_manage_roles: true,
  },
  moderator: {
    can_control_playback: true,
    can_stream_audio: true,
    can_stream_video: true,
    can_share_screen: true,
    can_send_messages: true,
    can_invite_users: true,
    can_kick_users: true,
    can_manage_roles: false,
  },
  member: {
    can_control_playback: true,
    can_stream_audio: true,
    can_stream_video: true,
    can_share_screen: true,
    can_send_messages: true,
    can_invite_users: false,
    can_kick_users: false,
    can_manage_roles: false,
  },
  guest: {
    can_control_playback: false,
    can_stream_audio: false,
    can_stream_video: false,
    can_share_screen: false,
    can_send_messages: true,
    can_invite_users: false,
    can_kick_users: false,
    can_manage_roles: false,
  },
};

export const roomService = {
  async createRoom(data: CreateRoomRequest): Promise<Room> {
    logger.info('RoomService: Creating room', { name: data.name, is_private: data.is_private });
    const response = await apiClient.post<Room | { room: Room }>('/rooms', data);
    if (response && 'room' in response && response.room) {
      return response.room;
    }
    return response as Room;
  },

  async joinRoom(roomId: string): Promise<Room> {
    logger.info('RoomService: Joining room', { roomId });
    await apiClient.post(`/rooms/${roomId}/join`);
    return this.getRoom(roomId);
  },

  async getRoom(roomId: string): Promise<Room> {
    logger.debug('RoomService: Fetching room details', { roomId });
    const response = await apiClient.get<Room | { room: Room }>(`/rooms/${roomId}`);
    if (response && 'room' in response && response.room) {
      return response.room;
    }
    return response as Room;
  },

  async listRooms(): Promise<Room[]> {
    try {
      logger.debug('RoomService: Listing public rooms');
      const response = await apiClient.get<Room[] | { rooms: Room[] }>('/rooms');
      if (Array.isArray(response)) {
        return response;
      }
      if (response && 'rooms' in response && Array.isArray(response.rooms)) {
        return response.rooms;
      }
      return [];
    } catch (error) {
      logger.warn('RoomService: Failed to list rooms', { error });
      return [];
    }
  },

  async updateMemberRole(roomId: string, userId: string, role: RoomRole): Promise<void> {
    logger.info('RoomService: Updating member role', { roomId, userId, role });
    await apiClient.put<void>(`/rooms/${roomId}/members/${userId}/role`, { role });
  },

  async kickMember(roomId: string, userId: string): Promise<void> {
    logger.info('RoomService: Kicking member', { roomId, userId });
    await apiClient.delete<void>(`/rooms/${roomId}/members/${userId}`);
  },

  async inviteUser(roomId: string, username: string): Promise<RoomInvitation> {
    logger.info('RoomService: Inviting user', { roomId, username });
    return apiClient.post<RoomInvitation>(`/rooms/${roomId}/invite`, { username });
  },

  async getPendingInvitations(): Promise<RoomInvitation[]> {
    try {
      logger.debug('RoomService: Fetching pending invitations');
      const response = await apiClient.get<RoomInvitation[]>('/invitations');
      return Array.isArray(response) ? response : [];
    } catch (error) {
      logger.warn('RoomService: Failed to fetch invitations', { error });
      return [];
    }
  },

  async acceptInvitation(invId: string): Promise<{ room: Room; member: any }> {
    logger.info('RoomService: Accepting invitation', { invId });
    return apiClient.post<{ room: Room; member: any }>(`/invitations/${invId}/accept`);
  },

  async declineInvitation(invId: string): Promise<void> {
    logger.info('RoomService: Declining invitation', { invId });
    await apiClient.post<void>(`/invitations/${invId}/decline`);
  },
};
