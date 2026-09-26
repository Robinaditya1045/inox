package sfu_test

import (
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
		browser.mu.Lock()
		defer browser.mu.Unlock()

		if err := pc.SetRemoteDescription(sdp); err != nil {
			t.Errorf("[%s] failed to apply sfu offer: %v", userID, err)
			return
		}
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			t.Errorf("[%s] failed to answer sfu: %v", userID, err)
			return
		}
		if err := pc.SetLocalDescription(answer); err != nil {
			t.Errorf("[%s] failed to apply answer: %v", userID, err)
			return
		}
		if err := peer.HandleAnswer(*pc.LocalDescription()); err != nil {
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

// publish adds a track and drives the exchange the browser would drive for it.
func (b *fakeBrowser) publish(t *testing.T, track webrtc.TrackLocal) {
	t.Helper()

	if _, err := b.pc.AddTrack(track); err != nil {
		t.Fatalf("[%s] failed to add track: %v", b.userID, err)
	}

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
	b.publish(t, streamSamples(t, webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "mic"))
}

func (b *fakeBrowser) publishScreen(t *testing.T) {
	t.Helper()
	b.publish(t, streamSamples(t, webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "screen"))
}

// streamSamples returns a track that writes filler media until the test ends.
func streamSamples(t *testing.T, codec webrtc.RTPCodecCapability, id string) *webrtc.TrackLocalStaticSample {
	t.Helper()

	track, err := webrtc.NewTrackLocalStaticSample(codec, id, "browser-stream")
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
