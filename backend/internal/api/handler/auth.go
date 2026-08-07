package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
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
		respond.WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	session, err := h.authService.Signup(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidInput) || errors.Is(err, auth.ErrEmailAlreadyTaken) || strings.Contains(err.Error(), "already") {
			respond.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create user account: %v", err))
		return
	}

	h.setSessionCookie(w, session.ID, session.ExpiresAt)
	respond.WriteJSON(w, http.StatusCreated, session)
}

// Login verifies user credentials and sets an HttpOnly session cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	session, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			respond.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("login failed due to server error: %v", err))
		return
	}

	h.setSessionCookie(w, session.ID, session.ExpiresAt)
	respond.WriteJSON(w, http.StatusOK, session)
}

// Logout revokes the Redis session and purges the cookie from browser memory.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("inox_session")
	if err == nil && cookie.Value != "" {
		_ = h.authService.Logout(r.Context(), cookie.Value)
	}

	sameSite := http.SameSiteLaxMode
	if h.isProd {
		sameSite = http.SameSiteNoneMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "inox_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: sameSite,
	})

	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// setSessionCookie applies secure attributes (HttpOnly, Secure, SameSite) to protect session identity.
func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	sameSite := http.SameSiteLaxMode
	if h.isProd {
		sameSite = http.SameSiteNoneMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "inox_session",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.isProd, // true in production (HTTPS), false in local development (HTTP)
		SameSite: sameSite,
	})
}

// Me returns the authenticated user's profile retrieved from the database.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok || session == nil {
		respond.WriteError(w, http.StatusUnauthorized, "session context missing")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), session.UserID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	// Mask sensitive data
	user.PasswordHash = ""

	respond.WriteJSON(w, http.StatusOK, user)
}

// UpdateAvatar handles user profile avatar updates.
func (h *AuthHandler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok || session == nil {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.authService.UpdateAvatar(r.Context(), session.UserID, req.AvatarURL, session.ID); err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to update avatar")
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "avatar updated successfully"})
}

// ForgotPassword handles initiating the password reset flow.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	token, err := h.authService.GeneratePasswordResetToken(r.Context(), req.Email)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to process request")
		return
	}

	// In a real application, send this token via email.
	// For this demo, we return it in the API response.
	respond.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "If an account exists, a reset token has been generated.",
		"token":   token, // Development only
	})
}

// ResetPassword handles completing the password reset flow.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request json payload")
		return
	}

	if err := h.authService.ResetPasswordWithToken(r.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidInput) {
			respond.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		respond.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}
