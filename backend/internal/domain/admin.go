package domain

import "time"

// AdminRole is a user's level of access to the admin portal.
//
// It is resolved per request and never stored on the user or their session, so it
// never reaches a response the main app reads: a normal user cannot tell who holds it.
type AdminRole string

const (
	// AdminRoleNone is everyone without admin portal access.
	AdminRoleNone AdminRole = ""
	// AdminRoleAdmin can use the admin portal but cannot change who has access.
	AdminRoleAdmin AdminRole = "admin"
	// AdminRoleOwner is an address listed in ADMIN_EMAILS: everything an admin can
	// do, plus adding and removing admins.
	AdminRoleOwner AdminRole = "owner"
)

// AdminAccount is one person with admin portal access, as shown to owners.
type AdminAccount struct {
	// UserID and Username are empty for an owner address nobody has signed up with yet.
	UserID   string    `json:"user_id,omitempty"`
	Username string    `json:"username,omitempty"`
	Email    string    `json:"email"`
	Role     AdminRole `json:"role"`
	// GrantedBy is the username of the owner who added an admin; empty for owners,
	// and for admins whose granting owner has since deleted their account.
	GrantedBy string     `json:"granted_by,omitempty"`
	GrantedAt *time.Time `json:"granted_at,omitempty"`
}
