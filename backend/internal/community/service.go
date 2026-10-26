package community

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrInvalidTitle    = errors.New("a post needs a title of up to 140 characters")
	ErrInvalidBody     = errors.New("a post needs a body of up to 10000 characters")
	ErrInvalidCategory = errors.New("category must be one of feature, improvement, fix or announcement")
)

const (
	maxTitleLength = 140
	maxBodyLength  = 10000

	// DefaultFeedLimit and MaxFeedLimit bound one page of the viewer feed.
	DefaultFeedLimit = 20
	MaxFeedLimit     = 50
	// adminListLimit caps the admin list, which is not paginated.
	adminListLimit = 500
)

var categories = map[string]bool{
	domain.CommunityCategoryFeature:      true,
	domain.CommunityCategoryImprovement:  true,
	domain.CommunityCategoryFix:          true,
	domain.CommunityCategoryAnnouncement: true,
}

// PostInput is everything an admin sets on a post. Saving with Published false
// makes (or turns it back into) a draft.
type PostInput struct {
	Category  string
	Title     string
	Body      string
	Pinned    bool
	Published bool
}

// Service is the business logic contract for community posts. Feed is what
// viewers read; everything else is for admins, which the router enforces.
type Service interface {
	// Feed returns one page of published posts and whether there are more. An
	// empty category means every category.
	Feed(ctx context.Context, category string, limit, offset int) ([]*domain.CommunityPost, bool, error)
	AdminList(ctx context.Context) ([]*domain.CommunityPost, error)
	Create(ctx context.Context, authorID string, in PostInput) (*domain.CommunityPost, error)
	Update(ctx context.Context, id string, in PostInput) (*domain.CommunityPost, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo Repository
	now  func() time.Time
}

// NewService constructs the community post service over its repository.
func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

// NewServiceWithClock is NewService with a fixed clock, for tests.
func NewServiceWithClock(repo Repository, now func() time.Time) Service {
	return &service{repo: repo, now: now}
}

func (s *service) Feed(ctx context.Context, category string, limit, offset int) ([]*domain.CommunityPost, bool, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category != "" && !categories[category] {
		return nil, false, ErrInvalidCategory
	}
	if limit <= 0 {
		limit = DefaultFeedLimit
	}
	if limit > MaxFeedLimit {
		limit = MaxFeedLimit
	}
	if offset < 0 {
		offset = 0
	}
	// One extra row says whether another page exists without a COUNT.
	posts, err := s.repo.List(ctx, ListOptions{Category: category, Limit: limit + 1, Offset: offset})
	if err != nil {
		return nil, false, err
	}
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}
	return posts, hasMore, nil
}

func (s *service) AdminList(ctx context.Context) ([]*domain.CommunityPost, error) {
	return s.repo.List(ctx, ListOptions{IncludeDrafts: true, Limit: adminListLimit})
}

func (s *service) Create(ctx context.Context, authorID string, in PostInput) (*domain.CommunityPost, error) {
	in, err := clean(in)
	if err != nil {
		return nil, err
	}
	p := &domain.CommunityPost{
		AuthorID: authorID,
		Category: in.Category,
		Title:    in.Title,
		Body:     in.Body,
		Pinned:   in.Pinned,
	}
	if in.Published {
		now := s.now()
		p.PublishedAt = &now
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, p.ID)
}

func (s *service) Update(ctx context.Context, id string, in PostInput) (*domain.CommunityPost, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	in, err = clean(in)
	if err != nil {
		return nil, err
	}
	p.Category, p.Title, p.Body, p.Pinned = in.Category, in.Title, in.Body, in.Pinned

	// A post keeps the date it was first published through later edits, so
	// fixing a typo does not resurface it as new. Unpublishing forgets the date:
	// publishing again later is a fresh announcement.
	switch {
	case !in.Published:
		p.PublishedAt = nil
	case p.PublishedAt == nil:
		now := s.now()
		p.PublishedAt = &now
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func clean(in PostInput) (PostInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	in.Category = strings.ToLower(strings.TrimSpace(in.Category))
	if in.Category == "" {
		in.Category = domain.CommunityCategoryAnnouncement
	}
	if in.Title == "" || utf8.RuneCountInString(in.Title) > maxTitleLength {
		return in, ErrInvalidTitle
	}
	if in.Body == "" || utf8.RuneCountInString(in.Body) > maxBodyLength {
		return in, ErrInvalidBody
	}
	if !categories[in.Category] {
		return in, ErrInvalidCategory
	}
	return in, nil
}
