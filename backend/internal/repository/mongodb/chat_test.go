package mongodb_test

import (
	"context"
	"os"
	"testing"
	"time"

	"backend/internal/database"
	"backend/internal/model"
	"backend/internal/repository/mongodb"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func getTestMongo(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017/llmchat"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := database.NewMongoClient(ctx, uri)
	if err != nil {
		t.Skipf("skipping mongodb integration test: %v", err)
	}

	db := client.Database("llmchat_test")
	if err := database.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("failed to ensure indexes: %v", err)
	}

	return client, db
}

func TestChatAndMessageRepositories(t *testing.T) {
	client, db := getTestMongo(t)
	ctx := context.Background()
	defer func() {
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}()

	chatRepo := mongodb.NewChatRepository(db)
	msgRepo := mongodb.NewMessageRepository(db)

	userID := int64(100500)
	chat := &model.Chat{
		UserID: userID,
		Title:  "Тестовый диалог",
		Model:  "openai/gpt-4o",
	}

	// 1. Create chat
	if err := chatRepo.Create(ctx, chat); err != nil {
		t.Fatalf("failed to create chat: %v", err)
	}
	if chat.ID.IsZero() {
		t.Fatal("expected non-zero chat ID")
	}

	// 2. Add messages
	msg1 := &model.Message{
		ChatID:           chat.ID,
		UserID:           userID,
		Role:             "user",
		Content:          "Привет!",
		PromptTokens:     5,
		CompletionTokens: 0,
		TotalTokens:      5,
		CostKopecks:      1,
	}
	if err := msgRepo.Create(ctx, msg1); err != nil {
		t.Fatalf("failed to create message 1: %v", err)
	}

	msg2 := &model.Message{
		ChatID:           chat.ID,
		UserID:           userID,
		Role:             "assistant",
		Content:          "Здравствуйте! Чем я могу помочь?",
		PromptTokens:     5,
		CompletionTokens: 10,
		TotalTokens:      15,
		CostKopecks:      3,
	}
	if err := msgRepo.Create(ctx, msg2); err != nil {
		t.Fatalf("failed to create message 2: %v", err)
	}

	// 3. List messages
	messages, err := msgRepo.ListByChatID(ctx, chat.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "Привет!" || messages[1].Content != "Здравствуйте! Чем я могу помочь?" {
		t.Errorf("unexpected message ordering or content")
	}

	// 4. Update title and Touch
	newTitle := "Обновленный диалог"
	if err := chatRepo.UpdateTitle(ctx, chat.ID, userID, newTitle); err != nil {
		t.Fatalf("failed to update title: %v", err)
	}

	if err := chatRepo.Touch(ctx, chat.ID, userID, "anthropic/claude-3-5-sonnet"); err != nil {
		t.Fatalf("failed to touch chat: %v", err)
	}

	updatedChat, err := chatRepo.GetByID(ctx, chat.ID, userID)
	if err != nil {
		t.Fatalf("failed to get chat: %v", err)
	}
	if updatedChat.Title != newTitle {
		t.Fatalf("expected title '%s', got '%s'", newTitle, updatedChat.Title)
	}
	if updatedChat.Model != "anthropic/claude-3-5-sonnet" {
		t.Fatalf("expected model anthropic/claude-3-5-sonnet, got '%s'", updatedChat.Model)
	}

	// 5. Delete chat and cascade delete messages
	if err := chatRepo.Delete(ctx, chat.ID, userID); err != nil {
		t.Fatalf("failed to delete chat: %v", err)
	}

	if err := msgRepo.DeleteByChatID(ctx, chat.ID); err != nil {
		t.Fatalf("failed to cascade delete messages: %v", err)
	}

	remainingMsgs, err := msgRepo.ListByChatID(ctx, chat.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list messages after cascade delete: %v", err)
	}
	if len(remainingMsgs) != 0 {
		t.Fatalf("expected 0 messages after cascade delete, got %d", len(remainingMsgs))
	}

	_, err = chatRepo.GetByID(ctx, chat.ID, userID)
	if err == nil {
		t.Fatal("expected error getting deleted chat, got nil")
	}
}
