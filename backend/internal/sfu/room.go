package sfu

import (
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/observability"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

var (
	ErrPeerNotFound = errors.New("peer not found in sfu room")
)

// keyframeInterval is how often the SFU asks a screen-share publisher for a fresh
// keyframe. A subscriber that joins mid-share cannot decode anything until one
// arrives, and browsers only emit them on their own schedule -- typically several
// seconds apart -- so without this a new viewer stares at a black tile.
const keyframeInterval = 2 * time.Second

// PeerState is one row of a room's voice roster.
//
// Mute is reported by the browser rather than observed: a muted microphone keeps
// sending silence, so the SFU cannot tell. Screen sharing is observed, because a
// share is a real published track.
type PeerState struct {
	UserID          string `json:"user_id"`
	Username        string `json:"username"`
	IsMuted         bool   `json:"is_muted"`
	IsScreenSharing bool   `json:"is_screen_sharing"`
}

// StateHandler receives a room's full roster whenever it changes. The hub installs
// one so every client -- including those not in voice -- can render who is in the
// call, which no client can work out from its own peer connection alone.
type StateHandler func(roomID string, peers []PeerState)

// publication is one live track a peer has published into the room.
type publication struct {
	name  string
	kind  string
	owner string
	track *webrtc.TrackLocalStaticRTP
	ssrc  webrtc.SSRC
	// stop closes when the publication ends, retiring its keyframe loop.
	stop chan struct{}
}

// Room manages media routing across all connected peers within a Watch Party workspace.
type Room struct {
	ID     string
	Peers  map[string]*Peer
	tracks map[string]*publication

	api        *webrtc.API
	iceServers []webrtc.ICEServer
	onState    StateHandler

	mu sync.RWMutex
}

// NewRoom initializes a new SFU media routing workspace. api and iceServers
// carry the deployment's NAT/port settings down to every peer the room creates;
// both may be nil for Pion's defaults.
func NewRoom(id string, api *webrtc.API, iceServers []webrtc.ICEServer) *Room {
	return &Room{
		ID:         id,
		Peers:      make(map[string]*Peer),
		tracks:     make(map[string]*publication),
		api:        api,
		iceServers: iceServers,
	}
}

// SetStateHandler installs the callback notified of roster changes.
func (r *Room) SetStateHandler(h StateHandler) {
	r.mu.Lock()
	r.onState = h
	r.mu.Unlock()
}

// NewPeer creates a peer bound to this room's WebRTC API and ICE configuration.
func (r *Room) NewPeer(id, userID, username string) (*Peer, error) {
	return NewPeer(id, userID, username, r.ID, r.api, r.iceServers)
}

// AddPeer registers a new WebRTC peer and subscribes them to all active media tracks in the room.
func (r *Room) AddPeer(peer *Peer) {
	r.mu.Lock()
	r.Peers[peer.UserID] = peer
	existing := make([]*publication, 0, len(r.tracks))
	for _, pub := range r.tracks {
		if pub.owner != peer.UserID {
			existing = append(existing, pub)
		}
	}
	r.mu.Unlock()

	slog.Info("sfu peer joined room", "room_id", r.ID, "user_id", peer.UserID)
	observability.Global().IncActiveSFUPeers()

	// Subscribe the new peer to every stream already in flight. This runs before
	// the peer's opening offer is answered, so these tracks ride along in that
	// first answer instead of costing an extra exchange.
	for _, pub := range existing {
		r.subscribe(peer, pub)
	}

	// Listen for incoming published media tracks from this peer
	peer.PC.OnTrack(func(remoteTrack *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		slog.Info("sfu received published media track",
			"room_id", r.ID,
			"sender_id", peer.UserID,
			"codec", remoteTrack.Codec().MimeType,
			"track_id", remoteTrack.ID(),
		)

		r.dispatchRemoteTrack(peer, remoteTrack)
	})

	// A browser that crashes, sleeps or loses its network never sends a leave. The
	// connection state is the only signal that it is gone, and without acting on it
	// the room keeps forwarding to a dead peer and shows it in the roster forever.
	peer.PC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("webrtc peer connection state changed", "peer_id", peer.ID, "user_id", peer.UserID, "state", state.String())
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			r.RemovePeer(peer.UserID)
		}
	})

	r.notifyState()
}

// RemovePeer unregisters a peer, retires everything it was publishing, and closes
// its connection. Calling it twice for the same peer is safe.
func (r *Room) RemovePeer(userID string) {
	r.mu.Lock()
	peer, ok := r.Peers[userID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.Peers, userID)
	published := make([]string, 0, 2)
	for name, pub := range r.tracks {
		if pub.owner == userID {
			published = append(published, name)
		}
	}
	r.mu.Unlock()

	// Detach this peer's tracks from everyone else explicitly. The RTP pump also
	// cleans up when its source dies, but only once the read actually fails, and a
	// subscriber left holding a dead sender shows a frozen tile until then.
	for _, name := range published {
		r.removePublication(name)
	}

	slog.Info("sfu peer left room", "room_id", r.ID, "user_id", userID)
	observability.Global().DecActiveSFUPeers()
	_ = peer.Close()
	r.notifyState()
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

// SetMuted records a peer's self-reported microphone state and republishes the
// roster when it changed.
func (r *Room) SetMuted(userID string, muted bool) {
	peer, err := r.GetPeer(userID)
	if err != nil {
		return
	}
	if peer.SetMuted(muted) {
		r.notifyState()
	}
}

// Roster reports who is in the call and what they are doing, sorted by username so
// clients get a stable order.
func (r *Room) Roster() []PeerState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sharing := make(map[string]bool, len(r.tracks))
	for _, pub := range r.tracks {
		if pub.kind == KindScreen {
			sharing[pub.owner] = true
		}
	}

	roster := make([]PeerState, 0, len(r.Peers))
	for _, peer := range r.Peers {
		roster = append(roster, PeerState{
			UserID:          peer.UserID,
			Username:        peer.Username,
			IsMuted:         peer.Muted(),
			IsScreenSharing: sharing[peer.UserID],
		})
	}
	sort.Slice(roster, func(i, j int) bool {
		if roster[i].Username == roster[j].Username {
			return roster[i].UserID < roster[j].UserID
		}
		return roster[i].Username < roster[j].Username
	})
	return roster
}

// GetPeerCount returns the number of active peers in the SFU room.
func (r *Room) GetPeerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Peers)
}

// Close disconnects every peer in the room and drops its media state.
func (r *Room) Close() {
	r.mu.Lock()
	peers := make([]*Peer, 0, len(r.Peers))
	for uid, peer := range r.Peers {
		peers = append(peers, peer)
		delete(r.Peers, uid)
	}
	pubs := make([]*publication, 0, len(r.tracks))
	for name, pub := range r.tracks {
		pubs = append(pubs, pub)
		delete(r.tracks, name)
	}
	r.mu.Unlock()

	for _, pub := range pubs {
		close(pub.stop)
	}
	for _, peer := range peers {
		observability.Global().DecActiveSFUPeers()
		_ = peer.Close()
	}
}

// subscribe attaches a publication to one peer and starts draining the feedback
// that peer sends back about it.
func (r *Room) subscribe(sub *Peer, pub *publication) {
	sender, added, err := sub.AddTrack(pub.track)
	if err != nil {
		slog.Error("failed to attach fan-out track to subscriber", "sub_id", sub.UserID, "track", pub.name, "error", err)
		return
	}
	if !added {
		return
	}
	go r.drainSenderFeedback(sender, pub)
}

// dispatchRemoteTrack creates a fan-out track for an incoming stream and pumps its
// RTP packets to every other peer in the room.
func (r *Room) dispatchRemoteTrack(publisher *Peer, remoteTrack *webrtc.TrackRemote) {
	kind := KindMic
	if remoteTrack.Kind() == webrtc.RTPCodecTypeVideo {
		kind = KindScreen
	}
	// The publisher's browser names its tracks with random UUIDs, which tell a
	// subscriber nothing. Re-naming the fan-out track after its owner is what lets
	// the far side label the tile, group a screen with its sharer, and drop the
	// right stream when someone leaves.
	name := TrackName(kind, publisher.UserID)

	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		remoteTrack.Codec().RTPCodecCapability,
		name,
		name,
	)
	if err != nil {
		slog.Error("failed to create local static rtp track", "error", err)
		return
	}

	// Publishing the same kind twice means the previous one is finished (a screen
	// share restarted, or a reconnect) -- retire it before its replacement lands.
	r.removePublication(name)

	pub := &publication{
		name:  name,
		kind:  kind,
		owner: publisher.UserID,
		track: localTrack,
		ssrc:  remoteTrack.SSRC(),
		stop:  make(chan struct{}),
	}

	r.mu.Lock()
	r.tracks[name] = pub
	subscribers := make([]*Peer, 0, len(r.Peers))
	for uid, p := range r.Peers {
		if uid != publisher.UserID {
			subscribers = append(subscribers, p)
		}
	}
	r.mu.Unlock()

	// Attaching a track to a connection that has already negotiated does nothing on
	// its own: until the browser is offered the new track set it has no receiver to
	// put the media in. This renegotiation is what makes a second person in a call
	// audible to the first.
	for _, sub := range subscribers {
		r.subscribe(sub, pub)
		sub.Negotiate()
	}
	r.notifyState()

	if kind == KindScreen {
		go r.keyframeLoop(pub)
	}

	// Selective forwarding loop: read from the publisher, write to the fan-out track.
	go func() {
		defer r.removePublication(name)

		rtpBuf := make([]byte, 1400)
		for {
			i, _, err := remoteTrack.Read(rtpBuf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.Debug("sfu remote track read ended", "track", name, "error", err)
				}
				return
			}

			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				slog.Error("error writing rtp packet to local track", "track", name, "error", err)
				return
			}
		}
	}()
}

// removePublication retires a track and detaches it from every subscriber.
func (r *Room) removePublication(name string) {
	r.mu.Lock()
	pub, ok := r.tracks[name]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.tracks, name)
	subscribers := make([]*Peer, 0, len(r.Peers))
	for uid, p := range r.Peers {
		if uid != pub.owner {
			subscribers = append(subscribers, p)
		}
	}
	r.mu.Unlock()

	close(pub.stop)
	for _, sub := range subscribers {
		sub.RemoveTrack(name)
		sub.Negotiate()
	}

	slog.Info("sfu stopped forwarding track", "room_id", r.ID, "track", name)
	r.notifyState()
}

// keyframeLoop keeps asking a screen-share publisher for keyframes for as long as
// the share lasts, so that anyone joining mid-share gets a picture within a couple
// of seconds instead of whenever the encoder next feels like it.
func (r *Room) keyframeLoop(pub *publication) {
	ticker := time.NewTicker(keyframeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pub.stop:
			return
		case <-ticker.C:
			r.requestKeyframe(pub)
		}
	}
}

// drainSenderFeedback reads the RTCP a subscriber sends about a forwarded track.
// Unread feedback piles up in Pion's buffers, and a subscriber asking for a
// keyframe has to be relayed to the publisher -- the SFU cannot produce one itself.
func (r *Room) drainSenderFeedback(sender *webrtc.RTPSender, pub *publication) {
	buf := make([]byte, 1500)
	for {
		n, _, err := sender.Read(buf)
		if err != nil {
			return
		}
		if pub.kind != KindScreen {
			continue
		}
		packets, err := rtcp.Unmarshal(buf[:n])
		if err != nil {
			continue
		}
		for _, packet := range packets {
			switch packet.(type) {
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				r.requestKeyframe(pub)
			}
		}
	}
}

// requestKeyframe asks a publication's owner for a fresh keyframe.
func (r *Room) requestKeyframe(pub *publication) {
	r.mu.RLock()
	owner, ok := r.Peers[pub.owner]
	r.mu.RUnlock()
	if !ok {
		return
	}
	_ = owner.PC.WriteRTCP([]rtcp.Packet{
		&rtcp.PictureLossIndication{MediaSSRC: uint32(pub.ssrc)},
	})
}

// notifyState publishes the current roster to whoever is listening.
func (r *Room) notifyState() {
	r.mu.RLock()
	handler := r.onState
	r.mu.RUnlock()

	if handler == nil {
		return
	}
	handler(r.ID, r.Roster())
}
