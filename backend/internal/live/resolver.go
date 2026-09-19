package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// Resolution is what every ingest path produces: a manifest URL that is playable
// right now, plus whatever the upstream demands in order to serve it.
type Resolution struct {
	ManifestURL string            `json:"manifest_url"`
	Headers     map[string]string `json:"-"`
	Protocol    string            `json:"protocol"`
	ExpiresAt   time.Time         `json:"expires_at"`
	IsDVR       bool              `json:"is_dvr"`
	WindowSecs  int               `json:"window_seconds"`
}

// Resolver turns one kind of operator-supplied source into a Resolution.
//
// This interface is the seam that keeps "I have a stream API" and "scrape this page"
// from becoming two features. They differ only in how the manifest URL is obtained;
// the refresh loop, the proxy, the room binding and the sync model behind this
// interface are written once. Supporting a new kind of source is a new
// implementation and a registry entry, never a change to the runtime.
type Resolver interface {
	ID() string
	Resolve(ctx context.Context, ch *domain.LiveChannel) (*Resolution, error)
}

// Registry maps a channel's resolver column to its implementation.
type Registry struct {
	byID map[string]Resolver
}

// NewRegistry indexes the available resolvers by ID.
func NewRegistry(resolvers ...Resolver) *Registry {
	r := &Registry{byID: make(map[string]Resolver, len(resolvers))}
	for _, res := range resolvers {
		r.byID[res.ID()] = res
	}
	return r
}

// Get returns the resolver for an ID, or an error naming what is available.
func (r *Registry) Get(id string) (Resolver, error) {
	res, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("unknown resolver %q (available: %s)", id, strings.Join(r.IDs(), ", "))
	}
	return res, nil
}

// IDs lists the registered resolver IDs in stable order.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ── direct ──────────────────────────────────────────────────────────────────

type directResolver struct{}

// NewDirectResolver handles the case where source_url already is the manifest.
func NewDirectResolver() Resolver { return &directResolver{} }

func (d *directResolver) ID() string { return domain.ResolverDirect }

func (d *directResolver) Resolve(_ context.Context, ch *domain.LiveChannel) (*Resolution, error) {
	if ch.SourceURL == "" {
		return nil, fmt.Errorf("direct resolver requires a manifest url")
	}
	return finish(ch, ch.SourceURL), nil
}

// ── api ─────────────────────────────────────────────────────────────────────

type apiResolver struct {
	fetcher *Fetcher
}

// NewAPIResolver calls a provider endpoint and reads the manifest URL out of its
// JSON response.
func NewAPIResolver(f *Fetcher) Resolver { return &apiResolver{fetcher: f} }

func (a *apiResolver) ID() string { return domain.ResolverAPI }

func (a *apiResolver) Resolve(ctx context.Context, ch *domain.LiveChannel) (*Resolution, error) {
	raw, err := a.fetcher.GetBytes(ctx, ch.SourceURL, configHeaders(ch, "api_headers"), MaxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("provider api request failed: %w", err)
	}

	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("provider api did not return JSON: %w", err)
	}

	path := configString(ch, "json_path")
	if path == "" {
		return nil, fmt.Errorf("api resolver requires resolver_config.json_path (e.g. \"data.hls.url\")")
	}
	found, ok := walkJSON(body, path)
	if !ok {
		return nil, fmt.Errorf("json_path %q not present in the provider response", path)
	}
	manifest, ok := found.(string)
	if !ok || manifest == "" {
		return nil, fmt.Errorf("json_path %q did not resolve to a url string", path)
	}
	return finish(ch, absolutizeAgainst(ch.SourceURL, manifest)), nil
}

// ── static ──────────────────────────────────────────────────────────────────

// Manifest URLs embedded in page markup or inline scripts. The \/ alternative
// catches URLs inside JSON string literals, which is where most players keep them.
var manifestPattern = regexp.MustCompile(`https?:(?:\\?/){2}[^\s"'<>()]+?\.(?:m3u8|mpd)[^\s"'<>()]*`)

// Relative or protocol-relative references, as a fallback when no absolute URL is
// present anywhere in the document.
var relativeManifestPattern = regexp.MustCompile(`["']((?:\.{0,2}/|//)[^"'\s]+\.(?:m3u8|mpd)[^"'\s]*)["']`)

type staticResolver struct {
	fetcher *Fetcher
}

// NewStaticResolver scans a page's HTML and inline scripts for a manifest URL.
//
// Deliberately the cheap tier: one HTTP request, no browser. It fails on any site
// that builds its manifest URL at runtime, which is the point at which an extractor
// tool earns its keep -- but it succeeds on a surprising number of them for the cost
// of a single GET.
func NewStaticResolver(f *Fetcher) Resolver { return &staticResolver{fetcher: f} }

func (s *staticResolver) ID() string { return domain.ResolverStatic }

func (s *staticResolver) Resolve(ctx context.Context, ch *domain.LiveChannel) (*Resolution, error) {
	page, err := s.fetcher.GetBytes(ctx, ch.SourceURL, configHeaders(ch, "page_headers"), MaxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("could not fetch source page: %w", err)
	}

	if m := manifestPattern.Find(page); m != nil {
		// JSON string literals escape their slashes; unescape before use.
		return finish(ch, strings.ReplaceAll(string(m), `\/`, `/`)), nil
	}
	if m := relativeManifestPattern.FindSubmatch(page); m != nil {
		return finish(ch, absolutizeAgainst(ch.SourceURL, string(m[1]))), nil
	}
	return nil, fmt.Errorf("no .m3u8 or .mpd URL found in the page source; this site likely builds its stream URL at runtime")
}

// ── shared helpers ──────────────────────────────────────────────────────────

// finish applies the channel's operator-configured playback headers and TTL to a
// freshly discovered manifest URL.
func finish(ch *domain.LiveChannel, manifestURL string) *Resolution {
	res := &Resolution{
		ManifestURL: manifestURL,
		Headers:     configHeaders(ch, "headers"),
		Protocol:    ch.Protocol,
		IsDVR:       ch.IsDVR,
		WindowSecs:  ch.DVRWindowSecs,
	}
	if res.Protocol == "" {
		if strings.Contains(manifestURL, ".mpd") {
			res.Protocol = "dash"
		} else {
			res.Protocol = "hls"
		}
	}
	// A provider that states its own expiry is rare, so operators set a TTL instead.
	// Leaving it zero means "unknown", and the refresh loop falls back to re-resolving
	// only when upstream actually starts failing.
	if ttl := configInt(ch, "ttl_seconds"); ttl > 0 {
		res.ExpiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
	}
	return res
}

func configString(ch *domain.LiveChannel, key string) string {
	if v, ok := ch.ResolverConfig[key].(string); ok {
		return v
	}
	return ""
}

func configInt(ch *domain.LiveChannel, key string) int {
	switch v := ch.ResolverConfig[key].(type) {
	case float64: // every number decoded from JSONB arrives as float64
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func configHeaders(ch *domain.LiveChannel, key string) map[string]string {
	raw, ok := ch.ResolverConfig[key].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// walkJSON follows a dotted path through decoded JSON, treating numeric segments as
// array indices: "data.streams.0.url".
func walkJSON(node any, path string) (any, bool) {
	for _, seg := range strings.Split(path, ".") {
		switch typed := node.(type) {
		case map[string]any:
			next, ok := typed[seg]
			if !ok {
				return nil, false
			}
			node = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, false
			}
			node = typed[idx]
		default:
			return nil, false
		}
	}
	return node, true
}

func absolutizeAgainst(baseURL, ref string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}
	return absolutize(base, ref)
}
