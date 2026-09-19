package live

import (
	"net/url"
	"strconv"
	"strings"
)

// URIKind distinguishes the two things a manifest can point at, because they are
// served by different proxy routes: another playlist has to be fetched and rewritten
// again, while a resource is streamed straight through.
type URIKind int

const (
	URIPlaylist URIKind = iota
	URIResource
)

// RewriteFunc maps an absolute upstream URI to the proxy URL clients should request.
type RewriteFunc func(absoluteURL string, kind URIKind) string

// Playlist is the subset of an HLS manifest the proxy actually needs.
type Playlist struct {
	IsMaster              bool
	TargetDuration        float64
	MediaSequence         int64
	DiscontinuitySequence int64
	HasEndList            bool
	// EdgeSequence is the media sequence number of the last segment in the playlist,
	// i.e. the live edge. The leader's reported position is clamped against it, so a
	// leader whose player has fallen far behind cannot park the whole room there.
	EdgeSequence int64
	Body         []byte
}

// IsLive reports whether the playlist is a sliding window rather than a finished
// recording. An EXT-X-ENDLIST means the stream has stopped.
func (p *Playlist) IsLive() bool { return !p.HasEndList }

// attribute tags whose URI="..." value must be rewritten, and what it points at.
var attrURITags = map[string]URIKind{
	"#EXT-X-MEDIA":              URIPlaylist,
	"#EXT-X-I-FRAME-STREAM-INF": URIPlaylist,
	"#EXT-X-RENDITION-REPORT":   URIPlaylist,
	"#EXT-X-KEY":                URIResource,
	"#EXT-X-SESSION-KEY":        URIResource,
	"#EXT-X-MAP":                URIResource,
	"#EXT-X-PART":               URIResource,
	"#EXT-X-PRELOAD-HINT":       URIResource,
}

// ParsePlaylist rewrites every URI in an HLS manifest to point back at the proxy,
// resolving relative references against the upstream URL first.
//
// Rewriting is not optional. Segment URIs in a scraped manifest are usually relative
// to an origin the browser cannot reach directly -- it has no CORS grant there, it
// cannot send the Referer the origin demands, and the signed URL it would need
// expires within minutes. Pointing every URI back at ourselves is what makes the
// stream playable at all.
func ParsePlaylist(raw []byte, upstreamURL string, rewrite RewriteFunc) *Playlist {
	base, err := url.Parse(upstreamURL)
	if err != nil {
		base = nil
	}

	// RFC 8216: EXT-X-MEDIA-SEQUENCE is optional and its absence means the first
	// segment is number 0. Treating that as unknown silently disabled the DVR-window
	// clamp and zeroed the window figures in the admin test panel.
	pl := &Playlist{MediaSequence: 0, EdgeSequence: -1}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))

	// Set by EXT-X-STREAM-INF, consumed by the following URI line. Its presence is
	// also what proves this is a master playlist.
	expectVariant := false
	seqCursor := int64(0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			out = append(out, line)
			continue

		case strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF"):
			pl.IsMaster = true
			expectVariant = true
			out = append(out, line)
			continue

		case strings.HasPrefix(trimmed, "#EXT-X-TARGETDURATION:"):
			pl.TargetDuration, _ = strconv.ParseFloat(strings.TrimSpace(after(trimmed, ":")), 64)
			out = append(out, line)
			continue

		case strings.HasPrefix(trimmed, "#EXT-X-MEDIA-SEQUENCE:"):
			pl.MediaSequence, _ = strconv.ParseInt(strings.TrimSpace(after(trimmed, ":")), 10, 64)
			seqCursor = pl.MediaSequence
			out = append(out, line)
			continue

		case strings.HasPrefix(trimmed, "#EXT-X-DISCONTINUITY-SEQUENCE:"):
			pl.DiscontinuitySequence, _ = strconv.ParseInt(strings.TrimSpace(after(trimmed, ":")), 10, 64)
			out = append(out, line)
			continue

		case trimmed == "#EXT-X-ENDLIST":
			pl.HasEndList = true
			out = append(out, line)
			continue

		case strings.HasPrefix(trimmed, "#"):
			if tag, kind, ok := matchAttrTag(trimmed); ok {
				out = append(out, rewriteAttrURI(line, tag, kind, base, rewrite))
				continue
			}
			out = append(out, line)
			continue
		}

		// A bare line is a URI: a variant playlist if EXT-X-STREAM-INF just
		// announced one, otherwise a media segment.
		kind := URIResource
		if expectVariant {
			kind = URIPlaylist
			expectVariant = false
		} else {
			pl.EdgeSequence = seqCursor
			seqCursor++
		}
		out = append(out, rewrite(absolutize(base, trimmed), kind))
	}

	pl.Body = []byte(strings.Join(out, "\n"))
	return pl
}

func matchAttrTag(line string) (string, URIKind, bool) {
	for tag, kind := range attrURITags {
		if strings.HasPrefix(line, tag+":") {
			return tag, kind, true
		}
	}
	return "", URIResource, false
}

// rewriteAttrURI replaces the URI="..." value of an attribute tag, leaving every
// other attribute on the line untouched.
func rewriteAttrURI(line, _ string, kind URIKind, base *url.URL, rewrite RewriteFunc) string {
	const marker = `URI="`
	start := strings.Index(line, marker)
	if start == -1 {
		return line
	}
	valueStart := start + len(marker)
	end := strings.Index(line[valueStart:], `"`)
	if end == -1 {
		return line
	}
	original := line[valueStart : valueStart+end]
	replaced := rewrite(absolutize(base, original), kind)
	return line[:valueStart] + replaced + line[valueStart+end:]
}

// absolutize resolves a possibly-relative manifest reference against the URL the
// manifest itself was fetched from.
func absolutize(base *url.URL, ref string) string {
	if base == nil {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(u).String()
}

func after(s, sep string) string {
	if _, rest, ok := strings.Cut(s, sep); ok {
		return rest
	}
	return ""
}
