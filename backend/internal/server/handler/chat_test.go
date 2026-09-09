package handler_test

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
	"backend/internal/repository/postgres"
	"backend/internal/server/handler"
	"backend/internal/server/middleware"
	"backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func getChatTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@127.0.0.1:5432/llmchat?sslmode=disable" //nolint:gosec
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping postgres integration test (postgres not reachable: %v)", err)
	}

	if err := database.Up(dbURL); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return pool
}

type fakeBillingService struct {
	canGen  bool
	balance int64
	charged *model.TokenUsage
}

func (f *fakeBillingService) CanGenerate(ctx context.Context, userID int64) (bool, int64, error) {
	return f.canGen, f.balance, nil
}

func (f *fakeBillingService) ChargeTokens(ctx context.Context, userID int64, usage model.TokenUsage, modelID string, refID *string) (*model.Balance, error) {
	f.charged = &usage
	f.balance -= usage.CostKopecks
	return &model.Balance{UserID: userID, AmountKopecks: f.balance}, nil
}

func setupChatTestRouterWithBilling(mock *llm.MockLLMProvider, billingSvc service.BillingService) (http.Handler, auth.TokenManager) {
	tm, _ := auth.NewJWTTokenManager("super-secret-key-at-least-16-chars")
	cache := llm.NewModelCache(mock, 15*time.Minute)
	h := handler.NewChatHandler(mock, cache, billingSvc)

	r := chi.NewRouter()
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.AuthMiddleware(tm))
		pr.Get("/api/v1/models", h.GetModels)
		pr.Post("/api/v1/chat/stream", h.StreamChat)
	})

	return r, tm
}

func TestGetModels_Unauthorized(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	billing := &fakeBillingService{canGen: true, balance: 500}
	r, _ := setupChatTestRouterWithBilling(mock, billing)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestGetModels_Success(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	billing := &fakeBillingService{canGen: true, balance: 500}
	r, tm := setupChatTestRouterWithBilling(mock, billing)

	token, err := tm.GenerateToken(12345, 12345, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var resp struct {
		Models []model.LLMModel `json:"models"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Models) == 0 {
		t.Fatalf("expected non-empty models catalog")
	}

	found := false
	for _, m := range resp.Models {
		if m.ID == "deepseek/deepseek-chat" {
			found = true
			if m.PromptPricePer1M <= 0 || m.CompletionPricePer1M <= 0 {
				t.Errorf("expected positive prices, got: %+v", m)
			}
		}
	}
	if !found {
		t.Errorf("expected deepseek/deepseek-chat in models")
	}
}

func TestStreamChat_Unauthorized(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	billing := &fakeBillingService{canGen: true, balance: 500}
	r, _ := setupChatTestRouterWithBilling(mock, billing)

	payload := `{"model":"deepseek/deepseek-chat","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestStreamChat_InvalidPayload(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	billing := &fakeBillingService{canGen: true, balance: 500}
	r, tm := setupChatTestRouterWithBilling(mock, billing)

	token, _ := tm.GenerateToken(12345, 12345, 24*time.Hour)

	tests := []struct {
		name    string
		payload string
	}{
		{"empty body", ""},
		{"malformed json", "{"},
		{"missing messages", `{"model":"deepseek/deepseek-chat"}`},
		{"missing model", `{"messages":[{"role":"user","content":"Hi"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", strings.NewReader(tt.payload))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 Bad Request, got %d", rec.Code)
			}
		})
	}
}

func TestStreamChat_InsufficientBalance_HTTP402(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	// User with 0 balance
	billing := &fakeBillingService{canGen: false, balance: 0}
	r, tm := setupChatTestRouterWithBilling(mock, billing)

	token, _ := tm.GenerateToken(12345, 12345, 24*time.Hour)

	chatReq := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Привет"},
		},
	}
	body, _ := json.Marshal(chatReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 Payment Required, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error          string `json:"error"`
		BalanceKopecks int64  `json:"balance_kopecks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode 402 JSON: %v", err)
	}
	if errResp.Error != "insufficient_balance" {
		t.Errorf("expected error insufficient_balance, got %s", errResp.Error)
	}
	if errResp.BalanceKopecks != 0 {
		t.Errorf("expected balance 0, got %d", errResp.BalanceKopecks)
	}
}

func TestStreamChat_Success_And_Billing(t *testing.T) {
	pool := getChatTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	billingSvc := service.NewBillingService(balanceRepo)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Чат",
		LastName:  "Биллинг",
		RefCode:   fmt.Sprintf("cb_%d", uniqueVKID),
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Seed 500 kopecks
	if _, err := balanceRepo.AddBonus(ctx, user.ID, 500, model.TxWelcomeBonus, nil, "Старт"); err != nil {
		t.Fatalf("failed to seed balance: %v", err)
	}

	mock := llm.NewMockLLMProvider()
	mock.SetChunkDelay(1 * time.Millisecond)
	mock.SetDefaultReply("Ответ от модели с биллингом")
	mock.SetSimulatedUsage(100, 200)

	r, tm := setupChatTestRouterWithBilling(mock, billingSvc)
	token, _ := tm.GenerateToken(user.ID, uniqueVKID, 24*time.Hour)

	chatReq := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Вопрос"},
		},
	}
	body, _ := json.Marshal(chatReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// Read stream
	scanner := bufio.NewScanner(rec.Body)
	var doneReceived bool
	var finalUsage *model.TokenUsage

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var event model.StreamEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err == nil {
				if event.Type == model.StreamEventDone {
					doneReceived = true
					finalUsage = event.Usage
				}
			}
		}
	}

	if !doneReceived || finalUsage == nil {
		t.Fatalf("expected stream done event with usage")
	}
	if finalUsage.CostKopecks <= 0 {
		t.Errorf("expected positive CostKopecks, got %d", finalUsage.CostKopecks)
	}

	// Verify balance was debited in PostgreSQL
	bal, err := balanceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	expectedBalance := 500 - finalUsage.CostKopecks
	if bal.AmountKopecks != expectedBalance {
		t.Errorf("expected balance %d, got %d", expectedBalance, bal.AmountKopecks)
	}

	// Verify ledger entry
	txs, err := balanceRepo.GetTransactions(ctx, user.ID, 5, 0)
	if err != nil {
		t.Fatalf("failed to get transactions: %v", err)
	}
	if len(txs) < 2 {
		t.Fatalf("expected at least 2 transactions, got %d", len(txs))
	}
	latestTx := txs[0]
	if latestTx.Type != model.TxTokenCharge {
		t.Errorf("expected latest tx type %s, got %s", model.TxTokenCharge, latestTx.Type)
	}
	if latestTx.AmountKopecks != -finalUsage.CostKopecks {
		t.Errorf("expected tx amount %d, got %d", -finalUsage.CostKopecks, latestTx.AmountKopecks)
	}
}

func TestStreamChat_Overdraft(t *testing.T) {
	pool := getChatTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	billingSvc := service.NewBillingService(balanceRepo)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Овердрафт",
		LastName:  "Чат",
		RefCode:   fmt.Sprintf("odchat_%d", uniqueVKID),
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Seed only 2 kopecks
	if _, err := balanceRepo.AddBonus(ctx, user.ID, 2, model.TxWelcomeBonus, nil, "Старт 2 коп"); err != nil {
		t.Fatalf("failed to seed balance: %v", err)
	}

	mock := llm.NewMockLLMProvider()
	mock.SetChunkDelay(1 * time.Millisecond)
	mock.SetDefaultReply("Длинный ответ вызывающий овердрафт")
	// 5000 prompt tokens + 10000 completion tokens at standard rates will cost > 2 kopecks
	mock.SetSimulatedUsage(5000, 10000)

	r, tm := setupChatTestRouterWithBilling(mock, billingSvc)
	token, _ := tm.GenerateToken(user.ID, uniqueVKID, 24*time.Hour)

	chatReq := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Большой запрос"},
		},
	}
	body, _ := json.Marshal(chatReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK even with overdraft, got %d", rec.Code)
	}

	// Verify balance is now negative in PostgreSQL
	bal, err := balanceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if bal.AmountKopecks >= 0 {
		t.Errorf("expected negative balance, got %d", bal.AmountKopecks)
	}

	// Verify next request is blocked with 402
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()

	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 Payment Required for subsequent request, got %d", rec2.Code)
	}
}

func TestStreamChat_ClientDisconnect_ChargesUsage(t *testing.T) {
	pool := getChatTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	balanceRepo := postgres.NewBalanceRepository(pool)
	billingSvc := service.NewBillingService(balanceRepo)

	uniqueVKID := time.Now().UnixNano()
	user := &model.User{
		VKID:      uniqueVKID,
		FirstName: "Дисконнект",
		LastName:  "Пользователь",
		RefCode:   fmt.Sprintf("dc_%d", uniqueVKID),
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if _, err := balanceRepo.AddBonus(ctx, user.ID, 200, model.TxWelcomeBonus, nil, "Старт"); err != nil {
		t.Fatalf("failed to seed balance: %v", err)
	}

	mock := llm.NewMockLLMProvider()
	// Slow stream so client disconnects during generation
	mock.SetChunkDelay(100 * time.Millisecond)
	mock.SetDefaultReply("Очень длинный ответ который будет прерван пользователем посреди стрима")

	r, tm := setupChatTestRouterWithBilling(mock, billingSvc)
	token, _ := tm.GenerateToken(user.ID, uniqueVKID, 24*time.Hour)

	chatReq := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Вопрос с прерыванием"},
		},
	}
	body, _ := json.Marshal(chatReq)

	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", bytes.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Cancel context quickly to simulate client closing browser tab
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	r.ServeHTTP(rec, req)

	// Wait briefly for background billing detached context to finish
	time.Sleep(100 * time.Millisecond)

	// Check if balance was debited despite disconnect
	txs, err := balanceRepo.GetTransactions(ctx, user.ID, 5, 0)
	if err != nil {
		t.Fatalf("failed to get transactions: %v", err)
	}

	foundCharge := false
	for _, tx := range txs {
		if tx.Type == model.TxTokenCharge {
			foundCharge = true
			if tx.AmountKopecks >= 0 {
				t.Errorf("expected negative amount for charge tx, got %d", tx.AmountKopecks)
			}
		}
	}
	if !foundCharge {
		t.Errorf("expected token_charge transaction after client disconnect (D-06)")
	}
}
