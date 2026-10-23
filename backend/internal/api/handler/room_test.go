package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/domain"
)

type mockRoomService struct{}

func (m *mockRoomService) CreateRoom(ctx context.Context, ownerID, name string, isPrivate bool) (*domain.Room, *domain.RoomMember, error) {
	rm := &domain.Room{ID: "room-http-123", Name: name, OwnerID: ownerID, IsPrivate: isPrivate}
	mem := &domain.RoomMember{RoomID: rm.ID, UserID: ownerID, Role: domain.RoleOwner, Permissions: domain.DefaultPermissionsForRole(domain.RoleOwner)}
	return rm, mem, nil
}

func (m *mockRoomService) JoinRoom(ctx context.Context, roomID, userID string) (*domain.RoomMember, error) {
	return &domain.RoomMember{RoomID: roomID, UserID: userID, Role: domain.RoleMember, Permissions: domain.DefaultPermissionsForRole(domain.RoleMember)}, nil
}

func (m *mockRoomService) GetRoomAndMember(ctx context.Context, roomID, userID string) (*domain.Room, *domain.RoomMember, error) {
	rm := &domain.Room{ID: roomID, Name: "Test Room", OwnerID: "owner-1"}
	mem := &domain.RoomMember{RoomID: roomID, UserID: userID, Role: domain.RoleMember}
	return rm, mem, nil
}

func (m *mockRoomService) AssignRole(ctx context.Context, roomID, requesterID, targetUserID string, newRole domain.Role, customPerms *domain.Permissions) (*domain.RoomMember, error) {
	return &domain.RoomMember{RoomID: roomID, UserID: targetUserID, Role: newRole}, nil
}

func (m *mockRoomService) KickMember(ctx context.Context, roomID, requesterID, targetUserID string) error {
	return nil
}

func (m *mockRoomService) CheckPlaybackPermission(ctx context.Context, roomID, userID string) error {
	return nil
}

func (m *mockRoomService) ListRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return nil, nil
}

func (m *mockRoomService) InviteUserByUsername(ctx context.Context, roomID, inviterID, inviteeUsername string) (*domain.RoomInvitation, error) {
	return nil, nil
}

func (m *mockRoomService) ListUserPendingInvitations(ctx context.Context, userID string) ([]*domain.RoomInvitation, error) {
	return nil, nil
}

func (m *mockRoomService) RespondToInvitation(ctx context.Context, invID, userID string, accept bool) (*domain.Room, *domain.RoomMember, error) {
	return nil, nil, nil
}

func (m *mockRoomService) GetRoomByID(ctx context.Context, roomID string) (*domain.Room, error) {
	return nil, nil
}

func (m *mockRoomService) UpdateRoomMediaURL(ctx context.Context, roomID, mediaURL string) error {
	return nil
}

func (m *mockRoomService) DeleteRoom(ctx context.Context, roomID, userID string) error {
	return nil
}

func (m *mockRoomService) LeaveRoom(ctx context.Context, roomID, userID string) error {
	return nil
}

func (m *mockRoomService) ListAllRooms(ctx context.Context) ([]*domain.Room, error) {
	return []*domain.Room{}, nil
}

func (m *mockRoomService) DeleteRoomsAsAdmin(ctx context.Context, roomIDs []string) ([]string, error) {
	return roomIDs, nil
}

func TestCreateRoomHTTPHandler(t *testing.T) {
	mockSvc := &mockRoomService{}
	roomHandler := handler.NewRoomHandler(mockSvc)

	payload := map[string]any{"name": "Saturday Cinema", "is_private": false}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/rooms", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Inject authenticated user session into context (simulating RequireAuth middleware)
	session := &domain.Session{ID: "sess_mock", UserID: "user-abc-1", Username: "robin", ExpiresAt: time.Now().Add(time.Hour)}
	ctx := middleware.WithSessionContext(req.Context(), session)
	req = req.WithContext(ctx)

	roomHandler.CreateRoom(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201 Created, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to parse response json: %v", err)
	}
	if resp["room"] == nil || resp["member"] == nil {
		t.Errorf("expected both room and member objects in response, got %+v", resp)
	}
}

// listingRoomService returns a fixed room list; everything else is the shared mock.
type listingRoomService struct {
	mockRoomService
	rooms []*domain.Room
}

func (s *listingRoomService) ListRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return s.rooms, nil
}

type fakeActivitySource struct {
	snapshot map[string]domain.RoomActivity
	err      error
}

func (f *fakeActivitySource) RoomActivity(ctx context.Context) (map[string]domain.RoomActivity, error) {
	return f.snapshot, f.err
}

func listRoomsAs(t *testing.T, h *handler.RoomHandler) []map[string]any {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/rooms", nil)
	session := &domain.Session{ID: "sess_mock", UserID: "user-abc-1", Username: "robin", ExpiresAt: time.Now().Add(time.Hour)}
	req = req.WithContext(middleware.WithSessionContext(req.Context(), session))
	rec := httptest.NewRecorder()

	h.ListRooms(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}
	var rooms []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&rooms); err != nil {
		t.Fatalf("failed to parse response json: %v", err)
	}
	return rooms
}

// Activity is attached only to rooms the caller can already see: a busy private
// room elsewhere in the snapshot must not leak into someone else's lobby.
func TestListRoomsAttachesActivityOnlyToListedRooms(t *testing.T) {
	svc := &listingRoomService{rooms: []*domain.Room{
		{ID: "busy", Name: "Busy"},
		{ID: "quiet", Name: "Quiet"},
	}}
	h := handler.NewRoomHandler(svc)
	h.SetActivitySource(&fakeActivitySource{snapshot: map[string]domain.RoomActivity{
		"busy":          {Viewers: 3, IsPlaying: true, MediaURL: "https://cdn.example/movie.m3u8", Kind: domain.MediaKindVOD},
		"someone-elses": {Viewers: 9, IsPlaying: true},
	}})

	rooms := listRoomsAs(t, h)
	if len(rooms) != 2 {
		t.Fatalf("expected the 2 listed rooms, got %d", len(rooms))
	}
	busy, ok := rooms[0]["activity"].(map[string]any)
	if !ok {
		t.Fatalf("expected activity on the busy room, got %+v", rooms[0])
	}
	if busy["viewers"] != float64(3) || busy["is_playing"] != true {
		t.Errorf("unexpected activity %+v", busy)
	}
	if _, present := rooms[1]["activity"]; present {
		t.Errorf("an idle room must carry no activity, got %+v", rooms[1]["activity"])
	}
}

// The hub being slow or down must not take the room list with it.
func TestListRoomsStillServesWhenActivityFails(t *testing.T) {
	svc := &listingRoomService{rooms: []*domain.Room{{ID: "r1", Name: "Room"}}}
	h := handler.NewRoomHandler(svc)
	h.SetActivitySource(&fakeActivitySource{err: context.DeadlineExceeded})

	rooms := listRoomsAs(t, h)
	if len(rooms) != 1 {
		t.Fatalf("expected the room to still be listed, got %d", len(rooms))
	}
	if _, present := rooms[0]["activity"]; present {
		t.Errorf("expected no activity when the source fails, got %+v", rooms[0]["activity"])
	}
}

// adminDeleteService deletes only the rooms it knows about and records what it
// was asked to delete.
type adminDeleteService struct {
	mockRoomService
	existing map[string]bool
	asked    []string
}

func (s *adminDeleteService) DeleteRoomsAsAdmin(ctx context.Context, roomIDs []string) ([]string, error) {
	s.asked = roomIDs
	deleted := []string{}
	for _, id := range roomIDs {
		if s.existing[id] {
			deleted = append(deleted, id)
		}
	}
	return deleted, nil
}

type fakeRoomCloser struct{ closed []string }

func (f *fakeRoomCloser) CloseRoom(roomID string) { f.closed = append(f.closed, roomID) }

const (
	roomA = "7d1c6f4e-8c1a-4a55-9a43-2b1f0f2b9e10"
	roomB = "0b5e1d0a-3f2c-4e7b-8d9a-6c1e2f3a4b5c"
	roomC = "f1e2d3c4-b5a6-4789-8a0b-1c2d3e4f5a6b"
)

func TestAdminDeleteRoomsSkipsUnknownAndClosesDeleted(t *testing.T) {
	svc := &adminDeleteService{existing: map[string]bool{roomA: true, roomB: true}}
	closer := &fakeRoomCloser{}
	h := handler.NewRoomHandler(svc)
	h.SetRoomCloser(closer)

	body, _ := json.Marshal(map[string]any{"room_ids": []string{roomA, roomB, roomA, roomC}})
	rec := httptest.NewRecorder()
	h.AdminDeleteRooms(rec, httptest.NewRequest("POST", "/api/v1/admin/rooms/delete", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(svc.asked) != 3 {
		t.Errorf("service asked to delete %v, want the 3 distinct IDs", svc.asked)
	}
	var resp struct {
		Deleted []string `json:"deleted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Deleted) != 2 || len(closer.closed) != 2 {
		t.Errorf("deleted %v, closed %v; want rooms A and B in both", resp.Deleted, closer.closed)
	}
}

func TestAdminDeleteRoomsRejectsBadInput(t *testing.T) {
	for name, body := range map[string]string{
		"empty":    `{"room_ids":[]}`,
		"not uuid": `{"room_ids":["` + roomA + `","../etc"]}`,
		"garbage":  `nope`,
	} {
		svc := &adminDeleteService{}
		h := handler.NewRoomHandler(svc)
		rec := httptest.NewRecorder()
		h.AdminDeleteRooms(rec, httptest.NewRequest("POST", "/api/v1/admin/rooms/delete", bytes.NewReader([]byte(body))))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
		if svc.asked != nil {
			t.Errorf("%s: nothing should have been deleted, service got %v", name, svc.asked)
		}
	}
}

func TestAdminDeleteRoomNotFound(t *testing.T) {
	h := handler.NewRoomHandler(&adminDeleteService{})
	for _, id := range []string{roomC, "not-a-uuid"} {
		req := httptest.NewRequest("DELETE", "/api/v1/admin/rooms/"+id, nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h.AdminDeleteRoom(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, rec.Code)
		}
	}
}
