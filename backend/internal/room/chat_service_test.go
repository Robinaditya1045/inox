package room_test

import (
	"context"
	"errors"
	"testing"
	// "time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
)

type mockChatRepository struct {
	messages []*domain.ChatMessage
}

func newMockChatRepository() *mockChatRepository {
	return &mockChatRepository{
		messages: make([]*domain.ChatMessage, 0),
	}
}

func (m *mockChatRepository) SaveMessage(ctx context.Context, msg *domain.ChatMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatRepository) GetRecentMessages(ctx context.Context, roomID string, limit int) ([]*domain.ChatMessage, error) {
	var roomMsgs []*domain.ChatMessage
	for _, msg := range m.messages {
		if msg.RoomID == roomID {
			roomMsgs = append(roomMsgs, msg)
		}
	}
	if len(roomMsgs) > limit {
		roomMsgs = roomMsgs[len(roomMsgs)-limit:]
	}
	return roomMsgs, nil
}

func TestChatServiceRBACAndHistoryWorkflow(t *testing.T) {
	ctx := context.Background()
	roomRepo := newMockRoomRepository()
	chatRepo := newMockChatRepository()

	roomService := room.NewRoomService(roomRepo, nil)
	chatService := room.NewChatService(chatRepo, roomRepo)

	ownerID := "user-alice"
	guestID := "user-bob"
	outsiderID := "user-charlie"

	// 1. Create Room by Alice
	rm, _, err := roomService.CreateRoom(ctx, ownerID, "Watch Party Chat Room", false)
	if err != nil {
		t.Fatalf("expected room creation to succeed, got %v", err)
	}

	// 2. Alice sends a chat message (Owner has CanSendMessages = true)
	msg1, err := chatService.SaveMessage(ctx, rm.ID, ownerID, "Alice", "Hello Watch Party!")
	if err != nil {
		t.Fatalf("expected owner to send message successfully, got %v", err)
	}
	if msg1.Message != "Hello Watch Party!" {
		t.Errorf("expected message content match, got %s", msg1.Message)
	}

	// 3. Outsider Charlie tries to send a message without joining
	_, err = chatService.SaveMessage(ctx, rm.ID, outsiderID, "Charlie", "I am not in the room!")
	if !errors.Is(err, room.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound for outsider sending chat, got %v", err)
	}

	// 4. Guest Bob joins room
	bobMem, err := roomService.JoinRoom(ctx, rm.ID, guestID)
	if err != nil {
		t.Fatalf("expected bob to join room, got %v", err)
	}

	// By default Guest has CanSendMessages = true in our domain matrix!
	if !bobMem.Permissions.CanSendMessages {
		t.Errorf("expected guest to have CanSendMessages = true by default")
	}

	// Bob sends a message
	_, err = chatService.SaveMessage(ctx, rm.ID, guestID, "Bob", "Hey Alice!")
	if err != nil {
		t.Fatalf("expected guest bob to send message, got %v", err)
	}

	// 5. Alice revokes Bob's CanSendMessages permission (Mute text chat!)
	mutedPerms := bobMem.Permissions
	mutedPerms.CanSendMessages = false
	_, err = roomService.AssignRole(ctx, rm.ID, ownerID, guestID, domain.RoleGuest, &mutedPerms)
	if err != nil {
		t.Fatalf("expected role assign to succeed, got %v", err)
	}

	// Bob tries to send another message while muted
	_, err = chatService.SaveMessage(ctx, rm.ID, guestID, "Bob", "Can anyone hear me?")
	if !errors.Is(err, room.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied when muted user tries to chat, got %v", err)
	}

	// 6. Test History Retrieval
	history, err := chatService.GetRecentMessages(ctx, rm.ID, guestID, 50)
	if err != nil {
		t.Fatalf("expected history retrieval to succeed, got %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 messages in room history, got %d", len(history))
	}

	// Outsider Charlie tries to read room chat history
	_, err = chatService.GetRecentMessages(ctx, rm.ID, outsiderID, 50)
	if !errors.Is(err, room.ErrMemberNotFound) {
		t.Errorf("expected ErrMemberNotFound when outsider reads history, got %v", err)
	}
}
