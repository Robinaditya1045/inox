package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/inox/inox/backend/internal/domain"
)

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
