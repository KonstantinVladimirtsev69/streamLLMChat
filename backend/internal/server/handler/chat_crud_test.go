package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/llm"
	"backend/internal/model"
	"backend/internal/server/handler"
	"backend/internal/server/middleware"
	"backend/internal/service"

	chi "github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeChatService struct {
	chats    map[string]*model.Chat
	messages map[string][]model.Message
}

func newFakeChatService() *fakeChatService {
	return &fakeChatService{
		chats:    make(map[string]*model.Chat),
		messages: make(map[string][]model.Message),
	}
}

func (f *fakeChatService) CreateChat(ctx context.Context, userID int64, title, modelName string) (*model.Chat, error) {
	if title == "" {
		title = service.DefaultChatTitle
	}
	if modelName == "" {
		modelName = service.DefaultChatModel
	}
	c := &model.Chat{
		ID:        bson.NewObjectID(),
		UserID:    userID,
		Title:     title,
		Model:     modelName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.chats[c.ID.Hex()] = c
	return c, nil
}

func (f *fakeChatService) ListChats(ctx context.Context, userID int64, limit, offset int64) ([]model.Chat, error) {
	var result []model.Chat
	for _, c := range f.chats {
		if c.UserID == userID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (f *fakeChatService) GetChat(ctx context.Context, chatID bson.ObjectID, userID int64) (*model.Chat, error) {
	c, ok := f.chats[chatID.Hex()]
	if !ok || c.UserID != userID {
		return nil, model.ErrNotFound
	}
	return c, nil
}

func (f *fakeChatService) UpdateChatTitle(ctx context.Context, chatID bson.ObjectID, userID int64, title string) (*model.Chat, error) {
	c, ok := f.chats[chatID.Hex()]
	if !ok || c.UserID != userID {
		return nil, model.ErrNotFound
	}
	c.Title = title
	c.UpdatedAt = time.Now()
	return c, nil
}

func (f *fakeChatService) DeleteChat(ctx context.Context, chatID bson.ObjectID, userID int64) error {
	c, ok := f.chats[chatID.Hex()]
	if !ok || c.UserID != userID {
		return model.ErrNotFound
	}
	delete(f.chats, chatID.Hex())
	delete(f.messages, chatID.Hex())
	return nil
}

func (f *fakeChatService) ListMessages(ctx context.Context, chatID bson.ObjectID, userID int64, limit, offset int64) ([]model.Message, error) {
	c, ok := f.chats[chatID.Hex()]
	if !ok || c.UserID != userID {
		return nil, model.ErrNotFound
	}
	return f.messages[chatID.Hex()], nil
}

func (f *fakeChatService) TouchChat(ctx context.Context, chatID bson.ObjectID, userID int64, modelName string) error {
	c, ok := f.chats[chatID.Hex()]
	if !ok || c.UserID != userID {
		return model.ErrNotFound
	}
	c.UpdatedAt = time.Now()
	if modelName != "" {
		c.Model = modelName
	}
	return nil
}

func setupChatCRUDTestRouter(chatSvc service.ChatService) (http.Handler, auth.TokenManager) {
	tm, _ := auth.NewJWTTokenManager("super-secret-key-at-least-16-chars")
	mock := llm.NewMockLLMProvider()
	cache := llm.NewModelCache(mock, 15*time.Minute)
	billing := &fakeBillingService{canGen: true, balance: 1000}
	h := handler.NewChatHandler(mock, cache, billing, chatSvc)

	r := chi.NewRouter()
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.AuthMiddleware(tm))
		pr.Route("/api/v1/chats", func(cr chi.Router) {
			cr.Post("/", h.CreateChat)
			cr.Get("/", h.ListChats)
			cr.Get("/{id}", h.GetChat)
			cr.Patch("/{id}", h.UpdateChatTitle)
			cr.Delete("/{id}", h.DeleteChat)
			cr.Get("/{id}/messages", h.ListMessages)
		})
	})

	return r, tm
}

func TestChatCRUD_CreateChat(t *testing.T) {
	fakeSvc := newFakeChatService()
	r, tm := setupChatCRUDTestRouter(fakeSvc)

	// 1. Unauthorized
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. Authorized create default
	token, _ := tm.GenerateToken(101, 1001, time.Hour)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/chats", bytes.NewBufferString("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Chat model.Chat `json:"chat"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Chat.Title != service.DefaultChatTitle {
		t.Errorf("expected title '%s', got '%s'", service.DefaultChatTitle, resp.Chat.Title)
	}
	if resp.Chat.UserID != 101 {
		t.Errorf("expected userID 101, got %d", resp.Chat.UserID)
	}
}

func TestChatCRUD_ListAndGetChat_TenantIsolation(t *testing.T) {
	fakeSvc := newFakeChatService()
	r, tm := setupChatCRUDTestRouter(fakeSvc)

	ctx := context.Background()
	user1Chat, _ := fakeSvc.CreateChat(ctx, 101, "Чат 1", "gpt-4o")
	user2Chat, _ := fakeSvc.CreateChat(ctx, 202, "Чат 2", "gpt-4o")

	token1, _ := tm.GenerateToken(101, 1001, time.Hour)
	token2, _ := tm.GenerateToken(202, 2002, time.Hour)

	// List chats for User 1: should only see user1Chat
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var listResp struct {
		Chats []model.Chat `json:"chats"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Chats) != 1 || listResp.Chats[0].ID != user1Chat.ID {
		t.Fatalf("expected only user 1 chat in list, got %+v", listResp.Chats)
	}

	// User 1 gets User 1 chat -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+user1Chat.ID.Hex(), nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// User 1 tries to get User 2 chat -> 404 Not Found (D-05)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+user2Chat.ID.Hex(), nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for non-owned chat (D-05), got %d", rec.Code)
	}

	// Invalid hex ID -> 400 Bad Request
	req = httptest.NewRequest(http.MethodGet, "/api/v1/chats/invalid-hex-id", nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid id, got %d", rec.Code)
	}
}

func TestChatCRUD_UpdateChatTitle(t *testing.T) {
	fakeSvc := newFakeChatService()
	r, tm := setupChatCRUDTestRouter(fakeSvc)

	ctx := context.Background()
	chat, _ := fakeSvc.CreateChat(ctx, 101, "Старый заголовок", "gpt-4o")

	token1, _ := tm.GenerateToken(101, 1001, time.Hour)
	token2, _ := tm.GenerateToken(202, 2002, time.Hour)

	// Empty title -> 400
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/chats/"+chat.ID.Hex(), bytes.NewBufferString(`{"title":"   "}`))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on empty title, got %d", rec.Code)
	}

	// Other user -> 404 (D-05)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/chats/"+chat.ID.Hex(), bytes.NewBufferString(`{"title":"Чужой заголовок"}`))
	req.Header.Set("Authorization", "Bearer "+token2)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for non-owned chat update, got %d", rec.Code)
	}

	// Owner update -> 200 OK
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/chats/"+chat.ID.Hex(), bytes.NewBufferString(`{"title":"Новый заголовок"}`))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var resp struct {
		Chat model.Chat `json:"chat"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Chat.Title != "Новый заголовок" {
		t.Errorf("expected updated title 'Новый заголовок', got '%s'", resp.Chat.Title)
	}
}

func TestChatCRUD_DeleteChat_And_ListMessages(t *testing.T) {
	fakeSvc := newFakeChatService()
	r, tm := setupChatCRUDTestRouter(fakeSvc)

	ctx := context.Background()
	chat, _ := fakeSvc.CreateChat(ctx, 101, "Чат на удаление", "gpt-4o")
	fakeSvc.messages[chat.ID.Hex()] = []model.Message{
		{ID: bson.NewObjectID(), ChatID: chat.ID, UserID: 101, Role: "user", Content: "Привет"},
	}

	token1, _ := tm.GenerateToken(101, 1001, time.Hour)
	token2, _ := tm.GenerateToken(202, 2002, time.Hour)

	// Other user list messages -> 404 (D-05)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+chat.ID.Hex()+"/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for listing non-owned messages, got %d", rec.Code)
	}

	// Owner list messages -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+chat.ID.Hex()+"/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// Other user delete chat -> 404 (D-05)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/chats/"+chat.ID.Hex(), nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleting non-owned chat, got %d", rec.Code)
	}

	// Owner delete chat -> 204 No Content
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/chats/"+chat.ID.Hex(), nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", rec.Code)
	}

	// Confirm deleted: GetChat returns 404
	req = httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+chat.ID.Hex(), nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted chat, got %d", rec.Code)
	}
}
