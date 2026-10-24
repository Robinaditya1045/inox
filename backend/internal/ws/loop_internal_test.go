package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// churn registers and unregisters clients in roomID until stop closes, so that
// the loop is busy changing h.rooms while a test reaches in from elsewhere.
func churn(hub *Hub, roomID string, stop <-chan struct{}) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			c := &Client{Hub: hub, Send: make(chan []byte, 64), RoomID: roomID, UserID: fmt.Sprintf("u%d", i)}
			hub.Register <- c
			hub.Unregister <- c
		}
	}()
	return done
}

// Events from other processes arrive on the Redis subscription's goroutine. They
// must reach clients through the loop: delivering one walks h.rooms, which the
// loop is changing at the same time. Run with -race.
func TestRemoteEventsAreDeliveredOnTheLoop(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	const roomID = "remote"
	watcher := &Client{Hub: hub, Send: make(chan []byte, 1024), RoomID: roomID, UserID: "watcher", Username: "watcher"}
	hub.Register <- watcher

	stop := make(chan struct{})
	churned := churn(hub, roomID, stop)

	const sent = 50
	chat, _ := json.Marshal(ChatPayload{Message: "from another node"})
	for range sent {
		hub.deliverRemote(context.Background(), &Event{Type: EventChatMessage, RoomID: roomID, SenderID: "elsewhere", Payload: chat})
	}
	close(stop)
	<-churned

	received := 0
	deadline := time.After(time.Second)
	for received < sent {
		select {
		case raw := <-watcher.Send:
			var evt Event
			if json.Unmarshal(raw, &evt) == nil && evt.Type == EventChatMessage {
				received++
			}
		case <-deadline:
			t.Fatalf("watcher got %d of %d remote messages", received, sent)
		}
	}

	// A subscription still delivering when the hub stops must not hang.
	hub.Shutdown()
	returned := make(chan struct{})
	go func() {
		hub.deliverRemote(context.Background(), &Event{Type: EventChatMessage, RoomID: roomID})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("deliverRemote blocked after the hub stopped")
	}
}

// The telemetry hub samples rooms from its own goroutine. Run with -race.
func TestInspectRoomsIsSafeFromOtherGoroutines(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Shutdown()

	const roomID = "inspected"
	hub.Register <- &Client{Hub: hub, Send: make(chan []byte, 1024), RoomID: roomID, UserID: "watcher"}

	stop := make(chan struct{})
	churned := churn(hub, roomID, stop)
	for range 50 {
		hub.InspectRooms()
	}
	close(stop)
	<-churned

	rooms := hub.InspectRooms()
	if len(rooms) != 1 || rooms[0].RoomID != roomID || rooms[0].ParticipantCount != 1 {
		t.Fatalf("InspectRooms = %+v, want the one room with its one remaining client", rooms)
	}
}
