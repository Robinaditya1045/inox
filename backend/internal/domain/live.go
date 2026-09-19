package domain

import "time"

// LiveChannelStatus tracks whether a channel currently has a usable upstream manifest.
type LiveChannelStatus string

const (
	LiveStatusIdle      LiveChannelStatus = "idle"
	LiveStatusResolving LiveChannelStatus = "resolving"
	LiveStatusLive      LiveChannelStatus = "live"
	LiveStatusDegraded  LiveChannelStatus = "degraded"
	LiveStatusError     LiveChannelStatus = "error"
	LiveStatusDisabled  LiveChannelStatus = "disabled"
)

// Resolver identifiers. Each names one strategy for turning an operator-supplied
// source into a playable manifest URL; everything downstream is shared.
const (
	ResolverDirect = "direct" // source_url already is the manifest
	ResolverAPI    = "api"    // source_url is a provider endpoint returning the manifest URL
	ResolverStatic = "static" // source_url is a page; scan its HTML/JS for a manifest
)

// LiveChannel is the ingest configuration behind a media asset of kind 'live'.
type LiveChannel struct {
	ID             string            `json:"id"`
	MediaAssetID   string            `json:"media_asset_id"`
	Slug           string            `json:"slug"`
	Resolver       string            `json:"resolver"`
	SourceURL      string            `json:"source_url"`
	ResolverConfig map[string]any    `json:"resolver_config"`
	Fallbacks      []string          `json:"fallback_sources"`
	Protocol       string            `json:"protocol"`
	IsDVR          bool              `json:"is_dvr"`
	DVRWindowSecs  int               `json:"dvr_window_seconds"`
	Status         LiveChannelStatus `json:"status"`

	// UpstreamURL and UpstreamHeaders are credentials, not content. They are
	// deliberately omitted from JSON so an admin API response can never leak the
	// feed itself; the proxy is the only thing that reads them.
	UpstreamURL     string            `json:"-"`
	UpstreamHeaders map[string]string `json:"-"`

	UpstreamExpiresAt *time.Time `json:"upstream_expires_at"`
	LastResolvedAt    *time.Time `json:"last_resolved_at"`
	LastError         string     `json:"last_error"`

	CreatedBy *string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Title mirrors the companion media asset so the admin list needs one query.
	Title string `json:"title,omitempty"`
}

// NeedsResolve reports whether the cached upstream manifest is missing or close
// enough to expiry that it should be re-resolved before the next request uses it.
func (c *LiveChannel) NeedsResolve(now time.Time, margin time.Duration) bool {
	if c.UpstreamURL == "" {
		return true
	}
	if c.UpstreamExpiresAt == nil {
		return false
	}
	return now.Add(margin).After(*c.UpstreamExpiresAt)
}

// LivePosition names a frame by where it sits in the stream rather than where it
// sits in any one client's timeline.
//
// This is the whole reason live sync needs its own coordinate. hls.js starts each
// client's media timeline near the live edge at the moment that client attached, so
// video.currentTime for the same frame differs per viewer and is meaningless to
// anyone else. Media sequence numbers come from the playlist itself and are
// therefore identical for everyone, with no clocks involved.
type LivePosition struct {
	MediaSequence int64   `json:"sn"`
	Discontinuity int64   `json:"cc"`
	OffsetSeconds float64 `json:"offset"`
}
