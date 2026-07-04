package sfu

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/pion/webrtc/v3"
)

// Peer represents an individual WebRTC client connection inside an SFU Room.
type Peer struct {
	ID       string
	UserID   string
	Username string
	RoomID   string

	PC     *webrtc.PeerConnection
	Tracks map[string]*webrtc.TrackLocalStaticRTP

	mu sync.RWMutex
}

// NewPeer initializes a WebRTC PeerConnection configured for SFU media routing.
func NewPeer(id, userID, username, roomID string, api *webrtc.API) (*Peer, error) {
	// Configure ICE servers (STUN/TURN) for NAT traversal
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

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

	p := &Peer{
		ID:       id,
		UserID:   userID,
		Username: username,
		RoomID:   roomID,
		PC:       pc,
		Tracks:   make(map[string]*webrtc.TrackLocalStaticRTP),
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("webrtc peer connection state changed", "peer_id", p.ID, "user_id", p.UserID, "state", state.String())
	})

	return p, nil
}

// AddTrack attaches a local static RTP track to this peer's WebRTC connection.
func (p *Peer) AddTrack(track *webrtc.TrackLocalStaticRTP) (*webrtc.RTPSender, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sender, err := p.PC.AddTrack(track)
	if err != nil {
		return nil, fmt.Errorf("failed to add track to peer: %w", err)
	}

	p.Tracks[track.ID()] = track
	return sender, nil
}

// RemoveTrack detaches a track from this peer's WebRTC connection.
func (p *Peer) RemoveTrack(trackID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.Tracks, trackID)
}

// CreateOffer generates an SDP offer for negotiating media tracks.
func (p *Peer) CreateOffer() (webrtc.SessionDescription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	offer, err := p.PC.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("create offer failed: %w", err)
	}

	if err := p.PC.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("set local description failed: %w", err)
	}

	return offer, nil
}

// SetRemoteDescription applies SDP from the browser client.
func (p *Peer) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.PC.SetRemoteDescription(sdp); err != nil {
		return fmt.Errorf("set remote description failed: %w", err)
	}
	return nil
}

// CreateAnswer generates an SDP answer in response to a browser client offer.
func (p *Peer) CreateAnswer() (webrtc.SessionDescription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	answer, err := p.PC.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("create answer failed: %w", err)
	}

	if err := p.PC.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("set local description failed: %w", err)
	}

	return answer, nil
}

// AddICECandidate adds a remote ICE candidate sent by the browser.
func (p *Peer) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.PC.AddICECandidate(candidate); err != nil {
		return fmt.Errorf("add ice candidate failed: %w", err)
	}
	return nil
}

// Close gracefully terminates the WebRTC PeerConnection.
func (p *Peer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	slog.Info("closing webrtc peer connection", "peer_id", p.ID, "user_id", p.UserID)
	return p.PC.Close()
}
