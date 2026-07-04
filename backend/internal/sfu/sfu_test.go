package sfu_test

import (
	"errors"
	"testing"

	"github.com/inox/inox/backend/internal/sfu"
	"github.com/pion/webrtc/v3"
)

func TestSFUManagerRoomLifecycle(t *testing.T) {
	mgr := sfu.NewManager()

	roomID := "cinema-sfu-101"

	// 1. Verify getting non-existent room returns ErrRoomNotFound
	_, err := mgr.GetRoom(roomID)
	if !errors.Is(err, sfu.ErrRoomNotFound) {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}

	// 2. Create room
	room := mgr.GetOrCreateRoom(roomID)
	if room == nil || room.ID != roomID {
		t.Fatalf("expected valid sfu room creation with matching ID")
	}

	// 3. Verify room can now be fetched
	fetched, err := mgr.GetRoom(roomID)
	if err != nil {
		t.Fatalf("expected GetRoom to succeed, got %v", err)
	}
	if fetched.ID != roomID {
		t.Errorf("expected room id match, got %s", fetched.ID)
	}

	// 4. Test Peer initialization and registration
	api := webrtc.NewAPI()
	peerAlice, err := sfu.NewPeer("peer-1", "user-alice", "Alice", roomID, api)
	if err != nil {
		t.Fatalf("expected peer creation to succeed, got %v", err)
	}

	room.AddPeer(peerAlice)

	retrievedPeer, err := room.GetPeer("user-alice")
	if err != nil {
		t.Fatalf("expected GetPeer to succeed, got %v", err)
	}
	if retrievedPeer.Username != "Alice" {
		t.Errorf("expected username Alice, got %s", retrievedPeer.Username)
	}

	// 5. Test removing peer
	room.RemovePeer("user-alice")
	_, err = room.GetPeer("user-alice")
	if !errors.Is(err, sfu.ErrPeerNotFound) {
		t.Errorf("expected ErrPeerNotFound after removal, got %v", err)
	}

	// 6. Test room removal and manager shutdown
	mgr.RemoveRoom(roomID)
	_, err = mgr.GetRoom(roomID)
	if !errors.Is(err, sfu.ErrRoomNotFound) {
		t.Errorf("expected ErrRoomNotFound after RemoveRoom, got %v", err)
	}

	mgr.Shutdown()
}
