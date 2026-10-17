package playlist

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrInvalidName        = errors.New("a playlist needs a name of up to 100 characters")
	ErrDescriptionTooLong = errors.New("a playlist description can be up to 1000 characters")
	ErrTooManyPlaylists   = errors.New("you have reached the limit of 100 playlists")
	ErrPlaylistFull       = errors.New("a playlist can hold up to 500 videos")
	ErrMediaNotFound      = errors.New("that video is not in the library")
	ErrMediaNotPlayable   = errors.New("only finished, on-demand videos can be added to a playlist")
	ErrInvalidOrder       = errors.New("the new order must list every video in the playlist exactly once")
)

const (
	maxNameLength        = 100
	maxDescriptionLength = 1000
	maxPlaylistsPerUser  = 100
	// MaxItems bounds a playlist, and with it a room's play queue (see ws.Hub),
	// which is sent to every member whenever it changes.
	MaxItems = 500
)

// MediaLookup is the part of the media library the service needs: whether a
// video exists and can be played. media.Repository satisfies it.
type MediaLookup interface {
	GetAssetByID(ctx context.Context, id string) (*domain.MediaAsset, error)
}

// Service is the business logic contract for playlists. Every method that takes
// a playlist ID also takes the caller's user ID: playlists are private, and one
// the caller does not own is reported as not found rather than forbidden, so
// IDs cannot be probed for existence.
type Service interface {
	List(ctx context.Context, ownerID string) ([]*domain.Playlist, error)
	Get(ctx context.Context, id, ownerID string) (*domain.Playlist, error)
	Create(ctx context.Context, ownerID, name, description string) (*domain.Playlist, error)
	Update(ctx context.Context, id, ownerID, name, description string) (*domain.Playlist, error)
	Delete(ctx context.Context, id, ownerID string) error
	AddItem(ctx context.Context, id, ownerID, mediaAssetID string) (*domain.Playlist, error)
	RemoveItem(ctx context.Context, id, ownerID, itemID string) (*domain.Playlist, error)
	Reorder(ctx context.Context, id, ownerID string, itemIDs []string) (*domain.Playlist, error)
}

type service struct {
	repo  Repository
	media MediaLookup
}

// NewService constructs the playlist service over its repository and the media library.
func NewService(repo Repository, media MediaLookup) Service {
	return &service{repo: repo, media: media}
}

func (s *service) List(ctx context.Context, ownerID string) ([]*domain.Playlist, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *service) Get(ctx context.Context, id, ownerID string) (*domain.Playlist, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.OwnerID != ownerID {
		return nil, ErrPlaylistNotFound
	}
	return p, nil
}

func (s *service) Create(ctx context.Context, ownerID, name, description string) (*domain.Playlist, error) {
	name, description, err := cleanDetails(name, description)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= maxPlaylistsPerUser {
		return nil, ErrTooManyPlaylists
	}

	p := &domain.Playlist{OwnerID: ownerID, Name: name, Description: description}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *service) Update(ctx context.Context, id, ownerID, name, description string) (*domain.Playlist, error) {
	if _, err := s.Get(ctx, id, ownerID); err != nil {
		return nil, err
	}
	name, description, err := cleanDetails(name, description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, name, description); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *service) Delete(ctx context.Context, id, ownerID string) error {
	if _, err := s.Get(ctx, id, ownerID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) AddItem(ctx context.Context, id, ownerID, mediaAssetID string) (*domain.Playlist, error) {
	p, err := s.Get(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	if len(p.Items) >= MaxItems {
		return nil, ErrPlaylistFull
	}

	asset, err := s.media.GetAssetByID(ctx, mediaAssetID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("failed to look up media: %w", err)
	}
	// A playlist plays through, one video after another; a live channel never
	// ends, and a video still transcoding cannot start.
	if asset.Kind == domain.MediaKindLive || asset.Status != domain.MediaStatusReady {
		return nil, ErrMediaNotPlayable
	}

	if err := s.repo.AddItem(ctx, id, mediaAssetID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *service) RemoveItem(ctx context.Context, id, ownerID, itemID string) (*domain.Playlist, error) {
	if _, err := s.Get(ctx, id, ownerID); err != nil {
		return nil, err
	}
	if err := s.repo.RemoveItem(ctx, id, itemID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *service) Reorder(ctx context.Context, id, ownerID string, itemIDs []string) (*domain.Playlist, error) {
	p, err := s.Get(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}

	// The order has to be a permutation of what is there now. Anything else is a
	// client working from a stale copy, and applying it would either lose items
	// or leave two at one position.
	if len(itemIDs) != len(p.Items) {
		return nil, ErrInvalidOrder
	}
	current := make(map[string]bool, len(p.Items))
	for _, item := range p.Items {
		current[item.ID] = true
	}
	for _, itemID := range itemIDs {
		if !current[itemID] {
			return nil, ErrInvalidOrder
		}
		delete(current, itemID)
	}

	if err := s.repo.Reorder(ctx, id, itemIDs); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func cleanDetails(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return "", "", ErrInvalidName
	}
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return "", "", ErrDescriptionTooLong
	}
	return name, description, nil
}
