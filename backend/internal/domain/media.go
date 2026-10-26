package domain

import "time"

type MediaStatus string

const (
	MediaStatusPending    MediaStatus = "pending"
	MediaStatusProcessing MediaStatus = "processing"
	MediaStatusReady      MediaStatus = "ready"
	MediaStatusFailed     MediaStatus = "failed"
)

// MediaKind separates on-demand assets from live channels. Live assets carry no
// duration and never transcode; their hls_master_url points at the live proxy.
type MediaKind string

const (
	MediaKindVOD  MediaKind = "vod"
	MediaKindLive MediaKind = "live"
)

type MediaAsset struct {
	ID              string      `json:"id"`
	Kind            MediaKind   `json:"kind"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	SourceURL       string      `json:"source_url"`
	Status          MediaStatus `json:"status"`
	DurationSeconds int         `json:"duration_seconds"`
	Progress        int         `json:"progress"`
	ThumbnailURL    string      `json:"thumbnail_url"`
	HLSMasterURL    string      `json:"hls_master_url"`
	// TranscodeProfiles names the qualities an upload is transcoded to, such as
	// "720p". Empty means the full ladder.
	TranscodeProfiles []string          `json:"transcode_profiles,omitempty"`
	CreatedBy         *string           `json:"created_by"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Renditions        []*MediaRendition `json:"renditions,omitempty"`
	Captions          []*MediaCaption   `json:"captions,omitempty"`
}

// MediaCaption is one language's caption track for a video, stored as WebVTT.
type MediaCaption struct {
	ID           string `json:"id"`
	MediaAssetID string `json:"media_asset_id"`
	// Language is a BCP 47 tag such as "en" or "pt-BR".
	Language   string    `json:"language"`
	Label      string    `json:"label"`
	URL        string    `json:"url"`
	StorageKey string    `json:"-"`
	CueCount   int       `json:"cue_count"`
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MediaRendition struct {
	ID           string    `json:"id"`
	MediaAssetID string    `json:"media_asset_id"`
	Resolution   string    `json:"resolution"`
	BitrateKbps  int       `json:"bitrate_kbps"`
	PlaylistURL  string    `json:"playlist_url"`
	CreatedAt    time.Time `json:"created_at"`
}
