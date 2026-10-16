package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/inox/inox/backend/internal/admin"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/domain"
)

// AdminAccessHandler serves who-has-access for the admin portal. Every route sits
// behind RequireAdminRole or RequireAdminOwner, so a normal user never gets an answer
// from any of them, not even about themselves.
type AdminAccessHandler struct {
	adminService admin.Service
}

func NewAdminAccessHandler(adminService admin.Service) *AdminAccessHandler {
	return &AdminAccessHandler{adminService: adminService}
}

type adminMeResponse struct {
	UserID   string           `json:"user_id"`
	Username string           `json:"username"`
	Email    string           `json:"email"`
	Role     domain.AdminRole `json:"role"`
}

type grantAdminRequest struct {
	// Identifier is an email address or a username.
	Identifier string `json:"identifier"`
}

// Me tells the admin portal which role the caller holds, so it shows owners the
// controls for managing admins.
func (h *AdminAccessHandler) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	role, hasRole := middleware.GetAdminRoleFromContext(r.Context())
	if !ok || !hasRole {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	respond.WriteJSON(w, http.StatusOK, adminMeResponse{
		UserID:   session.UserID,
		Username: session.Username,
		Email:    session.Email,
		Role:     role,
	})
}

// List returns every owner and admin. Owner only.
func (h *AdminAccessHandler) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.adminService.List(r.Context())
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to list admins")
		return
	}
	respond.WriteJSON(w, http.StatusOK, accounts)
}

// Grant makes an existing user an admin. Owner only.
func (h *AdminAccessHandler) Grant(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body grantAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	account, err := h.adminService.Grant(r.Context(), session, body.Identifier)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrIdentifierRequired):
			respond.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, admin.ErrUserNotFound):
			respond.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, admin.ErrAlreadyAdmin), errors.Is(err, admin.ErrAlreadyOwner):
			respond.WriteError(w, http.StatusConflict, err.Error())
		default:
			respond.WriteError(w, http.StatusInternalServerError, "failed to add admin")
		}
		return
	}

	respond.WriteJSON(w, http.StatusCreated, account)
}

// Revoke removes an admin. Owners live in ADMIN_EMAILS and cannot be removed here. Owner only.
func (h *AdminAccessHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing user id")
		return
	}

	if err := h.adminService.Revoke(r.Context(), userID); err != nil {
		switch {
		case errors.Is(err, admin.ErrNotAdmin):
			respond.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, admin.ErrCannotRevokeOwner):
			respond.WriteError(w, http.StatusConflict, err.Error())
		default:
			respond.WriteError(w, http.StatusInternalServerError, "failed to remove admin")
		}
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "admin removed"})
}
