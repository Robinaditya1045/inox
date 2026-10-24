package ws_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/ws"
)

// awaitPresence drains ch until a PRESENCE event lists exactly the wanted user
// IDs, and returns it. Earlier lists are skipped: each join or leave sends one,
// and a test usually cares only about where the room ended up.
func awaitPresence(t *testing.T, ch <-chan []byte, wantIDs ...string) []ws.PresenceUser {
	t.Helper()
	want := slices.Clone(wantIDs)
	slices.Sort(want)
	var last []string
	deadline := time.After(time.Second)
	for {
		select {
		case raw := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(raw, &evt); err != nil {
				t.Fatalf("malformed event on the wire: %v", err)
			}
			if evt.Type != ws.EventPresence {
				continue
			}
			var payload ws.PresencePayload
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("malformed presence payload: %v", err)
			}
			last = last[:0]
			for _, u := range payload.Users {
				last = append(last, u.UserID)
			}
			if slices.Equal(last, want) {
				return payload.Users
			}
		case <-deadline:
			t.Fatalf("timed out waiting for presence %v; last saw %v", want, last)
		}
	}
}

// Presence is about people, not connections: closing one of two tabs must not
// take someone offline, and a late joiner has to be told who is already there.
func TestPresenceListsPeopleNotConnections(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "presence-room"
	newClient := func(userID, name string, role domain.Role) *ws.Client {
		return &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: userID, Username: name, Role: role}
	}
	alice := newClient("alice", "Alice", domain.RoleOwner)
	aliceSecondTab := newClient("alice", "Alice", domain.RoleOwner)
	bob := newClient("bob", "Bob", domain.RoleMember)
	for _, c := range []*ws.Client{alice, aliceSecondTab, bob} {
		hub.Register <- c
	}

	users := awaitPresence(t, bob.Send, "alice", "bob")
	if users[0].Username != "Alice" || users[0].Role != domain.RoleOwner {
		t.Fatalf("expected alice listed by name and role, got %+v", users[0])
	}

	carol := newClient("carol", "Carol", domain.RoleMember)
	hub.Register <- carol
	awaitPresence(t, carol.Send, "alice", "bob", "carol")

	hub.Unregister <- alice
	awaitPresence(t, bob.Send, "alice", "bob", "carol")

	hub.Unregister <- aliceSecondTab
	awaitPresence(t, bob.Send, "bob", "carol")
	awaitPresence(t, carol.Send, "bob", "carol")
}

// A connection dropped for falling behind never sends LEAVE_ROOM, so presence
// is the only way the rest of the room finds out it has gone.
func TestPresenceDropsEvictedConnections(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "presence-evict"
	bob := &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: "bob", Username: "bob"}
	// Room for the events sent on joining and little else, and nobody reads it.
	slow := &ws.Client{Hub: hub, Send: make(chan []byte, 3), RoomID: roomID, UserID: "slow", Username: "slow"}
	hub.Register <- bob
	hub.Register <- slow
	awaitPresence(t, bob.Send, "bob", "slow")

	chat, _ := json.Marshal(ws.ChatPayload{Message: "hello?"})
	for range 4 {
		hub.Broadcast <- &ws.Event{Type: ws.EventChatMessage, RoomID: roomID, SenderID: bob.UserID, SenderName: bob.Username, Payload: chat}
	}
	awaitPresence(t, bob.Send, "bob")
}

// awaitClosed drains ch until the hub closes it.
func awaitClosed(t *testing.T, ch <-chan []byte) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the connection to be closed")
		}
	}
}

// A kicked member's sockets outlive their membership unless the hub closes
// them: they would keep receiving the room and keep showing as present.
func TestRemoveMemberDisconnectsEveryTabOfThatMemberOnly(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "kick-room"
	newClient := func(userID string) *ws.Client {
		return &ws.Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: userID, Username: userID}
	}
	alice, aliceSecondTab, bob := newClient("alice"), newClient("alice"), newClient("bob")
	for _, c := range []*ws.Client{alice, aliceSecondTab, bob} {
		hub.Register <- c
	}
	awaitPresence(t, bob.Send, "alice", "bob")

	hub.RemoveMember(roomID, "alice", ws.RemovalKicked)

	for _, tab := range []*ws.Client{alice, aliceSecondTab} {
		notice := awaitEvent(t, tab.Send, ws.EventRemovedFromRoom, time.Second)
		var payload ws.RemovedFromRoomPayload
		if err := json.Unmarshal(notice.Payload, &payload); err != nil || payload.Reason != ws.RemovalKicked {
			t.Fatalf("notice payload = %s, want reason kicked", notice.Payload)
		}
		awaitClosed(t, tab.Send)
	}

	awaitPresence(t, bob.Send, "bob")
	expectNoEvent(t, bob.Send, ws.EventRemovedFromRoom, 100*time.Millisecond)
	hub.Broadcast <- &ws.Event{Type: ws.EventPlay, RoomID: roomID, SenderID: "bob", SenderName: "bob", Payload: []byte(`{}`)}
	awaitEvent(t, bob.Send, ws.EventPlay, time.Second)
}
