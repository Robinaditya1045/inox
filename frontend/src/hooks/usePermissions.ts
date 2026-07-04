import { useRoom } from './useRoom';
import type { RoomRole, RoomPermissions } from '../types';

export interface PermissionsState extends RoomPermissions {
  role: RoomRole | null;
  isOwner: boolean;
  isModerator: boolean;
  isMember: boolean;
  isGuest: boolean;
}

export const usePermissions = (): PermissionsState => {
  const { userRole, permissions } = useRoom();

  const base: RoomPermissions = permissions || {
    can_control_playback: false,
    can_stream_audio: false,
    can_stream_video: false,
    can_share_screen: false,
    can_send_messages: false,
    can_invite_users: false,
    can_kick_users: false,
    can_manage_roles: false,
  };

  return {
    ...base,
    role: userRole,
    isOwner: userRole === 'owner',
    isModerator: userRole === 'moderator',
    isMember: userRole === 'member',
    isGuest: userRole === 'guest',
  };
};
