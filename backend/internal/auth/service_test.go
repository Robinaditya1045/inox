package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// mockUserRepository implements auth.UserRepository in memory for fast unit testing.
type mockUserRepository struct {
	users map[string]*domain.User
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{users: make(map[string]*domain.User)}
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error {
	for _, u := range m.users {
		if u.Email == user.Email {
			return auth.ErrEmailAlreadyTaken
		}
	}
	user.ID = "user-mock-uuid-1234"
	m.users[user.Email] = user
	return nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, exists := m.users[email]
	if !exists {
		return nil, auth.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, auth.ErrUserNotFound
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, auth.ErrUserNotFound
}

// mockSessionStore implements auth.SessionStore in memory.
type mockSessionStore struct {
	sessions map[string]*domain.Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]*domain.Session)}
}

func (m *mockSessionStore) Create(ctx context.Context, session *domain.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionStore) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	s, exists := m.sessions[sessionID]
	if !exists || time.Now().After(s.ExpiresAt) {
		return nil, auth.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockSessionStore) Delete(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockSessionStore) GenerateSessionID() (string, error) {
	return "sess_test_random_token_8899", nil
}

func TestSignupAndLoginWorkflow(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepository()
	sessionStore := newMockSessionStore()
	service := auth.NewAuthService(userRepo, sessionStore)

	// 1. Test Signup Success
	session, err := service.Signup(ctx, "robin", "robin@inox.com", "supersecret123")
	if err != nil {
		t.Fatalf("expected successful signup, got %v", err)
	}
	if session.Username != "robin" || session.Email != "robin@inox.com" {
		t.Errorf("unexpected session data returned: %+v", session)
	}

	// 2. Test Signup Duplicate Email
	_, err = service.Signup(ctx, "robin2", "robin@inox.com", "anotherpassword123")
	if !errors.Is(err, auth.ErrEmailAlreadyTaken) {
		t.Errorf("expected ErrEmailAlreadyTaken, got %v", err)
	}

	// 3. Test Signup Invalid Input (short password)
	_, err = service.Signup(ctx, "baduser", "bad@inox.com", "123")
	if !errors.Is(err, auth.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for short password, got %v", err)
	}

	// 4. Test Login Success
	loginSession, err := service.Login(ctx, "robin@inox.com", "supersecret123")
	if err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}
	if loginSession.ID != "sess_test_random_token_8899" {
		t.Errorf("expected generated session token, got %s", loginSession.ID)
	}

	// 5. Test Login Wrong Password (verifies anti-enumeration ErrInvalidCredentials)
	_, err = service.Login(ctx, "robin@inox.com", "wrongpassword!")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got %v", err)
	}

	// 6. Test Logout
	err = service.Logout(ctx, loginSession.ID)
	if err != nil {
		t.Fatalf("expected successful logout, got %v", err)
	}
	_, err = service.ValidateSession(ctx, loginSession.ID)
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound after logout, got %v", err)
	}
}
