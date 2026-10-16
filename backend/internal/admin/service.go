// Package admin decides who may use the admin portal.
//
// There are two tiers. Owners are the addresses listed in ADMIN_EMAILS; they can do
// everything, including adding and removing admins. Admins are users an owner added
// from the portal; they can use it but cannot change who has access.
package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrGranterRequired    = errors.New("the granting owner must be identified")
	ErrIdentifierRequired = errors.New("an email or username is required")
	ErrUserNotFound       = errors.New("no user found with that email or username")
	ErrAlreadyAdmin       = errors.New("this user is already an admin")
	ErrAlreadyOwner       = errors.New("this user is already an owner through ADMIN_EMAILS")
	ErrCannotRevokeOwner  = errors.New("owners are set in ADMIN_EMAILS and cannot be removed here")
	ErrNotAdmin           = errors.New("this user is not an admin")
)

// Service is the business logic contract for admin portal access.
type Service interface {
	// RoleFor resolves a session's admin portal role on every request, so adding or
	// removing an admin takes effect immediately rather than at their next login.
	RoleFor(ctx context.Context, session *domain.Session) (domain.AdminRole, error)
	// List returns the owners, in ADMIN_EMAILS order, followed by the admins.
	List(ctx context.Context) ([]*domain.AdminAccount, error)
	// Grant makes the user with the given email or username an admin.
	Grant(ctx context.Context, granter *domain.Session, identifier string) (*domain.AdminAccount, error)
	Revoke(ctx context.Context, userID string) error
}

type service struct {
	repo        Repository
	userRepo    auth.UserRepository
	ownerEmails []string
	owners      map[string]bool
}

// NewService builds the service. ownerEmails is the raw, comma-separated ADMIN_EMAILS value.
func NewService(repo Repository, userRepo auth.UserRepository, ownerEmails string) Service {
	s := &service{repo: repo, userRepo: userRepo, owners: map[string]bool{}}
	for _, e := range strings.Split(ownerEmails, ",") {
		e = normalizeEmail(e)
		if e == "" || s.owners[e] {
			continue
		}
		s.owners[e] = true
		s.ownerEmails = append(s.ownerEmails, e)
	}
	return s
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *service) isOwner(email string) bool {
	return s.owners[normalizeEmail(email)]
}

func (s *service) RoleFor(ctx context.Context, session *domain.Session) (domain.AdminRole, error) {
	if session == nil {
		return domain.AdminRoleNone, nil
	}
	if s.isOwner(session.Email) {
		return domain.AdminRoleOwner, nil
	}
	if session.UserID == "" {
		return domain.AdminRoleNone, nil
	}
	isAdmin, err := s.repo.IsAdmin(ctx, session.UserID)
	if err != nil {
		return domain.AdminRoleNone, err
	}
	if isAdmin {
		return domain.AdminRoleAdmin, nil
	}
	return domain.AdminRoleNone, nil
}

func (s *service) List(ctx context.Context) ([]*domain.AdminAccount, error) {
	accounts := make([]*domain.AdminAccount, 0, len(s.ownerEmails))
	for _, email := range s.ownerEmails {
		account := &domain.AdminAccount{Email: email, Role: domain.AdminRoleOwner}
		user, err := s.userRepo.GetByEmail(ctx, email)
		switch {
		case err == nil:
			account.UserID, account.Username = user.ID, user.Username
		case !errors.Is(err, auth.ErrUserNotFound):
			return nil, err
		}
		accounts = append(accounts, account)
	}

	admins, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range admins {
		// Someone added as an admin and later listed in ADMIN_EMAILS is an owner;
		// show them once, as that.
		if s.isOwner(a.Email) {
			continue
		}
		a.Role = domain.AdminRoleAdmin
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// Grant takes an email or a username, since owners know their teammates by either.
func (s *service) Grant(ctx context.Context, granter *domain.Session, identifier string) (*domain.AdminAccount, error) {
	// Every grant records who made it. A caller that can't be identified is refused
	// rather than written down as an anonymous grant.
	if granter == nil || granter.UserID == "" {
		return nil, ErrGranterRequired
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrIdentifierRequired
	}

	var user *domain.User
	var err error
	if strings.Contains(identifier, "@") {
		user, err = s.userRepo.GetByEmail(ctx, normalizeEmail(identifier))
	} else {
		user, err = s.userRepo.GetByUsername(ctx, identifier)
	}
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if s.isOwner(user.Email) {
		return nil, ErrAlreadyOwner
	}

	grantedAt, err := s.repo.Add(ctx, user.ID, granter.UserID)
	if err != nil {
		return nil, err
	}
	return &domain.AdminAccount{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      domain.AdminRoleAdmin,
		GrantedBy: granter.Username,
		GrantedAt: &grantedAt,
	}, nil
}

func (s *service) Revoke(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return ErrNotAdmin
		}
		return err
	}
	if s.isOwner(user.Email) {
		return ErrCannotRevokeOwner
	}
	return s.repo.Remove(ctx, userID)
}
