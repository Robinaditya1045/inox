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

func (m *mockAuthService) GeneratePasswordResetToken(ctx context.Context, email string) (string, error) {
	return "mock_token", nil
}

func (m *mockAuthService) ResetPasswordWithToken(ctx context.Context, token, newPassword string) error {
	return nil
}

func (m *mockAuthService) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	avatar := "https://cdn.example/avatar.png"
	return &domain.User{
		ID:           userID,
		Username:     "robin",
		Email:        "robin@inox.com",
		PasswordHash: "$2a$10$should-never-leave-the-server",
		AvatarURL:    &avatar,
		CreatedAt:    time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (m *mockAuthService) UpdateAvatar(ctx context.Context, userID, avatarURL string, sessionID string) error {
	return nil
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

// The frontend restores a session on page load by calling /users/me and
// reading lowercase keys (`id`, `username`, ...). domain.User carries no JSON
// tags, so serialising it directly emitted `ID`, `Username`, ... and every
// refresh bounced a logged-in user back to /login.
func TestMeHTTPHandler(t *testing.T) {
	authHandler := handler.NewAuthHandler(newMockAuthService(), false)

	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req = req.WithContext(middleware.WithSessionContext(req.Context(), &domain.Session{
		ID:     "sess_mock_http_token",
		UserID: "user-uuid-1",
	}))
	rec := httptest.NewRecorder()

	authHandler.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	want := map[string]any{
		"id":         "user-uuid-1",
		"username":   "robin",
		"email":      "robin@inox.com",
		"avatar_url": "https://cdn.example/avatar.png",
		"created_at": "2026-09-01T12:00:00Z",
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("expected %q = %v, got %v (body: %s)", key, value, body[key], rec.Body.String())
		}
	}

	for _, key := range []string{"password_hash", "PasswordHash"} {
		if _, ok := body[key]; ok {
			t.Errorf("response must not include %q (body: %s)", key, rec.Body.String())
		}
	}
}

// The browser attaches the session cookie by itself, so it can outlive the
// session the client actually holds. A stale cookie must not shadow a valid
// token sent explicitly alongside it, or a logged-in user gets a 401 and the
// frontend discards their session.
func TestRequireAuthCredentialPrecedence(t *testing.T) {
	mockSvc := newMockAuthService()
	mockSvc.sessions["sess_live"] = &domain.Session{ID: "sess_live", UserID: "user-live"}
	mockSvc.sessions["sess_other"] = &domain.Session{ID: "sess_other", UserID: "user-other"}

	var gotUserID string
	wrapped := middleware.RequireAuth(mockSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := middleware.GetSessionFromContext(r.Context())
		gotUserID = session.UserID
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name        string
		cookie      string
		bearer      string
		xSessionID  string
		query       string
		wantCode    int
		wantUserID  string
		wantCleared bool
	}{
		{name: "stale cookie, valid bearer", cookie: "sess_stale", bearer: "sess_live", wantCode: http.StatusOK, wantUserID: "user-live"},
		{name: "stale cookie, valid X-Session-ID", cookie: "sess_stale", xSessionID: "sess_live", wantCode: http.StatusOK, wantUserID: "user-live"},
		{name: "stale cookie, valid query param", cookie: "sess_stale", query: "sess_live", wantCode: http.StatusOK, wantUserID: "user-live"},
		{name: "stale bearer, valid cookie", cookie: "sess_live", bearer: "sess_stale", wantCode: http.StatusOK, wantUserID: "user-live"},
		{name: "cookie and bearer both valid, bearer wins", cookie: "sess_other", bearer: "sess_live", wantCode: http.StatusOK, wantUserID: "user-live"},
		{name: "nothing valid", cookie: "sess_stale", bearer: "sess_gone", wantCode: http.StatusUnauthorized, wantCleared: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID = ""
			target := "/api/v1/users/me"
			if tt.query != "" {
				target += "?session_id=" + tt.query
			}
			req := httptest.NewRequest("GET", target, nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: tt.cookie})
			}
			if tt.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tt.bearer)
			}
			if tt.xSessionID != "" {
				req.Header.Set("X-Session-ID", tt.xSessionID)
			}
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected HTTP %d, got %d", tt.wantCode, rec.Code)
			}
			if gotUserID != tt.wantUserID {
				t.Errorf("expected request to run as %q, got %q", tt.wantUserID, gotUserID)
			}
			cleared := false
			for _, c := range rec.Result().Cookies() {
				if c.Name == middleware.SessionCookieName && c.MaxAge < 0 {
					cleared = true
				}
			}
			if cleared != tt.wantCleared {
				t.Errorf("expected session cookie cleared = %v, got %v", tt.wantCleared, cleared)
			}
		})
	}
}
