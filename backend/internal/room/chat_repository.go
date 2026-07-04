package room

import (
	"context"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	// "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatRepository defines the persistence contract for Watch Party room chat histories.
type ChatRepository interface {
	SaveMessage(ctx context.Context, msg *domain.ChatMessage) error
	GetRecentMessages(ctx context.Context, roomID string, limit int) ([]*domain.ChatMessage, error)
}

type postgresChatRepository struct {
	db *pgxpool.Pool
}

// NewChatRepository initializes a PostgreSQL chat storage adapter.
func NewChatRepository(db *pgxpool.Pool) ChatRepository {
	return &postgresChatRepository{db: db}
}

// SaveMessage persists a new room chat message into PostgreSQL.
func (r *postgresChatRepository) SaveMessage(ctx context.Context, msg *domain.ChatMessage) error {
	query := `
		INSERT INTO chat_messages (id, room_id, user_id, username, message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, msg.ID, msg.RoomID, msg.UserID, msg.Username, msg.Message, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert chat message: %w", err)
	}
	return nil
}

// GetRecentMessages retrieves the latest chat messages for a room, ordered chronologically.
func (r *postgresChatRepository) GetRecentMessages(ctx context.Context, roomID string, limit int) ([]*domain.ChatMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, room_id, user_id, username, message, created_at
		FROM chat_messages
		WHERE room_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent messages failed: %w", err)
	}
	defer rows.Close()

	var messages []*domain.ChatMessage
	for rows.Next() {
		msg := &domain.ChatMessage{}
		if err := rows.Scan(&msg.ID, &msg.RoomID, &msg.UserID, &msg.Username, &msg.Message, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat message failed: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	// Reverse slice so messages are ordered oldest to newest for UI display
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
