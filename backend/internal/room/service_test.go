package room_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
)

type mockRoomRepository struct {
	rooms   map[string]*domain.Room
	members map[string]*domain.RoomMember // key: roomID + ":" + userID
}

func newMockRoomRepository() *mockRoomRepository {
	return &mockRoomRepository{
		rooms:   make(map[string]*domain.Room),
		members: make(map[string]*domain.RoomMember),
	}
}

func (m *mockRoomRepository) CreateRoomWithOwner(ctx context.Context, r *domain.Room, ownerPerms domain.Permissions) (*domain.RoomMember, error) {
	r.ID = "room-mock-uuid-101"
	r.CreatedAt = time.Now()
	m.rooms[r.ID] = r

	member := &domain.RoomMember{
		RoomID:      r.ID,
		UserID:      r.OwnerID,
		Role:        domain.RoleOwner,
		Permissions: ownerPerms,
		JoinedAt:    time.Now(),
	}
	m.members[r.ID+":"+r.OwnerID] = member
	return member, nil
}

func (m *mockRoomRepository) GetRoomByID(ctx context.Context, roomID string) (*domain.Room, error) {
	r, ok := m.rooms[roomID]
	if !ok {
		return nil, room.ErrRoomNotFound
	}
	return r, nil
}

func (m *mockRoomRepository) GetRoomOwnerID(ctx context.Context, roomID string) (string, error) {
	r, ok := m.rooms[roomID]
	if !ok {
		return "", room.ErrRoomNotFound
	}
	return r.OwnerID, nil
}

func (m *mockRoomRepository) GetMember(ctx context.Context, roomID, userID string) (*domain.RoomMember, error) {
	mem, ok := m.members[roomID+":"+userID]
	if !ok {
		return nil, room.ErrMemberNotFound
	}
	return mem, nil
}

func (m *mockRoomRepository) AddMember(ctx context.Context, mem *domain.RoomMember) error {
	m.members[mem.RoomID+":"+mem.UserID] = mem
	return nil
}

func (m *mockRoomRepository) UpdateMemberPermissions(ctx context.Context, roomID, userID string, role domain.Role, p domain.Permissions) (*domain.RoomMember, error) {
	mem, ok := m.members[roomID+":"+userID]
	if !ok {
		return nil, room.ErrMemberNotFound
	}
	mem.Role = role
	mem.Permissions = p
	return mem, nil
}

func (m *mockRoomRepository) RemoveMember(ctx context.Context, roomID, userID string) error {
	key := roomID + ":" + userID
	if _, ok := m.members[key]; !ok {
		return room.ErrMemberNotFound
	}
	delete(m.members, key)
	return nil
}

func (m *mockRoomRepository) ListRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return nil, nil
}
func (m *mockRoomRepository) ListRoomMembers(ctx context.Context, roomID string) ([]*domain.RoomMember, error) {
	var list []*domain.RoomMember
	for _, mem := range m.members {
		if mem.RoomID == roomID {
			list = append(list, mem)
		}
	}
	return list, nil
}
func (m *mockRoomRepository) CreateInvitation(ctx context.Context, inv *domain.RoomInvitation) error {
	return nil
}
func (m *mockRoomRepository) GetInvitationByID(ctx context.Context, invID string) (*domain.RoomInvitation, error) {
	return nil, nil
}
func (m *mockRoomRepository) ListPendingInvitationsForUser(ctx context.Context, userID string) ([]*domain.RoomInvitation, error) {
	return nil, nil
}
func (m *mockRoomRepository) UpdateInvitationStatus(ctx context.Context, invID string, status domain.InvitationStatus) error {
	return nil
}
func (m *mockRoomRepository) UpdateRoomMediaURL(ctx context.Context, roomID, mediaURL string) error {
	if r, ok := m.rooms[roomID]; ok {
		r.CurrentMediaURL = mediaURL
	}
	return nil
}

func (m *mockRoomRepository) DeleteRoom(ctx context.Context, roomID string) error {
	delete(m.rooms, roomID)
	return nil
}

func TestRoomCreationAndRBACWorkflow(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)

	ownerID := "user-owner-1"
	viewerID := "user-viewer-2"

	// 1. Create Room
	rm, ownerMem, err := service.CreateRoom(ctx, ownerID, "Friday Movie Night", false)
	if err != nil {
		t.Fatalf("expected successful room creation, got %v", err)
	}
	if !ownerMem.Permissions.CanControlPlayback {
		t.Errorf("expected room owner to have CanControlPlayback = true")
	}

	// 2. Viewer Joins Room
	viewerMem, err := service.JoinRoom(ctx, rm.ID, viewerID)
	if err != nil {
		t.Fatalf("expected successful room join, got %v", err)
	}
	if viewerMem.Permissions.CanControlPlayback {
		t.Errorf("expected standard viewer member to have CanControlPlayback = false by default")
	}

	// 3. Verify Viewer Cannot Pause Video
	err = service.CheckPlaybackPermission(ctx, rm.ID, viewerID)
	if !errors.Is(err, room.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied when viewer tries to pause video, got %v", err)
	}

	// 4. Room Owner Grants Viewer Permission to Control Playback
	customPerms := domain.DefaultPermissionsForRole(domain.RoleMember)
	customPerms.CanControlPlayback = true

	updatedViewer, err := service.AssignRole(ctx, rm.ID, ownerID, viewerID, domain.RoleMember, &customPerms)
	if err != nil {
		t.Fatalf("expected successful role assignment, got %v", err)
	}
	if !updatedViewer.Permissions.CanControlPlayback {
		t.Errorf("expected updated viewer to now have CanControlPlayback = true")
	}

	// 5. Verify Viewer Can Now Control Playback
	err = service.CheckPlaybackPermission(ctx, rm.ID, viewerID)
	if err != nil {
		t.Errorf("expected playback check to pass after permission update, got %v", err)
	}

	// 6. Test Privilege Escalation Prevention: Viewer attempts to demote Room Owner
	_, err = service.AssignRole(ctx, rm.ID, viewerID, ownerID, domain.RoleGuest, nil)
	if !errors.Is(err, room.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied when viewer tries to alter owner, got %v", err)
	}
}

// racingJoinRepository reproduces two joins for the same user landing together:
// the membership check misses (the other request has not inserted yet), then the
// insert collides with the row the other request just wrote.
type racingJoinRepository struct {
	*mockRoomRepository
	winner *domain.RoomMember
}

func (r *racingJoinRepository) AddMember(ctx context.Context, mem *domain.RoomMember) error {
	r.members[mem.RoomID+":"+mem.UserID] = r.winner
	return room.ErrAlreadyMember
}

func TestJoinRoomReturnsExistingMembershipWhenAConcurrentJoinWins(t *testing.T) {
	ctx := context.Background()
	winner := &domain.RoomMember{RoomID: "room-1", UserID: "user-2", Role: domain.RoleMember, JoinedAt: time.Now()}
	repo := &racingJoinRepository{mockRoomRepository: newMockRoomRepository(), winner: winner}
	service := room.NewRoomService(repo, nil, nil)

	got, err := service.JoinRoom(ctx, "room-1", "user-2")
	if err != nil {
		t.Fatalf("losing a join race must not be an error, got %v", err)
	}
	if got != winner {
		t.Fatalf("expected the membership the concurrent join created, got %+v", got)
	}
}

// newCoOwnedRoom creates a room owned by "creator" and joins each of the other IDs as members.
func newCoOwnedRoom(t *testing.T, others ...string) (room.RoomService, string) {
	t.Helper()
	ctx := context.Background()
	service := room.NewRoomService(newMockRoomRepository(), nil, nil)
	rm, _, err := service.CreateRoom(ctx, "creator", "Shared Movie Night", false)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	for _, id := range others {
		if _, err := service.JoinRoom(ctx, rm.ID, id); err != nil {
			t.Fatalf("join %s: %v", id, err)
		}
	}
	return service, rm.ID
}

func TestOwnerCanAddCoOwnerWithFullOwnerPermissions(t *testing.T) {
	ctx := context.Background()
	service, roomID := newCoOwnedRoom(t, "co-owner", "viewer")

	// Custom permissions are ignored for owners, so ownership cannot be handed out hollow.
	stripped := domain.Permissions{}
	got, err := service.AssignRole(ctx, roomID, "creator", "co-owner", domain.RoleOwner, &stripped)
	if err != nil {
		t.Fatalf("creator promoting a member to owner: %v", err)
	}
	if got.Role != domain.RoleOwner || got.Permissions != domain.DefaultPermissionsForRole(domain.RoleOwner) {
		t.Fatalf("expected full owner role and permissions, got %+v", got)
	}

	// A co-owner is an owner, so they can add owners too.
	if _, err := service.AssignRole(ctx, roomID, "co-owner", "viewer", domain.RoleOwner, nil); err != nil {
		t.Fatalf("co-owner promoting a member to owner: %v", err)
	}
}

func TestOnlyOwnersCanGrantOwnership(t *testing.T) {
	ctx := context.Background()
	service, roomID := newCoOwnedRoom(t, "mod", "viewer")

	// A moderator explicitly granted CanManageRoles still cannot mint owners.
	modPerms := domain.DefaultPermissionsForRole(domain.RoleModerator)
	modPerms.CanManageRoles = true
	if _, err := service.AssignRole(ctx, roomID, "creator", "mod", domain.RoleModerator, &modPerms); err != nil {
		t.Fatalf("setup moderator: %v", err)
	}

	for _, target := range []string{"viewer", "mod"} {
		if _, err := service.AssignRole(ctx, roomID, "mod", target, domain.RoleOwner, nil); !errors.Is(err, room.ErrOwnerOnly) {
			t.Errorf("moderator making %s an owner: expected ErrOwnerOnly, got %v", target, err)
		}
	}
}

func TestAssignRoleRejectsUnknownRoles(t *testing.T) {
	service, roomID := newCoOwnedRoom(t, "viewer")
	if _, err := service.AssignRole(context.Background(), roomID, "creator", "viewer", domain.Role("superadmin"), nil); !errors.Is(err, room.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestOnlyCreatorCanDemoteOrKickCoOwners(t *testing.T) {
	ctx := context.Background()
	service, roomID := newCoOwnedRoom(t, "owner-a", "owner-b")
	for _, id := range []string{"owner-a", "owner-b"} {
		if _, err := service.AssignRole(ctx, roomID, "creator", id, domain.RoleOwner, nil); err != nil {
			t.Fatalf("setup co-owner %s: %v", id, err)
		}
	}

	// Co-owners cannot touch each other or the creator.
	if _, err := service.AssignRole(ctx, roomID, "owner-a", "owner-b", domain.RoleMember, nil); !errors.Is(err, room.ErrCreatorOnly) {
		t.Errorf("co-owner demoting co-owner: expected ErrCreatorOnly, got %v", err)
	}
	if err := service.KickMember(ctx, roomID, "owner-a", "owner-b"); !errors.Is(err, room.ErrCreatorOnly) {
		t.Errorf("co-owner kicking co-owner: expected ErrCreatorOnly, got %v", err)
	}
	if _, err := service.AssignRole(ctx, roomID, "owner-a", "creator", domain.RoleMember, nil); !errors.Is(err, room.ErrCannotAlterOwner) {
		t.Errorf("co-owner demoting creator: expected ErrCannotAlterOwner, got %v", err)
	}
	if err := service.KickMember(ctx, roomID, "owner-a", "creator"); !errors.Is(err, room.ErrCannotAlterOwner) {
		t.Errorf("co-owner kicking creator: expected ErrCannotAlterOwner, got %v", err)
	}

	// The creator can.
	demoted, err := service.AssignRole(ctx, roomID, "creator", "owner-a", domain.RoleModerator, nil)
	if err != nil {
		t.Fatalf("creator demoting co-owner: %v", err)
	}
	if demoted.Role != domain.RoleModerator || demoted.Permissions.CanManageRoles {
		t.Errorf("expected plain moderator after demotion, got %+v", demoted)
	}
	if err := service.KickMember(ctx, roomID, "creator", "owner-b"); err != nil {
		t.Fatalf("creator kicking co-owner: %v", err)
	}
}

func TestCoOwnerCanLeaveButCreatorCannot(t *testing.T) {
	ctx := context.Background()
	service, roomID := newCoOwnedRoom(t, "co-owner")
	if _, err := service.AssignRole(ctx, roomID, "creator", "co-owner", domain.RoleOwner, nil); err != nil {
		t.Fatalf("setup co-owner: %v", err)
	}

	if err := service.LeaveRoom(ctx, roomID, "co-owner"); err != nil {
		t.Errorf("co-owner leaving: %v", err)
	}
	if err := service.LeaveRoom(ctx, roomID, "creator"); !errors.Is(err, room.ErrCannotAlterOwner) {
		t.Errorf("creator leaving: expected ErrCannotAlterOwner, got %v", err)
	}
}

// The creator is recognised by rooms.owner_id, not by their member row, so a row that
// has drifted from "owner" (legacy data, a manual fix) does not expose them.
func TestCreatorIsProtectedEvenIfTheirRoleRowDrifts(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)
	rm, _, err := service.CreateRoom(ctx, "creator", "Drifted Room", false)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := service.JoinRoom(ctx, rm.ID, "mod"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := service.AssignRole(ctx, rm.ID, "creator", "mod", domain.RoleModerator, nil); err != nil {
		t.Fatalf("setup moderator: %v", err)
	}
	repo.members[rm.ID+":creator"].Role = domain.RoleMember

	if err := service.KickMember(ctx, rm.ID, "mod", "creator"); !errors.Is(err, room.ErrCannotAlterOwner) {
		t.Errorf("moderator kicking creator: expected ErrCannotAlterOwner, got %v", err)
	}
	if err := service.LeaveRoom(ctx, rm.ID, "creator"); !errors.Is(err, room.ErrCannotAlterOwner) {
		t.Errorf("creator leaving: expected ErrCannotAlterOwner, got %v", err)
	}
}
