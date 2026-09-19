package domain

import "time"

type FriendshipStatus string

const (
	FriendshipStatusPending  FriendshipStatus = "pending"
	FriendshipStatusAccepted FriendshipStatus = "accepted"
	FriendshipStatusDeclined FriendshipStatus = "declined"
)

// Friendship is the stored relationship between two users. It is the only row that
// exists for a pair, whatever state that pair is in: requester_id asked
// addressee_id, and status records how that turned out.
type Friendship struct {
	ID          string           `json:"id"`
	RequesterID string           `json:"requester_id"`
	AddresseeID string           `json:"addressee_id"`
	Status      FriendshipStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Other returns the id of whichever party is not userID, so callers can render a
// friendship from one viewer's side without caring which way the request ran.
func (f *Friendship) Other(userID string) string {
	if f.RequesterID == userID {
		return f.AddresseeID
	}
	return f.RequesterID
}

// Involves reports whether userID is one of the two parties.
func (f *Friendship) Involves(userID string) bool {
	return f.RequesterID == userID || f.AddresseeID == userID
}

// Friend is one entry in a user's friends list: the *other* person, never the
// viewer, which is why it carries a user id rather than the pair.
type Friend struct {
	FriendshipID string    `json:"friendship_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	FriendsSince time.Time `json:"friends_since"`
}

type FriendRequestDirection string

const (
	FriendRequestIncoming FriendRequestDirection = "incoming"
	FriendRequestOutgoing FriendRequestDirection = "outgoing"
)

// FriendRequest is a pending friendship as seen by one viewer. Direction says
// whether the viewer is waiting on an answer or owes one.
type FriendRequest struct {
	ID        string                 `json:"id"`
	Direction FriendRequestDirection `json:"direction"`
	UserID    string                 `json:"user_id"`
	Username  string                 `json:"username"`
	AvatarURL *string                `json:"avatar_url,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// FriendRequestOutcome reports what sending a request actually did. Sending to
// someone who has already asked you is an acceptance, not a second request, so the
// caller needs to know which of the two it got back.
type FriendRequestOutcome struct {
	Status  FriendshipStatus `json:"status"`
	Request *FriendRequest   `json:"request,omitempty"`
	Friend  *Friend          `json:"friend,omitempty"`
}

// Relationship states reported by user search, so the add-friend UI can show the
// right action per result instead of offering "Add" to an existing friend.
const (
	RelationshipNone            = "none"
	RelationshipFriends         = "friends"
	RelationshipRequestSent     = "request_sent"
	RelationshipRequestReceived = "request_received"
)

// UserSearchResult is a directory hit annotated with the viewer's relationship to it.
type UserSearchResult struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
	Relationship string  `json:"relationship"`
}
