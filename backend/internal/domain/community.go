package domain

import "time"

// Community post categories. Each gets its own label and colour on the
// Community page, so the set is closed (and enforced by the table's CHECK).
const (
	CommunityCategoryFeature      = "feature"
	CommunityCategoryImprovement  = "improvement"
	CommunityCategoryFix          = "fix"
	CommunityCategoryAnnouncement = "announcement"
)

// CommunityPost is a platform update written by an admin: a new feature, an
// improvement, a fix or an announcement.
//
// PublishedAt is nil while the post is a draft. Drafts are only ever returned
// to admins.
type CommunityPost struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Title    string `json:"title"`
	// Body is plain text. Blank lines separate paragraphs and lines starting
	// with "- " are list items; both apps render it that way, never as HTML.
	Body        string     `json:"body"`
	Pinned      bool       `json:"pinned"`
	PublishedAt *time.Time `json:"published_at"`
	// The author is empty once their account has been deleted.
	AuthorID        string    `json:"author_id"`
	AuthorUsername  string    `json:"author_username"`
	AuthorAvatarURL string    `json:"author_avatar_url"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
