package ws_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
	"github.com/inox/inox/backend/internal/ws"
)

const liveMediaURL = "http://localhost:8080/api/v1/live/bbc-one/master.m3u8"

// fakeEdgeSource stands in for the live proxy: it classifies media URLs and reports
// where the live edge currently sits.
type fakeEdgeSource struct{ edge int64 }

func (f *fakeEdgeSource) IsLiveURL(u string) bool { return strings.Contains(u, "/api/v1/live/") }
func (f *fakeEdgeSource) EdgeSequenceForURL(u string) (int64, bool) {
	if !f.IsLiveURL(u) {
		return 0, false
	}
	return f.edge, true
}

func newLiveClient(hub *ws.Hub, roomID, userID, name string, role domain.Role, joinedAt time.Time) *ws.Client {
	return &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 32),
		RoomID:   roomID,
		UserID:   userID,
		Username: name,
		Role:     role,
		JoinedAt: joinedAt,
		CanLead:  true,
	}
}

// awaitEvent drains a client's channel until the wanted event type shows up.
func awaitEvent(t *testing.T, ch <-chan []byte, want ws.EventType, timeout time.Duration) *ws.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case raw := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(raw, &evt); err != nil {
				t.Fatalf("malformed event on the wire: %v", err)
			}
			if evt.Type == want {
				return &evt
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}

// awaitEvents drains until every wanted type has been seen, in any order. Waiting
// for them one at a time would discard the others while scanning.
func awaitEvents(t *testing.T, ch <-chan []byte, wants []ws.EventType, timeout time.Duration) {
	t.Helper()
	pending := make(map[ws.EventType]bool, len(wants))
	for _, w := range wants {
		pending[w] = true
	}
	deadline := time.After(timeout)
	for len(pending) > 0 {
		select {
		case raw := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(raw, &evt); err != nil {
				t.Fatalf("malformed event on the wire: %v", err)
			}
			delete(pending, evt.Type)
		case <-deadline:
			missing := make([]string, 0, len(pending))
			for typ := range pending {
				missing = append(missing, string(typ))
			}
			t.Fatalf("timed out; never received: %v", missing)
		}
	}
}

// expectNoEvent asserts an event type never arrives within the window.
func expectNoEvent(t *testing.T, ch <-chan []byte, unwanted ws.EventType, window time.Duration) {
	t.Helper()
	deadline := time.After(window)
	for {
		select {
		case raw := <-ch:
			var evt ws.Event
			if err := json.Unmarshal(raw, &evt); err == nil && evt.Type == unwanted {
				t.Fatalf("received %s, which should have been dropped", unwanted)
			}
		case <-deadline:
			return
		}
	}
}

func decodePosition(t *testing.T, evt *ws.Event) ws.LivePositionPayload {
	t.Helper()
	var p ws.LivePositionPayload
	if err := json.Unmarshal(evt.Payload, &p); err != nil {
		t.Fatalf("decode LIVE_POSITION: %v", err)
	}
	return p
}

// startLiveRoom brings up a hub with a live room containing an owner and a member,
// and returns them once leadership has settled.
func startLiveRoom(t *testing.T, edge int64) (*ws.Hub, *ws.Client, *ws.Client) {
	t.Helper()
	hub := ws.NewHub()
	hub.SetLiveEdgeSource(&fakeEdgeSource{edge: edge})
	go hub.Run()

	roomID := "live-room-1"
	base := time.Now()
	owner := newLiveClient(hub, roomID, "user-owner", "Owner", domain.RoleOwner, base)
	member := newLiveClient(hub, roomID, "user-member", "Member", domain.RoleMember, base.Add(-time.Hour))

	// The member connected first, and still must not outrank the owner.
	hub.Register <- member
	hub.Register <- owner

	changePayload, _ := json.Marshal(ws.VideoControlPayload{MediaURL: liveMediaURL})
	hub.Broadcast <- &ws.Event{
		Type:     ws.EventChangeMedia,
		RoomID:   roomID,
		SenderID: owner.UserID,
		Payload:  changePayload,
	}

	leaderEvt := awaitEvent(t, owner.Send, ws.EventLiveLeader, time.Second)
	var leader ws.LiveLeaderPayload
	if err := json.Unmarshal(leaderEvt.Payload, &leader); err != nil {
		t.Fatalf("decode LIVE_LEADER: %v", err)
	}
	if leader.LeaderID != owner.UserID {
		t.Fatalf("leader = %q (%s), want the owner to outrank a longer-connected member",
			leader.LeaderID, leader.LeaderName)
	}
	awaitEvent(t, member.Send, ws.EventLiveLeader, time.Second)
	return hub, owner, member
}

func TestLiveLeaderElectionPrefersOwnerOverEarlierMember(t *testing.T) {
	hub, _, _ := startLiveRoom(t, 500)
	defer hub.Shutdown()
}

func TestLiveLeaderPositionIsRelayedWithAgeReset(t *testing.T) {
	hub, owner, member := startLiveRoom(t, 500)
	defer hub.Shutdown()

	// A leader reporting a position it measured a moment ago: the server must
	// restamp the age, because it is the only party that knows how long the report
	// spent in flight and in its own queue.
	// Five segments behind a 500-segment edge: a normal viewing position, well
	// inside the window, so nothing here should be clamped.
	payload, _ := json.Marshal(ws.LivePositionPayload{
		Position:  domain.LivePosition{MediaSequence: 495, Discontinuity: 1, OffsetSeconds: 2.5},
		AgeMillis: 4000,
	})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: owner.RoomID,
		SenderID: owner.UserID, SenderName: owner.Username, Payload: payload,
	}

	got := decodePosition(t, awaitEvent(t, member.Send, ws.EventLivePosition, time.Second))
	if got.Position.MediaSequence != 495 || got.Position.OffsetSeconds != 2.5 {
		t.Errorf("position = %+v, want sn 495 offset 2.5 relayed unchanged", got.Position)
	}
	if got.Position.Discontinuity != 1 {
		t.Errorf("discontinuity = %d, want 1 preserved", got.Position.Discontinuity)
	}
	if got.AgeMillis != 0 {
		t.Errorf("age = %d, want 0: the server restamps age at fan-out", got.AgeMillis)
	}
	if got.LeaderID != owner.UserID {
		t.Errorf("leader_id = %q, want the event stamped with the room's leader", got.LeaderID)
	}
}

func TestLiveFollowerCannotDriveTheRoom(t *testing.T) {
	hub, owner, member := startLiveRoom(t, 500)
	defer hub.Shutdown()

	// Anyone can send this event. Only the leader may act on it -- otherwise any
	// participant could yank the whole room to a position of their choosing.
	payload, _ := json.Marshal(ws.LivePositionPayload{
		Position: domain.LivePosition{MediaSequence: 100},
	})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: member.RoomID,
		SenderID: member.UserID, SenderName: member.Username, Payload: payload,
	}

	expectNoEvent(t, owner.Send, ws.EventLivePosition, 300*time.Millisecond)
}

func TestLiveStalledLeaderReportIsNotRelayed(t *testing.T) {
	hub, owner, member := startLiveRoom(t, 500)
	defer hub.Shutdown()

	// While the leader buffers, its playhead stops but the broadcast does not.
	// Relaying the frozen reading would drag every follower backwards, then forwards
	// again on recovery. Followers keep extrapolating instead.
	payload, _ := json.Marshal(ws.LivePositionPayload{
		Position: domain.LivePosition{MediaSequence: 495},
		Stalled:  true,
	})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: owner.RoomID,
		SenderID: owner.UserID, SenderName: owner.Username, Payload: payload,
	}

	expectNoEvent(t, member.Send, ws.EventLivePosition, 300*time.Millisecond)
}

func TestLiveLeaderPositionIsClampedToTheWindow(t *testing.T) {
	const edge = 500
	hub, owner, member := startLiveRoom(t, edge)
	defer hub.Shutdown()

	t.Run("ahead of the edge", func(t *testing.T) {
		payload, _ := json.Marshal(ws.LivePositionPayload{
			Position: domain.LivePosition{MediaSequence: edge + 50, OffsetSeconds: 3},
		})
		hub.Broadcast <- &ws.Event{
			Type: ws.EventLivePosition, RoomID: owner.RoomID,
			SenderID: owner.UserID, SenderName: owner.Username, Payload: payload,
		}
		got := decodePosition(t, awaitEvent(t, member.Send, ws.EventLivePosition, time.Second))
		if got.Position.MediaSequence != edge {
			t.Errorf("sequence = %d, want it clamped to the edge %d", got.Position.MediaSequence, edge)
		}
	})

	t.Run("far behind the edge", func(t *testing.T) {
		// A leader on a bad connection must not park fifty followers a minute in the
		// past; the room is pulled back inside the window instead.
		payload, _ := json.Marshal(ws.LivePositionPayload{
			Position: domain.LivePosition{MediaSequence: 100},
		})
		hub.Broadcast <- &ws.Event{
			Type: ws.EventLivePosition, RoomID: owner.RoomID,
			SenderID: owner.UserID, SenderName: owner.Username, Payload: payload,
		}
		got := decodePosition(t, awaitEvent(t, member.Send, ws.EventLivePosition, time.Second))
		if got.Position.MediaSequence <= 100 {
			t.Errorf("sequence = %d, want it pulled forward towards the edge", got.Position.MediaSequence)
		}
		if got.Position.MediaSequence > edge {
			t.Errorf("sequence = %d, want it to stay at or behind the edge %d", got.Position.MediaSequence, edge)
		}
	})
}

func TestLiveLeadershipPassesOnWhenTheLeaderLeaves(t *testing.T) {
	hub, owner, member := startLiveRoom(t, 500)
	defer hub.Shutdown()

	// Rooms outlive their owner's presence. If leadership did not pass on, the room
	// would keep playing but stop being corrected by anyone.
	hub.Unregister <- owner

	evt := awaitEvent(t, member.Send, ws.EventLiveLeader, 2*time.Second)
	var leader ws.LiveLeaderPayload
	if err := json.Unmarshal(evt.Payload, &leader); err != nil {
		t.Fatalf("decode LIVE_LEADER: %v", err)
	}
	if leader.LeaderID != member.UserID {
		t.Fatalf("leader = %q, want the remaining member to take over", leader.LeaderID)
	}

	// And the new leader's reports must now be accepted.
	payload, _ := json.Marshal(ws.LivePositionPayload{
		Position: domain.LivePosition{MediaSequence: 496},
	})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: member.RoomID,
		SenderID: member.UserID, SenderName: member.Username, Payload: payload,
	}
	got := decodePosition(t, awaitEvent(t, member.Send, ws.EventLivePosition, time.Second))
	if got.Position.MediaSequence != 496 {
		t.Errorf("sequence = %d, want the new leader's report accepted", got.Position.MediaSequence)
	}
}

func TestVODRoomIgnoresLivePositionEvents(t *testing.T) {
	hub := ws.NewHub()
	hub.SetLiveEdgeSource(&fakeEdgeSource{edge: 500})
	go hub.Run()
	defer hub.Shutdown()

	roomID := "vod-room-1"
	alice := newLiveClient(hub, roomID, "user-a", "Alice", domain.RoleOwner, time.Now())
	bob := newLiveClient(hub, roomID, "user-b", "Bob", domain.RoleMember, time.Now())
	hub.Register <- alice
	hub.Register <- bob

	// The default room media is an on-demand file, so live position reports have no
	// meaning here and must not reach anyone.
	payload, _ := json.Marshal(ws.LivePositionPayload{
		Position: domain.LivePosition{MediaSequence: 480},
	})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: roomID,
		SenderID: alice.UserID, SenderName: alice.Username, Payload: payload,
	}

	expectNoEvent(t, bob.Send, ws.EventLivePosition, 300*time.Millisecond)
}

// ── membership verification must not eat server-originated events ────────────

// strictRoomService rejects every user it is asked about, standing in for the
// production path where a hub has a roomService wired.
type strictRoomService struct{ room.RoomService }

func (s *strictRoomService) GetRoomAndMember(_ context.Context, _, userID string) (*domain.Room, *domain.RoomMember, error) {
	if userID == "" {
		return nil, nil, errors.New("no such member")
	}
	return &domain.Room{ID: "live-room-1"}, &domain.RoomMember{UserID: userID, Role: domain.RoleOwner}, nil
}

func (s *strictRoomService) GetRoomByID(_ context.Context, roomID string) (*domain.Room, error) {
	return &domain.Room{ID: roomID, CurrentMediaURL: liveMediaURL}, nil
}

// Events the hub produces itself carry no sender. Running them through membership
// verification drops all of them, because GetRoomAndMember("") always fails — which
// silently disabled SYNC_PLAYBACK on join and LIVE_LEADER for every live room in any
// deployment where roomService is wired. Tests without a roomService cannot see it.
func TestServerOriginatedEventsSurviveMembershipVerification(t *testing.T) {
	hub := ws.NewHub()
	hub.SetLiveEdgeSource(&fakeEdgeSource{edge: 500})
	hub.SetRoomService(&strictRoomService{})
	go hub.Run()
	defer hub.Shutdown()

	owner := newLiveClient(hub, "live-room-1", "user-owner", "Owner", domain.RoleOwner, time.Now())
	hub.Register <- owner

	// The room's media URL comes back live from the room service, so joining should
	// produce both a sync handover and a leader announcement.
	awaitEvents(t, owner.Send,
		[]ws.EventType{ws.EventSyncPlayback, ws.EventLiveLeader}, 2*time.Second)
}

// ── leadership lifecycle ─────────────────────────────────────────────────────

func TestLiveLeadershipIsDeclinedByPlayersThatCannotPublish(t *testing.T) {
	hub, owner, member := startLiveRoom(t, 500)
	defer hub.Shutdown()

	// Native-HLS players have no fragment list and so cannot express a position at
	// all. A leader that holds the claim and publishes nothing leaves the room
	// uncorrected forever, so it hands the role back instead.
	payload, _ := json.Marshal(ws.LivePositionPayload{Unable: true})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: owner.RoomID,
		SenderID: owner.UserID, SenderName: owner.Username, Payload: payload,
	}

	evt := awaitEvent(t, member.Send, ws.EventLiveLeader, 2*time.Second)
	var leader ws.LiveLeaderPayload
	if err := json.Unmarshal(evt.Payload, &leader); err != nil {
		t.Fatalf("decode LIVE_LEADER: %v", err)
	}
	if leader.LeaderID != member.UserID {
		t.Fatalf("leader = %q, want the role handed to the member who can publish", leader.LeaderID)
	}
}

func TestLivePositionRejectedWhileRoomHasNoLeader(t *testing.T) {
	hub := ws.NewHub()
	hub.SetLiveEdgeSource(&fakeEdgeSource{edge: 500})
	go hub.Run()
	defer hub.Shutdown()

	roomID := "live-room-2"
	// Nobody here can lead, so the room has no leader and must accept no positions.
	// A node that loses the leadership claim race is in exactly this state, and
	// accepting reports there let any follower drive every node's copy of the room.
	a := newLiveClient(hub, roomID, "user-a", "A", domain.RoleOwner, time.Now())
	a.CanLead = false
	b := newLiveClient(hub, roomID, "user-b", "B", domain.RoleMember, time.Now())
	b.CanLead = false
	hub.Register <- a
	hub.Register <- b

	changePayload, _ := json.Marshal(ws.VideoControlPayload{MediaURL: liveMediaURL})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventChangeMedia, RoomID: roomID, SenderID: a.UserID, Payload: changePayload,
	}
	awaitEvent(t, b.Send, ws.EventChangeMedia, time.Second)

	payload, _ := json.Marshal(ws.LivePositionPayload{
		Position: domain.LivePosition{MediaSequence: 495},
	})
	hub.Broadcast <- &ws.Event{
		Type: ws.EventLivePosition, RoomID: roomID,
		SenderID: b.UserID, SenderName: b.Username, Payload: payload,
	}

	expectNoEvent(t, a.Send, ws.EventLivePosition, 300*time.Millisecond)
}

// ── upstream health reaches the room ─────────────────────────────────────────

func TestLiveStatusReachesRoomsWatchingTheChannel(t *testing.T) {
	hub, owner, _ := startLiveRoom(t, 500)
	defer hub.Shutdown()

	// Without this path a dead upstream leaves the room on a frozen frame with
	// nothing to read.
	hub.NotifyLiveChannelStatus("bbc-one", "degraded", "The broadcast stopped sending video.")

	evt := awaitEvent(t, owner.Send, ws.EventLiveStatus, 2*time.Second)
	var status ws.LiveStatusPayload
	if err := json.Unmarshal(evt.Payload, &status); err != nil {
		t.Fatalf("decode LIVE_STATUS: %v", err)
	}
	if status.Status != "degraded" || status.Message == "" {
		t.Errorf("status = %+v, want a degraded status carrying an explanation", status)
	}
}

func TestLiveStatusIgnoresRoomsOnOtherChannels(t *testing.T) {
	hub, owner, _ := startLiveRoom(t, 500)
	defer hub.Shutdown()

	hub.NotifyLiveChannelStatus("some-other-channel", "degraded", "unrelated outage")
	expectNoEvent(t, owner.Send, ws.EventLiveStatus, 400*time.Millisecond)
}
