package ws_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/ws"
)

// waitForEvent drains ch until an event of the wanted type arrives. The hub's
// select picks at random among ready channels, so a test that asks for activity
// right after queuing a broadcast has to wait for the broadcast to land first.
func waitForEvent(t *testing.T, ch <-chan []byte, want ws.EventType) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case data := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(data, &evt); err == nil && evt.Type == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}

func activityNow(t *testing.T, hub *ws.Hub) map[string]roomActivity {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	snapshot, err := hub.RoomActivity(ctx)
	if err != nil {
		t.Fatalf("RoomActivity: %v", err)
	}
	out := make(map[string]roomActivity, len(snapshot))
	for id, a := range snapshot {
		out[id] = roomActivity{viewers: a.Viewers, playing: a.IsPlaying, url: a.MediaURL}
	}
	return out
}

type roomActivity struct {
	viewers int
	playing bool
	url     string
}

// The lobby's "now streaming" section is built from this snapshot, so it has to
// count people rather than connections, follow play/pause, and forget a room the
// moment the last person leaves.
func TestRoomActivityTracksPeopleAndPlayback(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "movie-night"
	newClient := func(userID string) *ws.Client {
		return &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: userID, Username: userID}
	}
	alice := newClient("alice")
	aliceSecondTab := newClient("alice")
	bob := newClient("bob")
	for _, c := range []*ws.Client{alice, aliceSecondTab, bob} {
		hub.Register <- c
	}

	got := activityNow(t, hub)[roomID]
	if got.viewers != 2 {
		t.Fatalf("viewers = %d, want 2 (two people, three connections)", got.viewers)
	}
	if got.playing {
		t.Fatal("a room nobody pressed play in reported is_playing")
	}
	if got.url == "" {
		t.Fatal("expected the room's media URL in the snapshot")
	}

	hub.Broadcast <- &ws.Event{Type: ws.EventPlay, RoomID: roomID, SenderID: "alice", Payload: json.RawMessage(`{"media_time_seconds":12}`)}
	waitForEvent(t, bob.Send, ws.EventPlay)
	if !activityNow(t, hub)[roomID].playing {
		t.Fatal("expected is_playing after PLAY")
	}

	for _, c := range []*ws.Client{alice, aliceSecondTab, bob} {
		hub.Unregister <- c
	}
	if _, ok := activityNow(t, hub)[roomID]; ok {
		t.Fatal("an empty room must drop out of the activity snapshot")
	}
}

// The room list waits on this call inside an HTTP request; a hub that is not
// running must cost it the deadline, not hang it.
func TestRoomActivityHonoursContextWhenHubIsNotRunning(t *testing.T) {
	hub := ws.NewHub() // Run never started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := hub.RoomActivity(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("RoomActivity blocked for %s", elapsed)
	}
}
