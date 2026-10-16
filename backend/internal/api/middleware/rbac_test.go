package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// fakeAuthService implements only session validation; any other AuthService
// method panics through the nil embedded interface.
type fakeAuthService struct {
	auth.AuthService
	sessions   map[string]*domain.Session
	lookupErrs map[string]error
}

func (f *fakeAuthService) ValidateSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	if err, failing := f.lookupErrs[sessionID]; failing {
		return nil, err
	}
	if s, ok := f.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, auth.ErrSessionNotFound
}

// fakeAdminAccess resolves admin roles by session ID.
type fakeAdminAccess struct {
	roles map[string]domain.AdminRole
	err   error
}

func (f *fakeAdminAccess) RoleFor(ctx context.Context, session *domain.Session) (domain.AdminRole, error) {
	if f.err != nil {
		return domain.AdminRoleNone, f.err
	}
	return f.roles[session.ID], nil
}

// serveAsSession runs handler with session (if any) already placed in the context by RequireAuth.
func serveAsSession(handler http.Handler, session *domain.Session) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	if session != nil {
		req = req.WithContext(context.WithValue(req.Context(), sessionContextKey, session))
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRequireAdminRoleAndOwner(t *testing.T) {
	access := &fakeAdminAccess{roles: map[string]domain.AdminRole{
		"sess_owner": domain.AdminRoleOwner,
		"sess_admin": domain.AdminRoleAdmin,
	}}
	var seenRole domain.AdminRole
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRole, _ = GetAdminRoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name      string
		session   *domain.Session
		ownerOnly bool
		wantCode  int
		wantRole  domain.AdminRole
	}{
		{name: "no session", wantCode: http.StatusForbidden},
		{name: "normal user", session: &domain.Session{ID: "sess_user"}, wantCode: http.StatusForbidden},
		{name: "admin", session: &domain.Session{ID: "sess_admin"}, wantCode: http.StatusOK, wantRole: domain.AdminRoleAdmin},
		{name: "owner", session: &domain.Session{ID: "sess_owner"}, wantCode: http.StatusOK, wantRole: domain.AdminRoleOwner},
		{name: "owner-only: normal user", session: &domain.Session{ID: "sess_user"}, ownerOnly: true, wantCode: http.StatusForbidden},
		{name: "owner-only: admin", session: &domain.Session{ID: "sess_admin"}, ownerOnly: true, wantCode: http.StatusForbidden},
		{name: "owner-only: owner", session: &domain.Session{ID: "sess_owner"}, ownerOnly: true, wantCode: http.StatusOK, wantRole: domain.AdminRoleOwner},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seenRole = domain.AdminRoleNone
			gate := RequireAdminRole(access)
			if tt.ownerOnly {
				gate = RequireAdminOwner(access)
			}

			rec := serveAsSession(gate(next), tt.session)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected HTTP %d, got %d", tt.wantCode, rec.Code)
			}
			if seenRole != tt.wantRole {
				t.Errorf("expected role %q in context, got %q", tt.wantRole, seenRole)
			}
		})
	}
}

func TestRequireAdminRoleFailsClosed(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	session := &domain.Session{ID: "sess_admin"}

	// No resolver wired: nobody is an admin.
	if rec := serveAsSession(RequireAdminRole(nil)(next), session); rec.Code != http.StatusForbidden {
		t.Errorf("nil access: expected 403, got %d", rec.Code)
	}
	// Resolver unreachable: refuse rather than guess, and say it is temporary.
	failing := &fakeAdminAccess{err: errors.New("db: connection refused")}
	if rec := serveAsSession(RequireAdminRole(failing)(next), session); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("failing access: expected 503, got %d", rec.Code)
	}
}

func TestRequireMetricsAccess(t *testing.T) {
	origToken := os.Getenv("METRICS_AUTH_TOKEN")
	defer os.Setenv("METRICS_AUTH_TOKEN", origToken)
	os.Setenv("METRICS_AUTH_TOKEN", "secret-token-123")

	handler := RequireMetricsAccess(nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics"))
	}))

	// 1. Loopback IP -> 200 OK
	req1 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req1.RemoteAddr = "127.0.0.1:54321"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("expected 200 OK for 127.0.0.1 loopback, got %d", rec1.Code)
	}

	// 2. External IP without token -> 403 Forbidden
	req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req2.RemoteAddr = "192.168.1.100:54321"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for external IP without token, got %d", rec2.Code)
	}

	// 3. External IP with valid Bearer token -> 200 OK
	req3 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req3.RemoteAddr = "192.168.1.100:54321"
	req3.Header.Set("Authorization", "Bearer secret-token-123")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid Bearer token, got %d", rec3.Code)
	}
}

// An admin must be able to reach /metrics with the session the rest of the API
// accepts. This used to read a cookie named "session", which nothing ever sets.
func TestRequireMetricsAccessAdminSession(t *testing.T) {
	origToken := os.Getenv("METRICS_AUTH_TOKEN")
	defer os.Setenv("METRICS_AUTH_TOKEN", origToken)
	os.Setenv("METRICS_AUTH_TOKEN", "")

	svc := &fakeAuthService{
		sessions: map[string]*domain.Session{
			"sess_owner": {ID: "sess_owner", Email: "owner@inox.app"},
			"sess_admin": {ID: "sess_admin", Email: "admin@inox.app"},
			"sess_user":  {ID: "sess_user", Email: "user@inox.app"},
		},
		lookupErrs: map[string]error{"sess_unreachable": errors.New("redis: connection refused")},
	}
	access := &fakeAdminAccess{roles: map[string]domain.AdminRole{
		"sess_owner": domain.AdminRoleOwner,
		"sess_admin": domain.AdminRoleAdmin,
	}}
	handler := RequireMetricsAccess(svc, access)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name     string
		cookie   string
		bearer   string
		wantCode int
	}{
		{name: "owner session cookie", cookie: "sess_owner", wantCode: http.StatusOK},
		{name: "admin session cookie", cookie: "sess_admin", wantCode: http.StatusOK},
		{name: "admin session bearer", bearer: "sess_admin", wantCode: http.StatusOK},
		{name: "non-admin session", cookie: "sess_user", wantCode: http.StatusForbidden},
		{name: "unknown session", cookie: "sess_gone", wantCode: http.StatusForbidden},
		{name: "no credentials", wantCode: http.StatusForbidden},
		{name: "session store unreachable", bearer: "sess_unreachable", wantCode: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			req.RemoteAddr = "203.0.113.7:54321"
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tt.cookie})
			}
			if tt.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tt.bearer)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected HTTP %d, got %d", tt.wantCode, rec.Code)
			}
		})
	}
}
