package ws_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/ws"
)

// memoryStateRepo keeps playback state in a map, standing in for Redis.
type memoryStateRepo struct {
	mu     sync.Mutex
	states map[string]domain.RoomPlaybackState
}

func newMemoryStateRepo() *memoryStateRepo {
	return &memoryStateRepo{states: make(map[string]domain.RoomPlaybackState)}
}

func (r *memoryStateRepo) GetPlaybackState(_ context.Context, roomID string) (*domain.RoomPlaybackState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.states[roomID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (r *memoryStateRepo) SetPlaybackState(_ context.Context, roomID string, state *domain.RoomPlaybackState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[roomID] = *state
	return nil
}

func (r *memoryStateRepo) ClearPlaybackState(_ context.Context, roomID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, roomID)
	return nil
}

func (r *memoryStateRepo) CacheMemberVerification(context.Context, string, string) error {
	return nil
}

func (r *memoryStateRepo) IsMemberVerified(context.Context, string, string) (bool, error) {
	return true, nil
}

func (r *memoryStateRepo) InvalidateMemberVerification(context.Context, string, string) error {
	return nil
}

func (r *memoryStateRepo) TryClaimLiveLeader(context.Context, string, string, string, time.Duration) (bool, error) {
	return true, nil
}

func (r *memoryStateRepo) RefreshLiveLeader(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}

func (r *memoryStateRepo) GetLiveLeader(context.Context, string) (string, string, error) {
	return "", "", nil
}

func (r *memoryStateRepo) ReleaseLiveLeader(context.Context, string, string) error {
	return nil
}

// update edits the stored state, for winding the clock back on it.
func (r *memoryStateRepo) update(roomID string, edit func(*domain.RoomPlaybackState)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.states[roomID]
	edit(&s)
	r.states[roomID] = s
}

// awaitStored waits for the hub's background write that satisfies ok.
func (r *memoryStateRepo) awaitStored(t *testing.T, roomID string, ok func(domain.RoomPlaybackState) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		s, found := r.states[roomID]
		r.mu.Unlock()
		if found && ok(s) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the playback state to be saved")
}

func newClient(hub *ws.Hub, roomID, userID string) *ws.Client {
	return &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 32),
		RoomID:   roomID,
		UserID:   userID,
		Username: userID,
	}
}

func joinAndSync(t *testing.T, hub *ws.Hub, client *ws.Client) ws.VideoControlPayload {
	t.Helper()
	hub.Register <- client
	evt := awaitEvent(t, client.Send, ws.EventSyncPlayback, 500*time.Millisecond)
	var payload ws.VideoControlPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("malformed SYNC_PLAYBACK payload: %v", err)
	}
	return payload
}

func TestFirstInIsFlaggedOnlyForWhoeverOpensTheRoom(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	const roomID = "first-in-room"

	alice := newClient(hub, roomID, "alice")
	if !joinAndSync(t, hub, alice).FirstIn {
		t.Fatal("the first connection into an empty room should be told it is first in")
	}
	bob := newClient(hub, roomID, "bob")
	if joinAndSync(t, hub, bob).FirstIn {
		t.Fatal("joining a room someone is already in should not be first in")
	}

	hub.Unregister <- alice
	hub.Unregister <- bob
	carol := newClient(hub, roomID, "carol")
	if !joinAndSync(t, hub, carol).FirstIn {
		t.Fatal("reopening a room everyone left should be first in again")
	}
}

// playThenLeave has a lone viewer start the room playing at 100s and leave,
// and waits for the hub to park the state.
func playThenLeave(t *testing.T, hub *ws.Hub, repo *memoryStateRepo, roomID string) {
	t.Helper()
	alice := newClient(hub, roomID, "alice")
	joinAndSync(t, hub, alice)

	payload, _ := json.Marshal(ws.VideoControlPayload{MediaTimeSeconds: 100})
	hub.Broadcast <- &ws.Event{Type: ws.EventPlay, RoomID: roomID, SenderID: alice.UserID, Payload: payload}
	repo.awaitStored(t, roomID, func(s domain.RoomPlaybackState) bool { return s.IsPlaying })

	hub.Unregister <- alice
	repo.awaitStored(t, roomID, func(s domain.RoomPlaybackState) bool { return s.EmptiedAt > 0 })
}

func TestRoomLeftEmptyResumesPausedWhereItWasLeft(t *testing.T) {
	hub := ws.NewHub()
	repo := newMemoryStateRepo()
	hub.SetStateRepository(repo)
	go hub.Run()
	const roomID = "parked-room"

	playThenLeave(t, hub, repo, roomID)
	// An hour passes with nobody in the room.
	repo.update(roomID, func(s *domain.RoomPlaybackState) {
		s.EmptiedAt -= time.Hour.Milliseconds()
		s.LastUpdated -= time.Hour.Milliseconds()
	})

	sync := joinAndSync(t, hub, newClient(hub, roomID, "bob"))
	if sync.IsPlaying {
		t.Fatal("a room that sat empty should come back paused")
	}
	if sync.MediaTimeSeconds < 100 || sync.MediaTimeSeconds > 101 {
		t.Fatalf("expected the room back where it was left (~100s), got %.2fs", sync.MediaTimeSeconds)
	}
	if !sync.FirstIn {
		t.Fatal("expected the arrival to be first in")
	}
}

func TestQuickReconnectKeepsTheRoomPlaying(t *testing.T) {
	hub := ws.NewHub()
	repo := newMemoryStateRepo()
	hub.SetStateRepository(repo)
	go hub.Run()
	const roomID = "blip-room"

	playThenLeave(t, hub, repo, roomID)

	sync := joinAndSync(t, hub, newClient(hub, roomID, "alice"))
	if !sync.IsPlaying {
		t.Fatal("a viewer reconnecting straight away should find the room still playing")
	}
	if sync.MediaTimeSeconds < 100 {
		t.Fatalf("expected the position the room was left at, got %.2fs", sync.MediaTimeSeconds)
	}
}
