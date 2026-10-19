package room_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
)

type mockRoomRepository struct {
	rooms       map[string]*domain.Room
	members     map[string]*domain.RoomMember // key: roomID + ":" + userID
	invitations map[string]struct{}           // key: roomID + ":" + userID
}

func newMockRoomRepository() *mockRoomRepository {
	return &mockRoomRepository{
		rooms:       make(map[string]*domain.Room),
		members:     make(map[string]*domain.RoomMember),
		invitations: make(map[string]struct{}),
	}
}

func (m *mockRoomRepository) invite(roomID, userID string) {
	m.invitations[roomID+":"+userID] = struct{}{}
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
func (m *mockRoomRepository) HasInvitation(ctx context.Context, roomID, userID string) (bool, error) {
	_, ok := m.invitations[roomID+":"+userID]
	return ok, nil
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
	rm, ownerMem, err := service.CreateRoom(ctx, ownerID, "Friday Movie Night", false, "")
	if err != nil {
		t.Fatalf("expected successful room creation, got %v", err)
	}
	if !ownerMem.Permissions.CanControlPlayback {
		t.Errorf("expected room owner to have CanControlPlayback = true")
	}

	// 2. Viewer Joins Room
	viewerMem, err := service.JoinRoom(ctx, rm.ID, viewerID, "")
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
	// A join is authorized against the room, so the room has to exist: public and
	// without a password, which is the case where both racers get past the gate.
	repo.rooms["room-1"] = &domain.Room{ID: "room-1", Name: "Race", OwnerID: "user-owner"}
	service := room.NewRoomService(repo, nil, nil)

	got, err := service.JoinRoom(ctx, "room-1", "user-2", "")
	if err != nil {
		t.Fatalf("losing a join race must not be an error, got %v", err)
	}
	if got != winner {
		t.Fatalf("expected the membership the concurrent join created, got %+v", got)
	}
}

// TestJoinRoomRefusesAPrivateRoomToAStranger covers what being private has to
// mean: the room's URL on its own is not a way in. Before the join was
// authorized, is_private only hid a room from the lobby, so anyone who came by
// the link — or read the room's UUID off the admin telemetry feed — was made a
// member by simply opening it.
func TestJoinRoomRefusesAPrivateRoomToAStranger(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)

	rm, _, err := service.CreateRoom(ctx, "user-owner", "Private Night", true, "")
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}

	if _, err := service.JoinRoom(ctx, rm.ID, "user-stranger", ""); !errors.Is(err, room.ErrRoomPrivate) {
		t.Fatalf("expected ErrRoomPrivate for an uninvited stranger, got %v", err)
	}
}

func TestJoinRoomAdmitsAnInviteeToAPrivateRoom(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)

	rm, _, err := service.CreateRoom(ctx, "user-owner", "Private Night", true, "")
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}
	repo.invite(rm.ID, "user-guest")

	member, err := service.JoinRoom(ctx, rm.ID, "user-guest", "")
	if err != nil {
		t.Fatalf("expected an invitee to be admitted, got %v", err)
	}
	if member.Role != domain.RoleMember {
		t.Errorf("expected an invitee to join as RoleMember, got %q", member.Role)
	}
}

// TestJoinRoomWithAPassword walks the three answers a password can produce. The
// password is what lets a room be shared by link without the link alone being
// enough, which is the only one of these paths an operator reading room UUIDs
// out of the database cannot walk.
func TestJoinRoomWithAPassword(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)

	rm, _, err := service.CreateRoom(ctx, "user-owner", "Password Night", false, "correct horse")
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}
	if !rm.HasPassword {
		t.Error("expected HasPassword on a room created with one")
	}

	if _, err := service.JoinRoom(ctx, rm.ID, "user-guest", ""); !errors.Is(err, room.ErrRoomPasswordRequired) {
		t.Fatalf("expected ErrRoomPasswordRequired without a password, got %v", err)
	}
	if _, err := service.JoinRoom(ctx, rm.ID, "user-guest", "battery staple"); !errors.Is(err, room.ErrRoomPasswordIncorrect) {
		t.Fatalf("expected ErrRoomPasswordIncorrect for a wrong password, got %v", err)
	}
	if _, err := service.JoinRoom(ctx, rm.ID, "user-guest", "correct horse"); err != nil {
		t.Fatalf("expected the right password to admit, got %v", err)
	}
}

// Trimming matters because a room password is copied out of a chat message as
// often as it is typed.
func TestJoinRoomAcceptsAPasswordWithSurroundingWhitespace(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)

	rm, _, err := service.CreateRoom(ctx, "user-owner", "Password Night", false, "  hunter2  ")
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}
	if _, err := service.JoinRoom(ctx, rm.ID, "user-guest", "hunter2"); err != nil {
		t.Fatalf("expected the trimmed password to admit, got %v", err)
	}
}

func TestCreateRoomRejectsAnUnusablePassword(t *testing.T) {
	ctx := context.Background()
	service := room.NewRoomService(newMockRoomRepository(), nil, nil)

	if _, _, err := service.CreateRoom(ctx, "user-owner", "Short", false, "ab"); !errors.Is(err, room.ErrInvalidRoomPassword) {
		t.Fatalf("expected ErrInvalidRoomPassword for a 2-character password, got %v", err)
	}
	if _, _, err := service.CreateRoom(ctx, "user-owner", "Long", false, strings.Repeat("x", 129)); !errors.Is(err, room.ErrInvalidRoomPassword) {
		t.Fatalf("expected ErrInvalidRoomPassword for a 129-character password, got %v", err)
	}
}

// A member who is already in the room is let back in without the password, so a
// reconnect or a second tab does not become a prompt. The owner likewise never
// locks themselves out.
func TestJoinRoomDoesNotRechallengeMembersOrTheOwner(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil, nil)

	rm, _, err := service.CreateRoom(ctx, "user-owner", "Password Night", true, "hunter2")
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}

	if _, err := service.JoinRoom(ctx, rm.ID, "user-owner", ""); err != nil {
		t.Fatalf("expected the owner to be admitted without the password, got %v", err)
	}
	if _, err := service.JoinRoom(ctx, rm.ID, "user-guest", "hunter2"); err != nil {
		t.Fatalf("expected the password to admit a guest, got %v", err)
	}
	if _, err := service.JoinRoom(ctx, rm.ID, "user-guest", ""); err != nil {
		t.Fatalf("expected an existing member to rejoin without the password, got %v", err)
	}
}

// The hash must not reach a client: whoever can read it offline can attack it
// offline, and a room password is short by design.
func TestRoomPasswordHashIsNeverSerialized(t *testing.T) {
	ctx := context.Background()
	service := room.NewRoomService(newMockRoomRepository(), nil, nil)

	rm, _, err := service.CreateRoom(ctx, "user-owner", "Password Night", true, "hunter2")
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}

	payload, err := json.Marshal(rm)
	if err != nil {
		t.Fatalf("failed to marshal room: %v", err)
	}
	if strings.Contains(string(payload), "argon2") || strings.Contains(string(payload), rm.PasswordHash) {
		t.Fatalf("room JSON leaks the password hash: %s", payload)
	}
	if !strings.Contains(string(payload), `"has_password":true`) {
		t.Fatalf("expected has_password in room JSON, got %s", payload)
	}
}
