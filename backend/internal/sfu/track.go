package sfu

import "strings"

// Media kinds a peer can publish. A peer publishes at most one of each.
const (
	KindMic    = "mic"
	KindScreen = "screen"
)

// trackNameSeparator is a dot because user IDs are UUIDs, which contain hyphens
// but never dots, and because dots are valid in the SDP token that carries a
// track name to the browser.
const trackNameSeparator = "."

// TrackName is the identity the SFU gives a fan-out track, as "<kind>.<userID>".
//
// It replaces the random ID the publisher's browser generated. Both halves of the
// msid (stream and track) carry this name, so a subscriber can tell from the track
// event alone who is speaking and whether it is a microphone or a screen. Without
// it every remote stream is an anonymous UUID and the UI can only label it "Peer".
func TrackName(kind, userID string) string {
	return kind + trackNameSeparator + userID
}

// SplitTrackName recovers the kind and publisher from a name built by TrackName.
func SplitTrackName(name string) (kind, userID string, ok bool) {
	kind, userID, ok = strings.Cut(name, trackNameSeparator)
	if !ok || userID == "" {
		return "", "", false
	}
	if kind != KindMic && kind != KindScreen {
		return "", "", false
	}
	return kind, userID, true
}
