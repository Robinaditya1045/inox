package media

import (
	"errors"
	"fmt"
	"strings"
)

// TranscodeProfile is one rung of the HLS ladder: a target frame size and the
// bitrate x264 is held to for it.
type TranscodeProfile struct {
	Name        string
	Width       int
	Height      int
	BitrateKbps int
	MaxrateKbps int
	BufsizeKbps int
}

// transcodeLadder is every quality an upload can be transcoded to, best first.
// The master playlist lists variants in this order whatever order they were
// asked for in.
var transcodeLadder = []TranscodeProfile{
	{Name: "1080p", Width: 1920, Height: 1080, BitrateKbps: 4500, MaxrateKbps: 5000, BufsizeKbps: 6000},
	{Name: "720p", Width: 1280, Height: 720, BitrateKbps: 2500, MaxrateKbps: 2800, BufsizeKbps: 3500},
	{Name: "480p", Width: 854, Height: 480, BitrateKbps: 1000, MaxrateKbps: 1200, BufsizeKbps: 1500},
}

// ErrUnknownTranscodeProfile rejects a quality that is not on the ladder.
var ErrUnknownTranscodeProfile = errors.New("unknown transcode profile")

// NormalizeTranscodeProfiles validates the qualities chosen for an upload and
// returns them deduplicated in ladder order. Choosing none, or all of them,
// returns nil: the full ladder, stored the same way as on assets that predate
// the choice.
func NormalizeTranscodeProfiles(names []string) ([]string, error) {
	chosen := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		if profileByName(n) == nil {
			return nil, fmt.Errorf("%w %q; choose from %s", ErrUnknownTranscodeProfile, n, strings.Join(TranscodeProfileNames(), ", "))
		}
		chosen[n] = true
	}
	if len(chosen) == 0 || len(chosen) == len(transcodeLadder) {
		return nil, nil
	}
	var out []string
	for _, p := range transcodeLadder {
		if chosen[p.Name] {
			out = append(out, p.Name)
		}
	}
	return out, nil
}

// TranscodeProfileNames lists every quality on the ladder, best first.
func TranscodeProfileNames() []string {
	names := make([]string, len(transcodeLadder))
	for i, p := range transcodeLadder {
		names[i] = p.Name
	}
	return names
}

// resolveTranscodeProfiles turns an asset's stored choice into the profiles to
// encode. Names no longer on the ladder are skipped rather than failing a
// transcode that was valid when it was queued; if that leaves nothing, the full
// ladder is used.
func resolveTranscodeProfiles(names []string) []TranscodeProfile {
	var out []TranscodeProfile
	for _, p := range transcodeLadder {
		for _, n := range names {
			if n == p.Name {
				out = append(out, p)
				break
			}
		}
	}
	if len(out) == 0 {
		return transcodeLadder
	}
	return out
}

func profileByName(name string) *TranscodeProfile {
	for i := range transcodeLadder {
		if transcodeLadder[i].Name == name {
			return &transcodeLadder[i]
		}
	}
	return nil
}
