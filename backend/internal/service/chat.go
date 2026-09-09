package service

import (
	"context"
	"fmt"
	"strings"

	"backend/internal/model"
	"backend/internal/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// DefaultChatTitle is the fallback title for newly created chats.
const DefaultChatTitle = "Новый диалог"

// DefaultChatModel is the fallback model for newly created chats.
const DefaultChatModel = "openai/gpt-4o-mini"

// ChatService defines business logic operations for chats and messages.
type ChatService interface {
	CreateChat(ctx context.Context, userID int64, title, modelName string) (*model.Chat, error)
	ListChats(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error)
	GetChat(ctx context.Context, chatID bson.ObjectID, userID int64) (*model.Chat, error)
	UpdateChatTitle(ctx context.Context, chatID bson.ObjectID, userID int64, title string) (*model.Chat, error)
	DeleteChat(ctx context.Context, chatID bson.ObjectID, userID int64) error
	ListMessages(ctx context.Context, chatID bson.ObjectID, userID int64, limit, offset int64) ([]model.Message, error)
	TouchChat(ctx context.Context, chatID bson.ObjectID, userID int64, modelName string) error
}

type chatService struct {
	chatRepo    repository.ChatRepository
	messageRepo repository.MessageRepository
}

// NewChatService constructs a new ChatService.
func NewChatService(chatRepo repository.ChatRepository, messageRepo repository.MessageRepository) ChatService {
	return &chatService{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
	}
}

// CreateChat creates a new chat with default title and model if omitted.
func (s *chatService) CreateChat(ctx context.Context, userID int64, title, modelName string) (*model.Chat, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = DefaultChatTitle
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		modelName = DefaultChatModel
	}

	chat := &model.Chat{
		UserID: userID,
		Title:  title,
		Model:  modelName,
	}

	if err := s.chatRepo.Create(ctx, chat); err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	return chat, nil
}

// ListChats returns chats for a specific user ordered by updated_at descending.
func (s *chatService) ListChats(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error) {
	return s.chatRepo.ListByUserID(ctx, userID, limit, offset)
}

// GetChat retrieves a chat ensuring user ownership.
func (s *chatService) GetChat(ctx context.Context, chatID bson.ObjectID, userID int64) (*model.Chat, error) {
	return s.chatRepo.GetByID(ctx, chatID, userID)
}

// UpdateChatTitle renames a chat ensuring user ownership.
func (s *chatService) UpdateChatTitle(ctx context.Context, chatID bson.ObjectID, userID int64, title string) (*model.Chat, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("%w: title cannot be empty", model.ErrInvalidOperation)
	}

	if err := s.chatRepo.UpdateTitle(ctx, chatID, userID, title); err != nil {
		return nil, err
	}

	return s.chatRepo.GetByID(ctx, chatID, userID)
}

// DeleteChat deletes a chat and cascades deletion to all its messages.
func (s *chatService) DeleteChat(ctx context.Context, chatID bson.ObjectID, userID int64) error {
	// Delete chat record first (validates ownership)
	if err := s.chatRepo.Delete(ctx, chatID, userID); err != nil {
		return err
	}

	// Cascade delete messages for this chat
	if err := s.messageRepo.DeleteByChatID(ctx, chatID); err != nil {
		return fmt.Errorf("failed to cascade delete messages: %w", err)
	}

	return nil
}

// ListMessages retrieves messages for a chat after verifying user ownership of the chat.
func (s *chatService) ListMessages(ctx context.Context, chatID bson.ObjectID, userID int64, limit, offset int64) ([]model.Message, error) {
	// Verify user ownership of the chat
	if _, err := s.chatRepo.GetByID(ctx, chatID, userID); err != nil {
		return nil, err
	}

	return s.messageRepo.ListByChatID(ctx, chatID, limit, offset)
}

// TouchChat updates the updated_at timestamp and optionally the model of the chat.
func (s *chatService) TouchChat(ctx context.Context, chatID bson.ObjectID, userID int64, modelName string) error {
	return s.chatRepo.Touch(ctx, chatID, userID, modelName)
}
