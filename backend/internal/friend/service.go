package friend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrSelfFriendship     = errors.New("you cannot add yourself as a friend")
	ErrUserNotFound       = errors.New("no user found with that username")
	ErrAlreadyFriends     = errors.New("you are already friends with this user")
	ErrRequestAlreadySent = errors.New("a friend request to this user is already pending")
	ErrRequestNotFound    = errors.New("friend request not found")
	ErrNotYourRequest     = errors.New("that friend request is not yours to answer")
	ErrNotFriends         = errors.New("you are not friends with this user")
	ErrInvalidUsername    = errors.New("a username is required")
)

// minSearchQuery keeps the directory from being enumerated one letter at a time.
const (
	minSearchQuery = 2
	searchLimit    = 20
)

// Service is the business logic contract for the friends graph.
type Service interface {
	ListFriends(ctx context.Context, userID string) ([]*domain.Friend, error)
	ListRequests(ctx context.Context, userID string) (incoming, outgoing []*domain.FriendRequest, err error)
	SendRequest(ctx context.Context, requesterID, username string) (*domain.FriendRequestOutcome, error)
	RespondToRequest(ctx context.Context, requestID, userID string, accept bool) (*domain.Friend, error)
	CancelRequest(ctx context.Context, requestID, userID string) error
	RemoveFriend(ctx context.Context, userID, friendUserID string) error
	SearchUsers(ctx context.Context, viewerID, query string) ([]*domain.UserSearchResult, error)
	AreFriends(ctx context.Context, userA, userB string) (bool, error)
}

type service struct {
	repo     Repository
	userRepo auth.UserRepository
}

// NewService constructs the friends service over the friendship and user repositories.
func NewService(repo Repository, userRepo auth.UserRepository) Service {
	return &service{repo: repo, userRepo: userRepo}
}

func (s *service) ListFriends(ctx context.Context, userID string) ([]*domain.Friend, error) {
	return s.repo.ListFriends(ctx, userID)
}

// ListRequests splits the single pending query into the two piles the UI shows.
func (s *service) ListRequests(ctx context.Context, userID string) ([]*domain.FriendRequest, []*domain.FriendRequest, error) {
	all, err := s.repo.ListPendingRequests(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	incoming := []*domain.FriendRequest{}
	outgoing := []*domain.FriendRequest{}
	for _, req := range all {
		if req.Direction == domain.FriendRequestOutgoing {
			outgoing = append(outgoing, req)
			continue
		}
		incoming = append(incoming, req)
	}
	return incoming, outgoing, nil
}

// SendRequest addresses a user by username, the only handle one person can type for
// another here (emails are private and ids are not memorable).
//
// Sending to someone whose request is already waiting on you accepts it instead of
// opening a second one -- two people adding each other should end up friends, not
// deadlocked on two requests neither thinks to answer.
func (s *service) SendRequest(ctx context.Context, requesterID, username string) (*domain.FriendRequestOutcome, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrInvalidUsername
	}

	target, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to look up user: %w", err)
	}
	if target.ID == requesterID {
		return nil, ErrSelfFriendship
	}

	existing, err := s.repo.GetByPair(ctx, requesterID, target.ID)
	if err != nil && !errors.Is(err, ErrFriendshipNotFound) {
		return nil, fmt.Errorf("failed to check existing friendship: %w", err)
	}

	if existing != nil {
		switch existing.Status {
		case domain.FriendshipStatusAccepted:
			return nil, ErrAlreadyFriends
		case domain.FriendshipStatusPending:
			if existing.RequesterID == requesterID {
				return nil, ErrRequestAlreadySent
			}
			// They asked first: answering is the honest outcome.
			f, err := s.RespondToRequest(ctx, existing.ID, requesterID, true)
			if err != nil {
				return nil, err
			}
			return &domain.FriendRequestOutcome{
				Status: domain.FriendshipStatusAccepted,
				Friend: f,
			}, nil
		default:
			// A previously declined pair is reusable: the row is the pair, not the attempt.
			if err := s.repo.Reopen(ctx, existing.ID, requesterID, target.ID); err != nil {
				return nil, err
			}
			reopened, err := s.repo.GetByID(ctx, existing.ID)
			if err != nil {
				return nil, err
			}
			return &domain.FriendRequestOutcome{
				Status:  domain.FriendshipStatusPending,
				Request: outgoingRequest(reopened, target),
			}, nil
		}
	}

	friendship := &domain.Friendship{
		RequesterID: requesterID,
		AddresseeID: target.ID,
		Status:      domain.FriendshipStatusPending,
	}
	if err := s.repo.Create(ctx, friendship); err != nil {
		if errors.Is(err, ErrPairConflict) {
			// Lost a race with the other user's simultaneous request.
			return nil, ErrRequestAlreadySent
		}
		return nil, err
	}

	return &domain.FriendRequestOutcome{
		Status:  domain.FriendshipStatusPending,
		Request: outgoingRequest(friendship, target),
	}, nil
}

// RespondToRequest accepts or declines a pending request. Only the addressee may
// answer; the requester's way out is CancelRequest.
func (s *service) RespondToRequest(ctx context.Context, requestID, userID string, accept bool) (*domain.Friend, error) {
	f, err := s.repo.GetByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, ErrFriendshipNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}
	if f.AddresseeID != userID {
		return nil, ErrNotYourRequest
	}
	if f.Status != domain.FriendshipStatusPending {
		return nil, ErrRequestNotFound
	}

	if !accept {
		if err := s.repo.UpdateStatus(ctx, f.ID, domain.FriendshipStatusDeclined); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if err := s.repo.UpdateStatus(ctx, f.ID, domain.FriendshipStatusAccepted); err != nil {
		return nil, err
	}

	other, err := s.userRepo.GetByID(ctx, f.RequesterID)
	if err != nil {
		return nil, fmt.Errorf("failed to load new friend: %w", err)
	}
	updated, err := s.repo.GetByID(ctx, f.ID)
	if err != nil {
		return nil, err
	}

	return &domain.Friend{
		FriendshipID: updated.ID,
		UserID:       other.ID,
		Username:     other.Username,
		AvatarURL:    other.AvatarURL,
		FriendsSince: updated.UpdatedAt,
	}, nil
}

// CancelRequest withdraws a request the caller sent. The row is deleted rather than
// marked declined, so a withdrawn request leaves no trace on the recipient's side.
func (s *service) CancelRequest(ctx context.Context, requestID, userID string) error {
	f, err := s.repo.GetByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, ErrFriendshipNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if f.RequesterID != userID {
		return ErrNotYourRequest
	}
	if f.Status != domain.FriendshipStatusPending {
		return ErrRequestNotFound
	}
	return s.repo.Delete(ctx, f.ID)
}

// RemoveFriend unfriends by the other user's id -- the friends list is the only place
// this is reachable from, and it has ids, not friendship rows, in hand.
func (s *service) RemoveFriend(ctx context.Context, userID, friendUserID string) error {
	if userID == friendUserID {
		return ErrNotFriends
	}
	f, err := s.repo.GetByPair(ctx, userID, friendUserID)
	if err != nil {
		if errors.Is(err, ErrFriendshipNotFound) {
			return ErrNotFriends
		}
		return err
	}
	if f.Status != domain.FriendshipStatusAccepted {
		return ErrNotFriends
	}
	return s.repo.Delete(ctx, f.ID)
}

// SearchUsers backs the add-friend picker. Queries shorter than minSearchQuery return
// nothing rather than the whole user table.
func (s *service) SearchUsers(ctx context.Context, viewerID, query string) ([]*domain.UserSearchResult, error) {
	query = strings.TrimSpace(query)
	if len(query) < minSearchQuery {
		return []*domain.UserSearchResult{}, nil
	}
	return s.repo.SearchUsers(ctx, viewerID, query, searchLimit)
}

// AreFriends answers the membership question other features (room invites, DMs) ask
// of this graph without needing to know how it is stored.
func (s *service) AreFriends(ctx context.Context, userA, userB string) (bool, error) {
	if userA == userB {
		return false, nil
	}
	f, err := s.repo.GetByPair(ctx, userA, userB)
	if err != nil {
		if errors.Is(err, ErrFriendshipNotFound) {
			return false, nil
		}
		return false, err
	}
	return f.Status == domain.FriendshipStatusAccepted, nil
}

// outgoingRequest renders a freshly created row from the sender's point of view.
func outgoingRequest(f *domain.Friendship, target *domain.User) *domain.FriendRequest {
	return &domain.FriendRequest{
		ID:        f.ID,
		Direction: domain.FriendRequestOutgoing,
		UserID:    target.ID,
		Username:  target.Username,
		AvatarURL: target.AvatarURL,
		CreatedAt: f.CreatedAt,
	}
}
