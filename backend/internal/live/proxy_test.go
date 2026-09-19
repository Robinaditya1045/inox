package live_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

// ── in-memory fakes ─────────────────────────────────────────

type fakeRepo struct {
	channels map[string]*domain.LiveChannel
}

func (f *fakeRepo) Create(_ context.Context, ch *domain.LiveChannel) error {
	ch.ID = "chan-" + ch.Slug
	f.channels[ch.Slug] = ch
	return nil
}
func (f *fakeRepo) GetBySlug(_ context.Context, slug string) (*domain.LiveChannel, error) {
	ch, ok := f.channels[slug]
	if !ok {
		return nil, live.ErrChannelNotFound
	}
	clone := *ch // callers mutate what they get back
	return &clone, nil
}
func (f *fakeRepo) GetByID(context.Context, string) (*domain.LiveChannel, error) {
	return nil, live.ErrChannelNotFound
}
func (f *fakeRepo) List(context.Context) ([]*domain.LiveChannel, error) { return nil, nil }
func (f *fakeRepo) UpdateUpstream(_ context.Context, id, upstreamURL string, headers map[string]string, expiresAt *time.Time, isDVR bool, windowSecs int) error {
	for _, ch := range f.channels {
		if ch.ID == id {
			ch.UpstreamURL, ch.UpstreamHeaders, ch.UpstreamExpiresAt = upstreamURL, headers, expiresAt
			ch.Status = domain.LiveStatusLive
		}
	}
	return nil
}
func (f *fakeRepo) UpdateStatus(context.Context, string, domain.LiveChannelStatus, string) error {
	return nil
}
func (f *fakeRepo) Delete(context.Context, string) error { return nil }

type fakeAssets struct{ created int }

func (f *fakeAssets) CreateAsset(_ context.Context, a *domain.MediaAsset) error {
	f.created++
	a.ID = fmt.Sprintf("asset-%d", f.created)
	return nil
}
func (f *fakeAssets) DeleteAsset(context.Context, string) error { return nil }

// fakeOrigin is an upstream that behaves like a real one: it refuses any request
// that does not carry the Referer it demands, and it counts manifest hits.
type fakeOrigin struct {
	server        *httptest.Server
	manifestHits  atomic.Int64
	requiredRefer string
}

func newFakeOrigin(t *testing.T) *fakeOrigin {
	t.Helper()
	o := &fakeOrigin{requiredRefer: "https://watch.example.com/"}
	mux := http.NewServeMux()

	mux.HandleFunc("/live/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != o.requiredRefer {
			http.Error(w, "hotlink protection", http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720\n720p/index.m3u8\n")
	})
	mux.HandleFunc("/live/720p/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != o.requiredRefer {
			http.Error(w, "hotlink protection", http.StatusForbidden)
			return
		}
		o.manifestHits.Add(1)
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:1200\n"+
			"#EXTINF:6.0,\nseg1200.ts\n#EXTINF:6.0,\nseg1201.ts\n")
	})
	mux.HandleFunc("/live/720p/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != o.requiredRefer {
			http.Error(w, "hotlink protection", http.StatusForbidden)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/live/720p/")
		if !strings.HasSuffix(name, ".ts") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "video/mp2t")
		fmt.Fprintf(w, "SEGMENT-BYTES-%s", strings.TrimSuffix(name, ".ts"))
	})

	o.server = httptest.NewServer(mux)
	t.Cleanup(o.server.Close)
	return o
}

// buildProxy wires a real service and proxy around a fake origin.
func buildProxy(t *testing.T, origin *fakeOrigin) (*live.Proxy, live.Service) {
	t.Helper()
	fetcher := live.NewFetcher("", false, live.AllowLoopback())
	repo := &fakeRepo{channels: map[string]*domain.LiveChannel{}}
	svc := live.NewService(repo, &fakeAssets{}, live.NewRegistry(live.NewDirectResolver()), fetcher, "http://inox.test/api/v1/live")

	_, err := svc.Create(context.Background(), live.CreateRequest{
		Title:     "Test Channel",
		Slug:      "test-channel",
		Resolver:  domain.ResolverDirect,
		SourceURL: origin.server.URL + "/live/master.m3u8",
		ResolverConfig: map[string]any{
			"headers": map[string]any{"Referer": origin.requiredRefer},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	sealer, err := live.NewSealer(testSecret)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return live.NewProxy(svc, fetcher, sealer, live.NewTokenSigner(testSecret), "http://inox.test/api/v1/live"), svc
}

// followRewrittenURI pulls one rewritten URI out of a manifest and splits it into
// the path segment, sealed ref, and token the proxy routes on.
func followRewrittenURI(t *testing.T, body, wantKind string) (ref, token string) {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "http://inox.test/api/v1/live/test-channel/"+wantKind+"/") {
			continue
		}
		rest := strings.TrimPrefix(line, "http://inox.test/api/v1/live/test-channel/"+wantKind+"/")
		ref, token, _ = strings.Cut(rest, "?t=")
		return strings.TrimSuffix(ref, ".m3u8"), token
	}
	t.Fatalf("no %q URI found in manifest:\n%s", wantKind, body)
	return "", ""
}

// ── the end-to-end path ─────────────────────────────────────

func TestProxyServesFullChainFromMasterToSegment(t *testing.T) {
	origin := newFakeOrigin(t)
	proxy, _ := buildProxy(t, origin)

	// 1. Master playlist, authorized by session.
	rec := httptest.NewRecorder()
	proxy.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/api/v1/live/test-channel/master.m3u8", nil), "test-channel", "user-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("master status = %d, body: %s", rec.Code, rec.Body.String())
	}
	master := rec.Body.String()

	// The upstream origin must not appear anywhere in what the client receives.
	if strings.Contains(master, origin.server.URL) {
		t.Errorf("master playlist leaks the upstream origin:\n%s", master)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.apple.mpegurl" {
		t.Errorf("content-type = %q, want the HLS type", ct)
	}

	// 2. Variant playlist, authorized by the token minted above.
	variantRef, token := followRewrittenURI(t, master, "p")
	rec = httptest.NewRecorder()
	proxy.ServePlaylist(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", variantRef, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("variant status = %d, body: %s", rec.Code, rec.Body.String())
	}
	media := rec.Body.String()
	if strings.Contains(media, origin.server.URL) {
		t.Errorf("media playlist leaks the upstream origin:\n%s", media)
	}
	if !strings.Contains(media, "#EXT-X-MEDIA-SEQUENCE:1200") {
		t.Errorf("media playlist lost its sequence header:\n%s", media)
	}

	// 3. A segment, streamed through.
	segRef, segToken := followRewrittenURI(t, media, "r")
	rec = httptest.NewRecorder()
	proxy.ServeResource(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", segRef, segToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("segment status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SEGMENT-BYTES-") {
		t.Errorf("segment body = %q, want the upstream bytes", rec.Body.String())
	}

	// 4. The edge sequence the hub clamps against is now known.
	edge, ok := proxy.EdgeSequenceForURL("http://inox.test/api/v1/live/test-channel/master.m3u8")
	if !ok {
		t.Fatal("EdgeSequenceForURL did not recognize the channel's own master url")
	}
	if edge != 1201 {
		t.Errorf("edge = %d, want 1201 (last segment in the window)", edge)
	}
}

func TestProxyCollapsesConcurrentManifestRequests(t *testing.T) {
	origin := newFakeOrigin(t)
	proxy, _ := buildProxy(t, origin)

	rec := httptest.NewRecorder()
	proxy.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	variantRef, token := followRewrittenURI(t, rec.Body.String(), "p")

	// Twenty viewers polling the same media playlist is the normal steady state of a
	// room. Without the cache and singleflight this is twenty upstream requests
	// every couple of seconds, which is how you get rate-limited by a provider.
	for i := 0; i < 20; i++ {
		r := httptest.NewRecorder()
		proxy.ServePlaylist(r, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", variantRef, token)
		if r.Code != http.StatusOK {
			t.Fatalf("request %d failed: %d", i, r.Code)
		}
	}

	if hits := origin.manifestHits.Load(); hits != 1 {
		t.Errorf("upstream manifest hits = %d, want 1: viewers must share one fetch", hits)
	}
}

func TestProxyRejectsForgedTokensAndRefs(t *testing.T) {
	origin := newFakeOrigin(t)
	proxy, _ := buildProxy(t, origin)

	rec := httptest.NewRecorder()
	proxy.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	ref, token := followRewrittenURI(t, rec.Body.String(), "p")

	t.Run("forged token", func(t *testing.T) {
		r := httptest.NewRecorder()
		proxy.ServePlaylist(r, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", ref, "forged.token")
		if r.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", r.Code)
		}
	})

	t.Run("attacker-supplied ref", func(t *testing.T) {
		// The guard that stops this being an open relay: a ref we did not seal
		// cannot be opened, so the proxy will not fetch an arbitrary URL.
		r := httptest.NewRecorder()
		proxy.ServeResource(r, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "aHR0cDovL2V2aWwuZXhhbXBsZS5jb20v", token)
		if r.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", r.Code)
		}
	})

	t.Run("unknown channel", func(t *testing.T) {
		r := httptest.NewRecorder()
		proxy.ServeMaster(r, httptest.NewRequest(http.MethodGet, "/", nil), "no-such-channel", "user-1")
		if r.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", r.Code)
		}
	})
}

func TestProxyIsLiveURLDistinguishesOurOwnChannels(t *testing.T) {
	proxy, _ := buildProxy(t, newFakeOrigin(t))

	if !proxy.IsLiveURL("http://inox.test/api/v1/live/test-channel/master.m3u8") {
		t.Error("a live master url was not recognized")
	}
	// The hub uses this to pick a sync coordinate, so a VOD asset must never match.
	if proxy.IsLiveURL("http://localhost:8080/media/stream/hls/abc/master.m3u8") {
		t.Error("a transcoded VOD asset was misclassified as live")
	}
	if proxy.IsLiveURL("https://media.w3.org/2010/05/bunny/movie.mp4") {
		t.Error("a plain mp4 was misclassified as live")
	}
}
