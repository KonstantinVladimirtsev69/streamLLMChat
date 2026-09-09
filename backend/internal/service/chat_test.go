package service_test

import (
	"context"
	"errors"
	"testing"

	"backend/internal/model"
	"backend/internal/service"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Mock repositories for testing ChatService
type mockChatRepo struct {
	createFunc      func(ctx context.Context, chat *model.Chat) error
	getByIDFunc     func(ctx context.Context, id bson.ObjectID, userID int64) (*model.Chat, error)
	listByUIDFunc   func(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error)
	updateTitleFunc func(ctx context.Context, id bson.ObjectID, userID int64, title string) error
	touchFunc       func(ctx context.Context, id bson.ObjectID, userID int64, model string) error
	deleteFunc      func(ctx context.Context, id bson.ObjectID, userID int64) error
}

func (m *mockChatRepo) Create(ctx context.Context, chat *model.Chat) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, chat)
	}
	chat.ID = bson.NewObjectID()
	return nil
}

func (m *mockChatRepo) GetByID(ctx context.Context, id bson.ObjectID, userID int64) (*model.Chat, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id, userID)
	}
	return nil, model.ErrNotFound
}

func (m *mockChatRepo) ListByUserID(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error) {
	if m.listByUIDFunc != nil {
		return m.listByUIDFunc(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *mockChatRepo) UpdateTitle(ctx context.Context, id bson.ObjectID, userID int64, title string) error {
	if m.updateTitleFunc != nil {
		return m.updateTitleFunc(ctx, id, userID, title)
	}
	return nil
}

func (m *mockChatRepo) Touch(ctx context.Context, id bson.ObjectID, userID int64, model string) error {
	if m.touchFunc != nil {
		return m.touchFunc(ctx, id, userID, model)
	}
	return nil
}

func (m *mockChatRepo) Delete(ctx context.Context, id bson.ObjectID, userID int64) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id, userID)
	}
	return nil
}

type mockMessageRepo struct {
	createFunc         func(ctx context.Context, msg *model.Message) error
	listByChatIDFunc   func(ctx context.Context, chatID bson.ObjectID, limit, offset int64) ([]model.Message, error)
	deleteByChatIDFunc func(ctx context.Context, chatID bson.ObjectID) error
}

func (m *mockMessageRepo) Create(ctx context.Context, msg *model.Message) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, msg)
	}
	msg.ID = bson.NewObjectID()
	return nil
}

func (m *mockMessageRepo) ListByChatID(ctx context.Context, chatID bson.ObjectID, limit, offset int64) ([]model.Message, error) {
	if m.listByChatIDFunc != nil {
		return m.listByChatIDFunc(ctx, chatID, limit, offset)
	}
	return nil, nil
}

func (m *mockMessageRepo) DeleteByChatID(ctx context.Context, chatID bson.ObjectID) error {
	if m.deleteByChatIDFunc != nil {
		return m.deleteByChatIDFunc(ctx, chatID)
	}
	return nil
}

func TestChatService_CreateChat(t *testing.T) {
	ctx := context.Background()
	chatRepo := &mockChatRepo{}
	msgRepo := &mockMessageRepo{}
	svc := service.NewChatService(chatRepo, msgRepo)

	// Test default title and model
	c1, err := svc.CreateChat(ctx, 100, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c1.Title != service.DefaultChatTitle {
		t.Errorf("expected default title %s, got %s", service.DefaultChatTitle, c1.Title)
	}
	if c1.Model != service.DefaultChatModel {
		t.Errorf("expected default model %s, got %s", service.DefaultChatModel, c1.Model)
	}
	if c1.UserID != 100 {
		t.Errorf("expected userID 100, got %d", c1.UserID)
	}

	// Test custom title and model
	c2, err := svc.CreateChat(ctx, 100, "Мой чат", "anthropic/claude-3-5-sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c2.Title != "Мой чат" {
		t.Errorf("expected title 'Мой чат', got %s", c2.Title)
	}
	if c2.Model != "anthropic/claude-3-5-sonnet" {
		t.Errorf("expected model 'anthropic/claude-3-5-sonnet', got %s", c2.Model)
	}
}

func TestChatService_GetChat_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	ownerID := int64(1)
	otherID := int64(2)
	chatID := bson.NewObjectID()

	chatRepo := &mockChatRepo{
		getByIDFunc: func(ctx context.Context, id bson.ObjectID, userID int64) (*model.Chat, error) {
			if id == chatID && userID == ownerID {
				return &model.Chat{ID: chatID, UserID: ownerID, Title: "Диалог"}, nil
			}
			return nil, model.ErrNotFound
		},
	}
	svc := service.NewChatService(chatRepo, &mockMessageRepo{})

	// Owner can access
	chat, err := svc.GetChat(ctx, chatID, ownerID)
	if err != nil {
		t.Fatalf("expected owner access, got error: %v", err)
	}
	if chat.ID != chatID {
		t.Fatalf("expected chat ID %s, got %s", chatID.Hex(), chat.ID.Hex())
	}

	// Other user gets ErrNotFound (HTTP 404 behavior)
	_, err = svc.GetChat(ctx, chatID, otherID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-owner, got: %v", err)
	}
}

func TestChatService_UpdateChatTitle(t *testing.T) {
	ctx := context.Background()
	chatID := bson.NewObjectID()
	userID := int64(10)

	var updatedTitle string
	chatRepo := &mockChatRepo{
		updateTitleFunc: func(ctx context.Context, id bson.ObjectID, uid int64, title string) error {
			if id == chatID && uid == userID {
				updatedTitle = title
				return nil
			}
			return model.ErrNotFound
		},
		getByIDFunc: func(ctx context.Context, id bson.ObjectID, uid int64) (*model.Chat, error) {
			return &model.Chat{ID: id, UserID: uid, Title: updatedTitle}, nil
		},
	}
	svc := service.NewChatService(chatRepo, &mockMessageRepo{})

	// Reject empty title
	_, err := svc.UpdateChatTitle(ctx, chatID, userID, "   ")
	if err == nil {
		t.Fatal("expected error on empty title, got nil")
	}

	// Success update
	chat, err := svc.UpdateChatTitle(ctx, chatID, userID, "Новое название")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chat.Title != "Новое название" {
		t.Errorf("expected title 'Новое название', got %s", chat.Title)
	}
}

func TestChatService_DeleteChat_Cascade(t *testing.T) {
	ctx := context.Background()
	chatID := bson.NewObjectID()
	userID := int64(10)

	var chatDeleted bool
	var msgsDeleted bool

	chatRepo := &mockChatRepo{
		deleteFunc: func(ctx context.Context, id bson.ObjectID, uid int64) error {
			if id == chatID && uid == userID {
				chatDeleted = true
				return nil
			}
			return model.ErrNotFound
		},
	}
	msgRepo := &mockMessageRepo{
		deleteByChatIDFunc: func(ctx context.Context, id bson.ObjectID) error {
			if id == chatID {
				msgsDeleted = true
				return nil
			}
			return nil
		},
	}
	svc := service.NewChatService(chatRepo, msgRepo)

	// Delete non-owner -> ErrNotFound, messages NOT deleted
	err := svc.DeleteChat(ctx, chatID, 999)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if chatDeleted || msgsDeleted {
		t.Fatal("expected nothing deleted on non-owner call")
	}

	// Delete owner -> chat deleted AND messages cascade deleted
	err = svc.DeleteChat(ctx, chatID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !chatDeleted {
		t.Fatal("expected chat to be deleted")
	}
	if !msgsDeleted {
		t.Fatal("expected messages to be cascade deleted")
	}
}

func TestChatService_ListMessages_OwnershipCheck(t *testing.T) {
	ctx := context.Background()
	chatID := bson.NewObjectID()
	ownerID := int64(1)
	otherID := int64(2)

	chatRepo := &mockChatRepo{
		getByIDFunc: func(ctx context.Context, id bson.ObjectID, uid int64) (*model.Chat, error) {
			if id == chatID && uid == ownerID {
				return &model.Chat{ID: id, UserID: uid}, nil
			}
			return nil, model.ErrNotFound
		},
	}
	msgRepo := &mockMessageRepo{
		listByChatIDFunc: func(ctx context.Context, id bson.ObjectID, limit, offset int64) ([]model.Message, error) {
			return []model.Message{{ID: bson.NewObjectID(), ChatID: id, Content: "Hello"}}, nil
		},
	}
	svc := service.NewChatService(chatRepo, msgRepo)

	// Other user cannot list messages
	_, err := svc.ListMessages(ctx, chatID, otherID, 10, 0)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}

	// Owner can list messages
	msgs, err := svc.ListMessages(ctx, chatID, ownerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "Hello" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestChatService_TouchChat(t *testing.T) {
	ctx := context.Background()
	chatID := bson.NewObjectID()
	userID := int64(1)

	var touchedModel string
	chatRepo := &mockChatRepo{
		touchFunc: func(ctx context.Context, id bson.ObjectID, uid int64, m string) error {
			touchedModel = m
			return nil
		},
	}
	svc := service.NewChatService(chatRepo, &mockMessageRepo{})

	err := svc.TouchChat(ctx, chatID, userID, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if touchedModel != "gpt-4o" {
		t.Fatalf("expected touched model 'gpt-4o', got %s", touchedModel)
	}
}
