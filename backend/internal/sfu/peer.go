package sfu

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/pion/webrtc/v3"
)

// Signaler hands an SDP the SFU generated to the browser it belongs to. The hub
// installs one per peer so this package never needs to know how signaling is framed.
type Signaler func(webrtc.SessionDescription)

var (
	// ErrOfferCollision reports a browser offer that arrived while the SFU was still
	// waiting for the answer to an offer of its own. Only one side may drive an
	// exchange at a time, and the SFU keeps the wheel: the browser is the polite peer
	// and rolls its own offer back, so it will re-offer once this exchange settles.
	ErrOfferCollision = errors.New("sfu: offer collided with a renegotiation in flight")

	// ErrPeerClosed reports signaling that arrived after the peer connection went away.
	ErrPeerClosed = errors.New("sfu: peer connection is closed")
)

// Peer represents an individual WebRTC client connection inside an SFU Room.
//
// A peer is both a publisher (its microphone and screen) and a subscriber (everyone
// else's tracks). Subscriptions change whenever someone joins, leaves, or starts
// sharing, and every one of those changes needs a fresh offer/answer exchange —
// tracks attached to an already-negotiated connection carry no media until the
// browser has been told they exist. Negotiate drives those exchanges.
type Peer struct {
	ID       string
	UserID   string
	Username string
	RoomID   string

	PC *webrtc.PeerConnection

	mu      sync.Mutex
	senders map[string]*webrtc.RTPSender
	signal  Signaler

	// established gates server-initiated offers until the browser's first offer has
	// been answered. Offering before that races the browser's opening offer for no
	// gain: tracks attached beforehand are already folded into that first answer.
	established bool
	// dirty marks a track set the browser has not been told about yet.
	dirty bool
	// queued marks a renegotiation deferred because an exchange was already in flight.
	queued bool

	muted  bool
	closed bool
}

// NewPeer initializes a WebRTC PeerConnection configured for SFU media routing.
//
// A nil api uses Pion's package defaults, and empty iceServers fall back to a
// public STUN server; Room.NewPeer supplies both from the deployment's
// NetworkConfig so peers created for real traffic honour it.
func NewPeer(id, userID, username, roomID string, api *webrtc.API, iceServers []webrtc.ICEServer) (*Peer, error) {
	if len(iceServers) == 0 {
		iceServers = DefaultICEServers()
	}
	// Configure ICE servers (STUN/TURN) for NAT traversal
	config := webrtc.Configuration{ICEServers: iceServers}

	var pc *webrtc.PeerConnection
	var err error
	if api != nil {
		pc, err = api.NewPeerConnection(config)
	} else {
		pc, err = webrtc.NewPeerConnection(config)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	return &Peer{
		ID:       id,
		UserID:   userID,
		Username: username,
		RoomID:   roomID,
		PC:       pc,
		senders:  make(map[string]*webrtc.RTPSender),
	}, nil
}

// SetSignaler installs the transport used to deliver server-initiated offers.
func (p *Peer) SetSignaler(signal Signaler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.signal = signal
}

// AddTrack attaches a fan-out track to this peer's WebRTC connection and records
// that the browser now has to be told about it. It does not negotiate; the caller
// decides when, because a burst of subscriptions should cost one exchange, not one
// each.
//
// added is false when the peer was already subscribed to the track, so a caller
// that spawns work per subscription does not spawn it twice.
func (p *Peer) AddTrack(track *webrtc.TrackLocalStaticRTP) (sender *webrtc.RTPSender, added bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, false, ErrPeerClosed
	}
	if existing, ok := p.senders[track.ID()]; ok {
		return existing, false, nil
	}

	sender, err = p.PC.AddTrack(track)
	if err != nil {
		return nil, false, fmt.Errorf("failed to add track to peer: %w", err)
	}

	p.senders[track.ID()] = sender
	p.dirty = true
	return sender, true, nil
}

// RemoveTrack detaches a fan-out track and records the change for the next
// negotiation. Removing a track the peer never had is a no-op.
func (p *Peer) RemoveTrack(trackID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sender, ok := p.senders[trackID]
	if !ok {
		return
	}
	delete(p.senders, trackID)
	if p.closed {
		return
	}
	if err := p.PC.RemoveTrack(sender); err != nil {
		slog.Warn("failed to detach fan-out track from peer", "user_id", p.UserID, "track_id", trackID, "error", err)
		return
	}
	p.dirty = true
}

// HandleOffer applies a browser offer and returns the answer to send back.
func (p *Peer) HandleOffer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return webrtc.SessionDescription{}, ErrPeerClosed
	}
	if p.PC.SignalingState() != webrtc.SignalingStateStable {
		return webrtc.SessionDescription{}, ErrOfferCollision
	}

	if err := p.PC.SetRemoteDescription(offer); err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("set remote offer failed: %w", err)
	}
	answer, err := p.PC.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("create answer failed: %w", err)
	}
	if err := p.PC.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("set local answer failed: %w", err)
	}
	// The description Pion hands back from SetLocalDescription is the one to send:
	// CreateAnswer runs before the ICE gatherer is started, so its SDP can still be
	// missing the ICE credentials the browser needs to accept it at all.
	if local := p.PC.LocalDescription(); local != nil {
		answer = *local
	}

	p.established = true
	// An answer can only describe as many tracks as the offer had m-lines for. A
	// browser opening a call offers one microphone and gets back whatever fits;
	// anything left over is still waiting for an offer of its own, which the
	// caller sends by negotiating once this exchange is closed.
	p.dirty = p.hasUnnegotiatedSenders()
	return answer, nil
}

// hasUnnegotiatedSenders reports whether any attached track is still missing from
// the session description. A transceiver only gets a mid once it has been
// negotiated, so an attached sender without one is a track the browser cannot yet
// receive.
func (p *Peer) hasUnnegotiatedSenders() bool {
	for _, transceiver := range p.PC.GetTransceivers() {
		if transceiver.Mid() != "" {
			continue
		}
		if sender := transceiver.Sender(); sender != nil && sender.Track() != nil {
			return true
		}
	}
	return false
}

// HandleAnswer applies the browser's answer to a server-initiated offer, then runs
// any renegotiation that was deferred while this exchange was open.
func (p *Peer) HandleAnswer(answer webrtc.SessionDescription) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPeerClosed
	}
	if p.PC.SignalingState() != webrtc.SignalingStateHaveLocalOffer {
		// An answer to an offer this peer already rolled past: harmless, and applying
		// it would corrupt the current exchange.
		p.mu.Unlock()
		slog.Debug("ignoring sfu answer outside an open exchange", "user_id", p.UserID)
		return nil
	}
	err := p.PC.SetRemoteDescription(answer)
	queued := p.queued
	p.queued = false
	p.mu.Unlock()

	if err != nil {
		return fmt.Errorf("set remote answer failed: %w", err)
	}
	if queued {
		p.Negotiate()
	}
	return nil
}

// Negotiate offers the current track set to the browser if it has fallen behind.
//
// It is safe to call after every subscription change: it does nothing when the
// browser is already up to date, and defers to after the current exchange when one
// is open rather than producing an offer the browser would have to roll back.
func (p *Peer) Negotiate() {
	p.mu.Lock()
	if p.closed || !p.established || !p.dirty {
		p.mu.Unlock()
		return
	}
	if p.PC.SignalingState() != webrtc.SignalingStateStable {
		p.queued = true
		p.mu.Unlock()
		return
	}

	offer, err := p.PC.CreateOffer(nil)
	if err != nil {
		p.mu.Unlock()
		slog.Error("failed to create sfu renegotiation offer", "user_id", p.UserID, "error", err)
		return
	}
	if err := p.PC.SetLocalDescription(offer); err != nil {
		p.mu.Unlock()
		slog.Error("failed to apply sfu renegotiation offer", "user_id", p.UserID, "error", err)
		return
	}
	if local := p.PC.LocalDescription(); local != nil {
		offer = *local
	}
	p.dirty = false
	signal := p.signal
	p.mu.Unlock()

	if signal == nil {
		slog.Warn("sfu renegotiation has nowhere to go; no signaler installed", "user_id", p.UserID)
		return
	}
	slog.Debug("sfu renegotiating with browser", "user_id", p.UserID, "room_id", p.RoomID)
	signal(offer)
}

// AddICECandidate adds a remote ICE candidate sent by the browser.
func (p *Peer) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPeerClosed
	}
	if err := p.PC.AddICECandidate(candidate); err != nil {
		return fmt.Errorf("add ice candidate failed: %w", err)
	}
	return nil
}

// Usable reports whether this peer can still carry media. A browser that dies
// without hanging up leaves one behind, and answering a fresh call on a dead
// connection would fail in ways that look like the new call being broken.
func (p *Peer) Usable() bool {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()

	if closed {
		return false
	}
	switch p.PC.ConnectionState() {
	case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
		return false
	default:
		return true
	}
}

// SetMuted records the browser's own microphone state and reports whether it
// changed. Muting is local to the publisher -- the track keeps flowing, silent --
// so the SFU cannot observe it and has to be told.
func (p *Peer) SetMuted(muted bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.muted == muted {
		return false
	}
	p.muted = muted
	return true
}

// Muted reports the peer's last known microphone state.
func (p *Peer) Muted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.muted
}

// Close gracefully terminates the WebRTC PeerConnection. It is idempotent: a peer
// can be torn down by an explicit leave, a failed connection, or a room shutdown,
// and any two of those may race.
func (p *Peer) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.senders = make(map[string]*webrtc.RTPSender)
	p.mu.Unlock()

	slog.Info("closing webrtc peer connection", "peer_id", p.ID, "user_id", p.UserID)
	return p.PC.Close()
}
