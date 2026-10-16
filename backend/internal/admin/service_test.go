package admin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/admin"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// fakeUserRepo implements the lookups the admin service uses; any other
// UserRepository method panics through the nil embedded interface.
type fakeUserRepo struct {
	auth.UserRepository
	users []*domain.User
}

func (f *fakeUserRepo) find(match func(*domain.User) bool) (*domain.User, error) {
	for _, u := range f.users {
		if match(u) {
			return u, nil
		}
	}
	return nil, auth.ErrUserNotFound
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return f.find(func(u *domain.User) bool { return u.Email == email })
}

func (f *fakeUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return f.find(func(u *domain.User) bool { return u.Username == username })
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return f.find(func(u *domain.User) bool { return u.ID == id })
}

// fakeAdminRepo keeps admins in insertion order, keyed by user ID.
type fakeAdminRepo struct {
	users     *fakeUserRepo
	order     []string
	grantedBy map[string]string
	err       error
}

func newFakeAdminRepo(users *fakeUserRepo) *fakeAdminRepo {
	return &fakeAdminRepo{users: users, grantedBy: map[string]string{}}
}

func (f *fakeAdminRepo) IsAdmin(ctx context.Context, userID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.grantedBy[userID]
	return ok, nil
}

func (f *fakeAdminRepo) List(ctx context.Context) ([]*domain.AdminAccount, error) {
	accounts := []*domain.AdminAccount{}
	for _, id := range f.order {
		u, _ := f.users.GetByID(ctx, id)
		granter, _ := f.users.GetByID(ctx, f.grantedBy[id])
		a := &domain.AdminAccount{UserID: u.ID, Username: u.Username, Email: u.Email}
		if granter != nil {
			a.GrantedBy = granter.Username
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (f *fakeAdminRepo) Add(ctx context.Context, userID, grantedBy string) (time.Time, error) {
	if _, ok := f.grantedBy[userID]; ok {
		return time.Time{}, admin.ErrAlreadyAdmin
	}
	f.grantedBy[userID] = grantedBy
	f.order = append(f.order, userID)
	return time.Now(), nil
}

func (f *fakeAdminRepo) Remove(ctx context.Context, userID string) error {
	if _, ok := f.grantedBy[userID]; !ok {
		return admin.ErrNotAdmin
	}
	delete(f.grantedBy, userID)
	for i, id := range f.order {
		if id == userID {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return nil
}

// fixture: one owner with an account, one owner address nobody signed up with,
// and two normal users.
func newFixture() (admin.Service, *fakeAdminRepo, *domain.Session) {
	users := &fakeUserRepo{users: []*domain.User{
		{ID: "u-owner", Username: "olivia", Email: "olivia@inox.app"},
		{ID: "u-alice", Username: "alice", Email: "alice@example.com"},
		{ID: "u-bob", Username: "bob", Email: "bob@example.com"},
	}}
	repo := newFakeAdminRepo(users)
	// Mixed case, padding and a duplicate, as an env var is likely to be written.
	svc := admin.NewService(repo, users, " Olivia@Inox.app, pending@inox.app,olivia@inox.app,")
	owner := &domain.Session{ID: "s-owner", UserID: "u-owner", Username: "olivia", Email: "olivia@inox.app"}
	return svc, repo, owner
}

func TestRoleFor(t *testing.T) {
	ctx := context.Background()
	svc, _, owner := newFixture()
	if _, err := svc.Grant(ctx, owner, "alice"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name    string
		session *domain.Session
		want    domain.AdminRole
	}{
		{name: "owner by ADMIN_EMAILS", session: owner, want: domain.AdminRoleOwner},
		{name: "owner email differs in case", session: &domain.Session{UserID: "u-x", Email: "OLIVIA@inox.app"}, want: domain.AdminRoleOwner},
		{name: "added admin", session: &domain.Session{UserID: "u-alice", Email: "alice@example.com"}, want: domain.AdminRoleAdmin},
		{name: "normal user", session: &domain.Session{UserID: "u-bob", Email: "bob@example.com"}, want: domain.AdminRoleNone},
		{name: "no session", session: nil, want: domain.AdminRoleNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.RoleFor(ctx, tt.session)
			if err != nil {
				t.Fatalf("RoleFor: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRoleForSurfacesLookupFailures(t *testing.T) {
	svc, repo, _ := newFixture()
	repo.err = errors.New("db: connection refused")
	if _, err := svc.RoleFor(context.Background(), &domain.Session{UserID: "u-bob", Email: "bob@example.com"}); err == nil {
		t.Fatal("expected the lookup error, so the request is refused rather than treated as a normal user")
	}
}

func TestGrant(t *testing.T) {
	ctx := context.Background()
	svc, _, owner := newFixture()

	got, err := svc.Grant(ctx, owner, "  ALICE@example.com ")
	if err != nil {
		t.Fatalf("grant by email: %v", err)
	}
	if got.UserID != "u-alice" || got.Role != domain.AdminRoleAdmin || got.GrantedBy != "olivia" || got.GrantedAt == nil {
		t.Errorf("unexpected account: %+v", got)
	}
	if _, err := svc.Grant(ctx, owner, "bob"); err != nil {
		t.Fatalf("grant by username: %v", err)
	}

	failures := []struct {
		name       string
		identifier string
		want       error
	}{
		{name: "blank", identifier: "  ", want: admin.ErrIdentifierRequired},
		{name: "unknown user", identifier: "nobody@example.com", want: admin.ErrUserNotFound},
		{name: "already admin", identifier: "alice", want: admin.ErrAlreadyAdmin},
		{name: "owner", identifier: "olivia", want: admin.ErrAlreadyOwner},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Grant(ctx, owner, tt.identifier); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, err)
			}
		})
	}

	// A grant nobody can be credited with is refused, not stored as anonymous.
	for name, granter := range map[string]*domain.Session{"nil granter": nil, "granter without user ID": {Username: "ghost"}} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Grant(ctx, granter, "carol@example.com"); !errors.Is(err, admin.ErrGranterRequired) {
				t.Errorf("expected ErrGranterRequired, got %v", err)
			}
		})
	}
}

func TestRevoke(t *testing.T) {
	ctx := context.Background()
	svc, _, owner := newFixture()
	if _, err := svc.Grant(ctx, owner, "alice"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := svc.Revoke(ctx, "u-alice"); err != nil {
		t.Fatalf("revoke admin: %v", err)
	}
	role, _ := svc.RoleFor(ctx, &domain.Session{UserID: "u-alice", Email: "alice@example.com"})
	if role != domain.AdminRoleNone {
		t.Errorf("expected access gone straight after revoking, got %q", role)
	}

	if err := svc.Revoke(ctx, "u-owner"); !errors.Is(err, admin.ErrCannotRevokeOwner) {
		t.Errorf("revoking an owner: expected ErrCannotRevokeOwner, got %v", err)
	}
	if err := svc.Revoke(ctx, "u-bob"); !errors.Is(err, admin.ErrNotAdmin) {
		t.Errorf("revoking a normal user: expected ErrNotAdmin, got %v", err)
	}
	if err := svc.Revoke(ctx, "u-missing"); !errors.Is(err, admin.ErrNotAdmin) {
		t.Errorf("revoking an unknown user: expected ErrNotAdmin, got %v", err)
	}
}

func TestListShowsOwnersThenAdmins(t *testing.T) {
	ctx := context.Background()
	svc, repo, owner := newFixture()
	if _, err := svc.Grant(ctx, owner, "alice"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// An owner who was also added as an admin before being listed in ADMIN_EMAILS.
	repo.grantedBy["u-owner"] = "u-owner"
	repo.order = append(repo.order, "u-owner")

	got, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []struct {
		email    string
		userID   string
		role     domain.AdminRole
		hasGrant bool
	}{
		{email: "olivia@inox.app", userID: "u-owner", role: domain.AdminRoleOwner},
		{email: "pending@inox.app", userID: "", role: domain.AdminRoleOwner},
		{email: "alice@example.com", userID: "u-alice", role: domain.AdminRoleAdmin, hasGrant: true},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d accounts, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Email != w.email || g.UserID != w.userID || g.Role != w.role || (g.GrantedBy != "") != w.hasGrant {
			t.Errorf("account %d: expected %+v, got %+v", i, w, g)
		}
	}
}
