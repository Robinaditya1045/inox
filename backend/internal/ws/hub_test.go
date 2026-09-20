package ws_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/sfu"
	"github.com/inox/inox/backend/internal/ws"
	"github.com/pion/webrtc/v3"
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

// The hub owns the wiring between browser signaling and the media layer: it must
// answer an offer, publish the voice roster to the whole room -- not just to the
// caller -- and tear the peer down again on an explicit hang-up.
func TestHubSFUSignalingAndVoiceRoster(t *testing.T) {
	hub := ws.NewHub()
	hub.SetSFUManager(sfu.NewManager())
	go hub.Run()
	defer hub.Shutdown()

	roomID := "cinema-room-voice"

	alice := &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 32),
		RoomID:   roomID,
		UserID:   "user-alice",
		Username: "Alice",
	}
	bob := &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 32),
		RoomID:   roomID,
		UserID:   "user-bob",
		Username: "Bob",
	}
	hub.Register <- alice
	hub.Register <- bob

	// Alice joins voice with an offer her browser would have produced.
	browser, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create browser-side peer connection: %v", err)
	}
	defer browser.Close()
	if _, err := browser.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Fatalf("failed to add microphone transceiver: %v", err)
	}
	offer, err := browser.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create offer: %v", err)
	}
	if err := browser.SetLocalDescription(offer); err != nil {
		t.Fatalf("failed to apply offer: %v", err)
	}
	offerPayload, _ := json.Marshal(ws.SFUSDOPayload{SDP: offer.SDP, Type: "offer"})

	hub.Broadcast <- &ws.Event{
		Type:       ws.EventSFUOffer,
		RoomID:     roomID,
		SenderID:   alice.UserID,
		SenderName: alice.Username,
		Payload:    offerPayload,
	}

	answer := awaitEvent(t, alice.Send, ws.EventSFUAnswer, 3*time.Second)
	var answerPayload ws.SFUSDOPayload
	if err := json.Unmarshal(answer.Payload, &answerPayload); err != nil {
		t.Fatalf("failed to decode answer payload: %v", err)
	}
	if err := browser.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  answerPayload.SDP,
		Type: webrtc.SDPTypeAnswer,
	}); err != nil {
		t.Fatalf("expected a usable answer from the hub: %v", err)
	}

	// Bob is not in voice, but still has to be told who is.
	roster := decodeRoster(t, awaitEvent(t, bob.Send, ws.EventSFUPeers, 3*time.Second))
	if len(roster) != 1 || roster[0].UserID != alice.UserID || roster[0].Username != alice.Username {
		t.Fatalf("expected alice alone in the roster, got %+v", roster)
	}
	if roster[0].IsMuted {
		t.Error("expected alice to start unmuted")
	}

	// Muting is reported by the browser and echoed to the room.
	statePayload, _ := json.Marshal(ws.SFUStatePayload{IsMuted: true})
	hub.Broadcast <- &ws.Event{
		Type:       ws.EventSFUState,
		RoomID:     roomID,
		SenderID:   alice.UserID,
		SenderName: alice.Username,
		Payload:    statePayload,
	}
	roster = decodeRoster(t, awaitEvent(t, bob.Send, ws.EventSFUPeers, 3*time.Second))
	if len(roster) != 1 || !roster[0].IsMuted {
		t.Fatalf("expected alice to be reported muted, got %+v", roster)
	}

	// Hanging up leaves the call without leaving the room.
	hub.Broadcast <- &ws.Event{
		Type:       ws.EventSFULeave,
		RoomID:     roomID,
		SenderID:   alice.UserID,
		SenderName: alice.Username,
	}
	roster = decodeRoster(t, awaitEvent(t, bob.Send, ws.EventSFUPeers, 3*time.Second))
	if len(roster) != 0 {
		t.Fatalf("expected an empty roster after alice hung up, got %+v", roster)
	}
}

func decodeRoster(t *testing.T, evt *ws.Event) []sfu.PeerState {
	t.Helper()
	var payload ws.SFUPeersPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("failed to decode roster payload: %v", err)
	}
	return payload.Peers
}
