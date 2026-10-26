package ws

import (
	"log/slog"
	"time"
)

// voiceInviteCooldown is how long someone must wait before inviting the same
// person to voice again. The viewer keeps its invite button disabled for
// longer than this, so the limit only ever drops invites a client did not
// mean to repeat -- a script, or a stuck key -- each of which would otherwise
// put another toast in front of the person invited.
const voiceInviteCooldown = 15 * time.Second

// voiceInvitePair is one inviter asking one person.
type voiceInvitePair struct {
	from, to string
}

// isVoiceInvite reports whether an event is part of the voice invite exchange.
func isVoiceInvite(t EventType) bool {
	return t == EventVoiceInvite || t == EventVoiceInviteReply
}

// admitVoiceInvite decides whether a voice invite or reply from a verified
// member is relayed. Must only run on the hub loop.
//
// Both are addressed to one person: relayed without a target they would go to
// the whole room. An invite also has to come from someone in the call, since
// accepting one means joining the inviter there, and is subject to
// voiceInviteCooldown.
//
// This runs on the inviter's own process, where their socket and therefore
// their SFU peer live, so with the Redis event bus the check sees the same
// call and the same history as it would on a single node.
func (h *Hub) admitVoiceInvite(event *Event) bool {
	if event.TargetID == "" || event.TargetID == event.SenderID {
		return false
	}
	if event.Type != EventVoiceInvite {
		return true
	}

	if h.sfuManager != nil {
		sfuRoom, err := h.sfuManager.GetRoom(event.RoomID)
		if err != nil {
			return false
		}
		if _, err := sfuRoom.GetPeer(event.SenderID); err != nil {
			return false
		}
	}

	now := time.Now()
	pair := voiceInvitePair{from: event.SenderID, to: event.TargetID}
	sent := h.voiceInvites[event.RoomID]
	if last, ok := sent[pair]; ok && now.Sub(last) < voiceInviteCooldown {
		slog.Debug("dropping repeated voice invite", "room_id", event.RoomID, "user_id", event.SenderID, "target_id", event.TargetID)
		return false
	}
	if sent == nil {
		sent = make(map[voiceInvitePair]time.Time)
		h.voiceInvites[event.RoomID] = sent
	}
	// Stale entries go as new invites are made, so a long-lived room's history
	// stays the size of its recent invites.
	for p, at := range sent {
		if now.Sub(at) >= voiceInviteCooldown {
			delete(sent, p)
		}
	}
	sent[pair] = now
	return true
}
