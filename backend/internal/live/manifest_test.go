package live_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/inox/inox/backend/internal/live"
)

// stubRewrite marks each URI with what the parser decided it was, so a test can
// assert on classification as well as on substitution.
func stubRewrite(absolute string, kind live.URIKind) string {
	label := "RES"
	if kind == live.URIPlaylist {
		label = "PL"
	}
	return fmt.Sprintf("<%s:%s>", label, absolute)
}

func TestParseMasterPlaylistRewritesVariantsAndRenditions(t *testing.T) {
	raw := []byte(`#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aud",NAME="English",URI="audio/en.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=4500000,RESOLUTION=1920x1080
1080p/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720
https://other.cdn.example.com/720p/index.m3u8
`)

	pl := live.ParsePlaylist(raw, "https://cdn.example.com/live/master.m3u8", stubRewrite)
	body := string(pl.Body)

	if !pl.IsMaster {
		t.Fatal("expected a master playlist")
	}

	// A relative variant must be resolved against the manifest's own URL, not the
	// proxy's: getting this wrong yields 404s that only appear on some sources.
	if !strings.Contains(body, "<PL:https://cdn.example.com/live/1080p/index.m3u8>") {
		t.Errorf("relative variant URI not resolved against the upstream base:\n%s", body)
	}
	if !strings.Contains(body, "<PL:https://other.cdn.example.com/720p/index.m3u8>") {
		t.Errorf("absolute variant URI not preserved:\n%s", body)
	}
	// Alternate renditions live in an attribute, not on their own line.
	if !strings.Contains(body, `URI="<PL:https://cdn.example.com/live/audio/en.m3u8>"`) {
		t.Errorf("EXT-X-MEDIA URI attribute not rewritten:\n%s", body)
	}
	// Everything that is not a URI must survive untouched, or players lose the ladder.
	if !strings.Contains(body, "BANDWIDTH=4500000,RESOLUTION=1920x1080") {
		t.Errorf("stream attributes were altered:\n%s", body)
	}
}

func TestParseMediaPlaylistTracksEdgeAndRewritesSegments(t *testing.T) {
	raw := []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:48210
#EXT-X-DISCONTINUITY-SEQUENCE:2
#EXT-X-KEY:METHOD=AES-128,URI="https://keys.example.com/k1.bin",IV=0x00
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.000,
seg48210.ts
#EXTINF:6.000,
seg48211.ts
#EXTINF:6.000,
seg48212.ts
`)

	pl := live.ParsePlaylist(raw, "https://cdn.example.com/live/720p/index.m3u8", stubRewrite)
	body := string(pl.Body)

	if pl.IsMaster {
		t.Error("media playlist misclassified as a master")
	}
	if pl.TargetDuration != 6 {
		t.Errorf("target duration = %v, want 6", pl.TargetDuration)
	}
	if pl.MediaSequence != 48210 {
		t.Errorf("media sequence = %d, want 48210", pl.MediaSequence)
	}
	if pl.DiscontinuitySequence != 2 {
		t.Errorf("discontinuity sequence = %d, want 2", pl.DiscontinuitySequence)
	}
	// The edge is what a leader's reported position gets clamped against, so an
	// off-by-one here silently shifts every live room's ceiling.
	if pl.EdgeSequence != 48212 {
		t.Errorf("edge sequence = %d, want 48212 (first sequence + 2 more segments)", pl.EdgeSequence)
	}
	if !pl.IsLive() {
		t.Error("playlist without EXT-X-ENDLIST should be live")
	}

	for _, want := range []string{
		"<RES:https://cdn.example.com/live/720p/seg48210.ts>",
		"<RES:https://cdn.example.com/live/720p/seg48212.ts>",
		`URI="<RES:https://keys.example.com/k1.bin>"`,
		`URI="<RES:https://cdn.example.com/live/720p/init.mp4>"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in rewritten playlist:\n%s", want, body)
		}
	}
}

func TestParsePlaylistDetectsEndList(t *testing.T) {
	raw := []byte("#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:4.0,\na.ts\n#EXT-X-ENDLIST\n")
	pl := live.ParsePlaylist(raw, "https://cdn.example.com/vod/index.m3u8", stubRewrite)
	if pl.IsLive() {
		t.Error("a playlist carrying EXT-X-ENDLIST has stopped and must not be treated as live")
	}
}

func TestParseMediaPlaylistWithoutExplicitSequence(t *testing.T) {
	// EXT-X-MEDIA-SEQUENCE is optional in HLS and its absence means the first segment
	// is number 0. Treating that as "unknown" leaves EdgeSequence unset, which
	// silently disables the leader's DVR-window clamp and zeroes the window figures
	// the admin test panel reports.
	raw := []byte("#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4.0,\na.ts\n#EXTINF:4.0,\nb.ts\n#EXTINF:4.0,\nc.ts\n")

	pl := live.ParsePlaylist(raw, "https://cdn.example.com/live/index.m3u8", stubRewrite)

	if pl.MediaSequence != 0 {
		t.Errorf("media sequence = %d, want 0 when the tag is absent", pl.MediaSequence)
	}
	if pl.EdgeSequence != 2 {
		t.Errorf("edge sequence = %d, want 2 (three segments starting at 0)", pl.EdgeSequence)
	}
}

func TestParseMasterPlaylistReportsNoEdge(t *testing.T) {
	// A master playlist carries no segments, so it must not claim an edge — the hub
	// would then clamp every leader position against a meaningless number.
	raw := []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\nlow/index.m3u8\n")

	pl := live.ParsePlaylist(raw, "https://cdn.example.com/live/master.m3u8", stubRewrite)

	if !pl.IsMaster {
		t.Fatal("expected a master playlist")
	}
	if pl.EdgeSequence != -1 {
		t.Errorf("edge sequence = %d, want -1 (no segments in a master playlist)", pl.EdgeSequence)
	}
}
