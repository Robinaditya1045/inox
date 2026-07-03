-- Migration: Create rooms and room_members tables with granular RBAC permissions
CREATE TABLE IF NOT EXISTS rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_private BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rooms_owner_id ON rooms(owner_id);

-- room_members maps users to rooms with explicit granular permissions
CREATE TABLE IF NOT EXISTS room_members (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(30) NOT NULL DEFAULT 'member',
    can_control_playback BOOLEAN NOT NULL DEFAULT false,
    can_stream_audio BOOLEAN NOT NULL DEFAULT true,
    can_stream_video BOOLEAN NOT NULL DEFAULT true,
    can_share_screen BOOLEAN NOT NULL DEFAULT false,
    can_send_messages BOOLEAN NOT NULL DEFAULT true,
    can_invite_users BOOLEAN NOT NULL DEFAULT false,
    can_kick_users BOOLEAN NOT NULL DEFAULT false,
    can_manage_roles BOOLEAN NOT NULL DEFAULT false,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_room_members_user_id ON room_members(user_id);
