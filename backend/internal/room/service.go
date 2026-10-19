package room

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrInvalidRoomName  = errors.New("room name must be between 3 and 100 characters")
	ErrPermissionDenied = errors.New("you do not have permission to perform this action")
	ErrCannotAlterOwner = errors.New("cannot demote, modify permissions, or kick the room owner")

	ErrInvalidRoomPassword   = errors.New("room password must be between 4 and 128 characters")
	ErrRoomPrivate           = errors.New("this room is private: you need an invitation to join it")
	ErrRoomPasswordRequired  = errors.New("this room needs a password")
	ErrRoomPasswordIncorrect = errors.New("that room password is not right")
)

// A room password is a shared secret people type, not a login credential, so the
// floor is low enough to be memorable and the ceiling only exists to bound the
// work Argon2 is asked to do.
const (
	minRoomPasswordLength = 4
	maxRoomPasswordLength = 128
)

// RoomService defines the business logic contract for room management and RBAC governance.
type RoomService interface {
	CreateRoom(ctx context.Context, ownerID, name string, isPrivate bool, password string) (*domain.Room, *domain.RoomMember, error)
	JoinRoom(ctx context.Context, roomID, userID, password string) (*domain.RoomMember, error)
	GetRoomAndMember(ctx context.Context, roomID, userID string) (*domain.Room, *domain.RoomMember, error)
	AssignRole(ctx context.Context, roomID, requesterID, targetUserID string, newRole domain.Role, customPerms *domain.Permissions) (*domain.RoomMember, error)
	KickMember(ctx context.Context, roomID, requesterID, targetUserID string) error
	CheckPlaybackPermission(ctx context.Context, roomID, userID string) error
	ListRooms(ctx context.Context, userID string) ([]*domain.Room, error)
	InviteUserByUsername(ctx context.Context, roomID, inviterID, inviteeUsername string) (*domain.RoomInvitation, error)
	ListUserPendingInvitations(ctx context.Context, userID string) ([]*domain.RoomInvitation, error)
	RespondToInvitation(ctx context.Context, invID, userID string, accept bool) (*domain.Room, *domain.RoomMember, error)
	GetRoomByID(ctx context.Context, roomID string) (*domain.Room, error)
	UpdateRoomMediaURL(ctx context.Context, roomID, mediaURL string) error
	DeleteRoom(ctx context.Context, roomID, requesterID string) error
	LeaveRoom(ctx context.Context, roomID, userID string) error
}

type roomService struct {
	repo      RoomRepository
	userRepo  auth.UserRepository
	stateRepo StateRepository
}

func NewRoomService(repo RoomRepository, userRepo auth.UserRepository, stateRepo StateRepository) RoomService {
	return &roomService{repo: repo, userRepo: userRepo, stateRepo: stateRepo}
}

// CreateRoom validates input and atomically creates a room with the creator as RoleOwner.
// An empty password means the room has none.
func (s *roomService) CreateRoom(ctx context.Context, ownerID, name string, isPrivate bool, password string) (*domain.Room, *domain.RoomMember, error) {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 100 {
		return nil, nil, ErrInvalidRoomName
	}

	// Trimmed because this gets copied out of a chat message as often as it gets
	// typed, and a trailing space nobody can see would lock everyone out.
	password = strings.TrimSpace(password)
	if password != "" && (len(password) < minRoomPasswordLength || len(password) > maxRoomPasswordLength) {
		return nil, nil, ErrInvalidRoomPassword
	}

	room := &domain.Room{
		Name:      name,
		OwnerID:   ownerID,
		IsPrivate: isPrivate,
	}

	if password != "" {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to hash room password: %w", err)
		}
		room.PasswordHash = hash
		room.HasPassword = true
	}

	ownerPerms := domain.DefaultPermissionsForRole(domain.RoleOwner)

	member, err := s.repo.CreateRoomWithOwner(ctx, room, ownerPerms)
	if err != nil {
		return nil, nil, fmt.Errorf("service failed to create room: %w", err)
	}

	return room, member, nil
}

// JoinRoom adds a user to a room as a RoleMember if they are not already joined,
// once authorizeJoin is satisfied that they are allowed in at all.
func (s *roomService) JoinRoom(ctx context.Context, roomID, userID, password string) (*domain.RoomMember, error) {
	// Check if already joined
	existing, err := s.repo.GetMember(ctx, roomID, userID)
	if err == nil && existing != nil {
		return existing, nil // Already in room, return existing membership
	}
	if !errors.Is(err, ErrMemberNotFound) {
		return nil, fmt.Errorf("failed checking existing membership: %w", err)
	}

	rm, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeJoin(ctx, rm, userID, password); err != nil {
		return nil, err
	}

	member := &domain.RoomMember{
		RoomID:      roomID,
		UserID:      userID,
		Role:        domain.RoleMember,
		Permissions: domain.DefaultPermissionsForRole(domain.RoleMember),
	}

	member, err = s.addMemberOrGetExisting(ctx, member)
	if err != nil {
		return nil, fmt.Errorf("service failed to join room: %w", err)
	}
	return member, nil
}

// authorizeJoin decides whether userID may let themselves into rm with the
// credential they presented.
//
// The owner and anyone holding an invitation are always allowed. For everyone
// else the room's password is the credential, and a private room that has no
// password refuses the link outright: is_private used to do nothing but hide a
// room from the lobby, so its URL was enough to get in, which made "private" a
// promise the backend did not keep.
func (s *roomService) authorizeJoin(ctx context.Context, rm *domain.Room, userID, password string) error {
	if rm.OwnerID == userID {
		return nil
	}

	invited, err := s.repo.HasInvitation(ctx, rm.ID, userID)
	if err != nil {
		return fmt.Errorf("failed checking room invitation: %w", err)
	}
	if invited {
		return nil
	}

	if rm.PasswordHash != "" {
		if password == "" {
			return ErrRoomPasswordRequired
		}
		ok, err := auth.VerifyPassword(password, rm.PasswordHash)
		if err != nil {
			// A hash that will not parse is an operator problem, not a wrong
			// password. The caller is refused either way, but this one should be
			// visible somewhere other than their screen.
			slog.Error("failed to verify room password", "room_id", rm.ID, "error", err)
			return ErrRoomPasswordIncorrect
		}
		if !ok {
			return ErrRoomPasswordIncorrect
		}
		return nil
	}

	if rm.IsPrivate {
		return ErrRoomPrivate
	}
	return nil
}

// addMemberOrGetExisting inserts a membership, and if a concurrent request got
// there first returns that one instead of failing. The membership check before
// the insert cannot rule this out on its own: two joins for the same user (a
// double click, two tabs, React StrictMode running an effect twice) can both pass
// the check before either inserts, and the loser used to surface as a 500.
func (s *roomService) addMemberOrGetExisting(ctx context.Context, m *domain.RoomMember) (*domain.RoomMember, error) {
	err := s.repo.AddMember(ctx, m)
	if err == nil {
		return m, nil
	}
	if errors.Is(err, ErrAlreadyMember) {
		return s.repo.GetMember(ctx, m.RoomID, m.UserID)
	}
	return nil, err
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

	if err := s.repo.RemoveMember(ctx, roomID, targetUserID); err != nil {
		return err
	}

	if s.stateRepo != nil {
		_ = s.stateRepo.InvalidateMemberVerification(ctx, roomID, targetUserID)
	}
	return nil
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

// InviteUserByUsername invites a user by username into a room.
func (s *roomService) InviteUserByUsername(ctx context.Context, roomID, inviterID, inviteeUsername string) (*domain.RoomInvitation, error) {
	// 1. Verify inviter is a member and has permission to invite users
	_, inviterMember, err := s.GetRoomAndMember(ctx, roomID, inviterID)
	if err != nil {
		return nil, err
	}
	if !inviterMember.Permissions.CanInviteUsers {
		return nil, ErrPermissionDenied
	}

	// 2. Look up invitee by username
	if s.userRepo == nil {
		return nil, errors.New("user repository not initialized")
	}
	invitee, err := s.userRepo.GetByUsername(ctx, strings.TrimSpace(inviteeUsername))
	if err != nil {
		return nil, errors.New("user not found with that username")
	}

	if invitee.ID == inviterID {
		return nil, errors.New("cannot invite yourself")
	}

	// 3. Check if invitee is already a member of the room
	_, err = s.repo.GetMember(ctx, roomID, invitee.ID)
	if err == nil {
		return nil, errors.New("user is already a member of this room")
	}

	// 4. Create invitation
	inv := &domain.RoomInvitation{
		RoomID:    roomID,
		InviterID: inviterID,
		InviteeID: invitee.ID,
		Status:    domain.InvitationStatusPending,
	}
	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}

	// Fetch full invitation details (with usernames and room name)
	return s.repo.GetInvitationByID(ctx, inv.ID)
}

// ListUserPendingInvitations lists all pending invitations for a specific user.
func (s *roomService) ListUserPendingInvitations(ctx context.Context, userID string) ([]*domain.RoomInvitation, error) {
	return s.repo.ListPendingInvitationsForUser(ctx, userID)
}

// RespondToInvitation handles accepting or declining an invitation.
func (s *roomService) RespondToInvitation(ctx context.Context, invID, userID string, accept bool) (*domain.Room, *domain.RoomMember, error) {
	inv, err := s.repo.GetInvitationByID(ctx, invID)
	if err != nil {
		return nil, nil, err
	}
	if inv.InviteeID != userID {
		return nil, nil, errors.New("unauthorized: invitation belongs to another user")
	}
	if inv.Status != domain.InvitationStatusPending {
		return nil, nil, errors.New("invitation is no longer pending")
	}

	if !accept {
		if err := s.repo.UpdateInvitationStatus(ctx, invID, domain.InvitationStatusDeclined); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}

	// Accept invitation
	if err := s.repo.UpdateInvitationStatus(ctx, invID, domain.InvitationStatusAccepted); err != nil {
		return nil, nil, err
	}

	// Check if already a member just in case
	rm, member, err := s.GetRoomAndMember(ctx, inv.RoomID, userID)
	if err == nil {
		return rm, member, nil
	}

	// Add user to room as member
	newMember := &domain.RoomMember{
		RoomID:      inv.RoomID,
		UserID:      userID,
		Role:        domain.RoleMember,
		Permissions: domain.DefaultPermissionsForRole(domain.RoleMember),
	}
	newMember, err = s.addMemberOrGetExisting(ctx, newMember)
	if err != nil {
		return nil, nil, err
	}

	roomObj, err := s.repo.GetRoomByID(ctx, inv.RoomID)
	if err != nil {
		return nil, nil, err
	}
	return roomObj, newMember, nil
}

func (s *roomService) UpdateRoomMediaURL(ctx context.Context, roomID, mediaURL string) error {
	return s.repo.UpdateRoomMediaURL(ctx, roomID, mediaURL)
}

func (s *roomService) GetRoomByID(ctx context.Context, roomID string) (*domain.Room, error) {
	return s.repo.GetRoomByID(ctx, roomID)
}

func (s *roomService) DeleteRoom(ctx context.Context, roomID, requesterID string) error {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room.OwnerID != requesterID {
		return ErrPermissionDenied
	}
	return s.repo.DeleteRoom(ctx, roomID)
}

func (s *roomService) LeaveRoom(ctx context.Context, roomID, userID string) error {
	member, err := s.repo.GetMember(ctx, roomID, userID)
	if err != nil {
		return err
	}
	if member.Role == domain.RoleOwner {
		return ErrCannotAlterOwner
	}
	if err := s.repo.RemoveMember(ctx, roomID, userID); err != nil {
		return err
	}
	if s.stateRepo != nil {
		_ = s.stateRepo.InvalidateMemberVerification(ctx, roomID, userID)
	}
	return nil
}
