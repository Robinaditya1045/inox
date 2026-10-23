package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/inox/inox/backend/internal/domain"
)

// ErrInvalidDetails rejects an edit to a video's title, description or
// thumbnail link. The wrapped message says which field and why.
var ErrInvalidDetails = errors.New("invalid media details")

// ErrInvalidCaptionTrack rejects a track's name.
var ErrInvalidCaptionTrack = errors.New("invalid caption track")

// ErrLiveCaptions rejects captions for a live channel, whose stream is not a
// fixed timeline a track could be timed against.
var ErrLiveCaptions = errors.New("live channels cannot have caption tracks")

const (
	maxTitleRunes        = 255 // media_assets.title is VARCHAR(255)
	maxDescriptionRunes  = 5000
	maxThumbnailURLBytes = 1024 // media_assets.thumbnail_url is VARCHAR(1024)
	maxCaptionLabelRunes = 100  // media_captions.label is VARCHAR(100)
)

// DetailsUpdate is an edit to a video's descriptive fields; nil leaves a field as is.
type DetailsUpdate struct {
	Title       *string
	Description *string
	// ThumbnailURL points the poster at an image hosted elsewhere. It is not
	// fetched or copied: the library shows it from wherever it lives.
	ThumbnailURL *string
}

// CaptionInput is a new caption track, or a replacement for a language's track.
type CaptionInput struct {
	Language  string
	Label     string
	Content   []byte
	IsDefault *bool
}

// CaptionUpdate edits an existing track; nil leaves a field as is.
type CaptionUpdate struct {
	Language  *string
	Label     *string
	Content   []byte
	IsDefault *bool
}

// attachCaptions fills in each asset's caption tracks. A failure only costs the
// tracks: listing the library matters more than labelling it, and a database
// that has not run the captions migration yet should still list videos.
func (s *mediaService) attachCaptions(ctx context.Context, assets []*domain.MediaAsset) {
	if len(assets) == 0 {
		return
	}
	ids := make([]string, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}
	byAsset, err := s.repo.ListCaptionsForAssets(ctx, ids)
	if err != nil {
		slog.Warn("could not load caption tracks for media assets", "error", err)
		return
	}
	for _, a := range assets {
		a.Captions = byAsset[a.ID]
	}
}

func (s *mediaService) UpdateDetails(ctx context.Context, assetID string, upd DetailsUpdate) (*domain.MediaAsset, error) {
	asset, err := s.repo.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}

	title, description := asset.Title, asset.Description
	if upd.Title != nil {
		title = strings.TrimSpace(*upd.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: a title is required", ErrInvalidDetails)
		}
		if utf8.RuneCountInString(title) > maxTitleRunes {
			return nil, fmt.Errorf("%w: the title is longer than %d characters", ErrInvalidDetails, maxTitleRunes)
		}
	}
	if upd.Description != nil {
		description = strings.TrimSpace(*upd.Description)
		if utf8.RuneCountInString(description) > maxDescriptionRunes {
			return nil, fmt.Errorf("%w: the description is longer than %d characters", ErrInvalidDetails, maxDescriptionRunes)
		}
	}
	var thumbnailURL string
	if upd.ThumbnailURL != nil {
		thumbnailURL = strings.TrimSpace(*upd.ThumbnailURL)
		if err := validateImageLink(thumbnailURL); err != nil {
			return nil, err
		}
	}

	if title != asset.Title || description != asset.Description {
		if err := s.repo.UpdateAssetDetails(ctx, assetID, title, description); err != nil {
			return nil, err
		}
	}
	if upd.ThumbnailURL != nil && thumbnailURL != asset.ThumbnailURL {
		if err := s.repo.SetThumbnail(ctx, assetID, thumbnailURL, false); err != nil {
			return nil, err
		}
	}
	return s.GetMediaByID(ctx, assetID)
}

func validateImageLink(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: the thumbnail link is empty", ErrInvalidDetails)
	}
	if len(raw) > maxThumbnailURLBytes {
		return fmt.Errorf("%w: the thumbnail link is longer than %d characters", ErrInvalidDetails, maxThumbnailURLBytes)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%w: the thumbnail link must be an http(s) URL", ErrInvalidDetails)
	}
	return nil
}

// ResetThumbnail drops a video's poster, uploaded or linked, and captures a
// frame from the video in its place. Until that lands -- and for good, if no
// frame can be taken -- the library draws its default poster.
func (s *mediaService) ResetThumbnail(ctx context.Context, assetID string) (*domain.MediaAsset, error) {
	asset, err := s.repo.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetThumbnail(ctx, assetID, "", false); err != nil {
		return nil, err
	}
	asset.ThumbnailURL = ""

	// A video still transcoding gets its frame from the transcoder; live
	// channels have none to give.
	if asset.Kind != domain.MediaKindLive && asset.Status == domain.MediaStatusReady {
		switch {
		case s.queue != nil:
			if err := s.queue.EnqueueGenerateThumbnail(ctx, assetID); err != nil {
				slog.Warn("failed to enqueue poster capture", "asset_id", assetID, "error", err)
			}
		case s.processor != nil && s.processor.CanCaptureFrames():
			go func() {
				if err := s.processor.GenerateThumbnail(context.Background(), asset); err != nil {
					slog.Warn("could not capture a poster frame", "asset_id", assetID, "error", err)
				}
			}()
		}
	}
	return s.GetMediaByID(ctx, assetID)
}

// CaptionsForMediaURL returns the tracks of whichever library video a room is
// playing, found by the URL the room has; nothing if it is not a library video.
func (s *mediaService) CaptionsForMediaURL(ctx context.Context, mediaURL string) ([]*domain.MediaCaption, error) {
	clean := strings.SplitN(strings.SplitN(mediaURL, "#", 2)[0], "?", 2)[0]
	if clean == "" {
		return []*domain.MediaCaption{}, nil
	}
	key := ""
	if strings.Contains(clean, "hls/") || strings.Contains(clean, "raw/") {
		key = extractStorageKey(clean)
	}
	id, err := s.repo.FindAssetIDByMediaURL(ctx, clean, key)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return []*domain.MediaCaption{}, nil
	}
	captions, err := s.repo.ListCaptions(ctx, id)
	if err != nil {
		return nil, err
	}
	if captions == nil {
		captions = []*domain.MediaCaption{}
	}
	return captions, nil
}

// SaveCaption stores a caption file as a language's track. A language the
// video already has captions in is replaced, keeping its default flag.
func (s *mediaService) SaveCaption(ctx context.Context, assetID string, in CaptionInput) (*domain.MediaCaption, error) {
	asset, err := s.repo.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.Kind == domain.MediaKindLive {
		return nil, ErrLiveCaptions
	}
	language, err := CanonicalLanguageTag(in.Language)
	if err != nil {
		return nil, err
	}
	label, err := captionLabel(in.Label, language)
	if err != nil {
		return nil, err
	}
	vtt, cues, err := NormalizeCaptions(in.Content)
	if err != nil {
		return nil, err
	}

	c := &domain.MediaCaption{MediaAssetID: assetID, Language: language, Label: label, CueCount: cues}
	if err := s.storeCaptionFile(ctx, c, vtt); err != nil {
		return nil, err
	}
	replacedKey, err := s.repo.UpsertCaption(ctx, c)
	if err != nil {
		_ = s.storage.DeleteFile(ctx, c.StorageKey)
		return nil, err
	}
	if replacedKey != "" && replacedKey != c.StorageKey {
		_ = s.storage.DeleteFile(ctx, replacedKey)
	}
	if in.IsDefault != nil {
		if err := s.repo.SetDefaultCaption(ctx, assetID, c.ID, *in.IsDefault); err != nil {
			return nil, err
		}
	}
	return s.repo.GetCaption(ctx, assetID, c.ID)
}

func (s *mediaService) UpdateCaption(ctx context.Context, assetID, captionID string, in CaptionUpdate) (*domain.MediaCaption, error) {
	c, err := s.repo.GetCaption(ctx, assetID, captionID)
	if err != nil {
		return nil, err
	}
	if in.Language != nil {
		if c.Language, err = CanonicalLanguageTag(*in.Language); err != nil {
			return nil, err
		}
	}
	if in.Label != nil {
		if c.Label, err = captionLabel(*in.Label, c.Language); err != nil {
			return nil, err
		}
	}

	replacedKey := ""
	if in.Content != nil {
		vtt, cues, err := NormalizeCaptions(in.Content)
		if err != nil {
			return nil, err
		}
		replacedKey = c.StorageKey
		c.CueCount = cues
		if err := s.storeCaptionFile(ctx, c, vtt); err != nil {
			return nil, err
		}
	}

	if err := s.repo.UpdateCaption(ctx, c); err != nil {
		if replacedKey != "" {
			_ = s.storage.DeleteFile(ctx, c.StorageKey)
		}
		return nil, err
	}
	if replacedKey != "" {
		_ = s.storage.DeleteFile(ctx, replacedKey)
	}
	if in.IsDefault != nil && *in.IsDefault != c.IsDefault {
		if err := s.repo.SetDefaultCaption(ctx, assetID, captionID, *in.IsDefault); err != nil {
			return nil, err
		}
	}
	return s.repo.GetCaption(ctx, assetID, captionID)
}

func (s *mediaService) DeleteCaption(ctx context.Context, assetID, captionID string) error {
	c, err := s.repo.DeleteCaption(ctx, assetID, captionID)
	if err != nil {
		return err
	}
	_ = s.storage.DeleteFile(ctx, c.StorageKey)
	return nil
}

// storeCaptionFile writes a WebVTT document under a fresh key and points the
// track at it. Fresh keys let the stream proxy cache tracks indefinitely: an
// edited track arrives under a new URL.
func (s *mediaService) storeCaptionFile(ctx context.Context, c *domain.MediaCaption, vtt string) error {
	key := fmt.Sprintf("captions/%s/%s-%s.vtt", c.MediaAssetID, c.Language, generateUUID()[:8])
	u, err := s.storage.SaveFile(ctx, key, strings.NewReader(vtt), int64(len(vtt)), "text/vtt; charset=utf-8")
	if err != nil {
		return fmt.Errorf("storage save failed: %w", err)
	}
	c.StorageKey, c.URL = key, u
	return nil
}

// captionLabel is what the player's captions menu shows for a track, the
// language tag itself when no name was given.
func captionLabel(label, language string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return language, nil
	}
	if utf8.RuneCountInString(label) > maxCaptionLabelRunes {
		return "", fmt.Errorf("%w: the track name is longer than %d characters", ErrInvalidCaptionTrack, maxCaptionLabelRunes)
	}
	return label, nil
}
