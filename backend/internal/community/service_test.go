package community_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/community"
	"github.com/inox/inox/backend/internal/domain"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type mockRepository struct {
	posts  map[string]*domain.CommunityPost
	nextID int
}

func newMockRepository() *mockRepository {
	return &mockRepository{posts: make(map[string]*domain.CommunityPost)}
}

func clonePost(p *domain.CommunityPost) *domain.CommunityPost {
	copied := *p
	if p.PublishedAt != nil {
		at := *p.PublishedAt
		copied.PublishedAt = &at
	}
	return &copied
}

func (m *mockRepository) Create(ctx context.Context, p *domain.CommunityPost) error {
	m.nextID++
	p.ID = fmt.Sprintf("post-%d", m.nextID)
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	m.posts[p.ID] = clonePost(p)
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.CommunityPost, error) {
	p, ok := m.posts[id]
	if !ok {
		return nil, community.ErrPostNotFound
	}
	return clonePost(p), nil
}

// List mirrors the SQL ordering: drafts, then pinned, then newest first.
func (m *mockRepository) List(ctx context.Context, opts community.ListOptions) ([]*domain.CommunityPost, error) {
	out := []*domain.CommunityPost{}
	for _, p := range m.posts {
		if (opts.IncludeDrafts || p.PublishedAt != nil) && (opts.Category == "" || p.Category == opts.Category) {
			out = append(out, clonePost(p))
		}
	}
	sortKey := func(p *domain.CommunityPost) time.Time {
		if p.PublishedAt != nil {
			return *p.PublishedAt
		}
		return p.UpdatedAt
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.PublishedAt == nil) != (b.PublishedAt == nil) {
			return a.PublishedAt == nil
		}
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		return sortKey(a).After(sortKey(b))
	})
	if opts.Offset >= len(out) {
		return []*domain.CommunityPost{}, nil
	}
	out = out[opts.Offset:]
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (m *mockRepository) Update(ctx context.Context, p *domain.CommunityPost) error {
	if _, ok := m.posts[p.ID]; !ok {
		return community.ErrPostNotFound
	}
	p.UpdatedAt = time.Now()
	m.posts[p.ID] = clonePost(p)
	return nil
}

func (m *mockRepository) Delete(ctx context.Context, id string) error {
	if _, ok := m.posts[id]; !ok {
		return community.ErrPostNotFound
	}
	delete(m.posts, id)
	return nil
}

// clock hands out a later time on every call, so publish order is unambiguous.
type clock struct{ t time.Time }

func (c *clock) now() time.Time {
	c.t = c.t.Add(time.Minute)
	return c.t
}

func newService() (community.Service, *mockRepository, *clock) {
	repo := newMockRepository()
	c := &clock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	return community.NewServiceWithClock(repo, c.now), repo, c
}

func publish(t *testing.T, svc community.Service, title string, pinned bool) *domain.CommunityPost {
	t.Helper()
	p, err := svc.Create(context.Background(), "admin-1", community.PostInput{
		Category: domain.CommunityCategoryFeature, Title: title, Body: "body", Pinned: pinned, Published: true,
	})
	if err != nil {
		t.Fatalf("Create(%q): %v", title, err)
	}
	return p
}

func titles(posts []*domain.CommunityPost) string {
	names := make([]string, len(posts))
	for i, p := range posts {
		names[i] = p.Title
	}
	return strings.Join(names, ",")
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestCreate_ValidatesAndDefaults(t *testing.T) {
	svc, _, _ := newService()
	ctx := context.Background()

	p, err := svc.Create(ctx, "admin-1", community.PostInput{Title: "  Hello  ", Body: " world "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Title != "Hello" || p.Body != "world" {
		t.Errorf("title/body not trimmed: %q / %q", p.Title, p.Body)
	}
	if p.Category != domain.CommunityCategoryAnnouncement {
		t.Errorf("category = %q, want the announcement default", p.Category)
	}
	if p.PublishedAt != nil {
		t.Error("an unpublished post must be a draft")
	}
	if p.AuthorID != "admin-1" {
		t.Errorf("author = %q, want admin-1", p.AuthorID)
	}

	for name, tc := range map[string]struct {
		in   community.PostInput
		want error
	}{
		"blank title":    {community.PostInput{Title: "  ", Body: "b"}, community.ErrInvalidTitle},
		"long title":     {community.PostInput{Title: strings.Repeat("x", 141), Body: "b"}, community.ErrInvalidTitle},
		"blank body":     {community.PostInput{Title: "t", Body: "\n"}, community.ErrInvalidBody},
		"long body":      {community.PostInput{Title: "t", Body: strings.Repeat("x", 10001)}, community.ErrInvalidBody},
		"bad category":   {community.PostInput{Title: "t", Body: "b", Category: "rumour"}, community.ErrInvalidCategory},
		"cased category": {community.PostInput{Title: "t", Body: "b", Category: " Fix "}, nil},
	} {
		_, err := svc.Create(ctx, "admin-1", tc.in)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestFeed_HidesDraftsAndOrdersPinnedFirst(t *testing.T) {
	svc, _, _ := newService()
	ctx := context.Background()

	publish(t, svc, "old", false)
	publish(t, svc, "pinned", true)
	publish(t, svc, "new", false)
	if _, err := svc.Create(ctx, "admin-1", community.PostInput{Title: "draft", Body: "b"}); err != nil {
		t.Fatal(err)
	}

	posts, hasMore, err := svc.Feed(ctx, "", 0, 0)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if got := titles(posts); got != "pinned,new,old" {
		t.Errorf("feed = %s, want pinned,new,old", got)
	}
	if hasMore {
		t.Error("has_more on a feed that fits in one page")
	}

	all, err := svc.AdminList(ctx)
	if err != nil {
		t.Fatalf("AdminList: %v", err)
	}
	if got := titles(all); got != "draft,pinned,new,old" {
		t.Errorf("admin list = %s, want the draft first", got)
	}
}

func TestFeed_Pages(t *testing.T) {
	svc, _, _ := newService()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		publish(t, svc, fmt.Sprintf("p%d", i), false)
	}

	first, more, err := svc.Feed(ctx, "", 2, 0)
	if err != nil || titles(first) != "p4,p3" || !more {
		t.Fatalf("page 1 = %s more=%v err=%v", titles(first), more, err)
	}
	last, more, err := svc.Feed(ctx, "", 2, 4)
	if err != nil || titles(last) != "p0" || more {
		t.Fatalf("page 3 = %s more=%v err=%v", titles(last), more, err)
	}
	capped, _, _ := svc.Feed(ctx, "", 1000, -3)
	if len(capped) != 5 {
		t.Errorf("an oversized limit and negative offset should still return the feed, got %d", len(capped))
	}
}

func TestUpdate_KeepsPublishDateThroughEdits(t *testing.T) {
	svc, _, _ := newService()
	ctx := context.Background()

	p := publish(t, svc, "launch", false)
	published := *p.PublishedAt

	edited, err := svc.Update(ctx, p.ID, community.PostInput{Title: "launch!", Body: "fixed typo", Published: true})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if edited.Title != "launch!" || !edited.PublishedAt.Equal(published) {
		t.Errorf("an edit moved the publish date: %v -> %v", published, edited.PublishedAt)
	}

	unpublished, err := svc.Update(ctx, p.ID, community.PostInput{Title: "launch!", Body: "b", Published: false})
	if err != nil || unpublished.PublishedAt != nil {
		t.Fatalf("unpublishing should make a draft, got %v (err %v)", unpublished.PublishedAt, err)
	}

	republished, err := svc.Update(ctx, p.ID, community.PostInput{Title: "launch!", Body: "b", Published: true})
	if err != nil || republished.PublishedAt == nil || !republished.PublishedAt.After(published) {
		t.Fatalf("republishing should stamp a new date, got %v (err %v)", republished.PublishedAt, err)
	}
}

func TestUpdateAndDelete_UnknownPost(t *testing.T) {
	svc, _, _ := newService()
	ctx := context.Background()

	if _, err := svc.Update(ctx, "nope", community.PostInput{Title: "t", Body: "b"}); !errors.Is(err, community.ErrPostNotFound) {
		t.Errorf("Update: err = %v, want ErrPostNotFound", err)
	}
	if err := svc.Delete(ctx, "nope"); !errors.Is(err, community.ErrPostNotFound) {
		t.Errorf("Delete: err = %v, want ErrPostNotFound", err)
	}

	p := publish(t, svc, "gone soon", false)
	if err := svc.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if posts, _, _ := svc.Feed(ctx, "", 0, 0); len(posts) != 0 {
		t.Errorf("deleted post still in the feed: %s", titles(posts))
	}
}

func TestFeed_FiltersByCategory(t *testing.T) {
	svc, _, _ := newService()
	ctx := context.Background()
	publish(t, svc, "feature", false)
	if _, err := svc.Create(ctx, "admin-1", community.PostInput{
		Category: domain.CommunityCategoryFix, Title: "fix", Body: "b", Published: true,
	}); err != nil {
		t.Fatal(err)
	}

	fixes, _, err := svc.Feed(ctx, " FIX ", 0, 0)
	if err != nil || titles(fixes) != "fix" {
		t.Errorf("fix feed = %s (err %v), want just the fix", titles(fixes), err)
	}
	if _, _, err := svc.Feed(ctx, "rumour", 0, 0); !errors.Is(err, community.ErrInvalidCategory) {
		t.Errorf("unknown category: err = %v, want ErrInvalidCategory", err)
	}
}
