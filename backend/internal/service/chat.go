package service

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	SaveUserMessage(ctx context.Context, chatID bson.ObjectID, userID int64, content string) (*model.Message, error)
	SaveAssistantMessage(ctx context.Context, chatID bson.ObjectID, userID int64, content string, usage model.TokenUsage) (*model.Message, error)
	AutoUpdateTitleIfNeeded(ctx context.Context, chatID bson.ObjectID, userID int64, content string) error
	PreparePromptContext(ctx context.Context, chatID bson.ObjectID, userID int64, newMsg string, maxHistory int) ([]model.ChatMessage, *model.Chat, error)
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

// SaveUserMessage appends a user message to the conversation and triggers auto-titling if applicable.
func (s *chatService) SaveUserMessage(ctx context.Context, chatID bson.ObjectID, userID int64, content string) (*model.Message, error) {
	msg := &model.Message{
		ID:        bson.NewObjectID(),
		ChatID:    chatID,
		UserID:    userID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// Auto update title if this is the first user message
	_ = s.AutoUpdateTitleIfNeeded(ctx, chatID, userID, content)

	return msg, nil
}

// SaveAssistantMessage appends the generated assistant message with token usage metrics.
func (s *chatService) SaveAssistantMessage(ctx context.Context, chatID bson.ObjectID, userID int64, content string, usage model.TokenUsage) (*model.Message, error) {
	msg := &model.Message{
		ID:               bson.NewObjectID(),
		ChatID:           chatID,
		UserID:           userID,
		Role:             "assistant",
		Content:          content,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CostKopecks:      usage.CostKopecks,
		CreatedAt:        time.Now(),
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save assistant message: %w", err)
	}

	return msg, nil
}

// AutoUpdateTitleIfNeeded updates chat title from prompt if currently titled default ("Новый диалог").
func (s *chatService) AutoUpdateTitleIfNeeded(ctx context.Context, chatID bson.ObjectID, userID int64, content string) error {
	chat, err := s.chatRepo.GetByID(ctx, chatID, userID)
	if err != nil {
		return err
	}

	if chat.Title != DefaultChatTitle {
		return nil
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	runes := []rune(trimmed)
	var newTitle string
	if len(runes) > 45 {
		newTitle = string(runes[:45]) + "..."
	} else {
		newTitle = string(runes)
	}

	if newTitle != "" && newTitle != chat.Title {
		return s.chatRepo.UpdateTitle(ctx, chatID, userID, newTitle)
	}

	return nil
}

// PreparePromptContext loads the sliding window (up to maxHistory messages) and appends the new message.
func (s *chatService) PreparePromptContext(ctx context.Context, chatID bson.ObjectID, userID int64, newMsg string, maxHistory int) ([]model.ChatMessage, *model.Chat, error) {
	chat, err := s.chatRepo.GetByID(ctx, chatID, userID)
	if err != nil {
		return nil, nil, err
	}

	if maxHistory <= 0 {
		maxHistory = 20
	}

	// Fetch messages from repository (ordered chronologically ASC)
	msgs, err := s.messageRepo.ListByChatID(ctx, chatID, 100, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list chat messages: %w", err)
	}

	// Apply sliding window: take up to maxHistory latest messages
	if len(msgs) > maxHistory {
		msgs = msgs[len(msgs)-maxHistory:]
	}

	chatMessages := make([]model.ChatMessage, 0, len(msgs)+1)
	for _, m := range msgs {
		chatMessages = append(chatMessages, model.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	if strings.TrimSpace(newMsg) != "" {
		chatMessages = append(chatMessages, model.ChatMessage{
			Role:    "user",
			Content: newMsg,
		})
	}

	return chatMessages, chat, nil
}
