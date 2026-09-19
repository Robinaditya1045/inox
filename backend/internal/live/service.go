package live

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"golang.org/x/sync/singleflight"
)

// ResolveMargin is how far ahead of a stated expiry a manifest is re-resolved.
// Providers sign URLs tightly, and a token that dies mid-segment shows up as a
// stalled player rather than an error, so we never run one to the wire.
const ResolveMargin = 60 * time.Second

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
}

// Service owns live channel lifecycle and keeps a usable upstream manifest on hand.
type Service interface {
	Create(ctx context.Context, req CreateRequest, createdBy *string) (*domain.LiveChannel, error)
	List(ctx context.Context) ([]*domain.LiveChannel, error)
	GetBySlug(ctx context.Context, slug string) (*domain.LiveChannel, error)
	Delete(ctx context.Context, id string) error
	EnsureResolved(ctx context.Context, ch *domain.LiveChannel) error
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
}

// NewService wires the live channel service.
func NewService(repo Repository, assets AssetRepository, registry *Registry, fetcher *Fetcher, proxyBase string) Service {
	return &service{
		repo:      repo,
		assets:    assets,
		registry:  registry,
		fetcher:   fetcher,
		proxyBase: strings.TrimRight(proxyBase, "/"),
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
	return s.repo.Delete(ctx, id)
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

	resolved, err, _ := s.resolveGroup.Do(ch.ID, func() (any, error) {
		return s.resolve(ctx, ch)
	})
	if err != nil {
		_ = s.repo.UpdateStatus(context.WithoutCancel(ctx), ch.ID, domain.LiveStatusError, err.Error())
		return err
	}

	res := resolved.(*Resolution)
	ch.UpstreamURL = res.ManifestURL
	ch.UpstreamHeaders = res.Headers
	ch.Protocol = res.Protocol
	ch.Status = domain.LiveStatusLive
	ch.LastError = ""
	if !res.ExpiresAt.IsZero() {
		expiry := res.ExpiresAt
		ch.UpstreamExpiresAt = &expiry
	} else {
		ch.UpstreamExpiresAt = nil
	}
	return nil
}

func (s *service) resolve(ctx context.Context, ch *domain.LiveChannel) (*Resolution, error) {
	resolver, err := s.registry.Get(ch.Resolver)
	if err != nil {
		return nil, err
	}

	sources := append([]string{ch.SourceURL}, ch.Fallbacks...)
	var lastErr error
	for i, src := range sources {
		attempt := *ch
		attempt.SourceURL = src

		res, err := resolver.Resolve(ctx, &attempt)
		if err != nil {
			lastErr = err
			slog.Warn("live channel resolve attempt failed",
				"slug", ch.Slug, "resolver", ch.Resolver, "source_index", i, "error", err)
			continue
		}

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
		slog.Info("resolved live channel upstream",
			"slug", ch.Slug, "resolver", ch.Resolver, "protocol", res.Protocol, "expires_at", res.ExpiresAt)
		return res, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no sources configured")
	}
	return nil, fmt.Errorf("all sources failed for channel %q: %w", ch.Slug, lastErr)
}

// TestResolve runs a resolver against an unsaved configuration and reports what came
// back. Errors are returned in the result rather than as a Go error: every one of
// them is information the operator needs to see, not a server fault.
func (s *service) TestResolve(ctx context.Context, req CreateRequest) *TestResult {
	resolver, err := s.registry.Get(req.Resolver)
	if err != nil {
		return &TestResult{Error: err.Error()}
	}
	if err := s.fetcher.AllowsHost(req.SourceURL); err != nil {
		return &TestResult{Error: err.Error()}
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
		return &TestResult{Error: err.Error()}
	}

	out := &TestResult{OK: true, Protocol: res.Protocol, ManifestHost: hostOf(res.ManifestURL)}
	if !res.ExpiresAt.IsZero() {
		out.ExpiresAt = res.ExpiresAt.Format(time.RFC3339)
	}
	if res.Protocol != "hls" {
		return out // only HLS manifests are parsed below
	}

	raw, err := s.fetcher.GetBytes(ctx, res.ManifestURL, res.Headers, MaxManifestBytes)
	if err != nil {
		out.OK = false
		out.Error = fmt.Sprintf("resolved to %s but could not fetch it: %v", out.ManifestHost, err)
		return out
	}

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

var resolutionPattern = regexp.MustCompile(`RESOLUTION=(\d+x\d+)`)

func variantLabels(raw []byte) []string {
	matches := resolutionPattern.FindAllSubmatch(raw, -1)
	labels := make([]string, 0, len(matches))
	for _, m := range matches {
		labels = append(labels, string(m[1]))
	}
	return labels
}

func hostOf(rawURL string) string {
	if _, rest, ok := strings.Cut(rawURL, "://"); ok {
		host, _, _ := strings.Cut(rest, "/")
		return host
	}
	return rawURL
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
