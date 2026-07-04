package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrInvalidMessage = errors.New("message must be between 1 and 2000 characters")
)

// ChatService defines the business logic contract for room text chat governance and history retrieval.
type ChatService interface {
	SaveMessage(ctx context.Context, roomID, userID, username, messageText string) (*domain.ChatMessage, error)
	GetRecentMessages(ctx context.Context, roomID, requesterID string, limit int) ([]*domain.ChatMessage, error)
}

type chatService struct {
	chatRepo ChatRepository
	roomRepo RoomRepository
}

// NewChatService initializes a new room chat business logic controller.
func NewChatService(chatRepo ChatRepository, roomRepo RoomRepository) ChatService {
	return &chatService{
		chatRepo: chatRepo,
		roomRepo: roomRepo,
	}
}

// SaveMessage validates member chat permissions and persists the message to PostgreSQL.
func (s *chatService) SaveMessage(ctx context.Context, roomID, userID, username, messageText string) (*domain.ChatMessage, error) {
	messageText = strings.TrimSpace(messageText)
	if len(messageText) == 0 || len(messageText) > 2000 {
		return nil, ErrInvalidMessage
	}

	// 1. Verify user is a member of the room and has CanSendMessages permission
	member, err := s.roomRepo.GetMember(ctx, roomID, userID)
	if err != nil {
		return nil, ErrMemberNotFound
	}

	if !member.Permissions.CanSendMessages {
		return nil, ErrPermissionDenied
	}

	// 2. Generate unique message ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed to generate message id: %w", err)
	}

	msg := &domain.ChatMessage{
		ID:        hex.EncodeToString(idBytes),
		RoomID:    roomID,
		UserID:    userID,
		Username:  username,
		Message:   messageText,
		CreatedAt: time.Now().UTC(),
	}

	// 3. Persist to storage
	if err := s.chatRepo.SaveMessage(ctx, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

// GetRecentMessages verifies room membership and retrieves chronological message history.
func (s *chatService) GetRecentMessages(ctx context.Context, roomID, requesterID string, limit int) ([]*domain.ChatMessage, error) {
	// 1. Verify requester is an authorized room member
	_, err := s.roomRepo.GetMember(ctx, roomID, requesterID)
	if err != nil {
		return nil, ErrMemberNotFound
	}

	// 2. Fetch recent messages
	return s.chatRepo.GetRecentMessages(ctx, roomID, limit)
}
