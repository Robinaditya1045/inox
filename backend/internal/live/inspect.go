package live

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// manifestURL reports whether a URL's path names an HLS or DASH manifest, including
// ones an application serves rather than a file, like /manifest(format=m3u8-aapl).
func manifestURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	return strings.Contains(path, ".m3u8") || strings.Contains(path, ".mpd") ||
		strings.Contains(path, "format=m3u8") || strings.Contains(path, "format=mpd")
}

// manifestQuery reports whether a URL's query hints at a manifest (?format=m3u8).
// Weaker evidence than the path -- analytics beacons routinely carry the stream URL
// as a parameter -- so it makes a request worth checking but never ends the wait.
func manifestQuery(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	query := strings.ToLower(u.RawQuery)
	return strings.Contains(query, "m3u8") || strings.Contains(query, "format=mpd")
}

// manifestMIME reports whether a response content type is an HLS or DASH manifest,
// which catches playlists served from URLs that give no hint of it.
func manifestMIME(mimeType string) bool {
	switch mimeBase(mimeType) {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "application/mpegurl",
		"audio/mpegurl", "audio/x-mpegurl", "video/x-mpegurl",
		"application/dash+xml", "video/vnd.mpeg.dash.mpd":
		return true
	}
	return false
}

// servedAsManifest reports whether a loaded response is convincing enough to stop
// waiting for the player: a manifest content type, or a manifest-shaped path that
// did not come back as a page, script, image or media segment. Plenty of origins
// serve playlists as text/plain or application/octet-stream, so those still count.
func servedAsManifest(rawURL, mimeType string) bool {
	if manifestMIME(mimeType) {
		return true
	}
	if !manifestURL(rawURL) {
		return false
	}
	base := mimeBase(mimeType)
	switch {
	case strings.HasPrefix(base, "image/"), strings.HasPrefix(base, "font/"),
		strings.HasPrefix(base, "video/"), strings.HasPrefix(base, "audio/"),
		base == "text/html", base == "text/css", base == "application/json",
		strings.Contains(base, "javascript"):
		return false
	}
	return true
}

func mimeBase(mimeType string) string {
	base, _, _ := strings.Cut(mimeType, ";")
	return strings.ToLower(strings.TrimSpace(base))
}

// manifestShape is what a manifest body says about the stream behind it.
type manifestShape struct {
	protocol string // "hls", "dash", or "" when the body is not a manifest at all
	master   bool
	// ended marks a finished recording (EXT-X-ENDLIST, or a static MPD) rather
	// than a live window.
	ended bool
	// drm names the DRM systems the stream requires. AES-128 with a plain key URI
	// is not DRM: hls.js decrypts it and the proxy passes the key through.
	drm []string
	// schemes are the encryption schemes DRM applies: cenc or cbcs.
	schemes []string
	// renditions counts the renditions described, and protected how many of them
	// need DRM. An HLS master counts only once its playlists are read, in
	// inspectStream.
	renditions int
	protected  int
}

func inspectManifest(body []byte) manifestShape {
	text := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
	switch {
	// A bare #EXTM3U is also a plain M3U audio playlist; HLS always has EXT-X tags.
	case strings.HasPrefix(text, "#EXTM3U") && strings.Contains(text, "#EXT-X-"):
		return inspectHLS(text)
	case mpdElement.MatchString(text):
		return inspectDASH(text)
	}
	return manifestShape{}
}

func inspectHLS(text string) manifestShape {
	shape := manifestShape{protocol: "hls"}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF"):
			shape.master = true
		case line == "#EXT-X-ENDLIST":
			shape.ended = true
		case strings.HasPrefix(line, "#EXT-X-KEY:"), strings.HasPrefix(line, "#EXT-X-SESSION-KEY:"):
			attrs := after(line, ":")
			if system := hlsKeySystem(attrs); system != "" {
				shape.drm = appendUnique(shape.drm, system)
				if scheme := hlsScheme(hlsAttr(attrs, "METHOD")); scheme != "" {
					shape.schemes = appendUnique(shape.schemes, scheme)
				}
			}
		}
	}
	if !shape.master {
		shape.renditions = 1
		if len(shape.drm) > 0 {
			shape.protected = 1
		}
	}
	return shape
}

// hlsKeySystem names the DRM system an EXT-X-KEY requires, or "" for none.
func hlsKeySystem(attrs string) string {
	method := hlsAttr(attrs, "METHOD")
	if method == "" || strings.EqualFold(method, "NONE") {
		return ""
	}
	format := strings.ToLower(hlsAttr(attrs, "KEYFORMAT"))
	switch {
	case format == "" || format == "identity":
		return ""
	case strings.Contains(format, "streamingkeydelivery"):
		return "FairPlay"
	default:
		if system := drmSystemName(format); system != "" {
			return system
		}
		return format
	}
}

// hlsScheme maps an EXT-X-KEY METHOD to the encryption scheme it applies.
func hlsScheme(method string) string {
	switch strings.ToUpper(method) {
	case "SAMPLE-AES":
		return "cbcs"
	case "SAMPLE-AES-CTR", "SAMPLE-AES-CENC", "ISO-23001-7":
		return "cenc"
	}
	return ""
}

// hlsAttr reads one attribute from an HLS attribute list, honouring quoted values
// that contain commas.
func hlsAttr(list, name string) string {
	for list != "" {
		eq := strings.IndexByte(list, '=')
		if eq < 0 {
			return ""
		}
		key := strings.TrimSpace(list[:eq])
		rest := list[eq+1:]
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				value, rest = rest[1:], ""
			} else {
				value, rest = rest[1:1+end], rest[2+end:]
			}
		} else if end := strings.IndexByte(rest, ','); end >= 0 {
			value, rest = rest[:end], rest[end:]
		} else {
			value, rest = rest, ""
		}
		if strings.EqualFold(key, name) {
			return value
		}
		list = strings.TrimPrefix(strings.TrimSpace(rest), ",")
	}
	return ""
}

var (
	mpdElement = regexp.MustCompile(`<(?:\w+:)?MPD\b`)
	mpdType    = regexp.MustCompile(`<(?:\w+:)?MPD\b[^>]*?\btype\s*=\s*["'](static|dynamic)["']`)
	mpdDRM     = regexp.MustCompile(`(?i)<(?:\w+:)?ContentProtection\b[^>]*?\bschemeIdUri\s*=\s*["']([^"']+)["']`)
)

// The parts of an MPD that say what is protected. encoding/xml matches these by
// local name whatever namespace prefix a packager used, and never resolves external
// entities.
type mpdDocument struct {
	Type    string `xml:"type,attr"`
	Periods []struct {
		Sets []struct {
			Protection      []mpdProtection `xml:"ContentProtection"`
			Representations []struct {
				Protection []mpdProtection `xml:"ContentProtection"`
			} `xml:"Representation"`
		} `xml:"AdaptationSet"`
	} `xml:"Period"`
}

type mpdProtection struct {
	Scheme string   `xml:"schemeIdUri,attr"`
	Value  string   `xml:"value,attr"`
	PSSH   []string `xml:"pssh"` // cenc:pssh
	PRO    []string `xml:"pro"`  // mspr:pro, a PlayReady header
}

// mp4Protection marks content as encrypted without naming who can decrypt it; the
// systems that can are listed alongside it.
const mp4Protection = "urn:mpeg:dash:mp4protection:2011"

func inspectDASH(text string) manifestShape {
	var doc mpdDocument
	if err := xml.Unmarshal([]byte(text), &doc); err != nil {
		return inspectDASHLoosely(text)
	}
	// MPD@type defaults to static, i.e. on demand.
	shape := manifestShape{protocol: "dash", ended: doc.Type != "dynamic"}
	for _, period := range doc.Periods {
		for _, set := range period.Sets {
			setProtected := noteProtection(&shape, set.Protection)
			if len(set.Representations) == 0 {
				shape.renditions++
				if setProtected {
					shape.protected++
				}
				continue
			}
			for _, rep := range set.Representations {
				shape.renditions++
				if noteProtection(&shape, rep.Protection) || setProtected {
					shape.protected++
				}
			}
		}
	}
	if shape.protected > 0 && len(shape.drm) == 0 {
		shape.drm = []string{"Common Encryption"}
	}
	// A ContentProtection somewhere the structure above does not reach -- a newer
	// DASH edition allows them higher up -- still means protected.
	if loose := inspectDASHLoosely(text); len(loose.drm) > 0 && len(shape.drm) == 0 {
		shape.drm = loose.drm
		shape.protected = max(shape.protected, 1)
	}
	return shape
}

// noteProtection records what a set of ContentProtection descriptors says, and
// reports whether there were any.
func noteProtection(shape *manifestShape, descriptors []mpdProtection) bool {
	for _, d := range descriptors {
		scheme := strings.ToLower(strings.TrimSpace(d.Scheme))
		if scheme == mp4Protection {
			if v := strings.ToLower(strings.TrimSpace(d.Value)); isSchemeName(v) {
				shape.schemes = appendUnique(shape.schemes, v)
			}
		} else if name := drmSystemName(scheme); name != "" {
			shape.drm = appendUnique(shape.drm, name)
		} else if scheme != "" {
			shape.drm = appendUnique(shape.drm, scheme)
		}
		for _, pssh := range d.PSSH {
			if box, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pssh)); err == nil {
				for _, id := range psshSystemIDs(box) {
					if name := drmSystemName(id); name != "" {
						shape.drm = appendUnique(shape.drm, name)
					}
				}
			}
		}
		if len(d.PRO) > 0 {
			shape.drm = appendUnique(shape.drm, "PlayReady")
		}
	}
	return len(descriptors) > 0
}

// inspectDASHLoosely reads an MPD too malformed for encoding/xml, which players
// often tolerate, with patterns instead.
func inspectDASHLoosely(text string) manifestShape {
	shape := manifestShape{protocol: "dash", ended: true, renditions: 1}
	if m := mpdType.FindStringSubmatch(text); m != nil {
		shape.ended = m[1] == "static"
	}
	encrypted := false
	for _, m := range mpdDRM.FindAllStringSubmatch(text, -1) {
		scheme := strings.ToLower(m[1])
		if scheme == mp4Protection {
			encrypted = true
			continue
		}
		name := drmSystemName(scheme)
		if name == "" {
			name = scheme
		}
		shape.drm = appendUnique(shape.drm, name)
	}
	if encrypted && len(shape.drm) == 0 {
		shape.drm = []string{"Common Encryption"}
	}
	if len(shape.drm) > 0 {
		shape.protected = 1
	}
	return shape
}

func isSchemeName(s string) bool {
	switch s {
	case "cenc", "cbcs", "cens", "cbc1":
		return true
	}
	return false
}

// drmSystemName maps a key format, DASH scheme or DRM system ID to the DRM system it
// names.
func drmSystemName(id string) string {
	id = strings.ToLower(id)
	switch {
	case strings.Contains(id, "edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"), strings.Contains(id, "widevine"):
		return "Widevine"
	case strings.Contains(id, "9a04f079-9840-4286-ab92-e65be0885f95"), strings.Contains(id, "playready"):
		return "PlayReady"
	case strings.Contains(id, "94ce86fb-07ff-4f43-adb8-93d2fa968ca2"), strings.Contains(id, "fairplay"),
		strings.HasPrefix(id, "com.apple.fps"):
		return "FairPlay"
	case strings.Contains(id, "e2719d58-a985-b3c9-781a-b030af78d30e"),
		strings.Contains(id, "1077efec-c0b2-4d02-ace3-3c1e52e2fb4b"), strings.Contains(id, "clearkey"):
		return "ClearKey"
	}
	return ""
}

// psshSystemIDs lists the DRM system IDs of the PSSH boxes in data, as UUIDs. Only
// the IDs: they are public constants naming a DRM vendor, while the rest of a box --
// key IDs, vendor data -- is none of our business and is never kept.
func psshSystemIDs(data []byte) []string {
	var ids []string
	for off := 0; off+28 <= len(data) && len(ids) < 8; {
		size := int(binary.BigEndian.Uint32(data[off:]))
		if size < 28 || off+size > len(data) {
			break
		}
		if string(data[off+4:off+8]) == "pssh" {
			ids = append(ids, formatUUID(data[off+12:off+28]))
		}
		off += size
	}
	return ids
}

func formatUUID(b []byte) string {
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// renditionURLs lists the playlists an HLS master refers to -- variants first, then
// alternative renditions such as audio -- resolved against the master's URL.
func renditionURLs(body []byte, masterURL string) []string {
	var variants, renditions []string
	seen := map[string]bool{}
	add := func(list *[]string, ref string) {
		if ref == "" {
			return
		}
		u := absolutizeAgainst(masterURL, ref)
		if !seen[u] {
			seen[u] = true
			*list = append(*list, u)
		}
	}
	expectVariant := false
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF"):
			expectVariant = true
		case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			add(&renditions, hlsAttr(after(line, ":"), "URI"))
		case strings.HasPrefix(line, "#"):
		case expectVariant:
			add(&variants, line)
			expectVariant = false
		}
	}
	return append(variants, renditions...)
}

// maxRenditionChecks bounds how many playlists of one HLS stream are read.
const maxRenditionChecks = 12

// streamShape is what a stream's manifests say about the stream as a whole.
type streamShape struct {
	manifestShape
	body []byte // the top-level manifest as fetched
}

// inspectStream fetches a stream's manifest the way the proxy will and, for an HLS
// master playlist, the playlists it refers to: keys are declared per rendition, and
// so is whether the stream is live.
//
// With full set, every rendition is read, because one protected rendition is enough
// to break playback as soon as adaptive bitrate switches to it. Without, only the
// first -- enough to rank candidates cheaply.
func inspectStream(ctx context.Context, fetcher *Fetcher, manifestURL string, headers map[string]string, full bool) (streamShape, error) {
	body, err := fetcher.GetBytes(ctx, manifestURL, headers, MaxManifestBytes)
	if err != nil {
		return streamShape{}, err
	}
	shape := streamShape{manifestShape: inspectManifest(body), body: body}
	if shape.protocol != "hls" || !shape.master {
		return shape, nil
	}

	// Session keys in the master declare DRM for every rendition at once.
	sessionDRM := len(shape.drm) > 0
	urls := renditionURLs(body, manifestURL)
	if !full && len(urls) > 1 {
		urls = urls[:1]
	}
	if len(urls) > maxRenditionChecks {
		urls = urls[:maxRenditionChecks]
	}
	for i, u := range urls {
		rbody, err := fetcher.GetBytes(ctx, u, headers, MaxManifestBytes)
		if err != nil {
			return shape, fmt.Errorf("rendition %d of %d: %w", i+1, len(urls), err)
		}
		r := inspectManifest(rbody)
		shape.renditions++
		if len(r.drm) > 0 || sessionDRM {
			shape.protected++
		}
		shape.drm = appendUnique(shape.drm, r.drm...)
		shape.schemes = appendUnique(shape.schemes, r.schemes...)
		if i == 0 {
			shape.ended = r.ended
		}
	}
	return shape, nil
}

// maxSignedLifetime bounds what is believed as an expiry. A signed URL valid for
// longer than this is either not what we think it is or not worth refreshing early.
const maxSignedLifetime = 7 * 24 * time.Hour

// expiryKeys are parameter names CDNs sign an expiry into, as a Unix time.
var expiryKeys = map[string]bool{
	"exp": true, "expires": true, "expiry": true, "expiration": true, "expire": true, "e": true,
	"validto": true, "valid_to": true, "validuntil": true,
	"wowzatokenendtime": true, "endtime": true, "end_time": true,
}

// signedURLExpiry finds when a signed manifest URL stops working, if the URL says.
//
// Browser-discovered manifests are nearly always signed, and a signature that
// expires with no one re-resolving it leaves a channel dead until an operator
// notices. Reading the expiry lets the channel be re-resolved just before it, the
// same way an operator-set ttl_seconds does, without anyone having to guess a TTL.
// Covers plain parameters (?exp=, ?Expires=), tokens that nest them (Akamai's
// hdnts=st=…~exp=…), AWS presigned URLs, and JWTs carried in the query string.
func signedURLExpiry(rawURL string, now time.Time) (time.Time, bool) {
	var earliest time.Time
	consider := func(t time.Time) {
		if t.After(now) && t.Before(now.Add(maxSignedLifetime)) && (earliest.IsZero() || t.Before(earliest)) {
			earliest = t
		}
	}

	text := rawURL
	if unescaped, err := url.QueryUnescape(rawURL); err == nil {
		text = unescaped
	}
	pieces := strings.FieldsFunc(text, func(r rune) bool { return strings.ContainsRune("?&~;/,", r) })
	for _, piece := range pieces {
		key, value, ok := strings.Cut(piece, "=")
		if !ok {
			continue
		}
		// A value can be a key=value pair itself (hdnts=exp=…); use the innermost.
		for {
			k, v, nested := strings.Cut(value, "=")
			if !nested {
				break
			}
			key, value = k, v
		}
		if expiryKeys[strings.ToLower(key)] {
			if t, ok := parseUnixTime(value); ok {
				consider(t)
			}
		}
		if t, ok := jwtExpiry(value); ok {
			consider(t)
		}
	}

	if u, err := url.Parse(rawURL); err == nil {
		q := u.Query()
		if signed, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date")); err == nil {
			if secs, err := strconv.Atoi(q.Get("X-Amz-Expires")); err == nil {
				consider(signed.Add(time.Duration(secs) * time.Second))
			}
		}
		// Whole query values, in case a JWT's segments were split above.
		for _, values := range q {
			for _, v := range values {
				if t, ok := jwtExpiry(v); ok {
					consider(t)
				}
			}
		}
	}
	return earliest, !earliest.IsZero()
}

// parseUnixTime reads a Unix time in seconds (10 digits) or milliseconds (13).
func parseUnixTime(s string) (time.Time, bool) {
	if len(s) != 10 && len(s) != 13 {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	if len(s) == 13 {
		return time.UnixMilli(n), true
	}
	return time.Unix(n, 0), true
}

// jwtExpiry reads the exp claim of a JWT without verifying it. The signature is
// the CDN's business; all we want to know is when it stops accepting the token.
func jwtExpiry(s string) (time.Time, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 || len(parts[1]) < 8 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(claims.Exp), 0), true
}

func appendUnique(list []string, values ...string) []string {
	for _, v := range values {
		found := false
		for _, existing := range list {
			if existing == v {
				found = true
				break
			}
		}
		if !found {
			list = append(list, v)
		}
	}
	return list
}
