package room

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrInvalidRoomName  = errors.New("room name must be between 3 and 100 characters")
	ErrPermissionDenied = errors.New("you do not have permission to perform this action")
	ErrCannotAlterOwner = errors.New("cannot demote, modify permissions, or kick the room owner")
)

// RoomService defines the business logic contract for room management and RBAC governance.
type RoomService interface {
	CreateRoom(ctx context.Context, ownerID, name string, isPrivate bool) (*domain.Room, *domain.RoomMember, error)
	JoinRoom(ctx context.Context, roomID, userID string) (*domain.RoomMember, error)
	GetRoomAndMember(ctx context.Context, roomID, userID string) (*domain.Room, *domain.RoomMember, error)
	AssignRole(ctx context.Context, roomID, requesterID, targetUserID string, newRole domain.Role, customPerms *domain.Permissions) (*domain.RoomMember, error)
	KickMember(ctx context.Context, roomID, requesterID, targetUserID string) error
	CheckPlaybackPermission(ctx context.Context, roomID, userID string) error
	ListRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type roomService struct {
	repo RoomRepository
}

func NewRoomService(repo RoomRepository) RoomService {
	return &roomService{repo: repo}
}

// CreateRoom validates input and atomically creates a room with the creator as RoleOwner.
func (s *roomService) CreateRoom(ctx context.Context, ownerID, name string, isPrivate bool) (*domain.Room, *domain.RoomMember, error) {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 100 {
		return nil, nil, ErrInvalidRoomName
	}

	room := &domain.Room{
		Name:      name,
		OwnerID:   ownerID,
		IsPrivate: isPrivate,
	}

	ownerPerms := domain.DefaultPermissionsForRole(domain.RoleOwner)

	member, err := s.repo.CreateRoomWithOwner(ctx, room, ownerPerms)
	if err != nil {
		return nil, nil, fmt.Errorf("service failed to create room: %w", err)
	}

	return room, member, nil
}

// JoinRoom adds a user to a room as a RoleMember if they are not already joined.
func (s *roomService) JoinRoom(ctx context.Context, roomID, userID string) (*domain.RoomMember, error) {
	// Check if already joined
	existing, err := s.repo.GetMember(ctx, roomID, userID)
	if err == nil && existing != nil {
		return existing, nil // Already in room, return existing membership
	}
	if !errors.Is(err, ErrMemberNotFound) {
		return nil, fmt.Errorf("failed checking existing membership: %w", err)
	}

	member := &domain.RoomMember{
		RoomID:      roomID,
		UserID:      userID,
		Role:        domain.RoleMember,
		Permissions: domain.DefaultPermissionsForRole(domain.RoleMember),
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("service failed to join room: %w", err)
	}

	return member, nil
}

// GetRoomAndMember retrieves the workspace details along with the requester's permission matrix.
func (s *roomService) GetRoomAndMember(ctx context.Context, roomID, userID string) (*domain.Room, *domain.RoomMember, error) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, nil, err
	}

	member, err := s.repo.GetMember(ctx, roomID, userID)
	if err != nil {
		return nil, nil, err
	}

	return room, member, nil
}

// AssignRole allows room owners or authorized role managers to update a participant's role and explicit capability flags.
func (s *roomService) AssignRole(ctx context.Context, roomID, requesterID, targetUserID string, newRole domain.Role, customPerms *domain.Permissions) (*domain.RoomMember, error) {
	// 1. Verify requester has CanManageRoles permission
	requester, err := s.repo.GetMember(ctx, roomID, requesterID)
	if err != nil {
		return nil, fmt.Errorf("failed verifying requester role: %w", err)
	}
	if !requester.Permissions.CanManageRoles {
		return nil, ErrPermissionDenied
	}

	// 2. Fetch target participant
	target, err := s.repo.GetMember(ctx, roomID, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("failed verifying target member: %w", err)
	}

	// 3. Prevent modifying the Room Owner!
	if target.Role == domain.RoleOwner {
		return nil, ErrCannotAlterOwner
	}

	// 4. Calculate new permissions
	var perms domain.Permissions
	if customPerms != nil {
		perms = *customPerms
	} else {
		perms = domain.DefaultPermissionsForRole(newRole)
	}

	updated, err := s.repo.UpdateMemberPermissions(ctx, roomID, targetUserID, newRole, perms)
	if err != nil {
		return nil, fmt.Errorf("failed to update member role: %w", err)
	}

	return updated, nil
}

// KickMember removes a participant after verifying the requester holds CanKickUsers permission.
func (s *roomService) KickMember(ctx context.Context, roomID, requesterID, targetUserID string) error {
	requester, err := s.repo.GetMember(ctx, roomID, requesterID)
	if err != nil {
		return err
	}
	if !requester.Permissions.CanKickUsers {
		return ErrPermissionDenied
	}

	target, err := s.repo.GetMember(ctx, roomID, targetUserID)
	if err != nil {
		return err
	}
	if target.Role == domain.RoleOwner {
		return ErrCannotAlterOwner
	}

	return s.repo.RemoveMember(ctx, roomID, targetUserID)
}

// CheckPlaybackPermission verifies if a user is permitted to pause, play, or seek video playback.
func (s *roomService) CheckPlaybackPermission(ctx context.Context, roomID, userID string) error {
	member, err := s.repo.GetMember(ctx, roomID, userID)
	if err != nil {
		return err
	}
	if !member.Permissions.CanControlPlayback {
		return ErrPermissionDenied
	}
	return nil
}

// ListRooms retrieves all public rooms and private rooms where the user is an owner or member.
func (s *roomService) ListRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return s.repo.ListRooms(ctx, userID)
}
