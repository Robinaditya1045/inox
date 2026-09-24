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

func TestRequireAdminRole(t *testing.T) {
	// Set up admin emails env
	origAdminEmails := os.Getenv("ADMIN_EMAILS")
	defer os.Setenv("ADMIN_EMAILS", origAdminEmails)
	os.Setenv("ADMIN_EMAILS", "admin@inox.app,super@inox.app")

	handler := RequireAdminRole()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// 1. No session -> 403 Forbidden
	req1 := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when no session in context, got %d", rec1.Code)
	}

	// 2. Regular user session -> 403 Forbidden
	req2 := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	userSession := &domain.Session{
		ID:       "sess-1",
		UserID:   "usr-1",
		Username: "alice",
		Email:    "alice@example.com",
		Role:     domain.SystemRoleUser,
	}
	ctx2 := context.WithValue(req2.Context(), sessionContextKey, userSession)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2.WithContext(ctx2))
	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for regular user, got %d", rec2.Code)
	}

	// 3. System role admin -> 200 OK
	req3 := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	adminSession := &domain.Session{
		ID:       "sess-2",
		UserID:   "usr-2",
		Username: "admin",
		Email:    "bob@example.com",
		Role:     domain.SystemRoleAdmin,
	}
	ctx3 := context.WithValue(req3.Context(), sessionContextKey, adminSession)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3.WithContext(ctx3))
	if rec3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for user with SystemRoleAdmin, got %d", rec3.Code)
	}

	// 4. Admin email match via ADMIN_EMAILS -> 200 OK
	req4 := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	emailSession := &domain.Session{
		ID:       "sess-3",
		UserID:   "usr-3",
		Username: "super",
		Email:    "super@inox.app",
		Role:     domain.SystemRoleUser,
	}
	ctx4 := context.WithValue(req4.Context(), sessionContextKey, emailSession)
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4.WithContext(ctx4))
	if rec4.Code != http.StatusOK {
		t.Errorf("expected 200 OK for user matching ADMIN_EMAILS, got %d", rec4.Code)
	}
}

func TestRequireMetricsAccess(t *testing.T) {
	origToken := os.Getenv("METRICS_AUTH_TOKEN")
	defer os.Setenv("METRICS_AUTH_TOKEN", origToken)
	os.Setenv("METRICS_AUTH_TOKEN", "secret-token-123")

	handler := RequireMetricsAccess(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	origAdminEmails := os.Getenv("ADMIN_EMAILS")
	origToken := os.Getenv("METRICS_AUTH_TOKEN")
	defer os.Setenv("ADMIN_EMAILS", origAdminEmails)
	defer os.Setenv("METRICS_AUTH_TOKEN", origToken)
	os.Setenv("ADMIN_EMAILS", "admin@inox.app")
	os.Setenv("METRICS_AUTH_TOKEN", "")

	svc := &fakeAuthService{
		sessions: map[string]*domain.Session{
			"sess_admin": {ID: "sess_admin", Email: "admin@inox.app"},
			"sess_user":  {ID: "sess_user", Email: "user@inox.app"},
		},
		lookupErrs: map[string]error{"sess_unreachable": errors.New("redis: connection refused")},
	}
	handler := RequireMetricsAccess(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name     string
		cookie   string
		bearer   string
		wantCode int
	}{
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
