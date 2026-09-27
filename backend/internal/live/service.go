package live

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"golang.org/x/sync/singleflight"
)

// ResolveMargin is how far ahead of a stated expiry a manifest is re-resolved.
// Providers sign URLs tightly, and a token that dies mid-segment shows up as a
// stalled player rather than an error, so we never run one to the wire.
const ResolveMargin = 60 * time.Second

// resolveTimeout bounds one resolve, however it was started. Resolves run detached
// from the request that triggered them (see resolveShared), so this is what ends one
// that hangs.
const resolveTimeout = 2 * time.Minute

// resolveRetryAfter is how long a failed resolve is remembered and handed back to
// later callers instead of trying again. Every viewer of a broken channel retries on
// its own schedule, and a browser resolve is seconds of CPU on a one-core host;
// without this a dead source keeps a resolver busy for as long as anyone watches.
const resolveRetryAfter = 30 * time.Second

// drmRetryAfter replaces resolveRetryAfter for a source found to be DRM-protected;
// see retryAfter.
const drmRetryAfter = 10 * time.Minute

// refreshInterval is the shortest gap between forced re-resolves of one channel. A
// source that hands out URLs the server cannot use would otherwise be re-resolved on
// every master playlist request.
const refreshInterval = 2 * time.Minute

// AssetRepository is the slice of the media repository this package needs.
// Declared here rather than importing the media package so the dependency runs one
// way: live knows it needs somewhere to put an asset row, media knows nothing of
// live at all.
type AssetRepository interface {
	CreateAsset(ctx context.Context, a *domain.MediaAsset) error
	DeleteAsset(ctx context.Context, id string) error
}

// CreateRequest is the operator-supplied definition of a new live channel.
type CreateRequest struct {
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	ThumbnailURL   string         `json:"thumbnail_url"`
	Slug           string         `json:"slug"`
	Resolver       string         `json:"resolver"`
	SourceURL      string         `json:"source_url"`
	ResolverConfig map[string]any `json:"resolver_config"`
	Protocol       string         `json:"protocol"`
	IsDVR          bool           `json:"is_dvr"`
	DVRWindowSecs  int            `json:"dvr_window_seconds"`
}

// TestResult reports what a resolver actually produced, so an operator can see a
// stream's real shape before committing a channel. Without it, configuring a scraped
// source is edit, save, join a room, squint, repeat.
type TestResult struct {
	OK             bool     `json:"ok"`
	Error          string   `json:"error,omitempty"`
	ManifestHost   string   `json:"manifest_host,omitempty"`
	Protocol       string   `json:"protocol,omitempty"`
	IsMaster       bool     `json:"is_master"`
	IsLive         bool     `json:"is_live"`
	Variants       []string `json:"variants,omitempty"`
	TargetDuration float64  `json:"target_duration,omitempty"`
	MediaSequence  int64    `json:"media_sequence,omitempty"`
	SegmentCount   int      `json:"segment_count,omitempty"`
	WindowSeconds  float64  `json:"window_seconds,omitempty"`
	ExpiresAt      string   `json:"expires_at,omitempty"`
	// UpstreamHeaders names the headers the proxy will send upstream. Names only:
	// the values are as much a credential as the manifest URL.
	UpstreamHeaders []string `json:"upstream_headers,omitempty"`
	// Outcome, Reason and Diagnostics say what kind of result this was and what
	// the resolver saw, successful or not; see domain.ResolveDiagnostics.
	Outcome     domain.ResolveOutcome      `json:"outcome"`
	Reason      string                     `json:"reason,omitempty"`
	Diagnostics *domain.ResolveDiagnostics `json:"diagnostics,omitempty"`
}

// Service owns live channel lifecycle and keeps a usable upstream manifest on hand.
type Service interface {
	Create(ctx context.Context, req CreateRequest, createdBy *string) (*domain.LiveChannel, error)
	List(ctx context.Context) ([]*domain.LiveChannel, error)
	GetBySlug(ctx context.Context, slug string) (*domain.LiveChannel, error)
	Delete(ctx context.Context, id string) error
	EnsureResolved(ctx context.Context, ch *domain.LiveChannel) error
	Refresh(ctx context.Context, ch *domain.LiveChannel) error
	UpstreamCookies(ch *domain.LiveChannel) http.CookieJar
	TestResolve(ctx context.Context, req CreateRequest) *TestResult
	MasterURLFor(slug string) string
	ResolverIDs() []string
}

type service struct {
	repo      Repository
	assets    AssetRepository
	registry  *Registry
	fetcher   *Fetcher
	proxyBase string

	// resolveGroup collapses concurrent resolves of the same channel. Fifty viewers
	// joining a room at once must not become fifty requests to the provider.
	resolveGroup singleflight.Group

	mu sync.Mutex
	// attempts holds the outcome of each channel's latest resolve, for the retry
	// backoff and the refresh interval.
	attempts map[string]resolveAttempt
	// sessions holds each channel's upstream cookies. In memory only: they are
	// credentials, and a restart costs no more than one refused fetch and a
	// re-resolve, which the proxy already does for a refused master playlist.
	sessions map[string]*upstreamSession
}

// upstreamSession is the cookie jar that goes with one resolved upstream URL.
type upstreamSession struct {
	upstreamURL string
	jar         http.CookieJar
}

type resolveAttempt struct {
	at  time.Time
	err error
}

// NewService wires the live channel service.
func NewService(repo Repository, assets AssetRepository, registry *Registry, fetcher *Fetcher, proxyBase string) Service {
	return &service{
		repo:      repo,
		assets:    assets,
		registry:  registry,
		fetcher:   fetcher,
		proxyBase: strings.TrimRight(proxyBase, "/"),
		attempts:  make(map[string]resolveAttempt),
		sessions:  make(map[string]*upstreamSession),
	}
}

func (s *service) ResolverIDs() []string { return s.registry.IDs() }

// MasterURLFor is the stable, client-facing address of a channel. It never changes,
// which is the entire point: the upstream URL behind it rotates on every re-resolve.
func (s *service) MasterURLFor(slug string) string {
	return fmt.Sprintf("%s/%s/master.m3u8", s.proxyBase, slug)
}

func (s *service) Create(ctx context.Context, req CreateRequest, createdBy *string) (*domain.LiveChannel, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(req.SourceURL) == "" {
		return nil, fmt.Errorf("source url is required")
	}
	if _, err := s.registry.Get(req.Resolver); err != nil {
		return nil, err
	}
	// Reject a source we are not configured to pull from now, at the point the
	// operator can still do something about it, rather than at play time.
	if err := s.fetcher.AllowsHost(req.SourceURL); err != nil {
		return nil, err
	}

	slug := slugify(req.Slug)
	if slug == "" {
		slug = slugify(req.Title)
	}
	if slug == "" {
		return nil, fmt.Errorf("could not derive a url slug from the title; set one explicitly")
	}

	// The companion asset is what makes a live channel appear in the existing media
	// library, the room media picker, and rooms.current_media_url without any of
	// those paths learning that live channels exist.
	asset := &domain.MediaAsset{
		Kind:         domain.MediaKindLive,
		Title:        req.Title,
		Description:  req.Description,
		ThumbnailURL: req.ThumbnailURL,
		SourceURL:    s.MasterURLFor(slug),
		HLSMasterURL: s.MasterURLFor(slug),
		Status:       domain.MediaStatusReady,
		CreatedBy:    createdBy,
	}
	if err := s.assets.CreateAsset(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to create media asset for live channel: %w", err)
	}

	protocol := req.Protocol
	if protocol == "" {
		protocol = "hls"
	}
	ch := &domain.LiveChannel{
		MediaAssetID:   asset.ID,
		Slug:           slug,
		Resolver:       req.Resolver,
		SourceURL:      req.SourceURL,
		ResolverConfig: req.ResolverConfig,
		Protocol:       protocol,
		IsDVR:          req.IsDVR,
		DVRWindowSecs:  req.DVRWindowSecs,
		Status:         domain.LiveStatusIdle,
		CreatedBy:      createdBy,
		Title:          req.Title,
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		// Roll the asset back; a library entry pointing at a channel that does not
		// exist would 404 for every viewer who picked it.
		_ = s.assets.DeleteAsset(ctx, asset.ID)
		return nil, err
	}
	return ch, nil
}

func (s *service) List(ctx context.Context) ([]*domain.LiveChannel, error) {
	return s.repo.List(ctx)
}

func (s *service) GetBySlug(ctx context.Context, slug string) (*domain.LiveChannel, error) {
	return s.repo.GetBySlug(ctx, slug)
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.attempts, id)
	delete(s.sessions, id)
	s.mu.Unlock()
	return nil
}

// EnsureResolved guarantees ch carries an upstream manifest URL that has not expired,
// resolving it if needed. Safe to call on every request: it is a field comparison in
// the common case and a single collapsed resolve otherwise.
func (s *service) EnsureResolved(ctx context.Context, ch *domain.LiveChannel) error {
	if ch.Status == domain.LiveStatusDisabled {
		return fmt.Errorf("live channel %q is disabled", ch.Slug)
	}
	if !ch.NeedsResolve(time.Now(), ResolveMargin) {
		return nil
	}
	return s.resolveShared(ctx, ch)
}

// Refresh re-resolves a channel whose stored upstream has not expired on paper but
// is being refused -- a signed URL that never said when it would stop working. At
// most once per refreshInterval per channel.
func (s *service) Refresh(ctx context.Context, ch *domain.LiveChannel) error {
	if ch.Status == domain.LiveStatusDisabled {
		return fmt.Errorf("live channel %q is disabled", ch.Slug)
	}
	if last, ok := s.lastAttempt(ch.ID); ok && time.Since(last.at) < refreshInterval {
		return fmt.Errorf("live channel %q was resolved %s ago; not refreshing it again yet",
			ch.Slug, time.Since(last.at).Round(time.Second))
	}
	return s.resolveShared(ctx, ch)
}

// resolveShared runs one resolve per channel however many callers want it, and
// applies the result to ch.
//
// The resolve is detached from the request that started it. hls.js abandons a
// manifest request after 20 seconds and asks again, a browser resolve can take
// longer than that, and if the first viewer's request carried the resolve down with
// it, the resolve could never finish. Callers still stop waiting when their own
// context ends; the result is persisted for the next one either way.
func (s *service) resolveShared(ctx context.Context, ch *domain.LiveChannel) error {
	if last, ok := s.lastAttempt(ch.ID); ok && last.err != nil && time.Since(last.at) < retryAfter(last.err) {
		return last.err
	}

	result := s.resolveGroup.DoChan(ch.ID, func() (any, error) {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), resolveTimeout)
		defer cancel()
		res, err := s.resolve(rctx, ch)
		s.mu.Lock()
		s.attempts[ch.ID] = resolveAttempt{at: time.Now(), err: err}
		if err == nil {
			s.sessions[ch.ID] = &upstreamSession{upstreamURL: res.ManifestURL, jar: orNewJar(res.Cookies)}
		}
		s.mu.Unlock()
		if err != nil {
			// Not rctx: a resolve that failed by timing out has already spent it.
			_ = s.repo.UpdateStatus(context.WithoutCancel(rctx), ch.ID, domain.LiveStatusError, err.Error())
		}
		return res, err
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-result:
		if r.Err != nil {
			return r.Err
		}
		res := r.Val.(*Resolution)
		now := time.Now()
		ch.UpstreamURL = res.ManifestURL
		ch.UpstreamHeaders = res.Headers
		ch.Protocol = res.Protocol
		ch.Status = domain.LiveStatusLive
		ch.LastError = ""
		ch.LastResolvedAt = &now
		if !res.ExpiresAt.IsZero() {
			expiry := res.ExpiresAt
			ch.UpstreamExpiresAt = &expiry
		} else {
			ch.UpstreamExpiresAt = nil
		}
		return nil
	}
}

// UpstreamCookies returns the cookie jar for a channel's current upstream: the one
// its resolve started, with whatever the upstream has set in it since. A channel
// resolved before this process started, or by a resolver that found no cookies,
// gets an empty jar, so an upstream that hands out a session cookie on the manifest
// still gets it back on the segments.
func (s *service) UpstreamCookies(ch *domain.LiveChannel) http.CookieJar {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[ch.ID]; ok && sess.upstreamURL == ch.UpstreamURL {
		return sess.jar
	}
	sess := &upstreamSession{upstreamURL: ch.UpstreamURL, jar: newCookieJar()}
	s.sessions[ch.ID] = sess
	return sess.jar
}

func (s *service) lastAttempt(channelID string) (resolveAttempt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.attempts[channelID]
	return a, ok
}

func (s *service) resolve(ctx context.Context, ch *domain.LiveChannel) (*Resolution, error) {
	resolver, err := s.registry.Get(ch.Resolver)
	if err != nil {
		return nil, err
	}

	sources := append([]string{ch.SourceURL}, ch.Fallbacks...)
	var (
		lastErr  error
		lastDiag *domain.ResolveDiagnostics
	)
	for i, src := range sources {
		attempt := *ch
		attempt.SourceURL = src

		start := time.Now()
		res, err := resolver.Resolve(ctx, &attempt)
		if err != nil {
			lastErr = err
			lastDiag = failureDiagnostics(ch.Resolver, err, time.Since(start))
			logResolve(ch.Slug, i, lastDiag, err)
			continue
		}
		diag := successDiagnostics(ch.Resolver, res, time.Since(start))
		logResolve(ch.Slug, i, diag, nil)

		var expiresAt *time.Time
		if !res.ExpiresAt.IsZero() {
			e := res.ExpiresAt
			expiresAt = &e
		}
		// context.WithoutCancel: the resolve succeeded, so the result is worth
		// keeping even if the request that triggered it has since gone away.
		if err := s.repo.UpdateUpstream(context.WithoutCancel(ctx), ch.ID, res.ManifestURL, res.Headers, expiresAt, res.IsDVR, res.WindowSecs); err != nil {
			slog.Error("resolved live channel but failed to persist upstream", "slug", ch.Slug, "error", err)
		}
		s.recordResolution(ctx, ch, diag)
		return res, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no sources configured")
		lastDiag = failureDiagnostics(ch.Resolver, lastErr, 0)
	}
	s.recordResolution(ctx, ch, lastDiag)
	if len(sources) > 1 {
		return nil, fmt.Errorf("all %d sources failed for channel %q; the last: %w", len(sources), ch.Slug, lastErr)
	}
	return nil, lastErr
}

// recordResolution keeps a resolve's outcome and diagnostics on the channel, for
// the admin portal. Detached from ctx: it describes a resolve that has finished.
func (s *service) recordResolution(ctx context.Context, ch *domain.LiveChannel, diag *domain.ResolveDiagnostics) {
	if err := s.repo.RecordResolution(context.WithoutCancel(ctx), ch.ID, diag); err != nil {
		slog.Error("failed to record live channel resolve diagnostics", "slug", ch.Slug, "error", err)
	}
}

// TestResolve runs a resolver against an unsaved configuration and reports what came
// back. Errors are returned in the result rather than as a Go error: every one of
// them is information the operator needs to see, not a server fault.
func (s *service) TestResolve(ctx context.Context, req CreateRequest) *TestResult {
	start := time.Now()
	resolver, err := s.registry.Get(req.Resolver)
	if err != nil {
		return testFailure(req.Resolver, resolveFailure(domain.OutcomeFailed, "invalid_config", err.Error(), err), start)
	}
	if err := s.fetcher.AllowsHost(req.SourceURL); err != nil {
		return testFailure(req.Resolver, err, start)
	}

	probe := &domain.LiveChannel{
		Slug:           "probe",
		Resolver:       req.Resolver,
		SourceURL:      req.SourceURL,
		ResolverConfig: orEmptyMap(req.ResolverConfig),
		Protocol:       req.Protocol,
		IsDVR:          req.IsDVR,
		DVRWindowSecs:  req.DVRWindowSecs,
	}

	res, err := resolver.Resolve(ctx, probe)
	if err != nil {
		return testFailure(req.Resolver, err, start)
	}

	diag := successDiagnostics(req.Resolver, res, time.Since(start))
	out := &TestResult{
		OK:              true,
		Protocol:        res.Protocol,
		ManifestHost:    hostOf(res.ManifestURL),
		UpstreamHeaders: headerNames(res.Headers),
		Outcome:         diag.Outcome,
		Reason:          diag.Reason,
		Diagnostics:     diag,
	}
	if !res.ExpiresAt.IsZero() {
		out.ExpiresAt = res.ExpiresAt.Format(time.RFC3339)
	}

	// Read the whole stream the way the proxy will, whichever resolver found it: a
	// DRM-protected stream must not be reported as playable just because the
	// resolver that found it never looked inside the manifest.
	shape, err := inspectStream(ctx, s.fetcher.WithCookies(orNewJar(res.Cookies)), res.ManifestURL, res.Headers, true)
	switch {
	case err != nil:
		return out.fail(resolveFailure(domain.OutcomeFailed, "upstream_refused",
			fmt.Sprintf("resolved to %s but could not fetch it: %v", out.ManifestHost, err), err))
	case shape.protocol == "":
		return out.fail(resolveFailure(domain.OutcomeNoStreamFound, "not_a_manifest",
			fmt.Sprintf("resolved to %s, but it did not return an HLS or DASH manifest", out.ManifestHost), nil))
	case len(shape.drm) > 0:
		var evidence drmEvidence
		evidence.noteManifest(shape.manifestShape)
		diag.DRM = evidence.diagnostics()
		return out.fail(resolveFailure(domain.OutcomeDRMProtected, "manifest_protected",
			fmt.Sprintf("%s: the stream requires%s%s; only unencrypted or AES-128 HLS can be restreamed",
				ErrDRMProtected, systemsNote(&evidence), renditionsNote(&evidence)), ErrDRMProtected))
	}
	logResolve("test-resolve", 0, diag, nil)
	if shape.protocol != "hls" {
		return out // only HLS manifests are parsed below
	}
	raw := shape.body

	// Identity rewrite: parse for shape only, discard the rewritten body.
	pl := ParsePlaylist(raw, res.ManifestURL, func(u string, k URIKind) string { return u })
	out.IsMaster = pl.IsMaster
	out.IsLive = pl.IsLive()
	out.TargetDuration = pl.TargetDuration
	out.MediaSequence = pl.MediaSequence
	if pl.IsMaster {
		out.Variants = variantLabels(raw)
	} else if pl.EdgeSequence >= 0 && pl.MediaSequence >= 0 {
		out.SegmentCount = int(pl.EdgeSequence-pl.MediaSequence) + 1
		out.WindowSeconds = float64(out.SegmentCount) * pl.TargetDuration
	}
	return out
}

// fail turns a test result that got as far as a manifest into a failure.
func (t *TestResult) fail(err *ResolveError) *TestResult {
	t.OK = false
	t.Error = err.Message
	t.Outcome, t.Reason = err.Outcome, err.Reason
	if t.Diagnostics != nil {
		t.Diagnostics.Outcome, t.Diagnostics.Reason = err.Outcome, err.Reason
		logResolve("test-resolve", 0, t.Diagnostics, err)
	}
	return t
}

func testFailure(resolverID string, err error, start time.Time) *TestResult {
	diag := failureDiagnostics(resolverID, err, time.Since(start))
	logResolve("test-resolve", 0, diag, err)
	return &TestResult{Error: err.Error(), Outcome: diag.Outcome, Reason: diag.Reason, Diagnostics: diag}
}

var resolutionPattern = regexp.MustCompile(`RESOLUTION=(\d+x\d+)`)

func variantLabels(raw []byte) []string {
	matches := resolutionPattern.FindAllSubmatch(raw, -1)
	labels := make([]string, 0, len(matches))
	for _, m := range matches {
		labels = append(labels, string(m[1]))
	}
	return labels
}

func orNewJar(jar http.CookieJar) http.CookieJar {
	if jar == nil {
		return newCookieJar()
	}
	return jar
}

func headerNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, http.CanonicalHeaderKey(name))
	}
	sort.Strings(names)
	return names
}

// hostOf is the host (and port) of a URL, and never any other part of it: no
// userinfo, path or query, all of which can carry credentials.
func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Host
	}
	return "<unknown host>"
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	slug := nonSlugChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-")
	}
	return slug
}
