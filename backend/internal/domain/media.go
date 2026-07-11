package domain

import "time"

type MediaStatus string

const (
	MediaStatusPending    MediaStatus = "pending"
	MediaStatusProcessing MediaStatus = "processing"
	MediaStatusReady      MediaStatus = "ready"
	MediaStatusFailed     MediaStatus = "failed"
)

type MediaAsset struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	SourceURL       string            `json:"source_url"`
	Status          MediaStatus       `json:"status"`
	DurationSeconds int               `json:"duration_seconds"`
	Progress        int               `json:"progress"`
	ThumbnailURL    string            `json:"thumbnail_url"`
	HLSMasterURL    string            `json:"hls_master_url"`
	CreatedBy       *string           `json:"created_by"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Renditions      []*MediaRendition `json:"renditions,omitempty"`
}

type MediaRendition struct {
	ID           string    `json:"id"`
	MediaAssetID string    `json:"media_asset_id"`
	Resolution   string    `json:"resolution"`
	BitrateKbps  int       `json:"bitrate_kbps"`
	PlaylistURL  string    `json:"playlist_url"`
	CreatedAt    time.Time `json:"created_at"`
}
