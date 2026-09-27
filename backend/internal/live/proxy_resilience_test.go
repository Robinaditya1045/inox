package live_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

// scriptedOrigin is a live origin whose answer to each request a test decides, by
// path and by how many times that path has been asked for.
type scriptedOrigin struct {
	server *httptest.Server
	mu     sync.Mutex
	hits   map[string]int
	answer func(path string, n int, w http.ResponseWriter, r *http.Request) bool
}

func newScriptedOrigin(t *testing.T, answer func(path string, n int, w http.ResponseWriter, r *http.Request) bool) *scriptedOrigin {
	t.Helper()
	o := &scriptedOrigin{hits: map[string]int{}, answer: answer}
	o.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		o.hits[r.URL.Path]++
		n := o.hits[r.URL.Path]
		o.mu.Unlock()
		if o.answer != nil && o.answer(r.URL.Path, n, w, r) {
			return
		}
		switch {
		case r.URL.Path == "/master.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv.m3u8\n")
		case r.URL.Path == "/v.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:1\n"+
				"#EXTINF:2.0,\nseg1.ts\n#EXTINF:2.0,\nseg2.ts\n#EXTINF:2.0,\nseg3.ts\n")
		case strings.HasSuffix(r.URL.Path, ".ts"):
			fmt.Fprint(w, "BYTES"+r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(o.server.Close)
	return o
}

func (o *scriptedOrigin) count(path string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hits[path]
}

// healthLog records the upstream health notices a proxy sends to rooms.
type healthLog struct {
	mu      sync.Mutex
	notices []string
}

func (h *healthLog) record(_, status, _ string) {
	h.mu.Lock()
	h.notices = append(h.notices, status)
	h.mu.Unlock()
}

func (h *healthLog) all() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.notices...)
}

func proxyFor(t *testing.T, origin *scriptedOrigin) (*live.Proxy, *healthLog) {
	t.Helper()
	svc := live.NewService(&fakeRepo{channels: map[string]*domain.LiveChannel{}}, &fakeAssets{},
		live.NewRegistry(live.NewDirectResolver()), testFetcher(), "http://inox.test/api/v1/live")
	if _, err := svc.Create(context.Background(), live.CreateRequest{
		Title: "Resilience", Slug: "test-channel", Resolver: domain.ResolverDirect, SourceURL: origin.server.URL + "/master.m3u8",
	}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sealer, err := live.NewSealer(testSecret)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	proxy := live.NewProxy(svc, testFetcher(), sealer, live.NewTokenSigner(testSecret), "http://inox.test/api/v1/live")
	health := &healthLog{}
	proxy.SetStatusListener(health.record)
	return proxy, health
}

type rewritten struct{ ref, token string }

// rewrittenURIs pulls every URI of one kind out of a rewritten playlist, in order.
func rewrittenURIs(body, kind string) []rewritten {
	prefix := "http://inox.test/api/v1/live/test-channel/" + kind + "/"
	var out []rewritten
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			ref, token, _ := strings.Cut(rest, "?t=")
			out = append(out, rewritten{strings.TrimSuffix(ref, ".m3u8"), token})
		}
	}
	return out
}

// mediaPlaylist walks master -> variant and returns the variant's ref and the
// rewritten media playlist.
func mediaPlaylist(t *testing.T, proxy *live.Proxy) (rewritten, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	proxy.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	variants := rewrittenURIs(rec.Body.String(), "p")
	if rec.Code != http.StatusOK || len(variants) != 1 {
		t.Fatalf("master: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	proxy.ServePlaylist(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", variants[0].ref, variants[0].token)
	if rec.Code != http.StatusOK {
		t.Fatalf("media playlist: %d %q", rec.Code, rec.Body.String())
	}
	return variants[0], rec.Body.String()
}

func TestProxyTellsThePlayerWhichSegmentsAreGoneAndHidesBlips(t *testing.T) {
	origin := newScriptedOrigin(t, func(path string, n int, w http.ResponseWriter, _ *http.Request) bool {
		switch {
		case path == "/seg1.ts": // aged out of the window, or its edge never had it
			http.NotFound(w, nil)
		case path == "/seg2.ts" && n == 1: // an edge that hiccups once
			http.Error(w, "busy", http.StatusServiceUnavailable)
		case path == "/seg3.ts": // an edge that is down
			http.Error(w, "busy", http.StatusServiceUnavailable)
		default:
			return false
		}
		return true
	})
	proxy, _ := proxyFor(t, origin)
	_, media := mediaPlaylist(t, proxy)
	segments := rewrittenURIs(media, "r")
	if len(segments) != 3 {
		t.Fatalf("segments = %v", segments)
	}
	serve := func(s rewritten) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		proxy.ServeResource(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", s.ref, s.token)
		return rec
	}

	// hls.js skips a live segment that answers 404 at once, but retries a 502 for
	// half a minute while the picture stalls.
	if rec := serve(segments[0]); rec.Code != http.StatusNotFound {
		t.Errorf("missing segment: status %d, want 404", rec.Code)
	}
	if n := origin.count("/seg1.ts"); n != 1 {
		t.Errorf("missing segment fetched %d times; a 404 is not worth repeating", n)
	}

	if rec := serve(segments[1]); rec.Code != http.StatusOK || rec.Body.String() != "BYTES/seg2.ts" {
		t.Errorf("segment after one upstream failure: %d %q, want it served", rec.Code, rec.Body.String())
	}

	if rec := serve(segments[2]); rec.Code != http.StatusBadGateway {
		t.Errorf("segment on a failed edge: status %d, want 502 so the player may retry", rec.Code)
	}
	if n := origin.count("/seg3.ts"); n != 2 {
		t.Errorf("failed edge asked %d times, want exactly one retry", n)
	}
}

func TestProxyRidesOutAFailedPlaylistRefresh(t *testing.T) {
	var mu sync.Mutex
	refresh := 0 // what /v.m3u8 answers from now on: 0 serve, else this status
	origin := newScriptedOrigin(t, func(path string, _ int, w http.ResponseWriter, _ *http.Request) bool {
		mu.Lock()
		status := refresh
		mu.Unlock()
		if path != "/v.m3u8" || status == 0 {
			return false
		}
		http.Error(w, "no", status)
		return true
	})
	proxy, health := proxyFor(t, origin)
	variant, first := mediaPlaylist(t, proxy)
	playlist := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		proxy.ServePlaylist(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", variant.ref, variant.token)
		return rec
	}

	// The upstream playlist host is briefly unwell. The copy fetched a moment ago is
	// a few seconds behind at worst; an error would cost the player a retry cycle.
	mu.Lock()
	refresh = http.StatusServiceUnavailable
	mu.Unlock()
	time.Sleep(1100 * time.Millisecond) // past the one-second cache
	before := origin.count("/v.m3u8")
	if rec := playlist(); rec.Code != http.StatusOK || rec.Body.String() != first {
		t.Fatalf("refresh during an upstream blip: %d, want the last good playlist", rec.Code)
	}
	if n := origin.count("/v.m3u8") - before; n != 2 {
		t.Errorf("the failed refresh was attempted %d times, want one retry", n)
	}
	if notices := health.all(); len(notices) != 0 {
		t.Errorf("rooms were told %v about an outage viewers never saw", notices)
	}

	// An upstream refusing the URL is another matter: that needs a re-resolve, so
	// it must not be papered over.
	mu.Lock()
	refresh = http.StatusForbidden
	mu.Unlock()
	if rec := playlist(); rec.Code != http.StatusBadGateway {
		t.Errorf("refused playlist: status %d, want 502 rather than a stale copy", rec.Code)
	}
	if notices := health.all(); len(notices) != 1 || notices[0] != "degraded" {
		t.Errorf("health notices = %v, want the refusal announced", notices)
	}
}

// Every viewer polling a playlist waits on one upstream fetch. A player abandons its
// request whenever it switches quality; that must neither fail the fetch for the
// others nor be announced to the room as the broadcast failing.
func TestProxyPlaylistFetchOutlivesTheViewerWhoStartedIt(t *testing.T) {
	arrived, release := make(chan struct{}), make(chan struct{})
	origin := newScriptedOrigin(t, func(path string, n int, _ http.ResponseWriter, r *http.Request) bool {
		if path == "/v.m3u8" && n == 2 { // the first fetch after the warm-up below
			close(arrived)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
		return false
	})
	proxy, health := proxyFor(t, origin)
	variant, _ := mediaPlaylist(t, proxy) // warms the master; the variant fetch is hit 1
	time.Sleep(1100 * time.Millisecond)   // let that copy expire

	request := func(ctx context.Context) (*httptest.ResponseRecorder, chan struct{}) {
		rec, done := httptest.NewRecorder(), make(chan struct{})
		go func() {
			defer close(done)
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			proxy.ServePlaylist(rec, req, "test-channel", variant.ref, variant.token)
		}()
		return rec, done
	}

	abandonCtx, abandon := context.WithCancel(context.Background())
	abandoned, abandonedDone := request(abandonCtx)
	<-arrived
	waiting, waitingDone := request(context.Background())
	time.Sleep(50 * time.Millisecond) // let the second viewer join the fetch
	abandon()
	<-abandonedDone
	close(release)
	<-waitingDone

	if waiting.Code != http.StatusOK || len(rewrittenURIs(waiting.Body.String(), "r")) != 3 {
		t.Fatalf("viewer who stayed: %d %q, want the playlist", waiting.Code, waiting.Body.String())
	}
	if n := origin.count("/v.m3u8"); n != 2 {
		t.Errorf("playlist fetched %d times, want the abandoned fetch to have carried on for the other viewer", n)
	}
	if abandoned.Body.Len() != 0 {
		t.Errorf("wrote %q to a viewer who had left", abandoned.Body.String())
	}
	if notices := health.all(); len(notices) != 0 {
		t.Errorf("rooms were told %v because one viewer changed quality", notices)
	}
}
