package domain

import "time"

// Playlist is an ordered list of library videos owned by one user.
//
// Items are always populated on reads: the library needs them to show which
// playlists already hold a video, and a user has few enough playlists that
// returning them whole is cheaper than a second round trip per playlist.
type Playlist struct {
	ID          string          `json:"id"`
	OwnerID     string          `json:"owner_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Items       []*PlaylistItem `json:"items"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// PlaylistItem is one video's place in a playlist.
type PlaylistItem struct {
	ID           string      `json:"id"`
	PlaylistID   string      `json:"playlist_id"`
	MediaAssetID string      `json:"media_asset_id"`
	Position     int         `json:"position"`
	AddedAt      time.Time   `json:"added_at"`
	Media        *MediaAsset `json:"media"`
}

// PlaybackQueue is what a room plays through, in order: usually a playlist that
// a member started. It is a snapshot taken when the queue was started, so
// editing the playlist afterwards does not move a room that is mid-way through
// it, and every member sees the same list whoever owns the playlist.
//
// The hub treats a queue as immutable and replaces it on every change. Its state
// is persisted from other goroutines, which must never see it half-updated.
type PlaybackQueue struct {
	PlaylistID string      `json:"playlist_id,omitempty"`
	Title      string      `json:"title"`
	Items      []QueueItem `json:"items"`
	// Index is the item the room is on now.
	Index int `json:"index"`
}

// QueueItem is one entry of a PlaybackQueue: enough to play it and to show it in
// an "up next" list without another lookup.
type QueueItem struct {
	MediaURL        string `json:"media_url"`
	Title           string `json:"title"`
	ThumbnailURL    string `json:"thumbnail_url,omitempty"`
	MediaAssetID    string `json:"media_asset_id,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}
