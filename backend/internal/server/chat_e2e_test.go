package server_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/llm"
	"backend/internal/model"
	"backend/internal/repository/mongodb"
	"backend/internal/repository/postgres"
	"backend/internal/server"
	"backend/internal/server/handler"
	"backend/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func getE2EPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@127.0.0.1:5432/llmchat?sslmode=disable" //nolint:gosec
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping e2e test: postgres not reachable: %v", err)
	}
	if err := database.Up(dbURL); err != nil {
		t.Fatalf("failed to run postgres migrations: %v", err)
	}
	return pool
}

func getE2EMongo(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017/llmchat"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := database.NewMongoClient(ctx, uri)
	if err != nil {
		t.Skipf("skipping e2e test: mongodb not reachable: %v", err)
	}
	db := client.Database(fmt.Sprintf("llmchat_e2e_%d", time.Now().UnixNano()))
	if err := database.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("failed to ensure indexes: %v", err)
	}
	return client, db
}

func TestChatLifecycle_E2E(t *testing.T) {
	pgPool := getE2EPostgres(t)
	mongoClient, mongoDB := getE2EMongo(t)
	ctx := context.Background()

	defer func() {
		_ = mongoDB.Drop(ctx)
		_ = mongoClient.Disconnect(ctx)
	}()

	tm, _ := auth.NewJWTTokenManager("super-secret-key-at-least-16-chars")
	refGen := auth.NewRefCodeGenerator(8)
	userRepo := postgres.NewUserRepository(pgPool)
	balanceRepo := postgres.NewBalanceRepository(pgPool)
	referralRepo := postgres.NewReferralRepository(pgPool)
	txManager := database.NewTxManager(pgPool)

	authSvc := service.NewAuthService(userRepo, balanceRepo, referralRepo, txManager, refGen, "http://localhost:3000")
	billingSvc := service.NewBillingService(balanceRepo)

	chatRepo := mongodb.NewChatRepository(mongoDB)
	msgRepo := mongodb.NewMessageRepository(mongoDB)
	chatSvc := service.NewChatService(chatRepo, msgRepo)

	mockLLM := llm.NewMockLLMProvider()
	modelCache := llm.NewModelCache(mockLLM, 15*time.Minute)

	chatH := handler.NewChatHandler(mockLLM, modelCache, billingSvc, chatSvc)

	srv := server.New()
	srv.RegisterChatRoutes(chatH, tm)

	// Create user 1 with welcome bonus
	uniqueVK1 := time.Now().UnixNano()
	user1, err := authSvc.AuthenticateOrRegister(ctx, auth.VKProfile{
		ID:        uniqueVK1,
		FirstName: "Alice",
		LastName:  "Test",
	}, "")
	if err != nil {
		t.Fatalf("failed to register user 1: %v", err)
	}
	token1, _ := tm.GenerateToken(user1.ID, user1.VKID, time.Hour)

	// Create user 2 with welcome bonus
	uniqueVK2 := uniqueVK1 + 1
	user2, err := authSvc.AuthenticateOrRegister(ctx, auth.VKProfile{
		ID:        uniqueVK2,
		FirstName: "Bob",
		LastName:  "Test",
	}, "")
	if err != nil {
		t.Fatalf("failed to register user 2: %v", err)
	}
	token2, _ := tm.GenerateToken(user2.ID, user2.VKID, time.Hour)

	// -------------------------------------------------------------------------
	// 1. Create chat POST /api/v1/chats
	// -------------------------------------------------------------------------
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/chats", bytes.NewBufferString(`{}`))
	createReq.Header.Set("Authorization", "Bearer "+token1)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("step 1 failed: expected 201 Created, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var createResp struct {
		Chat model.Chat `json:"chat"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &createResp)
	chatID := createResp.Chat.ID
	if chatID.IsZero() {
		t.Fatal("step 1 failed: received zero ObjectID")
	}
	if createResp.Chat.Title != service.DefaultChatTitle {
		t.Errorf("step 1: expected title '%s', got '%s'", service.DefaultChatTitle, createResp.Chat.Title)
	}

	// -------------------------------------------------------------------------
	// 2. Stream first turn POST /api/v1/chat/stream with chat_id
	// -------------------------------------------------------------------------
	streamBody1, _ := json.Marshal(model.ChatCompletionRequest{
		ChatID:  chatID.Hex(),
		Content: "Как разработать масштабируемую архитектуру на Go?",
	})
	streamReq1 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(streamBody1))
	streamReq1.Header.Set("Authorization", "Bearer "+token1)
	streamReq1.Header.Set("Content-Type", "application/json")
	streamRec1 := httptest.NewRecorder()
	srv.Router.ServeHTTP(streamRec1, streamReq1)

	if streamRec1.Code != http.StatusOK {
		t.Fatalf("step 2 failed: expected 200 OK for stream, got %d: %s", streamRec1.Code, streamRec1.Body.String())
	}

	scanner := bufio.NewScanner(streamRec1.Body)
	var streamOutput string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			streamOutput += line + "\n"
		}
	}
	if !strings.Contains(streamOutput, `"type":"done"`) {
		t.Errorf("step 2: expected done event in stream output: %s", streamOutput)
	}

	// Verify messages saved in MongoDB (user message + assistant reply)
	msgsAfterTurn1, err := msgRepo.ListByChatID(ctx, chatID, 10, 0)
	if err != nil {
		t.Fatalf("step 2: failed to list messages: %v", err)
	}
	if len(msgsAfterTurn1) != 2 {
		t.Fatalf("step 2: expected 2 messages in MongoDB, got %d", len(msgsAfterTurn1))
	}
	if msgsAfterTurn1[0].Role != "user" || msgsAfterTurn1[1].Role != "assistant" {
		t.Errorf("step 2: unexpected message roles: %+v, %+v", msgsAfterTurn1[0], msgsAfterTurn1[1])
	}

	// Verify auto-title updated on chat document
	updatedChat1, err := chatRepo.GetByID(ctx, chatID, user1.ID)
	if err != nil {
		t.Fatalf("step 2: failed to get updated chat: %v", err)
	}
	if updatedChat1.Title == service.DefaultChatTitle {
		t.Errorf("step 2: expected chat title to be auto-updated, remained '%s'", service.DefaultChatTitle)
	}

	// -------------------------------------------------------------------------
	// 3. Stream second turn: verify context memory is assembled
	// -------------------------------------------------------------------------
	streamBody2, _ := json.Marshal(model.ChatCompletionRequest{
		ChatID:  chatID.Hex(),
		Content: "Приведи пример кода структуры проекта",
	})
	streamReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(streamBody2))
	streamReq2.Header.Set("Authorization", "Bearer "+token1)
	streamReq2.Header.Set("Content-Type", "application/json")
	streamRec2 := httptest.NewRecorder()
	srv.Router.ServeHTTP(streamRec2, streamReq2)

	if streamRec2.Code != http.StatusOK {
		t.Fatalf("step 3 failed: expected 200 OK for 2nd stream, got %d", streamRec2.Code)
	}

	// List messages via REST API GET /api/v1/chats/{id}/messages
	listMsgsReq := httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+chatID.Hex()+"/messages", nil)
	listMsgsReq.Header.Set("Authorization", "Bearer "+token1)
	listMsgsRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(listMsgsRec, listMsgsReq)

	if listMsgsRec.Code != http.StatusOK {
		t.Fatalf("step 3: expected 200 for messages list, got %d", listMsgsRec.Code)
	}
	var listMsgsResp struct {
		Messages []model.Message `json:"messages"`
	}
	_ = json.Unmarshal(listMsgsRec.Body.Bytes(), &listMsgsResp)
	if len(listMsgsResp.Messages) != 4 {
		t.Fatalf("step 3: expected 4 messages in conversation history, got %d", len(listMsgsResp.Messages))
	}

	// -------------------------------------------------------------------------
	// 4. Update title PATCH /api/v1/chats/{id}
	// -------------------------------------------------------------------------
	patchBody := `{"title": "Архитектура Go сервисов"}`
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/chats/"+chatID.Hex(), bytes.NewBufferString(patchBody))
	patchReq.Header.Set("Authorization", "Bearer "+token1)
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(patchRec, patchReq)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("step 4: expected 200 for rename, got %d", patchRec.Code)
	}

	// -------------------------------------------------------------------------
	// 5. Tenant isolation (User 2 attempts to access User 1's chat -> 404)
	// -------------------------------------------------------------------------
	// GET chat
	getOtherReq := httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+chatID.Hex(), nil)
	getOtherReq.Header.Set("Authorization", "Bearer "+token2)
	getOtherRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(getOtherRec, getOtherReq)
	if getOtherRec.Code != http.StatusNotFound {
		t.Errorf("step 5: expected 404 for User 2 GET chat, got %d", getOtherRec.Code)
	}

	// GET messages
	getOtherMsgs := httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+chatID.Hex()+"/messages", nil)
	getOtherMsgs.Header.Set("Authorization", "Bearer "+token2)
	getOtherMsgsRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(getOtherMsgsRec, getOtherMsgs)
	if getOtherMsgsRec.Code != http.StatusNotFound {
		t.Errorf("step 5: expected 404 for User 2 GET messages, got %d", getOtherMsgsRec.Code)
	}

	// POST stream into other user's chat
	postOtherStream := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewBufferString(fmt.Sprintf(`{"chat_id":"%s","content":"attack"}`, chatID.Hex())))
	postOtherStream.Header.Set("Authorization", "Bearer "+token2)
	postOtherStream.Header.Set("Content-Type", "application/json")
	postOtherRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(postOtherRec, postOtherStream)
	if postOtherRec.Code != http.StatusNotFound {
		t.Errorf("step 5: expected 404 for User 2 streaming into User 1 chat, got %d", postOtherRec.Code)
	}

	// -------------------------------------------------------------------------
	// 6. Delete chat DELETE /api/v1/chats/{id} and verify cascade deletion
	// -------------------------------------------------------------------------
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/chats/"+chatID.Hex(), nil)
	delReq.Header.Set("Authorization", "Bearer "+token1)
	delRec := httptest.NewRecorder()
	srv.Router.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("step 6: expected 204 No Content for delete, got %d", delRec.Code)
	}

	// Verify chat no longer exists
	_, err = chatRepo.GetByID(ctx, chatID, user1.ID)
	if err == nil {
		t.Error("step 6: expected chat to be deleted from MongoDB")
	}

	// Verify all messages cascade deleted
	remainingMsgs, err := msgRepo.ListByChatID(ctx, chatID, 10, 0)
	if err != nil {
		t.Fatalf("step 6: failed to check messages after delete: %v", err)
	}
	if len(remainingMsgs) != 0 {
		t.Errorf("step 6: expected 0 messages after cascade deletion, got %d", len(remainingMsgs))
	}
}
