package playlist_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/playlist"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type mockPlaylistRepository struct {
	playlists map[string]*domain.Playlist
	media     *mockMediaLookup
	nextID    int
}

func newMockPlaylistRepository(media *mockMediaLookup) *mockPlaylistRepository {
	return &mockPlaylistRepository{playlists: make(map[string]*domain.Playlist), media: media}
}

func (m *mockPlaylistRepository) id(prefix string) string {
	m.nextID++
	return fmt.Sprintf("%s-%d", prefix, m.nextID)
}

func clonePlaylist(p *domain.Playlist) *domain.Playlist {
	copied := *p
	copied.Items = make([]*domain.PlaylistItem, len(p.Items))
	for i, item := range p.Items {
		c := *item
		copied.Items[i] = &c
	}
	return &copied
}

func (m *mockPlaylistRepository) Create(ctx context.Context, p *domain.Playlist) error {
	p.ID = m.id("playlist")
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	p.Items = []*domain.PlaylistItem{}
	m.playlists[p.ID] = clonePlaylist(p)
	return nil
}

func (m *mockPlaylistRepository) GetByID(ctx context.Context, id string) (*domain.Playlist, error) {
	p, ok := m.playlists[id]
	if !ok {
		return nil, playlist.ErrPlaylistNotFound
	}
	return clonePlaylist(p), nil
}

func (m *mockPlaylistRepository) ListByOwner(ctx context.Context, ownerID string) ([]*domain.Playlist, error) {
	out := []*domain.Playlist{}
	for _, p := range m.playlists {
		if p.OwnerID == ownerID {
			out = append(out, clonePlaylist(p))
		}
	}
	return out, nil
}

func (m *mockPlaylistRepository) Update(ctx context.Context, id, name, description string) error {
	p, ok := m.playlists[id]
	if !ok {
		return playlist.ErrPlaylistNotFound
	}
	p.Name, p.Description = name, description
	return nil
}

func (m *mockPlaylistRepository) Delete(ctx context.Context, id string) error {
	if _, ok := m.playlists[id]; !ok {
		return playlist.ErrPlaylistNotFound
	}
	delete(m.playlists, id)
	return nil
}

func (m *mockPlaylistRepository) AddItem(ctx context.Context, playlistID, mediaAssetID string) error {
	p, ok := m.playlists[playlistID]
	if !ok {
		return playlist.ErrPlaylistNotFound
	}
	for _, item := range p.Items {
		if item.MediaAssetID == mediaAssetID {
			return playlist.ErrDuplicateItem
		}
	}
	p.Items = append(p.Items, &domain.PlaylistItem{
		ID:           m.id("item"),
		PlaylistID:   playlistID,
		MediaAssetID: mediaAssetID,
		Position:     len(p.Items),
		Media:        m.media.assets[mediaAssetID],
	})
	return nil
}

func (m *mockPlaylistRepository) RemoveItem(ctx context.Context, playlistID, itemID string) error {
	p := m.playlists[playlistID]
	for i, item := range p.Items {
		if item.ID == itemID {
			p.Items = append(p.Items[:i], p.Items[i+1:]...)
			for j, rest := range p.Items {
				rest.Position = j
			}
			return nil
		}
	}
	return playlist.ErrItemNotFound
}

func (m *mockPlaylistRepository) Reorder(ctx context.Context, playlistID string, itemIDs []string) error {
	p := m.playlists[playlistID]
	order := make(map[string]int, len(itemIDs))
	for i, id := range itemIDs {
		order[id] = i
	}
	for _, item := range p.Items {
		item.Position = order[item.ID]
	}
	sort.Slice(p.Items, func(a, b int) bool { return p.Items[a].Position < p.Items[b].Position })
	return nil
}

type mockMediaLookup struct {
	assets map[string]*domain.MediaAsset
}

func (m *mockMediaLookup) GetAssetByID(ctx context.Context, id string) (*domain.MediaAsset, error) {
	a, ok := m.assets[id]
	if !ok {
		return nil, errors.New("media asset not found")
	}
	return a, nil
}

func newTestService() (playlist.Service, *mockPlaylistRepository) {
	media := &mockMediaLookup{assets: map[string]*domain.MediaAsset{
		"film-1":   {ID: "film-1", Kind: domain.MediaKindVOD, Status: domain.MediaStatusReady, Title: "One"},
		"film-2":   {ID: "film-2", Kind: domain.MediaKindVOD, Status: domain.MediaStatusReady, Title: "Two"},
		"film-3":   {ID: "film-3", Kind: domain.MediaKindVOD, Status: domain.MediaStatusReady, Title: "Three"},
		"encoding": {ID: "encoding", Kind: domain.MediaKindVOD, Status: domain.MediaStatusProcessing},
		"channel":  {ID: "channel", Kind: domain.MediaKindLive, Status: domain.MediaStatusReady},
	}}
	repo := newMockPlaylistRepository(media)
	return playlist.NewService(repo, media), repo
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestCreateValidatesAndTrimsDetails(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	for _, name := range []string{"", "   ", strings.Repeat("x", 101)} {
		if _, err := svc.Create(ctx, "alice", name, ""); !errors.Is(err, playlist.ErrInvalidName) {
			t.Errorf("Create(name=%q) error = %v, want ErrInvalidName", name, err)
		}
	}
	if _, err := svc.Create(ctx, "alice", "Fine", strings.Repeat("d", 1001)); !errors.Is(err, playlist.ErrDescriptionTooLong) {
		t.Errorf("long description error = %v, want ErrDescriptionTooLong", err)
	}

	p, err := svc.Create(ctx, "alice", "  Friday films  ", "  for the group ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Name != "Friday films" || p.Description != "for the group" {
		t.Errorf("details not trimmed: %q / %q", p.Name, p.Description)
	}
	if p.Items == nil {
		t.Error("a new playlist should carry an empty item list, not null")
	}
}

func TestPlaylistsAreInvisibleToOtherUsers(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", "Mine", "")

	if _, err := svc.Get(ctx, p.ID, "bob"); !errors.Is(err, playlist.ErrPlaylistNotFound) {
		t.Errorf("Get by non-owner error = %v, want ErrPlaylistNotFound", err)
	}
	if _, err := svc.AddItem(ctx, p.ID, "bob", "film-1"); !errors.Is(err, playlist.ErrPlaylistNotFound) {
		t.Errorf("AddItem by non-owner error = %v, want ErrPlaylistNotFound", err)
	}
	if err := svc.Delete(ctx, p.ID, "bob"); !errors.Is(err, playlist.ErrPlaylistNotFound) {
		t.Errorf("Delete by non-owner error = %v, want ErrPlaylistNotFound", err)
	}
	if lists, _ := svc.List(ctx, "bob"); len(lists) != 0 {
		t.Errorf("bob sees %d playlists, want 0", len(lists))
	}
}

func TestAddItemOnlyAcceptsPlayableVideos(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", "Mix", "")

	cases := map[string]error{
		"missing":  playlist.ErrMediaNotFound,
		"encoding": playlist.ErrMediaNotPlayable,
		"channel":  playlist.ErrMediaNotPlayable,
	}
	for mediaID, want := range cases {
		if _, err := svc.AddItem(ctx, p.ID, "alice", mediaID); !errors.Is(err, want) {
			t.Errorf("AddItem(%s) error = %v, want %v", mediaID, err, want)
		}
	}

	got, err := svc.AddItem(ctx, p.ID, "alice", "film-1")
	if err != nil {
		t.Fatalf("AddItem(film-1): %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].MediaAssetID != "film-1" {
		t.Fatalf("items after add = %+v", got.Items)
	}
	if _, err := svc.AddItem(ctx, p.ID, "alice", "film-1"); !errors.Is(err, playlist.ErrDuplicateItem) {
		t.Errorf("second AddItem(film-1) error = %v, want ErrDuplicateItem", err)
	}
}

func TestReorderRequiresEveryItemExactlyOnce(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	p, _ := svc.Create(ctx, "alice", "Mix", "")
	for _, id := range []string{"film-1", "film-2", "film-3"} {
		p, _ = svc.AddItem(ctx, p.ID, "alice", id)
	}
	a, b, c := p.Items[0].ID, p.Items[1].ID, p.Items[2].ID

	for name, order := range map[string][]string{
		"missing one":  {a, b},
		"duplicated":   {a, a, b},
		"unknown item": {a, b, "item-999"},
	} {
		if _, err := svc.Reorder(ctx, p.ID, "alice", order); !errors.Is(err, playlist.ErrInvalidOrder) {
			t.Errorf("%s: error = %v, want ErrInvalidOrder", name, err)
		}
	}

	got, err := svc.Reorder(ctx, p.ID, "alice", []string{c, a, b})
	if err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	var titles []string
	for _, item := range got.Items {
		titles = append(titles, item.Media.Title)
	}
	if strings.Join(titles, ",") != "Three,One,Two" {
		t.Errorf("order after reorder = %v, want Three,One,Two", titles)
	}
}
