package ws_test

import (
	"encoding/json"
	"testing"

	"github.com/inox/inox/backend/internal/ws"
)

func lateJoinerSync(t *testing.T, hub *ws.Hub, roomID, userID string) ws.VideoControlPayload {
	t.Helper()
	c := &ws.Client{Hub: hub, Send: make(chan []byte, 32), RoomID: roomID, UserID: userID, Username: userID}
	hub.Register <- c
	evt := nextPlaybackEvent(t, c.Send)
	if evt.Type != ws.EventSyncPlayback {
		t.Fatalf("%s: first playback event = %s, want SYNC_PLAYBACK", userID, evt.Type)
	}
	var p ws.VideoControlPayload
	_ = json.Unmarshal(evt.Payload, &p)
	return p
}

func TestSetRateIsRelayedRememberedAndResetByMediaChange(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	alice := joinRoom(t, hub, "room-rate", "alice")
	bob := joinRoom(t, hub, "room-rate", "bob")

	send(hub, alice, ws.EventSetRate, ws.VideoControlPayload{PlaybackRate: 2, MediaTimeSeconds: 42})
	evt := nextPlaybackEvent(t, bob.Send)
	var p ws.VideoControlPayload
	_ = json.Unmarshal(evt.Payload, &p)
	if evt.Type != ws.EventSetRate || p.PlaybackRate != 2 || p.MediaTimeSeconds != 42 {
		t.Fatalf("bob got %s %+v, want SET_RATE at 2x from 42s", evt.Type, p)
	}

	// Out-of-range and garbage speeds never reach the room.
	for _, rate := range []float64{0, 0.1, 3.5, 16} {
		send(hub, alice, ws.EventSetRate, ws.VideoControlPayload{PlaybackRate: rate, MediaTimeSeconds: 50})
	}
	send(hub, alice, ws.EventSetRate, ws.VideoControlPayload{PlaybackRate: 1.5, MediaTimeSeconds: -1})
	expectNoPlaybackEvent(t, bob.Send)

	if s := lateJoinerSync(t, hub, "room-rate", "carol"); s.PlaybackRate != 2 || s.MediaTimeSeconds != 42 {
		t.Fatalf("late joiner sync = %+v, want 2x from 42s", s)
	}

	// A new video starts at normal speed.
	send(hub, alice, ws.EventChangeMedia, ws.VideoControlPayload{MediaURL: "https://cdn.test/next.mp4"})
	expectMediaChange(t, bob.Send, "https://cdn.test/next.mp4", false)
	if s := lateJoinerSync(t, hub, "room-rate", "dave"); s.PlaybackRate != 0 {
		t.Fatalf("sync after media change has rate %v, want normal speed", s.PlaybackRate)
	}
}
