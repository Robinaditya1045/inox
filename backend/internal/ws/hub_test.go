package ws_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/ws"
)

func TestHubRoomBroadcastingAndTargetedSignaling(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	roomID := "cinema-room-404"

	// 1. Create 3 clients in the same room
	alice := &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 10),
		RoomID:   roomID,
		UserID:   "user-alice",
		Username: "Alice",
	}
	bob := &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 10),
		RoomID:   roomID,
		UserID:   "user-bob",
		Username: "Bob",
	}
	charlie := &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 10),
		RoomID:   roomID,
		UserID:   "user-charlie",
		Username: "Charlie",
	}

	// 2. Register Alice & Bob
	hub.Register <- alice
	hub.Register <- bob

	// Draining initial JOIN_ROOM and SYNC_PLAYBACK notification events
	readEventWithin(t, alice.Send, 500*time.Millisecond) // Alice joined notification
	readEventWithin(t, alice.Send, 500*time.Millisecond) // Alice SYNC_PLAYBACK notification
	readEventWithin(t, alice.Send, 500*time.Millisecond) // Bob joined notification sent to Alice
	readEventWithin(t, bob.Send, 500*time.Millisecond)   // Bob joined notification sent to Bob
	readEventWithin(t, bob.Send, 500*time.Millisecond)   // Bob SYNC_PLAYBACK notification

	// Register Charlie
	hub.Register <- charlie
	readEventWithin(t, alice.Send, 500*time.Millisecond)   // Charlie joined -> Alice
	readEventWithin(t, bob.Send, 500*time.Millisecond)     // Charlie joined -> Bob
	readEventWithin(t, charlie.Send, 500*time.Millisecond) // Charlie joined -> Charlie
	readEventWithin(t, charlie.Send, 500*time.Millisecond) // Charlie SYNC_PLAYBACK notification

	// 3. Test Room Broadcast (Video Play Command)
	playEvt := &ws.Event{
		Type:       ws.EventPlay,
		RoomID:     roomID,
		SenderID:   alice.UserID,
		SenderName: alice.Username,
	}
	hub.Broadcast <- playEvt

	// Verify all members in the room receive the PLAY event
	verifyEventType(t, readEventWithin(t, alice.Send, 500*time.Millisecond), ws.EventPlay)
	verifyEventType(t, readEventWithin(t, bob.Send, 500*time.Millisecond), ws.EventPlay)
	verifyEventType(t, readEventWithin(t, charlie.Send, 500*time.Millisecond), ws.EventPlay)

	// 4. Test Peer-to-Peer Targeted WebRTC Signaling (Offer from Alice specifically to Bob)
	webrtcOffer := &ws.Event{
		Type:       ws.EventWebRTCOffer,
		RoomID:     roomID,
		SenderID:   alice.UserID,
		SenderName: alice.Username,
		TargetID:   bob.UserID, // TARGETED TO BOB ONLY
	}
	hub.Broadcast <- webrtcOffer

	// Verify Bob receives the WebRTC Offer
	verifyEventType(t, readEventWithin(t, bob.Send, 500*time.Millisecond), ws.EventWebRTCOffer)

	// Verify Charlie did NOT receive Alice's offer to Bob
	select {
	case unexpectedData := <-charlie.Send:
		t.Fatalf("expected charlie not to receive targeted peer offer, got %s", string(unexpectedData))
	case <-time.After(100 * time.Millisecond):
		// Expected timeout: Charlie correctly ignored
	}

	// 5. Test Unregistering Alice
	hub.Unregister <- alice
	verifyEventType(t, readEventWithin(t, bob.Send, 500*time.Millisecond), ws.EventLeaveRoom)
	verifyEventType(t, readEventWithin(t, charlie.Send, 500*time.Millisecond), ws.EventLeaveRoom)
}

func readEventWithin(t *testing.T, ch <-chan []byte, timeout time.Duration) *ws.Event {
	t.Helper()
	select {
	case data := <-ch:
		var evt ws.Event
		if err := json.Unmarshal(data, &evt); err != nil {
			t.Fatalf("failed to unmarshal event data: %v", err)
		}
		return &evt
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for websocket event on channel")
		return nil
	}
}

func verifyEventType(t *testing.T, evt *ws.Event, expected ws.EventType) {
	t.Helper()
	if evt.Type != expected {
		t.Errorf("expected event type %s, got %s", expected, evt.Type)
	}
}
