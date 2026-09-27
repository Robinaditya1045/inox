package live_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

func hasCookie(r *http.Request, name, value string) bool {
	c, err := r.Cookie(name)
	return err == nil && c.Value == value
}

func TestFetcherKeepsAnUpstreamSessionCookie(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			leaked.Store(true)
		}
	}))
	defer other.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			http.SetCookie(w, &http.Cookie{Name: "edge", Value: "token-1", Path: "/"})
			fmt.Fprint(w, "#EXTM3U\n")
		case "/v.m3u8":
			if !hasCookie(r, "edge", "token-1") {
				http.Error(w, "no session", http.StatusForbidden)
				return
			}
			fmt.Fprint(w, "#EXTM3U\n")
		}
	}))
	defer origin.Close()

	jar, _ := cookiejar.New(nil)
	session := testFetcher().WithCookies(jar)
	ctx := context.Background()
	for _, path := range []string{"/master.m3u8", "/v.m3u8"} {
		if _, err := session.GetBytes(ctx, origin.URL+path, nil, 1024); err != nil {
			t.Fatalf("%s with the session: %v", path, err)
		}
	}
	if _, err := testFetcher().GetBytes(ctx, origin.URL+"/v.m3u8", nil, 1024); err == nil {
		t.Error("a fetcher without the session was served; the test origin is not checking")
	}
	// A cookie belongs to the host that set it, not to every host a stream touches.
	elsewhere := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)
	if _, err := session.GetBytes(ctx, elsewhere+"/seg.ts", nil, 1024); err != nil {
		t.Fatalf("other host: %v", err)
	}
	if leaked.Load() {
		t.Error("the session cookie was sent to a host that did not set it")
	}
}

func TestProxyKeepsTheSessionCookieAnUpstreamHandsOut(t *testing.T) {
	// A CDN that sets a cookie on the master playlist and wants it back on
	// everything after it, as tokenising CDNs do.
	origin := newScriptedOrigin(t, func(path string, _ int, w http.ResponseWriter, r *http.Request) bool {
		if path == "/master.m3u8" {
			http.SetCookie(w, &http.Cookie{Name: "hdntl", Value: "exp=1~hmac=ok", Path: "/"})
			return false
		}
		if !hasCookie(r, "hdntl", "exp=1~hmac=ok") {
			http.Error(w, "no session", http.StatusForbidden)
			return true
		}
		return false
	})
	proxy, _ := proxyFor(t, origin)

	_, media := mediaPlaylist(t, proxy) // fails the test itself if the variant is refused
	segments := rewrittenURIs(media, "r")
	if len(segments) == 0 {
		t.Fatalf("no segments in %q", media)
	}
	rec := httptest.NewRecorder()
	proxy.ServeResource(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", segments[0].ref, segments[0].token)
	if rec.Code != http.StatusOK {
		t.Errorf("segment: %d %q, want it served with the session cookie", rec.Code, rec.Body.String())
	}
}

// sessionResolver stands in for a resolver that found its stream inside a session,
// as the browser resolver does, and hands the session's cookies over with it.
type sessionResolver struct {
	manifest string
	resolves atomic.Int64
}

func (s *sessionResolver) ID() string { return "session-test" }

func (s *sessionResolver) Resolve(context.Context, *domain.LiveChannel) (*live.Resolution, error) {
	s.resolves.Add(1)
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(s.manifest)
	jar.SetCookies(u, []*http.Cookie{{Name: "viewer", Value: "issued-to-the-page", Path: "/"}})
	return &live.Resolution{ManifestURL: s.manifest, Protocol: "hls", Cookies: jar}, nil
}

func TestProxySendsTheCookiesTheResolverFoundTheStreamWith(t *testing.T) {
	origin := newScriptedOrigin(t, func(_ string, _ int, w http.ResponseWriter, r *http.Request) bool {
		if !hasCookie(r, "viewer", "issued-to-the-page") {
			http.Error(w, "no session", http.StatusForbidden)
			return true
		}
		return false
	})
	resolver := &sessionResolver{manifest: origin.server.URL + "/master.m3u8"}
	repo := &fakeRepo{channels: map[string]*domain.LiveChannel{}}
	// Each call is a fresh process: the same database, nothing in memory.
	start := func() (live.Service, *live.Proxy) {
		svc := live.NewService(repo, &fakeAssets{}, live.NewRegistry(resolver), testFetcher(), "http://inox.test/api/v1/live")
		sealer, err := live.NewSealer(testSecret)
		if err != nil {
			t.Fatalf("NewSealer: %v", err)
		}
		return svc, live.NewProxy(svc, testFetcher(), sealer, live.NewTokenSigner(testSecret), "http://inox.test/api/v1/live")
	}
	svc, proxy := start()
	if _, err := svc.Create(context.Background(), live.CreateRequest{
		Title: "Session", Slug: "test-channel", Resolver: "session-test", SourceURL: "https://page.example/watch",
	}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, media := mediaPlaylist(t, proxy)
	segments := rewrittenURIs(media, "r")
	rec := httptest.NewRecorder()
	proxy.ServeResource(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", segments[0].ref, segments[0].token)
	if rec.Code != http.StatusOK {
		t.Fatalf("segment: %d, want it served with the page session's cookie", rec.Code)
	}

	// The cookies live in memory only. After a restart the stored upstream URL is
	// refused without them, and that one refusal is what gets them back.
	_, restarted := start()
	rec = httptest.NewRecorder()
	restarted.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("master after a restart: %d %q, want a re-resolve to recover the session", rec.Code, rec.Body.String())
	}
	if n := resolver.resolves.Load(); n != 2 {
		t.Errorf("resolved %d times, want once more after the restart", n)
	}
}

func TestOperatorCookieIsScopedToTheUpstreamHostNotBroadcast(t *testing.T) {
	// A stream whose segments live on a second host. The operator sets a Cookie for
	// the manifest host (a source they control); it must not be sent to the other.
	var leaked atomic.Bool
	segHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			leaked.Store(true)
		}
		fmt.Fprint(w, "SEGMENT")
	}))
	defer segHost.Close()
	segBase := strings.Replace(segHost.URL, "127.0.0.1", "localhost", 1) // a different host

	var gotManifestCookie atomic.Bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/live/master.m3u8":
			if c, err := r.Cookie("op"); err == nil && c.Value == "1" {
				gotManifestCookie.Store(true)
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:1\n#EXTINF:6.0,\n%s/seg1.ts\n", segBase)
		}
	}))
	defer origin.Close()

	repo := &fakeRepo{channels: map[string]*domain.LiveChannel{}}
	svc := live.NewService(repo, &fakeAssets{}, live.NewRegistry(live.NewDirectResolver()), testFetcher(), "http://inox.test/api/v1/live")
	if _, err := svc.Create(context.Background(), live.CreateRequest{
		Title: "Op", Slug: "test-channel", Resolver: domain.ResolverDirect, SourceURL: origin.URL + "/live/master.m3u8",
		ResolverConfig: map[string]any{"headers": map[string]any{"Cookie": "op=1"}},
	}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sealer, err := live.NewSealer(testSecret)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	proxy := live.NewProxy(svc, testFetcher(), sealer, live.NewTokenSigner(testSecret), "http://inox.test/api/v1/live")

	rec0 := httptest.NewRecorder()
	proxy.ServeMaster(rec0, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	if rec0.Code != http.StatusOK {
		t.Fatalf("master: %d %q", rec0.Code, rec0.Body.String())
	}
	if !gotManifestCookie.Load() {
		t.Error("the operator cookie never reached the manifest host it was set for")
	}
	segments := rewrittenURIs(rec0.Body.String(), "r")
	if len(segments) != 1 {
		t.Fatalf("segments = %v", segments)
	}
	rec := httptest.NewRecorder()
	proxy.ServeResource(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", segments[0].ref, segments[0].token)
	if rec.Code != http.StatusOK {
		t.Fatalf("segment on the other host: %d", rec.Code)
	}
	if leaked.Load() {
		t.Error("the operator cookie was broadcast to a host it was not set for")
	}
}
