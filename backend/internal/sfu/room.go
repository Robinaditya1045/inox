package sfu

import (
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/inox/inox/backend/internal/observability"
	"github.com/pion/webrtc/v3"
)

var (
	ErrPeerNotFound = errors.New("peer not found in sfu room")
)

// Room manages media routing across all connected peers within a Watch Party workspace.
type Room struct {
	ID     string
	Peers  map[string]*Peer
	tracks map[string]*webrtc.TrackLocalStaticRTP

	mu sync.RWMutex
}

// NewRoom initializes a new SFU media routing workspace.
func NewRoom(id string) *Room {
	return &Room{
		ID:     id,
		Peers:  make(map[string]*Peer),
		tracks: make(map[string]*webrtc.TrackLocalStaticRTP),
	}
}

// AddPeer registers a new WebRTC peer and subscribes them to all active media tracks in the room.
func (r *Room) AddPeer(peer *Peer) {
	r.mu.Lock()
	r.Peers[peer.UserID] = peer
	existingTracks := make([]*webrtc.TrackLocalStaticRTP, 0, len(r.tracks))
	for _, t := range r.tracks {
		existingTracks = append(existingTracks, t)
	}
	r.mu.Unlock()

	slog.Info("sfu peer joined room", "room_id", r.ID, "user_id", peer.UserID)
	observability.Global().IncActiveSFUPeers()

	// Subscribe new peer to all currently active voice/video streams
	for _, track := range existingTracks {
		if _, err := peer.AddTrack(track); err != nil {
			slog.Error("failed to attach existing track to new peer", "user_id", peer.UserID, "error", err)
		}
	}

	// Listen for incoming published media tracks from this peer
	peer.PC.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		slog.Info("sfu received published media track",
			"room_id", r.ID,
			"sender_id", peer.UserID,
			"codec", remoteTrack.Codec().MimeType,
			"track_id", remoteTrack.ID(),
		)

		r.dispatchRemoteTrack(peer.UserID, remoteTrack)
	})
}

// RemovePeer unregisters a peer and detaches their media tracks.
func (r *Room) RemovePeer(userID string) {
	r.mu.Lock()
	peer, ok := r.Peers[userID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.Peers, userID)
	r.mu.Unlock()

	slog.Info("sfu peer left room", "room_id", r.ID, "user_id", userID)
	observability.Global().DecActiveSFUPeers()
	_ = peer.Close()
}

// GetPeer retrieves an active peer by user ID.
func (r *Room) GetPeer(userID string) (*Peer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peer, ok := r.Peers[userID]
	if !ok {
		return nil, ErrPeerNotFound
	}
	return peer, nil
}

// dispatchRemoteTrack creates a local fan-out track and pumps incoming RTP packets to all subscribers.
func (r *Room) dispatchRemoteTrack(senderID string, remoteTrack *webrtc.TrackRemote) {
	// 1. Create a matching static local RTP track
	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		remoteTrack.Codec().RTPCodecCapability,
		remoteTrack.ID(),
		remoteTrack.StreamID(),
	)
	if err != nil {
		slog.Error("failed to create local static rtp track", "error", err)
		return
	}

	r.mu.Lock()
	r.tracks[localTrack.ID()] = localTrack
	subscribers := make([]*Peer, 0, len(r.Peers))
	for uid, p := range r.Peers {
		if uid != senderID {
			subscribers = append(subscribers, p)
		}
	}
	r.mu.Unlock()

	// 2. Attach local track to all other peers currently in the room
	for _, sub := range subscribers {
		if _, err := sub.AddTrack(localTrack); err != nil {
			slog.Error("failed to attach fan-out track to subscriber", "sub_id", sub.UserID, "error", err)
		}
	}

	// 3. Start RTP packet pump in background goroutine (Selective Forwarding loop!)
	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.tracks, localTrack.ID())
			r.mu.Unlock()
			slog.Info("sfu rtp forwarding stream terminated", "track_id", localTrack.ID())
		}()

		rtpBuf := make([]byte, 1400)
		for {
			i, _, err := remoteTrack.Read(rtpBuf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				slog.Error("error reading rtp packet from remote track", "error", err)
				return
			}

			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				slog.Error("error writing rtp packet to local track", "error", err)
				return
			}
		}
	}()
}

// GetPeerCount returns the number of active peers in the SFU room.
func (r *Room) GetPeerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Peers)
}

