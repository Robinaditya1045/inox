package domain

import (
	"time"
)

// User represents the core entity in the user management domain.
// Notice it contains no database annotations or transport layer JSON tags.
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
