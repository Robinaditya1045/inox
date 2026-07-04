package domain

import (
	"time"
)

// ChatMessage represents a persistent text message sent inside a Watch Party room workspace.
type ChatMessage struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
