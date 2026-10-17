package ws_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/ws"
)

// nextPlaybackEvent returns the next event on ch that is about what the room is
// playing, skipping presence and voice traffic.
func nextPlaybackEvent(t *testing.T, ch <-chan []byte) *ws.Event {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case data := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(data, &evt); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}
			switch evt.Type {
			case ws.EventJoinRoom, ws.EventLeaveRoom, ws.EventSFUPeers:
				continue
			}
			return &evt
		case <-deadline:
			t.Fatalf("timed out waiting for a playback event")
			return nil
		}
	}
}

// expectNoPlaybackEvent fails if anything about playback reaches ch shortly.
func expectNoPlaybackEvent(t *testing.T, ch <-chan []byte) {
	t.Helper()
	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case data := <-ch:
			var evt ws.Event
			_ = json.Unmarshal(data, &evt)
			if evt.Type == ws.EventJoinRoom || evt.Type == ws.EventLeaveRoom {
				continue
			}
			t.Fatalf("expected no playback event, got %s %s", evt.Type, evt.Payload)
		case <-deadline:
			return
		}
	}
}

func expectMediaChange(t *testing.T, ch <-chan []byte, wantURL string, wantPlaying bool) {
	t.Helper()
	evt := nextPlaybackEvent(t, ch)
	if evt.Type != ws.EventChangeMedia {
		t.Fatalf("got %s, want CHANGE_MEDIA", evt.Type)
	}
	var p ws.VideoControlPayload
	_ = json.Unmarshal(evt.Payload, &p)
	if p.MediaURL != wantURL || p.IsPlaying != wantPlaying {
		t.Fatalf("CHANGE_MEDIA = %q playing=%v, want %q playing=%v", p.MediaURL, p.IsPlaying, wantURL, wantPlaying)
	}
}

func expectQueueState(t *testing.T, ch <-chan []byte) *domain.PlaybackQueue {
	t.Helper()
	evt := nextPlaybackEvent(t, ch)
	if evt.Type != ws.EventQueueState {
		t.Fatalf("got %s, want QUEUE_STATE", evt.Type)
	}
	var p ws.QueueStatePayload
	if err := json.Unmarshal(evt.Payload, &p); err != nil {
		t.Fatalf("unmarshal QUEUE_STATE: %v", err)
	}
	return p.Queue
}

func send(hub *ws.Hub, from *ws.Client, typ ws.EventType, payload any) {
	data, _ := json.Marshal(payload)
	hub.Broadcast <- &ws.Event{Type: typ, RoomID: from.RoomID, SenderID: from.UserID, SenderName: from.Username, Payload: data}
}

func joinRoom(t *testing.T, hub *ws.Hub, roomID, userID string) *ws.Client {
	t.Helper()
	c := &ws.Client{Hub: hub, Send: make(chan []byte, 32), RoomID: roomID, UserID: userID, Username: userID}
	hub.Register <- c
	if evt := nextPlaybackEvent(t, c.Send); evt.Type != ws.EventSyncPlayback {
		t.Fatalf("%s: first playback event = %s, want SYNC_PLAYBACK", userID, evt.Type)
	}
	return c
}

var threeFilms = domain.PlaybackQueue{
	PlaylistID: "playlist-1",
	Title:      "Friday",
	Items: []domain.QueueItem{
		{MediaURL: "https://cdn.test/one.m3u8", Title: "One"},
		{MediaURL: "https://cdn.test/two.m3u8", Title: "Two"},
		{MediaURL: "https://cdn.test/three.m3u8", Title: "Three"},
	},
}

func TestQueueStartsPlayingAndAdvancesOncePerEnd(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	alice := joinRoom(t, hub, "room-q", "alice")
	bob := joinRoom(t, hub, "room-q", "bob")

	send(hub, alice, ws.EventQueueSet, ws.QueueSetPayload{Queue: threeFilms})
	for _, c := range []*ws.Client{alice, bob} {
		expectMediaChange(t, c.Send, "https://cdn.test/one.m3u8", true)
		if q := expectQueueState(t, c.Send); q == nil || q.Index != 0 || len(q.Items) != 3 || q.Title != "Friday" {
			t.Fatalf("queue after QUEUE_SET = %+v", q)
		}
	}

	// Both members' videos end; both ask to move on from item 0. The room must
	// move one item, not two.
	send(hub, alice, ws.EventQueueAdvance, ws.QueueAdvancePayload{FromIndex: 0})
	send(hub, bob, ws.EventQueueAdvance, ws.QueueAdvancePayload{FromIndex: 0})
	for _, c := range []*ws.Client{alice, bob} {
		expectMediaChange(t, c.Send, "https://cdn.test/two.m3u8", true)
		if q := expectQueueState(t, c.Send); q.Index != 1 {
			t.Fatalf("queue index after advance = %d, want 1", q.Index)
		}
		expectNoPlaybackEvent(t, c.Send)
	}

	// Jumping is an advance with a target; past the end is ignored.
	to := 2
	send(hub, bob, ws.EventQueueAdvance, ws.QueueAdvancePayload{FromIndex: 1, ToIndex: &to})
	expectMediaChange(t, alice.Send, "https://cdn.test/three.m3u8", true)
	expectQueueState(t, alice.Send)
	send(hub, alice, ws.EventQueueAdvance, ws.QueueAdvancePayload{FromIndex: 2})
	expectNoPlaybackEvent(t, alice.Send)

	// A late joiner learns the queue along with the position.
	carol := &ws.Client{Hub: hub, Send: make(chan []byte, 32), RoomID: "room-q", UserID: "carol", Username: "carol"}
	hub.Register <- carol
	sync := nextPlaybackEvent(t, carol.Send)
	var p ws.VideoControlPayload
	_ = json.Unmarshal(sync.Payload, &p)
	if sync.Type != ws.EventSyncPlayback || p.Queue == nil || p.Queue.Index != 2 || p.MediaURL != "https://cdn.test/three.m3u8" || !p.IsPlaying {
		t.Fatalf("late joiner sync = %s %+v", sync.Type, p)
	}
}

func TestQueueFollowsMediaPickedFromTheLibrary(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	alice := joinRoom(t, hub, "room-r", "alice")
	send(hub, alice, ws.EventQueueSet, ws.QueueSetPayload{Queue: threeFilms})
	expectMediaChange(t, alice.Send, "https://cdn.test/one.m3u8", true)
	expectQueueState(t, alice.Send)

	// Picking a video that is in the queue moves the queue there.
	send(hub, alice, ws.EventChangeMedia, ws.VideoControlPayload{MediaURL: "https://cdn.test/three.m3u8"})
	expectMediaChange(t, alice.Send, "https://cdn.test/three.m3u8", false)
	if q := expectQueueState(t, alice.Send); q == nil || q.Index != 2 {
		t.Fatalf("queue after picking an item = %+v, want index 2", q)
	}

	// Picking anything else ends the queue.
	send(hub, alice, ws.EventChangeMedia, ws.VideoControlPayload{MediaURL: "https://cdn.test/other.mp4"})
	expectMediaChange(t, alice.Send, "https://cdn.test/other.mp4", false)
	if q := expectQueueState(t, alice.Send); q != nil {
		t.Fatalf("queue after picking something else = %+v, want none", q)
	}

	// With no queue, a library pick says nothing about queues.
	send(hub, alice, ws.EventChangeMedia, ws.VideoControlPayload{MediaURL: "https://cdn.test/another.mp4"})
	expectMediaChange(t, alice.Send, "https://cdn.test/another.mp4", false)
	expectNoPlaybackEvent(t, alice.Send)
}

func TestQueueRejectsMalformedQueuesAndClears(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	alice := joinRoom(t, hub, "room-s", "alice")

	for name, q := range map[string]domain.PlaybackQueue{
		"empty":          {Title: "x"},
		"index too high": {Items: threeFilms.Items, Index: 3},
		"negative index": {Items: threeFilms.Items, Index: -1},
		"blank url":      {Items: []domain.QueueItem{{MediaURL: "  "}}},
	} {
		send(hub, alice, ws.EventQueueSet, ws.QueueSetPayload{Queue: q})
		select {
		case data := <-alice.Send:
			t.Fatalf("%s: expected the queue to be dropped, got %s", name, data)
		case <-time.After(50 * time.Millisecond):
		}
	}

	start := threeFilms
	start.Index = 1
	send(hub, alice, ws.EventQueueSet, ws.QueueSetPayload{Queue: start})
	expectMediaChange(t, alice.Send, "https://cdn.test/two.m3u8", true)
	expectQueueState(t, alice.Send)

	send(hub, alice, ws.EventQueueClear, struct{}{})
	if q := expectQueueState(t, alice.Send); q != nil {
		t.Fatalf("queue after clear = %+v, want none", q)
	}
	// Clearing stops the queue, not the video.
	expectNoPlaybackEvent(t, alice.Send)
}
