package live_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

func testFetcher() *live.Fetcher {
	return live.NewFetcher("", false, live.AllowLoopback())
}

func TestDirectResolverPassesThroughAndAppliesHeaders(t *testing.T) {
	r := live.NewDirectResolver()
	ch := &domain.LiveChannel{
		SourceURL: "https://cdn.example.com/live/master.m3u8",
		ResolverConfig: map[string]any{
			"headers":     map[string]any{"Referer": "https://example.com/watch"},
			"ttl_seconds": float64(3600), // JSONB numbers decode as float64
		},
	}

	res, err := r.Resolve(context.Background(), ch)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.ManifestURL != ch.SourceURL {
		t.Errorf("manifest = %q, want the source url unchanged", res.ManifestURL)
	}
	if res.Protocol != "hls" {
		t.Errorf("protocol = %q, want hls inferred from the extension", res.Protocol)
	}
	// Referer is a forbidden header in the browser, which is why it has to ride on
	// the server-side fetch instead.
	if res.Headers["Referer"] != "https://example.com/watch" {
		t.Errorf("headers = %v, want the configured Referer", res.Headers)
	}
	if res.ExpiresAt.IsZero() {
		t.Error("a configured ttl_seconds should produce an expiry")
	}
}

func TestDirectResolverInfersDashProtocol(t *testing.T) {
	res, err := live.NewDirectResolver().Resolve(context.Background(),
		&domain.LiveChannel{SourceURL: "https://cdn.example.com/live/manifest.mpd"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Protocol != "dash" {
		t.Errorf("protocol = %q, want dash", res.Protocol)
	}
}

func TestAPIResolverExtractsManifestByJSONPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Errorf("Authorization = %q, want the configured api header", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"streams":[{"url":"https://cdn.example.com/a.m3u8"}]}}`))
	}))
	defer srv.Close()

	res, err := live.NewAPIResolver(testFetcher()).Resolve(context.Background(), &domain.LiveChannel{
		SourceURL: srv.URL,
		ResolverConfig: map[string]any{
			"json_path":   "data.streams.0.url",
			"api_headers": map[string]any{"Authorization": "Bearer secret-key"},
		},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.ManifestURL != "https://cdn.example.com/a.m3u8" {
		t.Errorf("manifest = %q, want the url at data.streams.0.url", res.ManifestURL)
	}
}

func TestAPIResolverReportsUsableErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	f := testFetcher()

	// A missing json_path is a configuration mistake, and the message has to say so:
	// the operator is the only one who can fix it.
	_, err := live.NewAPIResolver(f).Resolve(context.Background(),
		&domain.LiveChannel{SourceURL: srv.URL, ResolverConfig: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), "json_path") {
		t.Errorf("error = %v, want it to name the missing json_path setting", err)
	}

	_, err = live.NewAPIResolver(f).Resolve(context.Background(),
		&domain.LiveChannel{SourceURL: srv.URL, ResolverConfig: map[string]any{"json_path": "data.missing.url"}})
	if err == nil || !strings.Contains(err.Error(), "not present") {
		t.Errorf("error = %v, want it to report the path was absent", err)
	}
}

func TestStaticResolverFindsManifestInPageMarkup(t *testing.T) {
	cases := []struct {
		name string
		page string
		want string
	}{
		{
			name: "absolute url in an inline script",
			page: `<html><script>var p={src:"https://cdn.example.com/live/master.m3u8?t=9"};</script></html>`,
			want: "https://cdn.example.com/live/master.m3u8?t=9",
		},
		{
			// Players commonly keep the URL inside a JSON blob, where every slash is
			// escaped. Missing this case makes the resolver look broken on sites it
			// actually supports.
			name: "json-escaped slashes",
			page: `<script>window.__DATA__={"hls":"https:\/\/cdn.example.com\/live\/x.m3u8"}</script>`,
			want: "https://cdn.example.com/live/x.m3u8",
		},
		{
			name: "relative reference resolved against the page url",
			page: `<video><source src="/streams/live.m3u8" type="application/x-mpegURL"></video>`,
			want: "/streams/live.m3u8", // asserted as a suffix below
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(tc.page))
			}))
			defer srv.Close()

			res, err := live.NewStaticResolver(testFetcher()).Resolve(context.Background(),
				&domain.LiveChannel{SourceURL: srv.URL + "/watch/channel-1"})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if !strings.HasSuffix(res.ManifestURL, tc.want) {
				t.Errorf("manifest = %q, want it to end with %q", res.ManifestURL, tc.want)
			}
			if strings.Contains(res.ManifestURL, `\/`) {
				t.Errorf("manifest still carries escaped slashes: %q", res.ManifestURL)
			}
		})
	}
}

func TestStaticResolverExplainsWhenNothingIsFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>player loads at runtime</body></html>`))
	}))
	defer srv.Close()

	_, err := live.NewStaticResolver(testFetcher()).Resolve(context.Background(),
		&domain.LiveChannel{SourceURL: srv.URL})
	if err == nil || !strings.Contains(err.Error(), "runtime") {
		t.Errorf("error = %v, want it to explain that the site builds its url at runtime", err)
	}
}

func TestRegistryReportsUnknownResolvers(t *testing.T) {
	reg := live.NewRegistry(live.NewDirectResolver(), live.NewAPIResolver(testFetcher()))

	if _, err := reg.Get(domain.ResolverDirect); err != nil {
		t.Errorf("registered resolver not found: %v", err)
	}
	_, err := reg.Get("headless")
	if err == nil || !strings.Contains(err.Error(), "available") {
		t.Errorf("error = %v, want it to list the available resolvers", err)
	}
	if ids := reg.IDs(); len(ids) != 2 || ids[0] != "api" || ids[1] != "direct" {
		t.Errorf("IDs() = %v, want a sorted [api direct]", ids)
	}
}
