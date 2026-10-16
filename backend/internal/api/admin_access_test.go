package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inox/inox/backend/internal/admin"
	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// sessionsAuth validates sessions from a fixed map; any other AuthService method
// panics through the nil embedded interface.
type sessionsAuth struct {
	auth.AuthService
	sessions map[string]*domain.Session
}

func (a *sessionsAuth) ValidateSession(ctx context.Context, id string) (*domain.Session, error) {
	if s, ok := a.sessions[id]; ok {
		return s, nil
	}
	return nil, auth.ErrSessionNotFound
}

// rolesAdmin resolves roles by session ID and accepts every grant and revoke.
type rolesAdmin struct {
	roles map[string]domain.AdminRole
}

func (a *rolesAdmin) RoleFor(ctx context.Context, s *domain.Session) (domain.AdminRole, error) {
	return a.roles[s.ID], nil
}

func (a *rolesAdmin) List(ctx context.Context) ([]*domain.AdminAccount, error) {
	return []*domain.AdminAccount{{Email: "owner@inox.app", Role: domain.AdminRoleOwner}}, nil
}

func (a *rolesAdmin) Grant(ctx context.Context, granter *domain.Session, identifier string) (*domain.AdminAccount, error) {
	return &domain.AdminAccount{Username: identifier, Role: domain.AdminRoleAdmin}, nil
}

func (a *rolesAdmin) Revoke(ctx context.Context, userID string) error { return nil }

var _ admin.Service = (*rolesAdmin)(nil)

// The admin role must stay invisible to normal users: every admin-access route
// answers them exactly as it answers anyone else without access, and only owners
// can see or change who the admins are.
func TestAdminAccessRoutesByRole(t *testing.T) {
	authSvc := &sessionsAuth{sessions: map[string]*domain.Session{
		"s-user":  {ID: "s-user", UserID: "u-user", Username: "user"},
		"s-admin": {ID: "s-admin", UserID: "u-admin", Username: "admin"},
		"s-owner": {ID: "s-owner", UserID: "u-owner", Username: "owner"},
	}}
	adminSvc := &rolesAdmin{roles: map[string]domain.AdminRole{
		"s-admin": domain.AdminRoleAdmin,
		"s-owner": domain.AdminRoleOwner,
	}}
	router := NewRouter(nil, authSvc, nil, nil, nil, nil, nil, adminSvc, handler.NewAdminAccessHandler(adminSvc), nil, nil, nil, nil, nil)

	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/me", ""},
		{http.MethodGet, "/api/v1/admin/admins", ""},
		{http.MethodPost, "/api/v1/admin/admins", `{"identifier":"alice"}`},
		{http.MethodDelete, "/api/v1/admin/admins/u-alice", ""},
	}
	want := map[string][]int{
		"":        {http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
		"s-user":  {http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
		"s-admin": {http.StatusOK, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
		"s-owner": {http.StatusOK, http.StatusOK, http.StatusCreated, http.StatusOK},
	}

	for session, codes := range want {
		for i, rt := range routes {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			if session != "" {
				req.Header.Set("Authorization", "Bearer "+session)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != codes[i] {
				t.Errorf("%s %s as %q: expected %d, got %d (%s)", rt.method, rt.path, session, codes[i], rec.Code, rec.Body)
			}
			if session == "s-user" && strings.Contains(rec.Body.String(), "owner") {
				t.Errorf("%s %s leaked role wording to a normal user: %s", rt.method, rt.path, rec.Body)
			}
		}
	}

	for session, role := range map[string]domain.AdminRole{"s-admin": domain.AdminRoleAdmin, "s-owner": domain.AdminRoleOwner} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
		req.Header.Set("Authorization", "Bearer "+session)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		var me struct {
			Role domain.AdminRole `json:"role"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&me); err != nil || me.Role != role {
			t.Errorf("/admin/me as %s: expected role %q, got %q (err %v)", session, role, me.Role, err)
		}
	}
}
