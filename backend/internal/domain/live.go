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
	ResolverDirect  = "direct"  // source_url already is the manifest
	ResolverAPI     = "api"     // source_url is a provider endpoint returning the manifest URL
	ResolverStatic  = "static"  // source_url is a page; scan its HTML/JS for a manifest
	ResolverBrowser = "browser" // source_url is a page; run it in Chromium and watch what its player requests
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

	// LastOutcome and LastDiagnostics describe the most recent resolve, whether it
	// found a stream or not, so an operator can see why a channel does not play.
	LastOutcome     ResolveOutcome      `json:"last_outcome,omitempty"`
	LastDiagnostics *ResolveDiagnostics `json:"last_diagnostics,omitempty"`

	CreatedBy *string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Title mirrors the companion media asset so the admin list needs one query.
	Title string `json:"title,omitempty"`
}

// ResolveOutcome is the broad result of resolving a channel's source.
type ResolveOutcome string

const (
	// OutcomeStreamFound: a playable manifest was found and accepted.
	OutcomeStreamFound ResolveOutcome = "STREAM_FOUND"
	// OutcomeDRMProtected: the source plays only through DRM. It is reported, never
	// worked around.
	OutcomeDRMProtected ResolveOutcome = "DRM_PROTECTED"
	// OutcomeNoStreamFound: the source was reachable but offered no stream.
	OutcomeNoStreamFound ResolveOutcome = "NO_STREAM_FOUND"
	// OutcomeServerReplayFailed: a stream was discovered and the browser played it,
	// but the same request from our server is refused because the source binds the
	// stream to the browser session, origin or network fingerprint. Discovery
	// works; server-side restreaming -- the only playback mode Inox supports -- does
	// not, and is not worked around. Distinct from OutcomeFailed so an operator can
	// tell "found it but this source can't be restreamed" from a plain failure.
	OutcomeServerReplayFailed ResolveOutcome = "SERVER_REPLAY_FAILED"
	// OutcomeTimeout: the source did not finish answering in the time allowed.
	OutcomeTimeout ResolveOutcome = "RESOLUTION_TIMEOUT"
	// OutcomeFailed: anything else -- an unreachable page, a refused upstream, a
	// configuration mistake.
	OutcomeFailed ResolveOutcome = "RESOLUTION_FAILED"
)

// ResolveDiagnostics records what a resolve saw, for an operator working out why a
// channel does or does not play.
//
// It is stored and shown in the admin portal, so it holds only what is safe there:
// hosts rather than URLs (signed URLs carry their credentials in the path and query),
// DRM system names rather than DRM data, counts rather than contents. Never a header
// value, cookie, token, key ID or licence payload.
type ResolveDiagnostics struct {
	Outcome ResolveOutcome `json:"outcome"`
	// Reason is a stable, machine-readable refinement of the outcome, such as
	// "manifest_protected" or "page_http_error".
	Reason    string               `json:"reason,omitempty"`
	Resolver  string               `json:"resolver,omitempty"`
	ElapsedMS int64                `json:"elapsed_ms,omitempty"`
	Page      *PageDiagnostics     `json:"page,omitempty"`
	Manifests []ManifestDiagnostic `json:"manifests,omitempty"`
	DRM       *DRMDiagnostics      `json:"drm,omitempty"`
}

// PageDiagnostics describes the page a browser resolve loaded.
type PageDiagnostics struct {
	Host          string `json:"host,omitempty"`
	Status        int    `json:"status,omitempty"`
	Loaded        bool   `json:"loaded"`
	Frames        int    `json:"frames"`
	Nudges        int    `json:"nudges"`
	PopupsClosed  int    `json:"popups_closed,omitempty"`
	BudgetSeconds int    `json:"budget_seconds,omitempty"`
	// Cookies is how many of the page session's cookies were carried over to the
	// server's requests for the stream; CookiesWithheld, how many were not because
	// they are bot-protection clearances. Counts only: cookies are credentials.
	Cookies         int `json:"cookies,omitempty"`
	CookiesWithheld int `json:"cookies_withheld,omitempty"`
	// CookiesWithheldNames are the names of the bot-protection cookies held back,
	// so the reason for the label is visible. Names only, never values.
	CookiesWithheldNames []string `json:"cookies_withheld_names,omitempty"`
	// CookieDomains are the hosts those cookies belong to, FrameHosts the hosts of
	// the frames the page loaded, and Redirects the hosts the page bounced through
	// on the way in. Hostnames only, never names, values or paths.
	CookieDomains []string `json:"cookie_domains,omitempty"`
	FrameHosts    []string `json:"frame_hosts,omitempty"`
	Redirects     []string `json:"redirects,omitempty"`
}

// ManifestDiagnostic is one manifest the source offered and what became of it.
type ManifestDiagnostic struct {
	Host     string `json:"host"`
	Protocol string `json:"protocol,omitempty"` // hls | dash
	Kind     string `json:"kind,omitempty"`     // master | media | mpd
	Live     bool   `json:"live"`
	// Verdict is one of: chosen, usable, drm_protected, refused, not_allowed,
	// not_a_manifest, not_loaded, unmatched, unchecked.
	Verdict string   `json:"verdict"`
	Detail  string   `json:"detail,omitempty"`
	DRM     []string `json:"drm,omitempty"`
	// BrowserCookies is how many cookies the page's player sent with its own
	// request for this manifest.
	BrowserCookies int `json:"browser_cookies,omitempty"`
}

// DRMDiagnostics is the evidence of DRM a resolve found.
type DRMDiagnostics struct {
	// Confirmed means the stream itself needs DRM -- its manifest says so, its
	// media is encrypted, or the page set up a key session or fetched a licence --
	// as opposed to a page merely asking which DRM the browser supports, which many
	// players and fingerprinting scripts do for unprotected streams too.
	Confirmed bool `json:"confirmed"`
	// Systems are the DRM systems the stream uses, by name: Widevine, PlayReady,
	// FairPlay, ClearKey. Only systems in play -- named by a manifest, by EME init
	// data, or by MediaKeys the page created -- not every one it asked about.
	Systems []string `json:"systems,omitempty"`
	// KeySystems are the EME key systems the page asked for, such as
	// com.widevine.alpha; ActiveKeySystems the ones it went on to create MediaKeys
	// for, which the browser only allows for systems it has.
	KeySystems       []string `json:"key_systems,omitempty"`
	ActiveKeySystems []string `json:"active_key_systems,omitempty"`
	// InitDataTypes are the kinds of EME initialisation data seen: cenc, keyids,
	// webm, sinf, skd.
	InitDataTypes []string `json:"init_data_types,omitempty"`
	// Schemes are the encryption schemes in use: cenc, cbcs.
	Schemes      []string `json:"schemes,omitempty"`
	LicenseHosts []string `json:"license_hosts,omitempty"`
	// Signals name each kind of evidence seen, such as manifest_protected,
	// key_session_created or license_server_contacted.
	Signals             []string `json:"signals,omitempty"`
	ProtectedRenditions int      `json:"protected_renditions,omitempty"`
	Renditions          int      `json:"renditions,omitempty"`
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

// LiveChannelSummary is the viewer-facing view of a channel: enough to pick one in
// the room's media library and to see whether it is up. It deliberately leaves out
// source_url, resolver config, fallbacks and diagnostics -- where a feed comes from
// is an operator concern, and the admin list is the place for it. The resolver is
// kept only as a label ("browser", "api", ...) of how the stream is obtained.
type LiveChannelSummary struct {
	MediaAssetID string            `json:"media_asset_id"`
	Slug         string            `json:"slug"`
	Title        string            `json:"title"`
	Resolver     string            `json:"resolver"`
	Status       LiveChannelStatus `json:"status"`
	IsDVR        bool              `json:"is_dvr"`
}

// Summary projects a channel onto what a viewer may see.
func (c *LiveChannel) Summary() LiveChannelSummary {
	return LiveChannelSummary{
		MediaAssetID: c.MediaAssetID,
		Slug:         c.Slug,
		Title:        c.Title,
		Resolver:     c.Resolver,
		Status:       c.Status,
		IsDVR:        c.IsDVR,
	}
}
