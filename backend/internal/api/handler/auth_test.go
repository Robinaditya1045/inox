package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// mockAuthService implements auth.AuthService for handler layer testing.
type mockAuthService struct {
	sessions map[string]*domain.Session
}

func newMockAuthService() *mockAuthService {
	return &mockAuthService{sessions: make(map[string]*domain.Session)}
}

func (m *mockAuthService) Signup(ctx context.Context, username, email, password string) (*domain.Session, error) {
	if username == "" || email == "" {
		return nil, auth.ErrInvalidInput
	}
	session := &domain.Session{
		ID:        "sess_mock_http_token",
		UserID:    "user-uuid-1",
		Username:  username,
		Email:     email,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *mockAuthService) Login(ctx context.Context, email, password string) (*domain.Session, error) {
	if password != "correct123" {
		return nil, auth.ErrInvalidCredentials
	}
	session := &domain.Session{
		ID:        "sess_mock_http_token",
		UserID:    "user-uuid-1",
		Username:  "tester",
		Email:     email,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *mockAuthService) Logout(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockAuthService) ValidateSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	s, exists := m.sessions[sessionID]
	if !exists {
		return nil, auth.ErrSessionNotFound
	}
	return s, nil
}

func TestSignupHTTPHandler(t *testing.T) {
	mockSvc := newMockAuthService()
	authHandler := handler.NewAuthHandler(mockSvc, false)

	payload := map[string]string{
		"username": "robin",
		"email":    "robin@inox.com",
		"password": "secretpassword123",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	authHandler.Signup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201 Created, got %d", rec.Code)
	}

	// Verify Set-Cookie header exists and has HttpOnly flag
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected Set-Cookie header in response")
	}
	if cookies[0].Name != "inox_session" || cookies[0].Value != "sess_mock_http_token" {
		t.Errorf("unexpected cookie returned: %+v", cookies[0])
	}
	if !cookies[0].HttpOnly {
		t.Errorf("expected session cookie to have HttpOnly flag set to true")
	}
}

func TestRequireAuthMiddleware(t *testing.T) {
	mockSvc := newMockAuthService()
	requireAuth := middleware.RequireAuth(mockSvc)

	// Dummy protected handler
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := middleware.GetSessionFromContext(r.Context())
		if !ok {
			http.Error(w, "missing context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello " + session.Username))
	})

	wrapped := requireAuth(protectedHandler)

	// 1. Test Unauthenticated Request (Missing Cookie)
	reqUnauth := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	recUnauth := httptest.NewRecorder()
	wrapped.ServeHTTP(recUnauth, reqUnauth)

	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected HTTP 401 Unauthorized for request without cookie, got %d", recUnauth.Code)
	}

	// 2. Test Authenticated Request (Valid Cookie)
	// First populate session in mock service
	_, _ = mockSvc.Signup(context.Background(), "robin", "robin@inox.com", "pass")

	reqAuth := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqAuth.AddCookie(&http.Cookie{Name: "inox_session", Value: "sess_mock_http_token"})
	recAuth := httptest.NewRecorder()
	wrapped.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 OK for request with valid cookie, got %d", recAuth.Code)
	}
}
