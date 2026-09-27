package live_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

// ── fixtures ────────────────────────────────────────────────

// browserForTest returns a shared-browser handle, skipping when no Chromium-family
// browser is installed. The sandbox is off unless LIVE_BROWSER_TEST_SANDBOX=true:
// the only pages opened are the fixtures below, and CI hosts often cannot provide
// the user namespaces it needs. Turn it on to check an image, as deployments do.
func browserForTest(t *testing.T) *live.Browser {
	t.Helper()
	return browserWith(t, testFetcher())
}

func browserWith(t *testing.T, fetcher *live.Fetcher) *live.Browser {
	t.Helper()
	path := os.Getenv("LIVE_BROWSER_PATH")
	if path == "" {
		found, ok := live.FindChromium()
		if !ok {
			t.Skip("no Chromium-family browser installed; set LIVE_BROWSER_PATH to run browser resolver tests")
		}
		path = found
	}
	noSandbox := os.Getenv("LIVE_BROWSER_TEST_SANDBOX") != "true"
	b := live.NewBrowser(live.BrowserConfig{ExecPath: path, MaxConcurrency: 2, NoSandbox: noSandbox}, fetcher)
	t.Cleanup(b.Close)
	return b
}

// streamSite is a page, the player it embeds, and the CDN the player streams from.
//
// The page is served on 127.0.0.1 and the player frame on localhost: two different
// sites, so Chromium renders the frame out of process and its requests are only
// visible through a session of its own -- the case that makes browser resolution
// hard. The CDN enforces the Referer the player's origin sends, as hotlink
// protection does.
type streamSite struct {
	site *httptest.Server
	cdn  *httptest.Server

	mu         sync.Mutex
	userAgents []string // of manifest requests the CDN accepted
}

type siteOptions struct {
	// player is the script run inside the innermost frame, with CDN replaced by the
	// CDN's base URL.
	player string
	// variant is the media playlist the CDN serves.
	variant string
	// requireToken makes the CDN demand a header only the player's own script adds.
	requireToken bool
}

const liveVariant = "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:100\n#EXTINF:6.0,\nseg100.ts\n#EXTINF:6.0,\nseg101.ts\n"

// clickToPlay only requests anything once the player is clicked, as players that
// load lazily do. The click has to cross two frame boundaries to land here.
const clickToPlay = `
document.getElementById('player').addEventListener('click', () => {
  fetch('CDN/live/master.m3u8', { headers: TOKEN })
    .then(r => r.text())
    .then(text => {
      const variant = text.split('\n').find(l => l && !l.startsWith('#'));
      return fetch(new URL(variant, 'CDN/live/master.m3u8'), { headers: TOKEN });
    });
}, { once: true });`

func newStreamSite(t *testing.T, opts siteOptions) *streamSite {
	t.Helper()
	s := &streamSite{}
	if opts.variant == "" {
		opts.variant = liveVariant
	}

	cdn := http.NewServeMux()
	accept := func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "X-Player-Token")
		if r.Method == http.MethodOptions {
			return false
		}
		// Only the frame's origin is sent cross-origin (strict-origin-when-cross-origin).
		if r.Header.Get("Referer") != s.site.URL+"/" {
			http.Error(w, "hotlink protection: referer "+r.Header.Get("Referer"), http.StatusForbidden)
			return false
		}
		if opts.requireToken && r.Header.Get("X-Player-Token") == "" {
			http.Error(w, "missing player token", http.StatusForbidden)
			return false
		}
		s.mu.Lock()
		s.userAgents = append(s.userAgents, r.Header.Get("User-Agent"))
		s.mu.Unlock()
		return true
	}
	cdn.HandleFunc("/live/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if accept(w, r) {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720\n720p/index.m3u8\n")
		}
	})
	cdn.HandleFunc("/live/720p/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if accept(w, r) {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, opts.variant)
		}
	})
	s.cdn = httptest.NewServer(cdn)
	t.Cleanup(s.cdn.Close)

	token := "{}"
	if opts.requireToken {
		token = `{'X-Player-Token': 'issued-to-this-browser'}`
	}
	player := strings.NewReplacer("CDN", s.cdn.URL, "TOKEN", token).Replace(opts.player)

	site := http.NewServeMux()
	site.HandleFunc("/watch", func(w http.ResponseWriter, r *http.Request) {
		// localhost, not 127.0.0.1: a different site, so an out-of-process frame.
		outer := strings.Replace(s.site.URL, "127.0.0.1", "localhost", 1) + "/outer"
		fmt.Fprintf(w, `<!doctype html><html><body style="margin:0">
<h1>Channel 7</h1><p>Some text above the player.</p>
<iframe src="%s" style="width:960px;height:540px;border:0"></iframe>
</body></html>`, outer)
	})
	site.HandleFunc("/outer", func(w http.ResponseWriter, r *http.Request) {
		// And back to 127.0.0.1: out of process relative to its own parent.
		fmt.Fprintf(w, `<!doctype html><html><body style="margin:0">
<iframe src="%s/inner" style="width:100vw;height:100vh;border:0"></iframe>
</body></html>`, s.site.URL)
	})
	site.HandleFunc("/inner", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!doctype html><html><body style="margin:0">
<div id="player" style="width:100vw;height:100vh;background:#000"><video style="width:100%%;height:100%%"></video></div>
<script>%s</script>
</body></html>`, player)
	})
	site.HandleFunc("/empty", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><body><p>Off air.</p></body></html>`)
	})
	s.site = httptest.NewServer(site)
	t.Cleanup(s.site.Close)
	return s
}

// expectOutcome checks a resolve error's outcome and reason code.
func expectOutcome(t *testing.T, err error, outcome domain.ResolveOutcome, reason string) {
	t.Helper()
	var re *live.ResolveError
	if !errors.As(err, &re) {
		t.Fatalf("error %v is not a ResolveError", err)
	}
	if re.Outcome != outcome || re.Reason != reason {
		t.Errorf("outcome = %s/%s, want %s/%s (%v)", re.Outcome, re.Reason, outcome, reason, err)
	}
	if re.Diagnostics == nil || re.Diagnostics.Outcome != outcome {
		t.Errorf("diagnostics = %+v, want them attached with the same outcome", re.Diagnostics)
	}
}

func (s *streamSite) channel(path string, config map[string]any) *domain.LiveChannel {
	if config == nil {
		config = map[string]any{}
	}
	return &domain.LiveChannel{Slug: "browser-test", SourceURL: s.site.URL + path, ResolverConfig: config}
}

// ── resolving ───────────────────────────────────────────────

func TestBrowserResolverFindsStreamInNestedCrossSiteFrame(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	site := newStreamSite(t, siteOptions{player: clickToPlay})
	resolver := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := resolver.Resolve(ctx, site.channel("/watch", nil))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if res.ManifestURL != site.cdn.URL+"/live/master.m3u8" {
		t.Errorf("manifest = %q, want the master playlist the player requested", res.ManifestURL)
	}
	if res.Protocol != "hls" {
		t.Errorf("protocol = %q, want hls", res.Protocol)
	}
	// The CDN only served the check fetch because the Referer the player's frame
	// sent was replayed, which is the same thing the proxy will now do.
	if res.Headers["Referer"] != site.site.URL+"/" {
		t.Errorf("Referer = %q, want the one the player's frame sent", res.Headers["Referer"])
	}
	if ua := res.Headers["User-Agent"]; !strings.Contains(ua, "Chrome") {
		t.Errorf("User-Agent = %q, want the browser's own", ua)
	}
	d := res.Diagnostics
	if d == nil || d.Outcome != domain.OutcomeStreamFound || d.Page == nil || d.Page.Frames != 2 || d.Page.Nudges < 1 || !d.Page.Loaded {
		t.Fatalf("diagnostics = %+v, want STREAM_FOUND with both frames and the click recorded", d)
	}
	// The two nested frames are on localhost and 127.0.0.1; the diagnostics name
	// the hosts, so an operator can see the hierarchy the player was found in.
	if len(d.Page.FrameHosts) == 0 {
		t.Errorf("frame hosts = %v, want the frames' hosts recorded", d.Page.FrameHosts)
	}
	if last := d.Manifests[len(d.Manifests)-1]; d.Manifests[0].Verdict != "chosen" || d.Manifests[0].Kind != "master" || last.Host == "" {
		t.Errorf("manifests = %+v, want the master chosen", d.Manifests)
	}
}

func TestBrowserResolverReportsDRM(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	// The master playlist is clean; the key only shows up in the variant, which is
	// where HLS puts it.
	site := newStreamSite(t, siteOptions{
		player: clickToPlay,
		variant: "#EXTM3U\n#EXT-X-TARGETDURATION:6\n" +
			`#EXT-X-KEY:METHOD=SAMPLE-AES,URI="skd://key-id",KEYFORMAT="com.apple.streamingkeydelivery",KEYFORMATVERSIONS="1"` +
			"\n#EXTINF:6.0,\nseg100.ts\n",
	})

	_, err := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second).
		Resolve(context.Background(), site.channel("/watch", nil))
	if !errors.Is(err, live.ErrDRMProtected) {
		t.Fatalf("error = %v, want ErrDRMProtected", err)
	}
	if !strings.Contains(err.Error(), "FairPlay") {
		t.Errorf("error = %v, want it to name the DRM system", err)
	}
	expectOutcome(t, err, domain.OutcomeDRMProtected, "manifest_protected")
}

func TestBrowserResolverRefusesStreamsTiedToTheBrowser(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	// The player adds a header of its own, computed by its script. Only Referer,
	// Origin, User-Agent and the page's cookies are carried over, so the server's
	// check fetch goes without it and is refused -- and the operator hears about it
	// now, not from a room.
	site := newStreamSite(t, siteOptions{player: clickToPlay, requireToken: true})

	_, err := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second).
		Resolve(context.Background(), site.channel("/watch", nil))
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("error = %v, want it to explain the server's request was refused", err)
	}
	// The token is computed by the player's script -- a browser-bound stream. The
	// server's request goes without it and is refused: discovered, not replayable.
	expectOutcome(t, err, domain.OutcomeServerReplayFailed, "browser_bound")
}

// sessionSite is a page that hands its visitor a cookie and a CDN that only serves
// the stream to requests carrying it -- one set by the page, the other by the CDN
// on the master playlist. cookie names the page's cookie.
func sessionSite(t *testing.T, cookie string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			if c, err := r.Cookie(cookie); err != nil || c.Value != "visitor-42" {
				http.Error(w, "no session", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			if r.URL.Path == "/live/master.m3u8" {
				http.SetCookie(w, &http.Cookie{Name: "edge", Value: "e-1", Path: "/live/"})
				fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv/index.m3u8\n")
				return
			}
			if c, err := r.Cookie("edge"); err != nil || c.Value != "e-1" {
				http.Error(w, "no edge session", http.StatusForbidden)
				return
			}
			fmt.Fprint(w, liveVariant)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: cookie, Value: "visitor-42", Path: "/", HttpOnly: true})
		fmt.Fprint(w, `<!doctype html><script>
fetch('/live/master.m3u8').then(r => r.text()).then(() => fetch('/live/v/index.m3u8'));
</script>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBrowserResolverCarriesThePageSessionToTheServer(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	site := sessionSite(t, "sid")

	res, err := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second).
		Resolve(context.Background(), &domain.LiveChannel{SourceURL: site.URL + "/watch"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Cookies == nil {
		t.Fatal("no session handed over with the resolution")
	}
	// The proxy asks with the cookies the resolution carries, and keeps any the
	// CDN adds -- here, the one it set on the master during the check.
	u, _ := url.Parse(site.URL + "/live/v/index.m3u8")
	names := map[string]bool{}
	for _, c := range res.Cookies.Cookies(u) {
		names[c.Name] = true
	}
	if !names["sid"] || !names["edge"] {
		t.Errorf("session cookies for the variant = %v, want the page's and the CDN's", names)
	}
	if _, err := testFetcher().WithCookies(res.Cookies).GetBytes(context.Background(), u.String(), res.Headers, 1<<16); err != nil {
		t.Errorf("variant through the handed-over session: %v", err)
	}
	// Both cookies were in the browser by the end: the page's and the one the CDN
	// set on the master the player loaded.
	d := res.Diagnostics
	if d == nil || d.Page.Cookies != 2 || d.Page.CookiesWithheld != 0 {
		t.Errorf("page diagnostics = %+v, want both cookies counted as carried, none withheld", d.Page)
	}
	// The player's own requests carried cookies. Which manifest records how many
	// depends on when the wire-header extra-info event lands, so require it of the
	// request set rather than of the master specifically.
	sawBrowserCookies := false
	for _, m := range d.Manifests {
		if m.BrowserCookies > 0 {
			sawBrowserCookies = true
		}
	}
	if !sawBrowserCookies {
		t.Errorf("manifests = %+v, want the player's requests recorded as carrying cookies", d.Manifests)
	}
}

func TestBrowserResolverWithholdsBotProtectionClearances(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	// The site's CDN only serves browsers that passed its bot check. Handing the
	// server that pass would get around the check, so the stream is reported.
	site := sessionSite(t, "cf_clearance")

	_, err := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second).
		Resolve(context.Background(), &domain.LiveChannel{SourceURL: site.URL + "/watch"})
	expectOutcome(t, err, domain.OutcomeServerReplayFailed, "bot_protected")
	if err == nil || !strings.Contains(err.Error(), "cf_clearance") || strings.Contains(err.Error(), "visitor-42") {
		t.Errorf("error = %v, want the clearance named and its value nowhere", err)
	}
	var re *live.ResolveError
	if errors.As(err, &re) && (re.Diagnostics.Page.CookiesWithheld != 1 || re.Diagnostics.Page.Cookies != 1) {
		t.Errorf("page diagnostics = %+v, want the clearance withheld and the CDN's own cookie carried", re.Diagnostics.Page)
	}
}

func TestBrowserResolverExplainsWhenNothingIsRequested(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	site := newStreamSite(t, siteOptions{player: clickToPlay})

	start := time.Now()
	_, err := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second).
		Resolve(context.Background(), site.channel("/empty", map[string]any{"timeout_seconds": float64(5)}))
	if err == nil || !strings.Contains(err.Error(), "no HLS or DASH manifest was requested") {
		t.Fatalf("error = %v, want it to say no manifest was requested", err)
	}
	expectOutcome(t, err, domain.OutcomeNoStreamFound, "no_manifest_requested")
	// timeout_seconds is honoured: the page is not watched for the default budget.
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Errorf("resolve took %s, want it bounded by timeout_seconds", elapsed)
	}
}

func TestBrowserResolverUsesManifestPatternToPickTheStream(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	// An autoplaying page that loads a preview before the channel itself. Both are
	// live media playlists, so nothing but the operator's pattern tells them apart.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, liveVariant)
			return
		}
		fmt.Fprint(w, `<!doctype html><script>
fetch('/preview/index.m3u8').then(() => fetch('/channel/main/index.m3u8'));
</script>`)
	}))
	defer srv.Close()
	resolver := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second)

	res, err := resolver.Resolve(context.Background(), &domain.LiveChannel{
		SourceURL: srv.URL + "/watch", ResolverConfig: map[string]any{"manifest_pattern": `/channel/main/`},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.HasSuffix(res.ManifestURL, "/channel/main/index.m3u8") {
		t.Errorf("manifest = %q, want the one manifest_pattern names", res.ManifestURL)
	}

	_, err = resolver.Resolve(context.Background(), &domain.LiveChannel{
		SourceURL: srv.URL + "/watch", ResolverConfig: map[string]any{"manifest_pattern": `(unclosed`},
	})
	if err == nil || !strings.Contains(err.Error(), "manifest_pattern") {
		t.Errorf("error = %v, want an invalid pattern reported as such", err)
	}
}

func TestBrowserTrafficCannotReachPrivateAddresses(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	var hits atomic.Int64
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `<html><body>internal admin console</body></html>`)
	}))
	defer internal.Close()

	// Allowlisted by name, as an operator could by mistake. The guard goes by the
	// address actually dialled, so the page -- and any script on it -- still cannot
	// reach a loopback service through the browser.
	guarded := live.NewFetcher("127.0.0.1", false)
	browser := browserWith(t, guarded)

	_, err := live.NewBrowserResolver(browser, guarded, 5*time.Second).
		Resolve(context.Background(), &domain.LiveChannel{SourceURL: internal.URL + "/"})
	if err == nil || !strings.Contains(err.Error(), "may not reach") {
		t.Errorf("error = %v, want it to say the page's address is off limits", err)
	}
	expectOutcome(t, err, domain.OutcomeFailed, "address_blocked")
	if n := hits.Load(); n != 0 {
		t.Errorf("the headless browser reached a loopback service %d time(s)", n)
	}
}

func TestBrowserResolverReportsPageFailures(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	hung := make(chan struct{})
	t.Cleanup(func() { close(hung) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hangs" {
			select { // never answers while the test runs
			case <-hung:
			case <-r.Context().Done():
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	resolver := live.NewBrowserResolver(browser, testFetcher(), 5*time.Second)

	_, err := resolver.Resolve(context.Background(), &domain.LiveChannel{SourceURL: srv.URL + "/gone"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("missing page error = %v, want it to report the 404", err)
	}
	// A 404 is a stale/wrong URL, told apart from a 403, a 429 or a 5xx.
	expectOutcome(t, err, domain.OutcomeFailed, "page_not_found")

	start := time.Now()
	_, err = resolver.Resolve(context.Background(), &domain.LiveChannel{SourceURL: srv.URL + "/hangs"})
	if err == nil || !strings.Contains(err.Error(), "did not respond") {
		t.Errorf("hanging page error = %v, want it to say the page never responded", err)
	}
	expectOutcome(t, err, domain.OutcomeTimeout, "page_unresponsive")
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Errorf("a page that never answers held the resolve for %s; the budget must cover navigation", elapsed)
	}
}

func TestBrowserResolverRefusesHostsOutsideTheAllowlist(t *testing.T) {
	// No browser needed: the page must never be opened at all.
	fetcher := live.NewFetcher("example.com", true, live.AllowLoopback())
	b := live.NewBrowser(live.BrowserConfig{ExecPath: "/nonexistent/chromium"}, fetcher)
	defer b.Close()

	_, err := live.NewBrowserResolver(b, fetcher, 10*time.Second).
		Resolve(context.Background(), &domain.LiveChannel{SourceURL: "https://elsewhere.test/watch"})
	if !errors.Is(err, live.ErrHostNotAllowed) {
		t.Fatalf("error = %v, want ErrHostNotAllowed", err)
	}
	var re *live.ResolveError
	if !errors.As(err, &re) || re.Outcome != domain.OutcomeFailed || re.Reason != "host_not_allowed" {
		t.Errorf("error = %#v, want RESOLUTION_FAILED/host_not_allowed", err)
	}
}

func TestBrowserResolverDistinguishesPageHTTPStatuses(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	browser := browserForTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.Error(w, "gone", http.StatusNotFound)
		case "/locked":
			http.Error(w, "no", http.StatusForbidden)
		case "/slowdown":
			http.Error(w, "later", http.StatusTooManyRequests)
		case "/broken":
			http.Error(w, "oops", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	resolver := live.NewBrowserResolver(browser, testFetcher(), 6*time.Second)

	for _, tc := range []struct {
		path            string
		outcome         domain.ResolveOutcome
		reason          string
		messageContains string
	}{
		{"/missing", domain.OutcomeFailed, "page_not_found", "stale"},
		{"/locked", domain.OutcomeFailed, "page_forbidden", "access-controlled"},
		{"/slowdown", domain.OutcomeFailed, "page_rate_limited", "rate-limiting"},
		{"/broken", domain.OutcomeFailed, "page_server_error", "server is failing"},
	} {
		_, err := resolver.Resolve(context.Background(),
			&domain.LiveChannel{SourceURL: srv.URL + tc.path, ResolverConfig: map[string]any{"timeout_seconds": float64(6)}})
		expectOutcome(t, err, tc.outcome, tc.reason)
		if err == nil || !strings.Contains(err.Error(), tc.messageContains) {
			t.Errorf("%s: error = %v, want it to mention %q", tc.path, err, tc.messageContains)
		}
	}
}
