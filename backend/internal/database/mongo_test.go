package database_test

import (
	"context"
	"testing"
	"time"

	"backend/internal/database"
	"backend/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestModelBSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	chatID := bson.NewObjectID()
	msgID := bson.NewObjectID()

	chat := model.Chat{
		ID:        chatID,
		UserID:    42,
		Title:     "Test Conversation",
		Model:     "openai/gpt-4o",
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := bson.Marshal(chat)
	if err != nil {
		t.Fatalf("failed to marshal chat: %v", err)
	}

	var decodedChat model.Chat
	if err := bson.Unmarshal(data, &decodedChat); err != nil {
		t.Fatalf("failed to unmarshal chat: %v", err)
	}

	if decodedChat.ID != chat.ID || decodedChat.UserID != chat.UserID || decodedChat.Title != chat.Title {
		t.Errorf("decoded chat mismatch: got %+v, want %+v", decodedChat, chat)
	}

	msg := model.Message{
		ID:               msgID,
		ChatID:           chatID,
		UserID:           42,
		Role:             "user",
		Content:          "Hello world",
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		CostKopecks:      15,
		CreatedAt:        now,
	}

	msgData, err := bson.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var decodedMsg model.Message
	if err := bson.Unmarshal(msgData, &decodedMsg); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decodedMsg.CostKopecks != 15 || decodedMsg.TotalTokens != 30 {
		t.Errorf("decoded message mismatch: got %+v, want %+v", decodedMsg, msg)
	}
}

func TestNewMongoClientInvalidURI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := database.NewMongoClient(ctx, "invalid-uri://")
	if err == nil {
		t.Error("expected error for invalid URI, got nil")
	}
}
