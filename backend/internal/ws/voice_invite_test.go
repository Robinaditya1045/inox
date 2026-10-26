package ws_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/sfu"
	"github.com/inox/inox/backend/internal/ws"
)

// awaitFirst drains ch until an event of one of the given types arrives, and
// returns it. Asserting that something was dropped is done by sending a later
// event the test expects instead: the hub handles events in order, so the
// dropped one would have arrived first.
func awaitFirst(t *testing.T, ch <-chan []byte, timeout time.Duration, types ...ws.EventType) *ws.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case raw := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(raw, &evt); err != nil {
				t.Fatalf("malformed event on the wire: %v", err)
			}
			if slices.Contains(types, evt.Type) {
				return &evt
			}
		case <-deadline:
			t.Fatalf("timed out waiting for any of %v", types)
		}
	}
}

func voiceEvent(typ ws.EventType, roomID string, from *ws.Client, targetID string, payload any) *ws.Event {
	evt := &ws.Event{
		Type:       typ,
		RoomID:     roomID,
		SenderID:   from.UserID,
		SenderName: from.Username,
		TargetID:   targetID,
		Timestamp:  time.Now().UnixMilli(),
	}
	if payload != nil {
		evt.Payload, _ = json.Marshal(payload)
	}
	return evt
}

// An invite reaches the one person it names, and nobody gets one they were not
// meant to: not the whole room, not the sender, and not the same person twice
// in a row.
func TestVoiceInviteIsDeliveredOnlyToItsTarget(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "voice-invite-room"
	newClient := func(userID, name string) *ws.Client {
		return &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: userID, Username: name}
	}
	alice := newClient("alice", "Alice")
	bob := newClient("bob", "Bob")
	carol := newClient("carol", "Carol")
	for _, c := range []*ws.Client{alice, bob, carol} {
		hub.Register <- c
	}

	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, alice, "", nil)      // untargeted
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, alice, "alice", nil) // herself
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, alice, "bob", nil)
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, alice, "bob", nil) // too soon
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, carol, "bob", nil)
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, alice, "carol", nil)

	first := awaitFirst(t, bob.Send, time.Second, ws.EventVoiceInvite)
	if first.SenderID != "alice" || first.SenderName != "Alice" || first.TargetID != "bob" {
		t.Fatalf("expected bob's first invite to be alice's, got %+v", first)
	}
	if second := awaitFirst(t, bob.Send, time.Second, ws.EventVoiceInvite); second.SenderID != "carol" {
		t.Fatalf("expected alice's repeat invite to be dropped, but bob got another from %s", second.SenderID)
	}
	if got := awaitFirst(t, carol.Send, time.Second, ws.EventVoiceInvite); got.TargetID != "carol" {
		t.Fatalf("expected carol to see only the invite addressed to her, got one for %q", got.TargetID)
	}

	// Bob's answer goes back to alice alone.
	hub.Broadcast <- voiceEvent(ws.EventVoiceInviteReply, roomID, bob, "alice", ws.VoiceInviteReplyPayload{Accepted: true})
	hub.Broadcast <- voiceEvent(ws.EventVoiceInviteReply, roomID, carol, "alice", ws.VoiceInviteReplyPayload{Accepted: false})

	reply := awaitFirst(t, alice.Send, time.Second, ws.EventVoiceInviteReply)
	var payload ws.VoiceInviteReplyPayload
	if err := json.Unmarshal(reply.Payload, &payload); err != nil {
		t.Fatalf("malformed reply payload: %v", err)
	}
	if reply.SenderID != "bob" || !payload.Accepted {
		t.Fatalf("expected bob's acceptance, got %+v (accepted=%v)", reply, payload.Accepted)
	}
	// Neither reply was addressed to carol, so the first she sees is this one.
	hub.Broadcast <- voiceEvent(ws.EventVoiceInviteReply, roomID, alice, "carol", ws.VoiceInviteReplyPayload{})
	if got := awaitFirst(t, carol.Send, time.Second, ws.EventVoiceInviteReply); got.SenderID != "alice" {
		t.Fatalf("expected carol to receive only the reply addressed to her, got %+v", got)
	}
}

// Accepting an invite means joining the inviter in voice, so only someone in
// the call can send one.
func TestVoiceInviteRequiresInviterInVoice(t *testing.T) {
	hub := ws.NewHub()
	hub.SetSFUManager(sfu.NewManager())
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "voice-invite-sfu-room"
	alice := &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: "alice", Username: "Alice"}
	bob := &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: "bob", Username: "Bob"}
	hub.Register <- alice
	hub.Register <- bob

	// Bob is not in voice: his invite must not reach alice before her own
	// answer from the SFU does.
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, bob, "alice", nil)

	_, offer := newBrowserOffer(t)
	hub.Broadcast <- &ws.Event{Type: ws.EventSFUOffer, RoomID: roomID, SenderID: alice.UserID, SenderName: alice.Username, Payload: offer}
	if got := awaitFirst(t, alice.Send, 3*time.Second, ws.EventVoiceInvite, ws.EventSFUAnswer); got.Type != ws.EventSFUAnswer {
		t.Fatalf("expected bob's invite to be dropped while he is out of voice, got %+v", got)
	}

	// Alice is in voice now, so hers goes through.
	hub.Broadcast <- voiceEvent(ws.EventVoiceInvite, roomID, alice, "bob", nil)
	if got := awaitFirst(t, bob.Send, 3*time.Second, ws.EventVoiceInvite); got.SenderID != "alice" {
		t.Fatalf("expected an invite from alice, got %+v", got)
	}
}
