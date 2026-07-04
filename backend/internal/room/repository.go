package room

import (
	"context"
	"errors"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRoomNotFound   = errors.New("room not found")
	ErrMemberNotFound = errors.New("user is not a member of this room")
	ErrAlreadyMember  = errors.New("user is already a member of this room")
)

// RoomRepository defines the storage contract for rooms and RBAC member permissions.
type RoomRepository interface {
	CreateRoomWithOwner(ctx context.Context, room *domain.Room, ownerPerms domain.Permissions) (*domain.RoomMember, error)
	GetRoomByID(ctx context.Context, roomID string) (*domain.Room, error)
	GetMember(ctx context.Context, roomID, userID string) (*domain.RoomMember, error)
	AddMember(ctx context.Context, member *domain.RoomMember) error
	UpdateMemberPermissions(ctx context.Context, roomID, userID string, role domain.Role, perms domain.Permissions) (*domain.RoomMember, error)
	RemoveMember(ctx context.Context, roomID, userID string) error
	ListRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type postgresRoomRepository struct {
	db *pgxpool.Pool
}

func NewRoomRepository(db *pgxpool.Pool) RoomRepository {
	return &postgresRoomRepository{db: db}
}

// CreateRoomWithOwner atomically inserts the room record AND assigns the creator as Room Owner within a PostgreSQL transaction.
func (r *postgresRoomRepository) CreateRoomWithOwner(ctx context.Context, room *domain.Room, ownerPerms domain.Permissions) (*domain.RoomMember, error) {
	// 1. Begin atomic SQL transaction block
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Defer Rollback ensures changes are discarded if any query fails before Commit()
	defer tx.Rollback(ctx)

	// 2. Insert room record
	roomQuery := `
		INSERT INTO rooms (name, owner_id, is_private)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, roomQuery, room.Name, room.OwnerID, room.IsPrivate).Scan(
		&room.ID,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert room: %w", err)
	}

	// 3. Insert creator into room_members with RoleOwner and full permissions
	memberQuery := `
		INSERT INTO room_members (
			room_id, user_id, role,
			can_control_playback, can_stream_audio, can_stream_video,
			can_share_screen, can_send_messages, can_invite_users,
			can_kick_users, can_manage_roles
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING joined_at
	`
	member := &domain.RoomMember{
		RoomID:      room.ID,
		UserID:      room.OwnerID,
		Role:        domain.RoleOwner,
		Permissions: ownerPerms,
	}

	err = tx.QueryRow(ctx, memberQuery,
		member.RoomID, member.UserID, member.Role,
		ownerPerms.CanControlPlayback, ownerPerms.CanStreamAudio, ownerPerms.CanStreamVideo,
		ownerPerms.CanShareScreen, ownerPerms.CanSendMessages, ownerPerms.CanInviteUsers,
		ownerPerms.CanKickUsers, ownerPerms.CanManageRoles,
	).Scan(&member.JoinedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert room owner member row: %w", err)
	}

	// 4. Commit atomic transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit room creation transaction: %w", err)
	}

	return member, nil
}

// GetRoomByID queries a room by its UUID.
func (r *postgresRoomRepository) GetRoomByID(ctx context.Context, roomID string) (*domain.Room, error) {
	query := `SELECT id, name, owner_id, is_private, created_at, updated_at FROM rooms WHERE id = $1`
	var room domain.Room
	err := r.db.QueryRow(ctx, query, roomID).Scan(
		&room.ID, &room.Name, &room.OwnerID, &room.IsPrivate, &room.CreatedAt, &room.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		return nil, fmt.Errorf("failed to query room: %w", err)
	}
	return &room, nil
}

// GetMember fetches a participant's role and explicit boolean capabilities in a specific room.
func (r *postgresRoomRepository) GetMember(ctx context.Context, roomID, userID string) (*domain.RoomMember, error) {
	query := `
		SELECT room_id, user_id, role,
			can_control_playback, can_stream_audio, can_stream_video,
			can_share_screen, can_send_messages, can_invite_users,
			can_kick_users, can_manage_roles, joined_at
		FROM room_members
		WHERE room_id = $1 AND user_id = $2
	`
	var m domain.RoomMember
	err := r.db.QueryRow(ctx, query, roomID, userID).Scan(
		&m.RoomID, &m.UserID, &m.Role,
		&m.Permissions.CanControlPlayback, &m.Permissions.CanStreamAudio, &m.Permissions.CanStreamVideo,
		&m.Permissions.CanShareScreen, &m.Permissions.CanSendMessages, &m.Permissions.CanInviteUsers,
		&m.Permissions.CanKickUsers, &m.Permissions.CanManageRoles, &m.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to fetch room member: %w", err)
	}
	return &m, nil
}

// AddMember adds a user to a room with preset permissions.
func (r *postgresRoomRepository) AddMember(ctx context.Context, m *domain.RoomMember) error {
	query := `
		INSERT INTO room_members (
			room_id, user_id, role,
			can_control_playback, can_stream_audio, can_stream_video,
			can_share_screen, can_send_messages, can_invite_users,
			can_kick_users, can_manage_roles
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING joined_at
	`
	err := r.db.QueryRow(ctx, query,
		m.RoomID, m.UserID, m.Role,
		m.Permissions.CanControlPlayback, m.Permissions.CanStreamAudio, m.Permissions.CanStreamVideo,
		m.Permissions.CanShareScreen, m.Permissions.CanSendMessages, m.Permissions.CanInviteUsers,
		m.Permissions.CanKickUsers, m.Permissions.CanManageRoles,
	).Scan(&m.JoinedAt)
	if err != nil {
		return fmt.Errorf("failed to add member to room: %w", err)
	}
	return nil
}

// UpdateMemberPermissions updates a user's role and explicit boolean capability flags.
func (r *postgresRoomRepository) UpdateMemberPermissions(ctx context.Context, roomID, userID string, role domain.Role, p domain.Permissions) (*domain.RoomMember, error) {
	query := `
		UPDATE room_members
		SET role = $3,
			can_control_playback = $4, can_stream_audio = $5, can_stream_video = $6,
			can_share_screen = $7, can_send_messages = $8, can_invite_users = $9,
			can_kick_users = $10, can_manage_roles = $11
		WHERE room_id = $1 AND user_id = $2
		RETURNING joined_at
	`
	m := &domain.RoomMember{
		RoomID:      roomID,
		UserID:      userID,
		Role:        role,
		Permissions: p,
	}
	err := r.db.QueryRow(ctx, query,
		roomID, userID, role,
		p.CanControlPlayback, p.CanStreamAudio, p.CanStreamVideo,
		p.CanShareScreen, p.CanSendMessages, p.CanInviteUsers,
		p.CanKickUsers, p.CanManageRoles,
	).Scan(&m.JoinedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to update permissions: %w", err)
	}
	return m, nil
}

// RemoveMember removes or kicks a participant from a room.
func (r *postgresRoomRepository) RemoveMember(ctx context.Context, roomID, userID string) error {
	query := `DELETE FROM room_members WHERE room_id = $1 AND user_id = $2`
	res, err := r.db.Exec(ctx, query, roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete room member: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// ListRooms returns all public rooms and private rooms where the user is an owner or member.
func (r *postgresRoomRepository) ListRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	query := `
		SELECT DISTINCT r.id, r.name, r.owner_id, r.is_private, r.created_at, r.updated_at
		FROM rooms r
		LEFT JOIN room_members rm ON r.id = rm.room_id
		WHERE r.is_private = false OR r.owner_id = $1 OR rm.user_id = $1
		ORDER BY r.created_at DESC
		LIMIT 50
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms: %w", err)
	}
	defer rows.Close()

	var rooms []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		if err := rows.Scan(&room.ID, &room.Name, &room.OwnerID, &room.IsPrivate, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}
	if rooms == nil {
		rooms = []*domain.Room{}
	}
	return rooms, nil
}
