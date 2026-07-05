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

func TestRoomCreationAndRBACWorkflow(t *testing.T) {
	ctx := context.Background()
	repo := newMockRoomRepository()
	service := room.NewRoomService(repo, nil)

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
