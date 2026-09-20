package sfu_test

import (
	"errors"
	"testing"
	"time"

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
	peerAlice, err := sfu.NewPeer("peer-1", "user-alice", "Alice", roomID, api, nil)
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

func TestTrackNameCarriesPublisherIdentity(t *testing.T) {
	name := sfu.TrackName(sfu.KindMic, "6f1c0d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f")
	kind, userID, ok := sfu.SplitTrackName(name)
	if !ok {
		t.Fatalf("expected %q to parse as a track name", name)
	}
	if kind != sfu.KindMic {
		t.Errorf("expected kind %q, got %q", sfu.KindMic, kind)
	}
	if userID != "6f1c0d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f" {
		t.Errorf("expected the publisher's user id back, got %q", userID)
	}

	// A browser-generated track ID must not be mistaken for one of ours.
	if _, _, ok := sfu.SplitTrackName("d7a1b2c3-dead-beef-0000-000000000000"); ok {
		t.Error("expected a random browser track id to be rejected")
	}
	if _, _, ok := sfu.SplitTrackName("camera.user-1"); ok {
		t.Error("expected an unknown kind to be rejected")
	}
}

func TestRoomRosterTracksMembershipAndMute(t *testing.T) {
	room := sfu.NewRoom("roster-room", nil, nil)

	changes := make(chan []sfu.PeerState, 8)
	room.SetStateHandler(func(_ string, peers []sfu.PeerState) {
		changes <- peers
	})

	alice, err := room.NewPeer("peer-a", "user-alice", "alice")
	if err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	room.AddPeer(alice)

	roster := <-changes
	if len(roster) != 1 || roster[0].UserID != "user-alice" || roster[0].IsMuted {
		t.Fatalf("expected one unmuted peer in the roster, got %+v", roster)
	}

	room.SetMuted("user-alice", true)
	roster = <-changes
	if len(roster) != 1 || !roster[0].IsMuted {
		t.Fatalf("expected alice to be reported muted, got %+v", roster)
	}

	// Reporting the same state again is not a change and must not be republished.
	room.SetMuted("user-alice", true)
	select {
	case extra := <-changes:
		t.Fatalf("expected no roster update for an unchanged mute state, got %+v", extra)
	default:
	}

	room.RemovePeer("user-alice")
	roster = <-changes
	if len(roster) != 0 {
		t.Fatalf("expected an empty roster after the peer left, got %+v", roster)
	}
}

// newBrowserPeerConnection returns a peer connection that gathers no ICE
// candidates, for tests about SDP rather than connectivity.
//
// Letting these connect for real is not just wasted work. pion's
// ICETransport.stop re-reads t.mux outside its lock (v3.3.6 icetransport.go:224)
// while Start writes it on connect, so closing a peer connection at the moment
// ICE comes up is a data race inside pion itself -- which CI caught and a fast
// machine does not. With no interfaces to gather from, no transport starts and
// the negotiation under test is unchanged.
func newBrowserPeerConnection(t *testing.T) *webrtc.PeerConnection {
	t.Helper()

	settings := webrtc.SettingEngine{}
	settings.SetInterfaceFilter(func(string) bool { return false })

	// A hand-built API starts with an empty media engine, which would reject every
	// m-line it is offered.
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		t.Fatalf("failed to register codecs: %v", err)
	}

	pc, err := webrtc.NewAPI(
		webrtc.WithSettingEngine(settings),
		webrtc.WithMediaEngine(mediaEngine),
	).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create browser-side peer connection: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	return pc
}

// A track attached after the browser's opening exchange carries no media until the
// browser has been offered it. This is the bug that made the second person to join
// a call inaudible to the first, so it is tested end to end against a real
// browser-side peer connection.
func TestPeerRenegotiatesTracksAddedAfterConnecting(t *testing.T) {
	// A nil API means Pion's defaults, which include the default codec set. An API
	// built by hand with no codecs registered rejects every m-line it is offered.
	room := sfu.NewRoom("renegotiation-room", nil, nil)

	peer, err := room.NewPeer("peer-a", "user-alice", "alice")
	if err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	defer peer.Close()

	offers := make(chan webrtc.SessionDescription, 4)
	peer.SetSignaler(func(sdp webrtc.SessionDescription) { offers <- sdp })
	room.AddPeer(peer)

	browser := newBrowserPeerConnection(t)

	if _, err := browser.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Fatalf("failed to add microphone transceiver: %v", err)
	}

	// 1. Opening exchange, driven by the browser.
	offer, err := browser.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create browser offer: %v", err)
	}
	if err := browser.SetLocalDescription(offer); err != nil {
		t.Fatalf("failed to apply browser offer: %v", err)
	}
	answer, err := peer.HandleOffer(offer)
	if err != nil {
		t.Fatalf("expected the sfu to answer the opening offer: %v", err)
	}
	if err := browser.SetRemoteDescription(answer); err != nil {
		t.Fatalf("failed to apply sfu answer: %v", err)
	}

	// Nothing has changed since that answer, so there is nothing to renegotiate.
	peer.Negotiate()
	select {
	case sdp := <-offers:
		t.Fatalf("expected no renegotiation for an unchanged track set, got %q", sdp.Type)
	default:
	}

	// 2. Someone else joins the call and starts publishing.
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		sfu.TrackName(sfu.KindMic, "user-bob"),
		sfu.TrackName(sfu.KindMic, "user-bob"),
	)
	if err != nil {
		t.Fatalf("failed to create fan-out track: %v", err)
	}
	if _, added, err := peer.AddTrack(track); err != nil || !added {
		t.Fatalf("expected the track to be attached, added=%v err=%v", added, err)
	}
	peer.Negotiate()

	var renegotiation webrtc.SessionDescription
	select {
	case renegotiation = <-offers:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the sfu to offer the newly attached track to the browser")
	}
	if renegotiation.Type != webrtc.SDPTypeOffer {
		t.Fatalf("expected an offer, got %q", renegotiation.Type)
	}

	// 3. A browser offer that collides with that one is refused rather than applied
	//    half-way; the browser rolls back and re-offers once this exchange closes.
	collidingOffer, err := browser.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create colliding offer: %v", err)
	}
	if _, err := peer.HandleOffer(collidingOffer); !errors.Is(err, sfu.ErrOfferCollision) {
		t.Fatalf("expected ErrOfferCollision, got %v", err)
	}

	// 4. The browser answers, closing the exchange.
	if err := browser.SetRemoteDescription(renegotiation); err != nil {
		t.Fatalf("failed to apply the sfu renegotiation offer: %v", err)
	}
	browserAnswer, err := browser.CreateAnswer(nil)
	if err != nil {
		t.Fatalf("failed to answer the sfu: %v", err)
	}
	if err := browser.SetLocalDescription(browserAnswer); err != nil {
		t.Fatalf("failed to apply the browser answer: %v", err)
	}
	if err := peer.HandleAnswer(browserAnswer); err != nil {
		t.Fatalf("failed to apply the browser answer on the sfu: %v", err)
	}

	if state := peer.PC.SignalingState(); state != webrtc.SignalingStateStable {
		t.Fatalf("expected a stable connection after the exchange, got %s", state)
	}

	// The browser now has a receiver for bob's microphone, named after him.
	var found bool
	for _, receiver := range browser.GetReceivers() {
		if receiver.Track() != nil && receiver.Track().ID() == sfu.TrackName(sfu.KindMic, "user-bob") {
			found = true
		}
	}
	if !found {
		t.Error("expected the browser to receive a track identified by its publisher")
	}
}

// A browser opens a call offering one microphone, so its opening exchange has
// room for one track back. Joining a call that already has two people in it must
// therefore renegotiate immediately, or the third person hears only the first.
func TestPeerFlushesTracksThatDidNotFitTheOpeningAnswer(t *testing.T) {
	room := sfu.NewRoom("busy-room", nil, nil)

	peer, err := room.NewPeer("peer-c", "user-carol", "carol")
	if err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	defer peer.Close()

	offers := make(chan webrtc.SessionDescription, 4)
	peer.SetSignaler(func(sdp webrtc.SessionDescription) { offers <- sdp })
	room.AddPeer(peer)

	// Two people are already talking when this peer arrives.
	for _, publisher := range []string{"user-alice", "user-bob"} {
		name := sfu.TrackName(sfu.KindMic, publisher)
		track, err := webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, name, name)
		if err != nil {
			t.Fatalf("failed to create fan-out track: %v", err)
		}
		if _, _, err := peer.AddTrack(track); err != nil {
			t.Fatalf("failed to attach fan-out track: %v", err)
		}
	}

	browser := newBrowserPeerConnection(t)

	if _, err := browser.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Fatalf("failed to add microphone transceiver: %v", err)
	}

	offer, err := browser.CreateOffer(nil)
	if err != nil {
		t.Fatalf("failed to create browser offer: %v", err)
	}
	if err := browser.SetLocalDescription(offer); err != nil {
		t.Fatalf("failed to apply browser offer: %v", err)
	}
	answer, err := peer.HandleOffer(offer)
	if err != nil {
		t.Fatalf("expected the sfu to answer the opening offer: %v", err)
	}
	if err := browser.SetRemoteDescription(answer); err != nil {
		t.Fatalf("failed to apply sfu answer: %v", err)
	}

	peer.Negotiate()

	var renegotiation webrtc.SessionDescription
	select {
	case renegotiation = <-offers:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the sfu to offer the track that did not fit the answer")
	}

	if err := browser.SetRemoteDescription(renegotiation); err != nil {
		t.Fatalf("failed to apply the sfu renegotiation offer: %v", err)
	}
	browserAnswer, err := browser.CreateAnswer(nil)
	if err != nil {
		t.Fatalf("failed to answer the sfu: %v", err)
	}
	if err := browser.SetLocalDescription(browserAnswer); err != nil {
		t.Fatalf("failed to apply the browser answer: %v", err)
	}
	if err := peer.HandleAnswer(browserAnswer); err != nil {
		t.Fatalf("failed to apply the browser answer on the sfu: %v", err)
	}

	received := make(map[string]bool)
	for _, receiver := range browser.GetReceivers() {
		if track := receiver.Track(); track != nil {
			received[track.ID()] = true
		}
	}
	for _, publisher := range []string{"user-alice", "user-bob"} {
		if !received[sfu.TrackName(sfu.KindMic, publisher)] {
			t.Errorf("expected the browser to receive %s's microphone, got %v", publisher, received)
		}
	}
}
