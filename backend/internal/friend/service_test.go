package friend_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/friend"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type mockFriendRepository struct {
	rows   map[string]*domain.Friendship
	nextID int
}

func newMockFriendRepository() *mockFriendRepository {
	return &mockFriendRepository{rows: make(map[string]*domain.Friendship)}
}

func (m *mockFriendRepository) Create(ctx context.Context, f *domain.Friendship) error {
	if existing, _ := m.GetByPair(ctx, f.RequesterID, f.AddresseeID); existing != nil {
		return friend.ErrPairConflict
	}
	m.nextID++
	f.ID = fmt.Sprintf("friendship-%d", m.nextID)
	f.CreatedAt = time.Now()
	f.UpdatedAt = f.CreatedAt
	stored := *f
	m.rows[f.ID] = &stored
	return nil
}

func (m *mockFriendRepository) GetByID(ctx context.Context, id string) (*domain.Friendship, error) {
	f, ok := m.rows[id]
	if !ok {
		return nil, friend.ErrFriendshipNotFound
	}
	copied := *f
	return &copied, nil
}

func (m *mockFriendRepository) GetByPair(ctx context.Context, userA, userB string) (*domain.Friendship, error) {
	for _, f := range m.rows {
		if f.Involves(userA) && f.Involves(userB) {
			copied := *f
			return &copied, nil
		}
	}
	return nil, friend.ErrFriendshipNotFound
}

func (m *mockFriendRepository) UpdateStatus(ctx context.Context, id string, status domain.FriendshipStatus) error {
	f, ok := m.rows[id]
	if !ok {
		return friend.ErrFriendshipNotFound
	}
	f.Status = status
	f.UpdatedAt = time.Now()
	return nil
}

func (m *mockFriendRepository) Reopen(ctx context.Context, id, requesterID, addresseeID string) error {
	f, ok := m.rows[id]
	if !ok {
		return friend.ErrFriendshipNotFound
	}
	f.RequesterID = requesterID
	f.AddresseeID = addresseeID
	f.Status = domain.FriendshipStatusPending
	f.UpdatedAt = time.Now()
	return nil
}

func (m *mockFriendRepository) Delete(ctx context.Context, id string) error {
	if _, ok := m.rows[id]; !ok {
		return friend.ErrFriendshipNotFound
	}
	delete(m.rows, id)
	return nil
}

func (m *mockFriendRepository) ListFriends(ctx context.Context, userID string) ([]*domain.Friend, error) {
	out := []*domain.Friend{}
	for _, f := range m.rows {
		if f.Status == domain.FriendshipStatusAccepted && f.Involves(userID) {
			out = append(out, &domain.Friend{
				FriendshipID: f.ID,
				UserID:       f.Other(userID),
				Username:     "user-" + f.Other(userID),
				FriendsSince: f.UpdatedAt,
			})
		}
	}
	return out, nil
}

func (m *mockFriendRepository) ListPendingRequests(ctx context.Context, userID string) ([]*domain.FriendRequest, error) {
	out := []*domain.FriendRequest{}
	for _, f := range m.rows {
		if f.Status != domain.FriendshipStatusPending || !f.Involves(userID) {
			continue
		}
		direction := domain.FriendRequestIncoming
		if f.RequesterID == userID {
			direction = domain.FriendRequestOutgoing
		}
		out = append(out, &domain.FriendRequest{
			ID:        f.ID,
			Direction: direction,
			UserID:    f.Other(userID),
			Username:  "user-" + f.Other(userID),
			CreatedAt: f.CreatedAt,
		})
	}
	return out, nil
}

func (m *mockFriendRepository) SearchUsers(ctx context.Context, viewerID, query string, limit int) ([]*domain.UserSearchResult, error) {
	return []*domain.UserSearchResult{{UserID: "searched", Username: query}}, nil
}

type mockUserRepository struct {
	users map[string]*domain.User // keyed by username
}

func newMockUserRepository(usernames ...string) *mockUserRepository {
	m := &mockUserRepository{users: make(map[string]*domain.User)}
	for _, name := range usernames {
		m.users[name] = &domain.User{ID: "id-" + name, Username: name, Email: name + "@inox.test"}
	}
	return m
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, auth.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, auth.ErrUserNotFound
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, auth.ErrUserNotFound
}
func (m *mockUserRepository) UpdateAvatar(ctx context.Context, userID, avatarURL string) error {
	return nil
}
func (m *mockUserRepository) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	return nil
}
func (m *mockUserRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	return "", nil
}
func (m *mockUserRepository) DeletePasswordResetToken(ctx context.Context, tokenHash string) error {
	return nil
}
func (m *mockUserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return nil
}

func newTestService(usernames ...string) (friend.Service, *mockFriendRepository) {
	repo := newMockFriendRepository()
	return friend.NewService(repo, newMockUserRepository(usernames...)), repo
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestSendRequest_CreatesPendingRequest(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, err := svc.SendRequest(ctx, "id-alice", "bob")
	if err != nil {
		t.Fatalf("unexpected error sending friend request: %v", err)
	}
	if outcome.Status != domain.FriendshipStatusPending {
		t.Errorf("expected pending outcome, got %q", outcome.Status)
	}
	if outcome.Request == nil || outcome.Request.UserID != "id-bob" {
		t.Fatalf("expected outgoing request addressed to bob, got %+v", outcome.Request)
	}
	if outcome.Request.Direction != domain.FriendRequestOutgoing {
		t.Errorf("expected outgoing direction, got %q", outcome.Request.Direction)
	}

	// Bob sees it as incoming, alice as outgoing.
	incoming, outgoing, err := svc.ListRequests(ctx, "id-bob")
	if err != nil {
		t.Fatalf("unexpected error listing bob's requests: %v", err)
	}
	if len(incoming) != 1 || len(outgoing) != 0 {
		t.Fatalf("expected bob to have 1 incoming and 0 outgoing, got %d/%d", len(incoming), len(outgoing))
	}
	if incoming[0].UserID != "id-alice" {
		t.Errorf("expected bob's incoming request to come from alice, got %q", incoming[0].UserID)
	}
}

func TestSendRequest_RejectsSelfAndUnknownUsers(t *testing.T) {
	svc, _ := newTestService("alice")
	ctx := context.Background()

	if _, err := svc.SendRequest(ctx, "id-alice", "alice"); !errors.Is(err, friend.ErrSelfFriendship) {
		t.Errorf("expected ErrSelfFriendship adding yourself, got %v", err)
	}
	if _, err := svc.SendRequest(ctx, "id-alice", "nobody"); !errors.Is(err, friend.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound for unknown username, got %v", err)
	}
	if _, err := svc.SendRequest(ctx, "id-alice", "   "); !errors.Is(err, friend.ErrInvalidUsername) {
		t.Errorf("expected ErrInvalidUsername for a blank username, got %v", err)
	}
}

func TestSendRequest_DuplicateIsRejected(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	if _, err := svc.SendRequest(ctx, "id-alice", "bob"); err != nil {
		t.Fatalf("unexpected error on first request: %v", err)
	}
	if _, err := svc.SendRequest(ctx, "id-alice", "bob"); !errors.Is(err, friend.ErrRequestAlreadySent) {
		t.Errorf("expected ErrRequestAlreadySent on repeat request, got %v", err)
	}
}

// Two people adding each other must end up friends rather than each holding a
// request the other never sees as answerable.
func TestSendRequest_MutualRequestBecomesFriendship(t *testing.T) {
	svc, repo := newTestService("alice", "bob")
	ctx := context.Background()

	if _, err := svc.SendRequest(ctx, "id-alice", "bob"); err != nil {
		t.Fatalf("unexpected error on alice's request: %v", err)
	}

	outcome, err := svc.SendRequest(ctx, "id-bob", "alice")
	if err != nil {
		t.Fatalf("unexpected error on bob's reciprocal request: %v", err)
	}
	if outcome.Status != domain.FriendshipStatusAccepted {
		t.Fatalf("expected reciprocal request to accept, got status %q", outcome.Status)
	}
	if outcome.Friend == nil || outcome.Friend.UserID != "id-alice" {
		t.Fatalf("expected bob to be handed alice as a friend, got %+v", outcome.Friend)
	}
	if len(repo.rows) != 1 {
		t.Errorf("expected exactly one friendship row for the pair, got %d", len(repo.rows))
	}

	friends, err := svc.ListFriends(ctx, "id-alice")
	if err != nil {
		t.Fatalf("unexpected error listing alice's friends: %v", err)
	}
	if len(friends) != 1 || friends[0].UserID != "id-bob" {
		t.Errorf("expected alice to be friends with bob, got %+v", friends)
	}
}

func TestSendRequest_AlreadyFriendsIsRejected(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, err := svc.SendRequest(ctx, "id-alice", "bob")
	if err != nil {
		t.Fatalf("unexpected error sending request: %v", err)
	}
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", true); err != nil {
		t.Fatalf("unexpected error accepting request: %v", err)
	}

	if _, err := svc.SendRequest(ctx, "id-alice", "bob"); !errors.Is(err, friend.ErrAlreadyFriends) {
		t.Errorf("expected ErrAlreadyFriends, got %v", err)
	}
}

func TestRespondToRequest_OnlyAddresseeMayAnswer(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, err := svc.SendRequest(ctx, "id-alice", "bob")
	if err != nil {
		t.Fatalf("unexpected error sending request: %v", err)
	}

	// The sender must not be able to accept their own request.
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-alice", true); !errors.Is(err, friend.ErrNotYourRequest) {
		t.Errorf("expected ErrNotYourRequest when the requester answers, got %v", err)
	}
	// Nor may an unrelated user.
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-carol", true); !errors.Is(err, friend.ErrNotYourRequest) {
		t.Errorf("expected ErrNotYourRequest for a third party, got %v", err)
	}

	newFriend, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", true)
	if err != nil {
		t.Fatalf("unexpected error accepting as addressee: %v", err)
	}
	if newFriend == nil || newFriend.UserID != "id-alice" {
		t.Fatalf("expected the accepted friend to be alice, got %+v", newFriend)
	}
}

func TestRespondToRequest_AnsweringTwiceFails(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, _ := svc.SendRequest(ctx, "id-alice", "bob")
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", true); err != nil {
		t.Fatalf("unexpected error on first answer: %v", err)
	}
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", true); !errors.Is(err, friend.ErrRequestNotFound) {
		t.Errorf("expected ErrRequestNotFound answering a settled request, got %v", err)
	}
}

// A declined request is not a permanent block: the pair may be asked again, in
// either direction, and that reuses the same row.
func TestSendRequest_DeclinedPairCanBeAskedAgain(t *testing.T) {
	svc, repo := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, _ := svc.SendRequest(ctx, "id-alice", "bob")
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", false); err != nil {
		t.Fatalf("unexpected error declining: %v", err)
	}

	incoming, _, _ := svc.ListRequests(ctx, "id-bob")
	if len(incoming) != 0 {
		t.Errorf("expected a declined request to leave bob's pending list, got %d", len(incoming))
	}

	// Now bob asks alice: same pair, opposite direction.
	reopened, err := svc.SendRequest(ctx, "id-bob", "alice")
	if err != nil {
		t.Fatalf("unexpected error re-requesting after decline: %v", err)
	}
	if reopened.Status != domain.FriendshipStatusPending {
		t.Errorf("expected a reopened pending request, got %q", reopened.Status)
	}
	if len(repo.rows) != 1 {
		t.Errorf("expected the pair to still hold one row, got %d", len(repo.rows))
	}

	aliceIncoming, _, _ := svc.ListRequests(ctx, "id-alice")
	if len(aliceIncoming) != 1 || aliceIncoming[0].UserID != "id-bob" {
		t.Errorf("expected alice to now see an incoming request from bob, got %+v", aliceIncoming)
	}
}

func TestCancelRequest_OnlyRequesterMayWithdraw(t *testing.T) {
	svc, repo := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, _ := svc.SendRequest(ctx, "id-alice", "bob")

	if err := svc.CancelRequest(ctx, outcome.Request.ID, "id-bob"); !errors.Is(err, friend.ErrNotYourRequest) {
		t.Errorf("expected ErrNotYourRequest when the addressee cancels, got %v", err)
	}
	if err := svc.CancelRequest(ctx, outcome.Request.ID, "id-alice"); err != nil {
		t.Fatalf("unexpected error cancelling own request: %v", err)
	}
	if len(repo.rows) != 0 {
		t.Errorf("expected a cancelled request to leave no row behind, got %d", len(repo.rows))
	}
}

func TestRemoveFriend(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, _ := svc.SendRequest(ctx, "id-alice", "bob")
	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", true); err != nil {
		t.Fatalf("unexpected error accepting: %v", err)
	}

	if err := svc.RemoveFriend(ctx, "id-bob", "id-alice"); err != nil {
		t.Fatalf("unexpected error removing friend: %v", err)
	}

	friends, _ := svc.ListFriends(ctx, "id-alice")
	if len(friends) != 0 {
		t.Errorf("expected alice's friend list to be empty after removal, got %d", len(friends))
	}
	if err := svc.RemoveFriend(ctx, "id-bob", "id-alice"); !errors.Is(err, friend.ErrNotFriends) {
		t.Errorf("expected ErrNotFriends removing a non-friend, got %v", err)
	}
}

func TestAreFriends(t *testing.T) {
	svc, _ := newTestService("alice", "bob")
	ctx := context.Background()

	outcome, _ := svc.SendRequest(ctx, "id-alice", "bob")

	// A pending request is not yet a friendship.
	if ok, err := svc.AreFriends(ctx, "id-alice", "id-bob"); err != nil || ok {
		t.Errorf("expected pending pair to not be friends, got %v (err %v)", ok, err)
	}

	if _, err := svc.RespondToRequest(ctx, outcome.Request.ID, "id-bob", true); err != nil {
		t.Fatalf("unexpected error accepting: %v", err)
	}
	if ok, err := svc.AreFriends(ctx, "id-alice", "id-bob"); err != nil || !ok {
		t.Errorf("expected accepted pair to be friends, got %v (err %v)", ok, err)
	}
	if ok, _ := svc.AreFriends(ctx, "id-alice", "id-alice"); ok {
		t.Error("expected a user not to be their own friend")
	}
}

func TestSearchUsers_IgnoresQueriesShorterThanTwoCharacters(t *testing.T) {
	svc, _ := newTestService("alice")
	ctx := context.Background()

	results, err := svc.SearchUsers(ctx, "id-alice", "a")
	if err != nil {
		t.Fatalf("unexpected error searching: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected a one-character query to return nothing, got %d results", len(results))
	}

	results, err = svc.SearchUsers(ctx, "id-alice", "  bo  ")
	if err != nil {
		t.Fatalf("unexpected error searching: %v", err)
	}
	if len(results) != 1 || results[0].Username != "bo" {
		t.Errorf("expected the query to be trimmed and passed through, got %+v", results)
	}
}
