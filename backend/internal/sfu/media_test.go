package sfu_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/sfu"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

// fakeBrowser is a real peer connection playing the part of a browser in a call:
// it drives the opening exchange, answers the offers the SFU sends it as the room
// changes, and reports the media it actually receives.
type fakeBrowser struct {
	userID string
	pc     *webrtc.PeerConnection
	peer   *sfu.Peer

	// mu serializes this connection's signaling, which the SFU may drive from its
	// own goroutines at the same time as the test drives it from this one.
	mu sync.Mutex

	receivedMu sync.Mutex
	received   map[string]bool

	// streams is what this browser declares its published streams to be. Nil
	// plays a browser from before cameras, which declares nothing.
	streams map[string]string
}

func joinCall(t *testing.T, room *sfu.Room, userID string) *fakeBrowser {
	t.Helper()

	peer, err := room.NewPeer(userID, userID, userID)
	if err != nil {
		t.Fatalf("failed to create sfu peer: %v", err)
	}
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create browser peer connection: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	browser := &fakeBrowser{
		userID:   userID,
		pc:       pc,
		peer:     peer,
		received: make(map[string]bool),
	}

	// Renegotiation from the SFU: somebody joined, left, or started sharing.
	peer.SetSignaler(func(sdp webrtc.SessionDescription) {
		// The test can finish with an exchange still in flight; the call being
		// over by the time it lands is not a failure.
		hungUp := func(err error) bool {
			return errors.Is(err, sfu.ErrPeerClosed) || pc.SignalingState() == webrtc.SignalingStateClosed
		}

		browser.mu.Lock()
		err := pc.SetRemoteDescription(sdp)
		var answer webrtc.SessionDescription
		if err == nil {
			answer, err = pc.CreateAnswer(nil)
		}
		if err == nil {
			err = pc.SetLocalDescription(answer)
		}
		local := pc.LocalDescription()
		browser.declareStreams()
		browser.mu.Unlock()
		if err != nil {
			if !hungUp(err) {
				t.Errorf("[%s] failed to answer sfu: %v", userID, err)
			}
			return
		}

		// Handed back without the lock held: applying the answer can flush a
		// renegotiation the SFU deferred, which lands straight back in here.
		if err := peer.HandleAnswer(*local); err != nil && !hungUp(err) {
			t.Errorf("[%s] sfu rejected answer: %v", userID, err)
		}
	})

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = peer.AddICECandidate(c.ToJSON())
		}
	})
	peer.PC.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = pc.AddICECandidate(c.ToJSON())
		}
	})

	// Media only counts once packets arrive: a receiver with no RTP behind it is
	// exactly the symptom a missing renegotiation produces.
	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		go func() {
			buf := make([]byte, 1500)
			for {
				if _, _, err := remote.Read(buf); err != nil {
					return
				}
				browser.receivedMu.Lock()
				browser.received[remote.ID()] = true
				browser.receivedMu.Unlock()
			}
		}()
	})

	room.AddPeer(peer)
	return browser
}

// declareStreams tells the SFU what this browser's streams are, as the hub does
// with the declaration a browser sends alongside every description.
func (b *fakeBrowser) declareStreams() {
	if b.streams == nil {
		return
	}
	declared := make(map[string]string, len(b.streams))
	for id, kind := range b.streams {
		declared[id] = kind
	}
	b.peer.DeclareStreams(declared)
}

// publish adds a track and drives the exchange the browser would drive for it.
func (b *fakeBrowser) publish(t *testing.T, track webrtc.TrackLocal) *webrtc.RTPSender {
	t.Helper()

	sender, err := b.pc.AddTrack(track)
	if err != nil {
		t.Fatalf("[%s] failed to add track: %v", b.userID, err)
	}
	b.renegotiate(t)
	return sender
}

// unpublish stops sending a track and drives the exchange that tells the SFU.
func (b *fakeBrowser) unpublish(t *testing.T, sender *webrtc.RTPSender) {
	t.Helper()

	if err := b.pc.RemoveTrack(sender); err != nil {
		t.Fatalf("[%s] failed to remove track: %v", b.userID, err)
	}
	b.renegotiate(t)
}

// renegotiate offers the browser's current track set and applies the answer.
func (b *fakeBrowser) renegotiate(t *testing.T) {
	t.Helper()

	// A real browser rolls back an offer that collides with one from the SFU.
	// Pion cannot roll back, so this harness waits for a quiet moment instead.
	waitUntil(t, func() bool {
		return b.peer.PC.SignalingState() == webrtc.SignalingStateStable
	}, "sfu peer to settle before publishing")

	b.mu.Lock()
	offer, err := b.pc.CreateOffer(nil)
	if err == nil {
		err = b.pc.SetLocalDescription(offer)
	}
	local := b.pc.LocalDescription()
	b.declareStreams()
	b.mu.Unlock()
	if err != nil {
		t.Fatalf("[%s] failed to offer: %v", b.userID, err)
	}

	answer, err := b.peer.HandleOffer(*local)
	if err != nil {
		t.Fatalf("[%s] sfu failed to answer: %v", b.userID, err)
	}

	b.mu.Lock()
	err = b.pc.SetRemoteDescription(answer)
	b.mu.Unlock()
	if err != nil {
		t.Fatalf("[%s] failed to apply sfu answer: %v", b.userID, err)
	}

	// Anything the answer had no room for -- other people already in the call --
	// follows in an offer from the SFU.
	b.peer.Negotiate()
}

func (b *fakeBrowser) publishMicrophone(t *testing.T) {
	t.Helper()
	b.publish(t, streamSamples(t, webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "mic", "browser-stream"))
}

// publishScreen publishes video without saying what it is, as a browser from
// before cameras did.
func (b *fakeBrowser) publishScreen(t *testing.T) {
	t.Helper()
	b.publish(t, streamSamples(t, webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "screen", "browser-stream"))
}

// publishVideo publishes video in its own stream and declares that stream to be
// kind, as a camera-aware browser does.
func (b *fakeBrowser) publishVideo(t *testing.T, kind string) *webrtc.RTPSender {
	t.Helper()

	streamID := kind + "-stream"
	b.mu.Lock()
	if b.streams == nil {
		b.streams = make(map[string]string)
	}
	b.streams[streamID] = kind
	b.mu.Unlock()

	return b.publish(t, streamSamples(t, webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, kind, streamID))
}

// streamSamples returns a track that writes filler media until the test ends.
func streamSamples(t *testing.T, codec webrtc.RTPCodecCapability, id, streamID string) *webrtc.TrackLocalStaticSample {
	t.Helper()

	track, err := webrtc.NewTrackLocalStaticSample(codec, id, streamID)
	if err != nil {
		t.Fatalf("failed to create local track: %v", err)
	}

	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := track.WriteSample(media.Sample{
					Data:     []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
					Duration: 20 * time.Millisecond,
				}); err != nil {
					return
				}
			}
		}
	}()

	return track
}

func (b *fakeBrowser) awaitMedia(t *testing.T, trackName string) {
	t.Helper()
	waitUntil(t, func() bool {
		b.receivedMu.Lock()
		defer b.receivedMu.Unlock()
		return b.received[trackName]
	}, b.userID+" to receive "+trackName)
}

// awaitPeerState waits for the room's roster to report userID as want describes.
func awaitPeerState(t *testing.T, room *sfu.Room, userID string, want func(sfu.PeerState) bool, what string) {
	t.Helper()
	waitUntil(t, func() bool {
		for _, entry := range room.Roster() {
			if entry.UserID == userID {
				return want(entry)
			}
		}
		return false
	}, what)
}

func waitUntil(t *testing.T, condition func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The whole point of an SFU, exercised over real peer connections: everyone hears
// everyone, however late they arrive, and a screen started mid-call reaches
// people whose connection was negotiated long before it existed.
func TestRoomForwardsMediaToEveryoneInTheCall(t *testing.T) {
	room := sfu.NewRoom("media-room", nil, nil)
	defer room.Close()

	alice := joinCall(t, room, "user-alice")
	alice.publishMicrophone(t)

	bob := joinCall(t, room, "user-bob")
	bob.publishMicrophone(t)

	// Bob hearing Alice only needs the answer to Bob's own offer. Alice hearing
	// Bob needs the SFU to renegotiate a connection that was already up, which is
	// the half that used to be missing.
	bob.awaitMedia(t, sfu.TrackName(sfu.KindMic, "user-alice"))
	alice.awaitMedia(t, sfu.TrackName(sfu.KindMic, "user-bob"))

	alice.publishScreen(t)
	bob.awaitMedia(t, sfu.TrackName(sfu.KindScreen, "user-alice"))

	if count := room.GetPeerCount(); count != 2 {
		t.Errorf("expected 2 peers in the room, got %d", count)
	}
	roster := room.Roster()
	if len(roster) != 2 {
		t.Fatalf("expected 2 rows in the roster, got %+v", roster)
	}
	for _, entry := range roster {
		if entry.UserID == "user-alice" && !entry.IsScreenSharing {
			t.Error("expected alice to be reported as sharing her screen")
		}
		if entry.UserID == "user-bob" && entry.IsScreenSharing {
			t.Error("expected bob not to be reported as sharing")
		}
	}
}

// A camera and a screen share are both just video on the wire. The browser's
// declaration is what tells them apart, so that each reaches the far side under
// its own name, the roster reports both, and turning one off leaves the other.
func TestRoomTellsCamerasFromScreenShares(t *testing.T) {
	room := sfu.NewRoom("camera-room", nil, nil)
	defer room.Close()

	alice := joinCall(t, room, "user-alice")
	alice.publishMicrophone(t)

	bob := joinCall(t, room, "user-bob")
	bob.publishMicrophone(t)

	// Let the call settle before Alice publishes again. Alice hearing Bob takes an
	// offer from the SFU, sent whenever Bob's first packets happen to arrive; an
	// offer of Alice's own crossing it collides, which a browser rolls back but
	// Pion -- and so this harness -- cannot.
	alice.awaitMedia(t, sfu.TrackName(sfu.KindMic, "user-bob"))
	bob.awaitMedia(t, sfu.TrackName(sfu.KindMic, "user-alice"))

	camera := alice.publishVideo(t, sfu.KindCamera)
	alice.publishVideo(t, sfu.KindScreen)

	bob.awaitMedia(t, sfu.TrackName(sfu.KindCamera, "user-alice"))
	bob.awaitMedia(t, sfu.TrackName(sfu.KindScreen, "user-alice"))
	awaitPeerState(t, room, "user-alice", func(s sfu.PeerState) bool {
		return s.IsCameraOn && s.IsScreenSharing
	}, "alice's camera and screen share in the roster")
	awaitPeerState(t, room, "user-bob", func(s sfu.PeerState) bool {
		return !s.IsCameraOn && !s.IsScreenSharing
	}, "bob to have published neither camera nor screen")

	alice.unpublish(t, camera)
	awaitPeerState(t, room, "user-alice", func(s sfu.PeerState) bool {
		return !s.IsCameraOn && s.IsScreenSharing
	}, "alice's camera to go off while her screen share carries on")
}
