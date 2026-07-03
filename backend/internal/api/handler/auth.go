package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/inox/inox/backend/internal/auth"
)

type AuthHandler struct {
	authService auth.AuthService
	isProd      bool
}

func NewAuthHandler(authService auth.AuthService, isProd bool) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		isProd:      isProd,
	}
}

type signupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Signup handles user registration and sets an HttpOnly session cookie upon success.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	session, err := h.authService.Signup(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidInput) || errors.Is(err, auth.ErrEmailAlreadyTaken) {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to create user account")
		return
	}

	h.setSessionCookie(w, session.ID, session.ExpiresAt)
	WriteJSON(w, http.StatusCreated, session)
}

// Login verifies user credentials and sets an HttpOnly session cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	session, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "login failed due to server error")
		return
	}

	h.setSessionCookie(w, session.ID, session.ExpiresAt)
	WriteJSON(w, http.StatusOK, session)
}

// Logout revokes the Redis session and purges the cookie from browser memory.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("inox_session")
	if err == nil && cookie.Value != "" {
		_ = h.authService.Logout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "inox_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: http.SameSiteLaxMode,
	})

	WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// setSessionCookie applies secure attributes (HttpOnly, Secure, SameSite) to protect session identity.
func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "inox_session",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.isProd, // true in production (HTTPS), false in local development (HTTP)
		SameSite: http.SameSiteLaxMode,
	})
}
