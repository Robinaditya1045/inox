package live

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

func TestInspectManifestClassifiesHLS(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		master bool
		ended  bool
		drm    string
	}{
		{
			name:   "master playlist",
			body:   "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000\nlow.m3u8\n",
			master: true,
		},
		{
			name: "live media playlist",
			body: "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\na.ts\n",
		},
		{
			name:  "finished recording",
			body:  "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\na.ts\n#EXT-X-ENDLIST\n",
			ended: true,
		},
		{
			// AES-128 with a plain key URI is ordinary encryption hls.js handles and
			// the proxy passes through. Calling it DRM would refuse streams that play.
			name: "AES-128 is not DRM",
			body: "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\n#EXTINF:4,\na.ts\n",
		},
		{
			name: "identity key format is not DRM",
			body: "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"k\",KEYFORMAT=\"identity\"\n#EXTINF:4,\na.ts\n",
		},
		{
			name: "FairPlay",
			body: "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://x\",KEYFORMAT=\"com.apple.streamingkeydelivery\"\n#EXTINF:4,\na.ts\n",
			drm:  "FairPlay",
		},
		{
			name:   "Widevine session key in a master",
			body:   "#EXTM3U\n#EXT-X-SESSION-KEY:METHOD=SAMPLE-AES-CTR,URI=\"data:text/plain;base64,AAAA\",KEYFORMAT=\"urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv.m3u8\n",
			master: true,
			drm:    "Widevine",
		},
		{
			name: "PlayReady",
			body: "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"k\",KEYFORMAT=\"com.microsoft.playready\"\n#EXTINF:4,\na.ts\n",
			drm:  "PlayReady",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shape := inspectManifest([]byte(tc.body))
			if shape.protocol != "hls" {
				t.Fatalf("protocol = %q, want hls", shape.protocol)
			}
			if shape.master != tc.master || shape.ended != tc.ended {
				t.Errorf("master=%v ended=%v, want master=%v ended=%v", shape.master, shape.ended, tc.master, tc.ended)
			}
			if got := strings.Join(shape.drm, ","); got != tc.drm {
				t.Errorf("drm = %q, want %q", got, tc.drm)
			}
		})
	}
}

func TestInspectManifestClassifiesDASHAndRejectsTheRest(t *testing.T) {
	live := inspectManifest([]byte(`<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic"><Period/></MPD>`))
	if live.protocol != "dash" || live.ended {
		t.Errorf("dynamic MPD = %+v, want a live dash stream", live)
	}
	// MPD@type defaults to static.
	if vod := inspectManifest([]byte(`<MPD><Period/></MPD>`)); vod.protocol != "dash" || !vod.ended {
		t.Errorf("MPD without type = %+v, want an on-demand dash stream", vod)
	}

	protected := inspectManifest([]byte(`<MPD type="dynamic"><AdaptationSet>
  <ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc"/>
  <ContentProtection schemeIdUri="urn:uuid:EDEF8BA9-79D6-4ACE-A3C8-27DCD51D21ED"/>
</AdaptationSet></MPD>`))
	if got := strings.Join(protected.drm, ","); got != "Widevine" {
		t.Errorf("drm = %q, want Widevine (the CENC marker alone names no system)", got)
	}
	if cenc := inspectManifest([]byte(`<MPD><ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011"/></MPD>`)); len(cenc.drm) == 0 {
		t.Error("a stream marked only as CENC-encrypted must still count as protected")
	}

	for _, body := range []string{
		"#EXTM3U\n#EXTINF:123,Artist - Song\nsong.mp3\n", // plain M3U audio playlist, not HLS
		"<html><body>player</body></html>",
		`{"hls":"https://cdn.example.com/a.m3u8"}`,
	} {
		if shape := inspectManifest([]byte(body)); shape.protocol != "" {
			t.Errorf("%q classified as %q, want not a manifest", body, shape.protocol)
		}
	}
}

func TestHLSAttrHonoursQuotedCommas(t *testing.T) {
	attrs := `METHOD=SAMPLE-AES,URI="skd://a,b,c",KEYFORMAT="com.apple.streamingkeydelivery",KEYFORMATVERSIONS="1"`
	if got := hlsAttr(attrs, "URI"); got != "skd://a,b,c" {
		t.Errorf("URI = %q", got)
	}
	if got := hlsAttr(attrs, "keyformat"); got != "com.apple.streamingkeydelivery" {
		t.Errorf("KEYFORMAT = %q", got)
	}
	if got := hlsAttr(attrs, "IV"); got != "" {
		t.Errorf("missing attribute = %q, want empty", got)
	}
}

func TestManifestRecognition(t *testing.T) {
	for _, u := range []string{
		"https://cdn.example.com/live/master.m3u8",
		"https://cdn.example.com/live/master.m3u8?token=abc",
		"https://cdn.example.com/dash/manifest.mpd",
		"https://ams.example.net/video.ism/manifest(format=m3u8-aapl)",
	} {
		if !manifestURL(u) {
			t.Errorf("manifestURL(%q) = false", u)
		}
	}
	for _, u := range []string{"https://cdn.example.com/seg-1.ts", "https://example.com/player.js"} {
		if manifestURL(u) {
			t.Errorf("manifestURL(%q) = true", u)
		}
	}

	// A beacon that merely carries the stream URL is worth a look, never proof.
	beacon := "https://stats.example.com/collect?src=https%3A%2F%2Fcdn.example.com%2Fa.m3u8"
	if !manifestQuery(beacon) || manifestURL(beacon) {
		t.Errorf("beacon: query=%v path=%v, want only the query to hint", manifestQuery(beacon), manifestURL(beacon))
	}
	if servedAsManifest(beacon, "image/gif") {
		t.Error("an analytics pixel must not end the wait for the player")
	}

	if !servedAsManifest("https://api.example.com/stream?id=7", "application/vnd.apple.mpegurl; charset=utf-8") {
		t.Error("a manifest content type is proof whatever the URL looks like")
	}
	// Origins serving playlists as text/plain are common and must still count.
	if !servedAsManifest("https://cdn.example.com/a.m3u8", "text/plain") {
		t.Error("an .m3u8 served as text/plain should count")
	}
	if servedAsManifest("https://cdn.example.com/a.m3u8", "text/html") {
		t.Error("an .m3u8 URL that returned an HTML error page should not count")
	}
}

func TestSignedURLExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	in := func(d time.Duration) int64 { return now.Add(d).Unix() }

	jwt := func(exp int64) string {
		enc := base64.RawURLEncoding
		return enc.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." +
			enc.EncodeToString([]byte(fmt.Sprintf(`{"sub":"x","exp":%d}`, exp))) + ".c2ln"
	}

	cases := []struct {
		name string
		url  string
		want int64 // 0: no expiry believed
	}{
		{"plain exp", fmt.Sprintf("https://cdn.example.com/a.m3u8?exp=%d&sig=x", in(time.Hour)), in(time.Hour)},
		{"CloudFront Expires", fmt.Sprintf("https://d1.cloudfront.net/a.m3u8?Expires=%d&Signature=x&Key-Pair-Id=K", in(2*time.Hour)), in(2 * time.Hour)},
		{"milliseconds", fmt.Sprintf("https://cdn.example.com/a.m3u8?expires=%d", now.Add(time.Hour).UnixMilli()), in(time.Hour)},
		{"Akamai token", fmt.Sprintf("https://x.akamaized.net/a.m3u8?hdnts=st=%d~exp=%d~acl=/*~hmac=ab", now.Unix(), in(30*time.Minute)), in(30 * time.Minute)},
		{"Akamai token, encoded", fmt.Sprintf("https://x.akamaized.net/a.m3u8?hdnts=st%%3D%d%%7Eexp%%3D%d%%7Eacl%%3D%%2F*", now.Unix(), in(30*time.Minute)), in(30 * time.Minute)},
		{"token in the path", fmt.Sprintf("https://cdn.example.com/exp=%d~acl=%%2F*~hmac=ab/live/a.m3u8", in(time.Hour)), in(time.Hour)},
		{"AWS presigned", "https://b.s3.amazonaws.com/a.m3u8?X-Amz-Date=" + now.UTC().Format("20060102T150405Z") + "&X-Amz-Expires=900&X-Amz-Signature=x", in(15 * time.Minute)},
		{"JWT", "https://cdn.example.com/a.m3u8?token=" + jwt(in(45*time.Minute)), in(45 * time.Minute)},
		{"earliest wins", fmt.Sprintf("https://cdn.example.com/a.m3u8?exp=%d&token=%s", in(3*time.Hour), jwt(in(time.Hour))), in(time.Hour)},
		{"already expired", fmt.Sprintf("https://cdn.example.com/a.m3u8?exp=%d", in(-time.Hour)), 0},
		{"implausibly far off", fmt.Sprintf("https://cdn.example.com/a.m3u8?exp=%d", in(30*24*time.Hour)), 0},
		{"not a timestamp", "https://cdn.example.com/a.m3u8?e=12345&exp=soon", 0},
		{"unsigned", "https://cdn.example.com/live/a.m3u8", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := signedURLExpiry(tc.url, now)
			switch {
			case tc.want == 0 && ok:
				t.Errorf("expiry = %v, want none", got)
			case tc.want != 0 && !ok:
				t.Errorf("no expiry found, want %v", time.Unix(tc.want, 0))
			case tc.want != 0 && got.Unix() != tc.want:
				t.Errorf("expiry = %v, want %v", got, time.Unix(tc.want, 0))
			}
		})
	}
}

func TestRequestContextReplaysOnlyHotlinkHeaders(t *testing.T) {
	wire := map[string]string{
		"Referer":        "https://player.example.net/",
		"Origin":         "https://player.example.net",
		"User-Agent":     "Mozilla/5.0 HeadlessChrome/152",
		"Cookie":         "session=secret",
		"Authorization":  "Bearer secret",
		"X-Player-Token": "secret",
		"Sec-Fetch-Mode": "cors",
	}
	got := requestContext(wire, map[string]string{"referer": "https://operator.example/"})

	if got["Referer"] != "https://operator.example/" {
		t.Errorf("Referer = %q, want the operator's explicit value to win", got["Referer"])
	}
	if got["Origin"] != wire["Origin"] || got["User-Agent"] != wire["User-Agent"] {
		t.Errorf("headers = %v, want Origin and User-Agent carried over", got)
	}
	for _, name := range []string{"Cookie", "Authorization", "X-Player-Token", "Sec-Fetch-Mode"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s was replayed; credentials and fingerprint headers must stay in the browser", name)
		}
	}
}

// ── choosing among what the page loaded ─────────────────────

// origin serves fixed bodies by path, like a CDN with several streams on it.
func origin(t *testing.T, bodies map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if body == "403" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func loadedRequest(u string, seq int) *manifestRequest {
	return &manifestRequest{url: u, status: 200, seq: seq, headers: map[string]string{"Referer": "https://page.example/"}}
}

func TestChooseStreamPrefersTheLiveMasterPlaylist(t *testing.T) {
	const master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv/index.m3u8\n"
	srv := origin(t, map[string]string{
		// An ad pre-roll: complete, and requested first.
		"/ad/master.m3u8":  master,
		"/ad/v/index.m3u8": "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXTINF:5,\nad.ts\n#EXT-X-ENDLIST\n",
		// The channel, whose variant the player also loaded.
		"/tv/master.m3u8":  master,
		"/tv/v/index.m3u8": "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nlive.ts\n",
		"/tv/dash.mpd":     `<MPD type="dynamic"></MPD>`,
	})
	f := NewFetcher("", false, AllowLoopback())

	var evidence drmEvidence
	choice, diags, err := chooseStream(context.Background(), f, []*manifestRequest{
		loadedRequest(srv.URL+"/ad/master.m3u8", 1),
		loadedRequest(srv.URL+"/tv/dash.mpd", 2),
		loadedRequest(srv.URL+"/tv/v/index.m3u8", 3),
		loadedRequest(srv.URL+"/tv/master.m3u8", 4),
	}, nil, 0, &evidence)
	if err != nil {
		t.Fatalf("chooseStream: %v", err)
	}
	if choice.url != srv.URL+"/tv/master.m3u8" || !choice.master || choice.protocol != "hls" || choice.ended {
		t.Errorf("chose %+v, want the live HLS master over the ad, the DASH copy and the bare variant", choice)
	}
	if choice.headers["Referer"] != "https://page.example/" {
		t.Errorf("headers = %v, want the page's Referer carried with the choice", choice.headers)
	}
	verdicts := map[string]string{}
	for i, d := range diags {
		verdicts[fmt.Sprint(i)] = d.Verdict + " " + d.Kind
	}
	want := map[string]string{"0": "usable master", "1": "usable mpd", "2": "usable media", "3": "chosen master"}
	for k, v := range want {
		if verdicts[k] != v {
			t.Errorf("manifest %s verdict = %q, want %q (all: %v)", k, verdicts[k], v, verdicts)
		}
	}
	if len(evidence.signals) != 0 {
		t.Errorf("unprotected streams produced DRM evidence: %v", evidence.signals)
	}
}

func TestChooseStreamExplainsWhatWentWrong(t *testing.T) {
	srv := origin(t, map[string]string{
		"/drm/master.m3u8":   "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv.m3u8\n",
		"/drm/v.m3u8":        "#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://k\",KEYFORMAT=\"com.apple.streamingkeydelivery\"\n#EXTINF:4,\na.ts\n",
		"/bound/master.m3u8": "403",
		"/html/master.m3u8":  "<html>not found</html>",
	})
	f := NewFetcher("", false, AllowLoopback())
	choose := func(fetcher *Fetcher, path string) (error, []domain.ManifestDiagnostic) {
		var evidence drmEvidence
		_, diags, err := chooseStream(context.Background(), fetcher, []*manifestRequest{loadedRequest(srv.URL+path, 1)}, nil, 0, &evidence)
		return err, diags
	}
	expect := func(name string, err error, outcome domain.ResolveOutcome, reason, text string) {
		t.Helper()
		gotOutcome, gotReason := failureOutcome(err)
		if gotOutcome != outcome || gotReason != reason || err == nil || !strings.Contains(err.Error(), text) {
			t.Errorf("%s: %s/%s %v, want %s/%s mentioning %q", name, gotOutcome, gotReason, err, outcome, reason, text)
		}
	}

	err, diags := choose(f, "/drm/master.m3u8")
	expect("DRM stream", err, domain.OutcomeDRMProtected, "manifest_protected", "FairPlay")
	if !errors.Is(err, ErrDRMProtected) || diags[0].Verdict != "drm_protected" {
		t.Errorf("DRM stream: errors.Is=%v verdict=%q", errors.Is(err, ErrDRMProtected), diags[0].Verdict)
	}
	err, diags = choose(f, "/bound/master.m3u8")
	// The browser loaded it; the server's 403 means the source binds it to the
	// browser -- discovered, but not server-replayable.
	expect("browser-bound stream", err, domain.OutcomeServerReplayFailed, "browser_bound", "refused")
	if diags[0].Verdict != "browser_only" || diags[0].Detail != "upstream returned 403" {
		t.Errorf("browser-bound stream diagnostic = %+v", diags[0])
	}
	err, _ = choose(f, "/html/master.m3u8")
	expect("non-manifest", err, domain.OutcomeNoStreamFound, "not_a_manifest", "none of them returned")

	// The stream's host has to be allowlisted like any other source, and the error
	// must say which one, since it is usually a CDN the operator has never seen.
	err, diags = choose(NewFetcher("page.example", true, AllowLoopback()), "/drm/master.m3u8")
	expect("disallowed host", err, domain.OutcomeFailed, "host_not_allowed", "LIVE_SOURCE_ALLOWED_HOSTS")
	if diags[0].Verdict != "not_allowed" || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("disallowed host: verdict %q, error %v", diags[0].Verdict, err)
	}
}

// A master whose first rendition is clear and whose second needs DRM plays until
// adaptive bitrate switches up -- and then fails. It must be caught before it is
// accepted, which means reading every rendition, not the first.
func TestChooseStreamReadsEveryRenditionBeforeAccepting(t *testing.T) {
	srv := origin(t, map[string]string{
		"/tv/master.m3u8": "#EXTM3U\n" +
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"a\",NAME=\"en\",URI=\"audio.m3u8\"\n" +
			"#EXT-X-STREAM-INF:BANDWIDTH=800000,AUDIO=\"a\"\nlow.m3u8\n" +
			"#EXT-X-STREAM-INF:BANDWIDTH=6000000,AUDIO=\"a\"\nhd.m3u8\n",
		"/tv/low.m3u8":   "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nlow.ts\n",
		"/tv/audio.m3u8": "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\naudio.aac\n",
		"/tv/hd.m3u8": "#EXTM3U\n#EXT-X-TARGETDURATION:6\n" +
			"#EXT-X-KEY:METHOD=SAMPLE-AES-CTR,URI=\"data:text/plain;base64,AAAA\",KEYFORMAT=\"urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed\"\n" +
			"#EXTINF:6,\nhd.ts\n",
	})
	var evidence drmEvidence
	_, diags, err := chooseStream(context.Background(), NewFetcher("", false, AllowLoopback()),
		[]*manifestRequest{loadedRequest(srv.URL+"/tv/master.m3u8", 1)}, nil, 0, &evidence)

	if outcome, reason := failureOutcome(err); outcome != domain.OutcomeDRMProtected || reason != "renditions_protected" {
		t.Fatalf("outcome = %s/%s (%v), want DRM_PROTECTED/renditions_protected", outcome, reason, err)
	}
	if !strings.Contains(err.Error(), "Widevine") || !strings.Contains(err.Error(), "1 of its 3 renditions") {
		t.Errorf("error = %v, want it to name Widevine and the share of renditions protected", err)
	}
	if diags[0].Verdict != "drm_protected" || diags[0].Detail != "1 of 3 renditions protected" {
		t.Errorf("diagnostic = %+v", diags[0])
	}
	if got := strings.Join(evidence.schemes, ","); got != "cenc" {
		t.Errorf("schemes = %q, want cenc from SAMPLE-AES-CTR", got)
	}
}

func TestInspectDASHCountsProtectedRepresentations(t *testing.T) {
	// A Widevine PSSH box: size, "pssh", version 0 and flags, then the system ID.
	box := append([]byte{0, 0, 0, 32, 'p', 's', 's', 'h', 0, 0, 0, 0},
		0xed, 0xef, 0x8b, 0xa9, 0x79, 0xd6, 0x4a, 0xce, 0xa3, 0xc8, 0x27, 0xdc, 0xd5, 0x1d, 0x21, 0xed)
	box = append(box, 0, 0, 0, 0) // no data
	mpd := `<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" xmlns:cenc="urn:mpeg:cenc:2013" xmlns:mspr="urn:microsoft:playready" type="dynamic">
  <Period>
    <AdaptationSet contentType="video">
      <ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cbcs" cenc:default_KID="10000000-1000-1000-1000-100000000001"/>
      <ContentProtection schemeIdUri="urn:uuid:5E629AF5-38DA-4063-8977-97FFBD9902D4"><cenc:pssh>` + base64.StdEncoding.EncodeToString(box) + `</cenc:pssh></ContentProtection>
      <ContentProtection schemeIdUri="urn:uuid:9a04f079-9840-4286-ab92-e65be0885f95"><mspr:pro>AAAA</mspr:pro></ContentProtection>
      <Representation id="v1" bandwidth="800000"/>
      <Representation id="v2" bandwidth="3000000"/>
    </AdaptationSet>
    <AdaptationSet contentType="audio"><Representation id="a1" bandwidth="128000"/></AdaptationSet>
  </Period>
</MPD>`
	shape := inspectManifest([]byte(mpd))
	if shape.protocol != "dash" || shape.ended {
		t.Fatalf("shape = %+v, want a live dash stream", shape)
	}
	if shape.renditions != 3 || shape.protected != 2 {
		t.Errorf("renditions = %d protected = %d, want 3 and 2 (clear audio)", shape.renditions, shape.protected)
	}
	// Marlin's UUID is unknown here and is kept as is; Widevine is only named by the
	// PSSH box inside it.
	for _, want := range []string{"Widevine", "PlayReady"} {
		if !contains(shape.drm, want) {
			t.Errorf("drm = %v, want %s", shape.drm, want)
		}
	}
	if got := strings.Join(shape.schemes, ","); got != "cbcs" {
		t.Errorf("schemes = %q, want cbcs", got)
	}
	// The key ID is data about the content, not about the stream's shape.
	if strings.Contains(fmt.Sprint(shape), "10000000-1000") {
		t.Error("the key ID leaked into the inspected shape")
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
