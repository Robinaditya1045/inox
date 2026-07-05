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
	ListRoomMembers(ctx context.Context, roomID string) ([]*domain.RoomMember, error)
	CreateInvitation(ctx context.Context, inv *domain.RoomInvitation) error
	GetInvitationByID(ctx context.Context, invID string) (*domain.RoomInvitation, error)
	ListPendingInvitationsForUser(ctx context.Context, userID string) ([]*domain.RoomInvitation, error)
	UpdateInvitationStatus(ctx context.Context, invID string, status domain.InvitationStatus) error
	UpdateRoomMediaURL(ctx context.Context, roomID, mediaURL string) error
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
	if room.CurrentMediaURL == "" {
		room.CurrentMediaURL = "https://media.w3.org/2010/05/bunny/movie.mp4"
	}
	roomQuery := `
		INSERT INTO rooms (name, owner_id, is_private, current_media_url)
		VALUES ($1, $2, $3, $4)
		RETURNING id, current_media_url, created_at, updated_at
	`
	err = tx.QueryRow(ctx, roomQuery, room.Name, room.OwnerID, room.IsPrivate, room.CurrentMediaURL).Scan(
		&room.ID,
		&room.CurrentMediaURL,
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
	query := `SELECT id, name, owner_id, is_private, COALESCE(current_media_url, 'https://media.w3.org/2010/05/bunny/movie.mp4'), created_at, updated_at FROM rooms WHERE id = $1`
	var room domain.Room
	err := r.db.QueryRow(ctx, query, roomID).Scan(
		&room.ID, &room.Name, &room.OwnerID, &room.IsPrivate, &room.CurrentMediaURL, &room.CreatedAt, &room.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		return nil, fmt.Errorf("failed to query room: %w", err)
	}
	members, err := r.ListRoomMembers(ctx, roomID)
	if err == nil {
		room.Members = members
	}
	return &room, nil
}

// GetMember fetches a participant's role and explicit boolean capabilities in a specific room.
func (r *postgresRoomRepository) GetMember(ctx context.Context, roomID, userID string) (*domain.RoomMember, error) {
	query := `
		SELECT rm.room_id, rm.user_id, u.username, rm.role,
			rm.can_control_playback, rm.can_stream_audio, rm.can_stream_video,
			rm.can_share_screen, rm.can_send_messages, rm.can_invite_users,
			rm.can_kick_users, rm.can_manage_roles, rm.joined_at
		FROM room_members rm
		JOIN users u ON rm.user_id = u.id
		WHERE rm.room_id = $1 AND rm.user_id = $2
	`
	var m domain.RoomMember
	err := r.db.QueryRow(ctx, query, roomID, userID).Scan(
		&m.RoomID, &m.UserID, &m.Username, &m.Role,
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

// ListRoomMembers returns all participants currently in a room with their usernames and permissions.
func (r *postgresRoomRepository) ListRoomMembers(ctx context.Context, roomID string) ([]*domain.RoomMember, error) {
	query := `
		SELECT rm.room_id, rm.user_id, u.username, rm.role,
			rm.can_control_playback, rm.can_stream_audio, rm.can_stream_video,
			rm.can_share_screen, rm.can_send_messages, rm.can_invite_users,
			rm.can_kick_users, rm.can_manage_roles, rm.joined_at
		FROM room_members rm
		JOIN users u ON rm.user_id = u.id
		WHERE rm.room_id = $1
		ORDER BY rm.joined_at ASC
	`
	rows, err := r.db.Query(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to query room members: %w", err)
	}
	defer rows.Close()

	var members []*domain.RoomMember
	for rows.Next() {
		var m domain.RoomMember
		if err := rows.Scan(
			&m.RoomID, &m.UserID, &m.Username, &m.Role,
			&m.Permissions.CanControlPlayback, &m.Permissions.CanStreamAudio, &m.Permissions.CanStreamVideo,
			&m.Permissions.CanShareScreen, &m.Permissions.CanSendMessages, &m.Permissions.CanInviteUsers,
			&m.Permissions.CanKickUsers, &m.Permissions.CanManageRoles, &m.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan room member: %w", err)
		}
		members = append(members, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating room members: %w", err)
	}
	return members, nil
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
		SELECT DISTINCT r.id, r.name, r.owner_id, r.is_private, COALESCE(r.current_media_url, 'https://media.w3.org/2010/05/bunny/movie.mp4'), r.created_at, r.updated_at
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
		if err := rows.Scan(&room.ID, &room.Name, &room.OwnerID, &room.IsPrivate, &room.CurrentMediaURL, &room.CreatedAt, &room.UpdatedAt); err != nil {
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
	for _, room := range rooms {
		members, err := r.ListRoomMembers(ctx, room.ID)
		if err == nil {
			room.Members = members
		}
	}
	return rooms, nil
}

// CreateInvitation inserts or updates an invitation record for a user to join a room.
func (r *postgresRoomRepository) CreateInvitation(ctx context.Context, inv *domain.RoomInvitation) error {
	query := `
		INSERT INTO room_invitations (room_id, inviter_id, invitee_id, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (room_id, invitee_id) DO UPDATE
		SET status = EXCLUDED.status,
		    inviter_id = EXCLUDED.inviter_id,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, inv.RoomID, inv.InviterID, inv.InviteeID, inv.Status).Scan(
		&inv.ID, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create room invitation: %w", err)
	}
	return nil
}

// GetInvitationByID retrieves an invitation along with room and user names.
func (r *postgresRoomRepository) GetInvitationByID(ctx context.Context, invID string) (*domain.RoomInvitation, error) {
	query := `
		SELECT i.id, i.room_id, r.name, i.inviter_id, u1.username, i.invitee_id, u2.username, i.status, i.created_at, i.updated_at
		FROM room_invitations i
		JOIN rooms r ON i.room_id = r.id
		JOIN users u1 ON i.inviter_id = u1.id
		JOIN users u2 ON i.invitee_id = u2.id
		WHERE i.id = $1
	`
	var inv domain.RoomInvitation
	err := r.db.QueryRow(ctx, query, invID).Scan(
		&inv.ID, &inv.RoomID, &inv.RoomName, &inv.InviterID, &inv.InviterName, &inv.InviteeID, &inv.InviteeName, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("invitation not found")
		}
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	return &inv, nil
}

// ListPendingInvitationsForUser lists all pending invitations for a specific user.
func (r *postgresRoomRepository) ListPendingInvitationsForUser(ctx context.Context, userID string) ([]*domain.RoomInvitation, error) {
	query := `
		SELECT i.id, i.room_id, r.name, i.inviter_id, u1.username, i.invitee_id, u2.username, i.status, i.created_at, i.updated_at
		FROM room_invitations i
		JOIN rooms r ON i.room_id = r.id
		JOIN users u1 ON i.inviter_id = u1.id
		JOIN users u2 ON i.invitee_id = u2.id
		WHERE i.invitee_id = $1 AND i.status = 'pending'
		ORDER BY i.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user invitations: %w", err)
	}
	defer rows.Close()

	var invs []*domain.RoomInvitation
	for rows.Next() {
		inv := &domain.RoomInvitation{}
		if err := rows.Scan(&inv.ID, &inv.RoomID, &inv.RoomName, &inv.InviterID, &inv.InviterName, &inv.InviteeID, &inv.InviteeName, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan invitation: %w", err)
		}
		invs = append(invs, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}
	if invs == nil {
		invs = []*domain.RoomInvitation{}
	}
	return invs, nil
}

// UpdateInvitationStatus updates the status of an invitation.
func (r *postgresRoomRepository) UpdateInvitationStatus(ctx context.Context, invID string, status domain.InvitationStatus) error {
	query := `UPDATE room_invitations SET status = $2, updated_at = NOW() WHERE id = $1`
	res, err := r.db.Exec(ctx, query, invID, status)
	if err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("invitation not found")
	}
	return nil
}

// UpdateRoomMediaURL updates the current media url of a room.
func (r *postgresRoomRepository) UpdateRoomMediaURL(ctx context.Context, roomID, mediaURL string) error {
	query := `UPDATE rooms SET current_media_url = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, roomID, mediaURL)
	if err != nil {
		return fmt.Errorf("failed to update room media url: %w", err)
	}
	return nil
}

