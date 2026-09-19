package live

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"golang.org/x/sync/singleflight"
)

// Route path segments. Short because they are repeated on every segment URI in
// every playlist we serve.
const (
	pathPlaylist = "p" // another manifest: fetch, rewrite, serve
	pathResource = "r" // segment, key, or init file: stream through untouched
)

// masterCacheTTL bounds how long a variant list is reused. Master playlists change
// only when the ladder itself changes, which is close to never.
const masterCacheTTL = 30 * time.Second

type cachedPlaylist struct {
	body      []byte
	expiresAt time.Time
}

// Proxy serves live manifests and segments from our own origin.
//
// Four independent constraints each force this to exist, and none of them can be
// worked around client-side: upstreams demand Referer/Origin/User-Agent values that
// browser JavaScript is forbidden from setting; they send no Access-Control-Allow-Origin,
// so hls.js cannot read them cross-origin; their signed URLs expire in minutes while
// clients need an address that stays put; and the upstream URL is itself a credential
// that must not reach a viewer.
type Proxy struct {
	svc     Service
	fetcher *Fetcher
	sealer  *Sealer
	tokens  *TokenSigner
	base    string
	// basePath is the path component of base, used to recognise our own URLs.
	// Compared without the host so that a channel stays recognisable when the
	// backend is reached through a tunnel or a different public origin.
	basePath string

	group singleflight.Group

	// onStatusChange is notified when a channel's upstream starts or stops working.
	// Optional: the proxy serves fine without it, but rooms then see a frozen frame
	// with no explanation when a broadcast dies.
	onStatusChange func(slug, status, message string)

	mu    sync.RWMutex
	cache map[string]*cachedPlaylist
	// healthy tracks the last reported state per channel so only transitions are
	// announced, rather than one notice per failed request per viewer.
	healthy map[string]bool
	// edges tracks the live edge media sequence per channel slug, which is what the
	// hub clamps a sync leader's reported position against.
	edges map[string]int64
}

// NewProxy wires the live streaming proxy.
func NewProxy(svc Service, fetcher *Fetcher, sealer *Sealer, tokens *TokenSigner, baseURL string) *Proxy {
	base := strings.TrimRight(baseURL, "/")
	basePath := ""
	if u, err := url.Parse(base); err == nil {
		basePath = strings.TrimRight(u.Path, "/")
	}
	return &Proxy{
		svc:      svc,
		fetcher:  fetcher,
		sealer:   sealer,
		tokens:   tokens,
		base:     base,
		basePath: basePath,
		cache:    make(map[string]*cachedPlaylist),
		edges:    make(map[string]int64),
		healthy:  make(map[string]bool),
	}
}

// SetStatusListener registers a callback for upstream health transitions. Called from
// request goroutines, so the listener must be safe for concurrent use.
func (p *Proxy) SetStatusListener(fn func(slug, status, message string)) {
	p.onStatusChange = fn
}

// reportHealth announces a channel's upstream health, but only when it changes.
func (p *Proxy) reportHealth(slug string, ok bool, message string) {
	p.mu.Lock()
	previous, seen := p.healthy[slug]
	p.healthy[slug] = ok
	p.mu.Unlock()

	// First observation of a working channel is not news; a first failure is.
	if (seen && previous == ok) || (!seen && ok) {
		return
	}
	if p.onStatusChange == nil {
		return
	}
	if ok {
		p.onStatusChange(slug, "live", "")
		return
	}
	p.onStatusChange(slug, "degraded", message)
}

// EdgeSequenceForURL reports the newest media sequence seen for the channel a room's
// media URL points at. Returns false for a URL that is not one of ours.
func (p *Proxy) EdgeSequenceForURL(masterURL string) (int64, bool) {
	slug, ok := p.slugFromMasterURL(masterURL)
	if !ok {
		return 0, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	edge, ok := p.edges[slug]
	return edge, ok
}

// IsLiveURL reports whether a media URL addresses a channel on this proxy. Used by
// the hub to decide which sync coordinate a room is on, so it must stay a cheap
// string check rather than a database lookup.
func (p *Proxy) IsLiveURL(masterURL string) bool {
	_, ok := p.slugFromMasterURL(masterURL)
	return ok
}

// slugFromMasterURL extracts the channel slug from a proxy master playlist URL of
// the form {base}/{slug}/master.m3u8.
//
// The base path has to be part of the match. Matching on the filename alone also
// catches a transcoded VOD asset at /media/stream/hls/{id}/master.m3u8, which would
// put its room on the live sync coordinate and break playback for everyone in it.
func (p *Proxy) slugFromMasterURL(masterURL string) (string, bool) {
	u, err := url.Parse(masterURL)
	if err != nil {
		return "", false
	}
	rest := strings.TrimRight(u.Path, "/")
	if p.basePath != "" {
		if !strings.HasPrefix(rest, p.basePath+"/") {
			return "", false
		}
		rest = strings.TrimPrefix(rest, p.basePath+"/")
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "master.m3u8" || parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

// ServeMaster handles the one request that requires a real session. It mints a
// playback token and writes it into every URI of the returned manifest, so segment
// requests carry a credential scoped to this channel alone rather than the caller's
// session ID.
func (p *Proxy) ServeMaster(w http.ResponseWriter, r *http.Request, slug, userID string) {
	ch, err := p.svc.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "live channel not found", http.StatusNotFound)
		return
	}
	if err := p.svc.EnsureResolved(r.Context(), ch); err != nil {
		slog.Warn("live channel could not be resolved", "slug", slug, "error", err)
		p.reportHealth(slug, false, "The stream source could not be reached.")
		http.Error(w, "live stream is currently unavailable", http.StatusBadGateway)
		return
	}

	token := p.tokens.Mint(ch.ID, userID, time.Now())
	body, pl, err := p.loadPlaylist(r.Context(), ch, ch.UpstreamURL, token, masterCacheTTL)
	if err != nil {
		slog.Warn("failed to fetch live master playlist", "slug", slug, "error", err)
		p.reportHealth(slug, false, "The broadcast is not responding.")
		http.Error(w, "live stream is currently unavailable", http.StatusBadGateway)
		return
	}
	p.reportHealth(slug, true, "")
	p.recordEdge(slug, pl)
	writePlaylist(w, body)
}

// ServePlaylist serves a variant or media playlist referenced by a sealed ref.
func (p *Proxy) ServePlaylist(w http.ResponseWriter, r *http.Request, slug, ref, token string) {
	ch, upstream, ok := p.authorize(w, r, slug, ref, token)
	if !ok {
		return
	}

	// Media playlists are re-fetched constantly by every viewer. Caching for half a
	// target duration behind a singleflight turns N viewers polling every 2s into
	// roughly one upstream request per interval, whatever N is.
	ttl := 2 * time.Second
	body, pl, err := p.loadPlaylist(r.Context(), ch, upstream, token, ttl)
	if err != nil {
		slog.Warn("failed to fetch live media playlist", "slug", slug, "error", err)
		p.reportHealth(slug, false, "The broadcast stopped sending video.")
		http.Error(w, "live stream is currently unavailable", http.StatusBadGateway)
		return
	}
	p.reportHealth(slug, true, "")
	p.recordEdge(slug, pl)
	writePlaylist(w, body)
}

// ServeResource streams a segment, key, or init file straight through.
func (p *Proxy) ServeResource(w http.ResponseWriter, r *http.Request, slug, ref, token string) {
	ch, upstream, ok := p.authorize(w, r, slug, ref, token)
	if !ok {
		return
	}

	resp, err := p.fetcher.Get(r.Context(), upstream, ch.UpstreamHeaders)
	if err != nil {
		slog.Warn("failed to fetch live segment", "slug", slug, "error", err)
		http.Error(w, "segment unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	// Segments are immutable once published, so they cache hard. This is also what
	// makes putting a CDN in front of this route viable later without changing it.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Debug("live segment copy interrupted", "slug", slug, "error", err)
	}
}

// authorize verifies the playback token and opens the sealed upstream reference.
func (p *Proxy) authorize(w http.ResponseWriter, r *http.Request, slug, ref, token string) (*domain.LiveChannel, string, bool) {
	ch, err := p.svc.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "live channel not found", http.StatusNotFound)
		return nil, "", false
	}
	if _, err := p.tokens.Verify(ch.ID, token, time.Now()); err != nil {
		http.Error(w, "invalid or expired playback token", http.StatusForbidden)
		return nil, "", false
	}
	upstream, err := p.sealer.Open(ch.ID, ref)
	if err != nil {
		http.Error(w, "invalid stream reference", http.StatusBadRequest)
		return nil, "", false
	}
	return ch, upstream, true
}

// loadPlaylist fetches and rewrites a manifest, collapsing concurrent requests for
// the same upstream URL and reusing the result for ttl.
func (p *Proxy) loadPlaylist(ctx context.Context, ch *domain.LiveChannel, upstreamURL, token string, ttl time.Duration) ([]byte, *Playlist, error) {
	key := ch.ID + "|" + upstreamURL

	p.mu.RLock()
	entry, ok := p.cache[key]
	p.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.body, nil, nil
	}

	type result struct {
		body []byte
		pl   *Playlist
	}
	out, err, _ := p.group.Do(key, func() (any, error) {
		raw, err := p.fetcher.GetBytes(ctx, upstreamURL, ch.UpstreamHeaders, MaxManifestBytes)
		if err != nil {
			return nil, err
		}

		pl := ParsePlaylist(raw, upstreamURL, func(absolute string, kind URIKind) string {
			return p.rewriteURI(ch, absolute, kind, token)
		})

		// A finished stream must not be cached for long, or viewers keep polling a
		// playlist that will never advance again.
		effectiveTTL := ttl
		if pl.TargetDuration > 0 && !pl.IsMaster {
			if half := time.Duration(pl.TargetDuration*float64(time.Second)) / 2; half < effectiveTTL {
				effectiveTTL = half
			}
		}
		if effectiveTTL < time.Second {
			effectiveTTL = time.Second
		}

		p.mu.Lock()
		p.cache[key] = &cachedPlaylist{body: pl.Body, expiresAt: time.Now().Add(effectiveTTL)}
		p.pruneLocked()
		p.mu.Unlock()

		return &result{body: pl.Body, pl: pl}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	res := out.(*result)
	return res.body, res.pl, nil
}

// rewriteURI maps one upstream URI to its address on this proxy. The upstream URL is
// sealed rather than encoded, so a client can neither read it nor point us at
// somewhere we did not choose.
func (p *Proxy) rewriteURI(ch *domain.LiveChannel, absolute string, kind URIKind, token string) string {
	sealed, err := p.sealer.Seal(ch.ID, absolute)
	if err != nil {
		slog.Error("failed to seal upstream uri", "slug", ch.Slug, "error", err)
		return absolute
	}
	segment := pathResource
	suffix := ""
	if kind == URIPlaylist {
		segment = pathPlaylist
		// hls.js and Safari both branch on the file extension in places, so keep
		// nested playlists looking like playlists.
		suffix = ".m3u8"
	}
	return fmt.Sprintf("%s/%s/%s/%s%s?t=%s", p.base, ch.Slug, segment, sealed, suffix, url.QueryEscape(token))
}

func (p *Proxy) recordEdge(slug string, pl *Playlist) {
	if pl == nil || pl.IsMaster || pl.EdgeSequence < 0 {
		return
	}
	p.mu.Lock()
	p.edges[slug] = pl.EdgeSequence
	p.mu.Unlock()
}

// pruneLocked drops expired playlist entries. Called under the write lock on every
// insert, which is often enough to bound the map without a background sweeper.
func (p *Proxy) pruneLocked() {
	if len(p.cache) < 64 {
		return
	}
	now := time.Now()
	for k, v := range p.cache {
		if now.After(v.expiresAt) {
			delete(p.cache, k)
		}
	}
}

func writePlaylist(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	// Never cache a live playlist at the edge: its whole job is to change.
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
