package live_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

// scriptedResolver answers each resolve with whatever its script says, counting
// how often it was asked.
type scriptedResolver struct {
	calls  atomic.Int64
	script func(ctx context.Context, call int64) (*live.Resolution, error)
}

func (s *scriptedResolver) ID() string { return "scripted" }

func (s *scriptedResolver) Resolve(ctx context.Context, _ *domain.LiveChannel) (*live.Resolution, error) {
	return s.script(ctx, s.calls.Add(1))
}

func newScriptedChannel(t *testing.T, r *scriptedResolver) (live.Service, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{channels: map[string]*domain.LiveChannel{}}
	svc := live.NewService(repo, &fakeAssets{}, live.NewRegistry(r), testFetcher(), "http://inox.test/api/v1/live")
	if _, err := svc.Create(context.Background(), live.CreateRequest{
		Title: "Scripted", Slug: "test-channel", Resolver: r.ID(), SourceURL: "https://page.example.com/watch",
	}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return svc, repo
}

func (f *fakeRepo) stored(t *testing.T, slug string) domain.LiveChannel {
	t.Helper()
	ch, err := f.GetBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	return *ch
}

func TestFailedResolveIsNotRetriedStraightAway(t *testing.T) {
	r := &scriptedResolver{script: func(context.Context, int64) (*live.Resolution, error) {
		return nil, errors.New("source is down")
	}}
	svc, repo := newScriptedChannel(t, r)

	// Every viewer of a dead channel keeps retrying. With a resolve costing seconds
	// of browser time, each retry must not become another resolve.
	for i := 0; i < 5; i++ {
		ch := repo.stored(t, "test-channel")
		if err := svc.EnsureResolved(context.Background(), &ch); err == nil || !strings.Contains(err.Error(), "source is down") {
			t.Fatalf("attempt %d: error = %v, want the resolve failure", i, err)
		}
	}
	if n := r.calls.Load(); n != 1 {
		t.Errorf("resolver called %d times, want 1 inside the retry backoff", n)
	}
	if ch := repo.stored(t, "test-channel"); ch.Status != domain.LiveStatusError || !strings.Contains(ch.LastError, "source is down") {
		t.Errorf("stored status = %q %q, want the failure recorded for the admin list", ch.Status, ch.LastError)
	}
}

func TestResolveOutlivesTheRequestThatStartedIt(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	r := &scriptedResolver{script: func(ctx context.Context, _ int64) (*live.Resolution, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
			return &live.Resolution{ManifestURL: "https://cdn.example.com/live.m3u8", Protocol: "hls"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc, repo := newScriptedChannel(t, r)

	// hls.js gives up on the master request long before a slow resolve finishes.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		ch := repo.stored(t, "test-channel")
		done <- svc.EnsureResolved(ctx, &ch)
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("abandoned request error = %v, want context.Canceled", err)
	}

	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for repo.stored(t, "test-channel").UpstreamURL == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	ch := repo.stored(t, "test-channel")
	if ch.UpstreamURL != "https://cdn.example.com/live.m3u8" {
		t.Fatalf("stored upstream = %q, want the resolve to have finished and persisted without its caller", ch.UpstreamURL)
	}
	// The retry that follows is served from what was stored.
	if err := svc.EnsureResolved(context.Background(), &ch); err != nil {
		t.Fatalf("EnsureResolved after the resolve landed: %v", err)
	}
	if n := r.calls.Load(); n != 1 {
		t.Errorf("resolver called %d times, want 1", n)
	}
}

// rotatingOrigin serves one manifest URL per generation; each can be made to refuse
// requests or to fail, the way an expired signature and a sick upstream do.
type rotatingOrigin struct {
	server *httptest.Server
	mu     sync.Mutex
	status map[string]int
}

func newRotatingOrigin(t *testing.T) *rotatingOrigin {
	t.Helper()
	o := &rotatingOrigin{status: map[string]int{}}
	o.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		status := o.status[r.URL.Path]
		o.mu.Unlock()
		if status != 0 {
			http.Error(w, "no", status)
			return
		}
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:7\n#EXTINF:6.0,\nseg-%s.ts\n", strings.Trim(r.URL.Path, "/"))
	}))
	t.Cleanup(o.server.Close)
	return o
}

func (o *rotatingOrigin) set(path string, status int) {
	o.mu.Lock()
	o.status[path] = status
	o.mu.Unlock()
}

func TestProxyReResolvesWhenTheUpstreamRefusesItsManifest(t *testing.T) {
	origin := newRotatingOrigin(t)
	r := &scriptedResolver{script: func(context.Context, int64) (*live.Resolution, error) {
		return &live.Resolution{ManifestURL: origin.server.URL + "/v2", Protocol: "hls"}, nil
	}}
	svc, repo := newScriptedChannel(t, r)
	sealer, err := live.NewSealer(testSecret)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	proxy := live.NewProxy(svc, testFetcher(), sealer, live.NewTokenSigner(testSecret), "http://inox.test/api/v1/live")

	// An upstream stored earlier -- by this process before a restart, or another
	// one -- whose signature has since expired without having said when.
	repo.mu.Lock()
	repo.channels["test-channel"].UpstreamURL = origin.server.URL + "/v1"
	repo.mu.Unlock()
	origin.set("/v1", http.StatusForbidden)

	rec := httptest.NewRecorder()
	proxy.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want the channel recovered by a re-resolve", rec.Code, rec.Body.String())
	}
	if n := r.calls.Load(); n != 1 {
		t.Errorf("resolver called %d times, want exactly 1", n)
	}
	if got := repo.stored(t, "test-channel").UpstreamURL; got != origin.server.URL+"/v2" {
		t.Errorf("stored upstream = %q, want the fresh one", got)
	}

	// A source whose fresh URLs are refused too must not be re-resolved on every
	// master request.
	ch := repo.stored(t, "test-channel")
	if err := svc.Refresh(context.Background(), &ch); err == nil {
		t.Error("a second refresh right away was allowed; it must wait out the refresh interval")
	}
	if n := r.calls.Load(); n != 1 {
		t.Errorf("resolver called %d times after a throttled refresh, want still 1", n)
	}
}

func TestProxyDoesNotReResolveATransientUpstreamFailure(t *testing.T) {
	origin := newRotatingOrigin(t)
	r := &scriptedResolver{script: func(context.Context, int64) (*live.Resolution, error) {
		return &live.Resolution{ManifestURL: origin.server.URL + "/v2", Protocol: "hls"}, nil
	}}
	svc, repo := newScriptedChannel(t, r)
	sealer, _ := live.NewSealer(testSecret)
	proxy := live.NewProxy(svc, testFetcher(), sealer, live.NewTokenSigner(testSecret), "http://inox.test/api/v1/live")

	repo.mu.Lock()
	repo.channels["test-channel"].UpstreamURL = origin.server.URL + "/v1"
	repo.mu.Unlock()
	// A 503 says the upstream is unwell, not that the URL went bad.
	origin.set("/v1", http.StatusServiceUnavailable)

	rec := httptest.NewRecorder()
	proxy.ServeMaster(rec, httptest.NewRequest(http.MethodGet, "/", nil), "test-channel", "user-1")
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 while the upstream is down", rec.Code)
	}
	if n := r.calls.Load(); n != 0 {
		t.Errorf("resolver called %d times, want 0: a 5xx is no reason to re-resolve", n)
	}
}

func TestTestResolveNamesUpstreamHeadersWithoutTheirValues(t *testing.T) {
	origin := newFakeOrigin(t)
	svc := live.NewService(&fakeRepo{channels: map[string]*domain.LiveChannel{}}, &fakeAssets{},
		live.NewRegistry(live.NewDirectResolver()), testFetcher(), "http://inox.test/api/v1/live")

	res := svc.TestResolve(context.Background(), live.CreateRequest{
		Resolver:       domain.ResolverDirect,
		SourceURL:      origin.server.URL + "/live/master.m3u8",
		ResolverConfig: map[string]any{"headers": map[string]any{"referer": origin.requiredRefer}},
	})
	if !res.OK {
		t.Fatalf("TestResolve failed: %s", res.Error)
	}
	if len(res.UpstreamHeaders) != 1 || res.UpstreamHeaders[0] != "Referer" {
		t.Errorf("upstream headers = %v, want [Referer]", res.UpstreamHeaders)
	}
	encoded, _ := json.Marshal(res)
	if strings.Contains(string(encoded), origin.requiredRefer) {
		t.Errorf("test result leaks a header value: %s", encoded)
	}
	if res.Outcome != domain.OutcomeStreamFound || res.Diagnostics == nil || res.Diagnostics.Resolver != domain.ResolverDirect {
		t.Errorf("outcome = %s, diagnostics = %+v, want STREAM_FOUND from the direct resolver", res.Outcome, res.Diagnostics)
	}
}

// The direct resolver never looks inside a manifest, so the test has to: an operator
// pasting a DRM-protected manifest URL must hear so before creating the channel.
func TestTestResolveReportsDRMWhicheverResolverFoundTheStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv.m3u8\n")
		case "/v.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n"+
				`#EXT-X-KEY:METHOD=SAMPLE-AES,URI="skd://key",KEYFORMAT="com.apple.streamingkeydelivery"`+"\n#EXTINF:6,\na.ts\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	svc := live.NewService(&fakeRepo{channels: map[string]*domain.LiveChannel{}}, &fakeAssets{},
		live.NewRegistry(live.NewDirectResolver()), testFetcher(), "http://inox.test/api/v1/live")

	res := svc.TestResolve(context.Background(), live.CreateRequest{Resolver: domain.ResolverDirect, SourceURL: srv.URL + "/master.m3u8"})
	if res.OK || res.Outcome != domain.OutcomeDRMProtected || res.Reason != "manifest_protected" {
		t.Fatalf("result = %+v, want a failed DRM_PROTECTED test", res)
	}
	if res.Diagnostics == nil || res.Diagnostics.DRM == nil || strings.Join(res.Diagnostics.DRM.Systems, ",") != "FairPlay" ||
		strings.Join(res.Diagnostics.DRM.Schemes, ",") != "cbcs" {
		t.Errorf("diagnostics = %+v, want FairPlay over cbcs", res.Diagnostics)
	}
}

func TestResolveOutcomeIsKeptOnTheChannel(t *testing.T) {
	calls := 0
	r := &scriptedResolver{script: func(context.Context, int64) (*live.Resolution, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("wrapped: %w", &live.ResolveError{
				Outcome: domain.OutcomeDRMProtected, Reason: "eme_session", Message: "stream is DRM-protected",
				Diagnostics: &domain.ResolveDiagnostics{DRM: &domain.DRMDiagnostics{Confirmed: true, Systems: []string{"Widevine"}}},
			})
		}
		return &live.Resolution{ManifestURL: "https://cdn.example.com/live.m3u8", Protocol: "hls"}, nil
	}}
	svc, repo := newScriptedChannel(t, r)

	ch := repo.stored(t, "test-channel")
	if err := svc.EnsureResolved(context.Background(), &ch); err == nil {
		t.Fatal("a DRM-protected source resolved")
	}
	stored := repo.stored(t, "test-channel")
	if stored.LastOutcome != domain.OutcomeDRMProtected || stored.LastDiagnostics == nil ||
		stored.LastDiagnostics.Reason != "eme_session" || stored.LastDiagnostics.Resolver != "scripted" ||
		stored.LastDiagnostics.DRM == nil || stored.LastDiagnostics.DRM.Systems[0] != "Widevine" {
		t.Fatalf("stored outcome = %s, diagnostics = %+v", stored.LastOutcome, stored.LastDiagnostics)
	}
	if stored.Status != domain.LiveStatusError {
		t.Errorf("status = %s, want a DRM-protected channel never shown as live", stored.Status)
	}

	// The failure is remembered: DRM does not lapse between retries.
	if err := svc.EnsureResolved(context.Background(), &ch); err == nil || calls != 1 {
		t.Errorf("retry err = %v after %d resolver calls, want the remembered failure", err, calls)
	}
}
