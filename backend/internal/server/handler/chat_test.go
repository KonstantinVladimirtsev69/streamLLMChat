package handler_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/llm"
	"backend/internal/model"
	"backend/internal/server/handler"
	"backend/internal/server/middleware"

	"github.com/go-chi/chi/v5"
)

func setupChatTestRouter(mock *llm.MockLLMProvider) (http.Handler, auth.TokenManager) {
	tm, _ := auth.NewJWTTokenManager("super-secret-key-at-least-16-chars")
	cache := llm.NewModelCache(mock, 15*time.Minute)
	h := handler.NewChatHandler(mock, cache)

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
	r, _ := setupChatTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestGetModels_Success(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	r, tm := setupChatTestRouter(mock)

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
	r, _ := setupChatTestRouter(mock)

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
	r, tm := setupChatTestRouter(mock)

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

func TestStreamChat_Success(t *testing.T) {
	mock := llm.NewMockLLMProvider()
	mock.SetChunkDelay(1 * time.Millisecond)
	mock.SetDefaultReply("Тестовый ответ от ИИ")
	mock.SetSimulatedUsage(15, 30)

	r, tm := setupChatTestRouter(mock)

	token, _ := tm.GenerateToken(12345, 12345, 24*time.Hour)

	chatReq := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Привет!"},
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

	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}

	// Parse SSE stream
	scanner := bufio.NewScanner(rec.Body)
	var fullText string
	var doneReceived bool
	var usageReceived *model.TokenUsage

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			var event model.StreamEvent
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				t.Fatalf("failed to parse SSE event JSON: %v", err)
			}

			switch event.Type {
			case model.StreamEventDelta:
				fullText += event.Content
			case model.StreamEventDone:
				doneReceived = true
				usageReceived = event.Usage
			case model.StreamEventError:
				t.Fatalf("unexpected stream error event: %s", event.Error)
			}
		}
	}

	if !doneReceived {
		t.Errorf("expected StreamEventDone event")
	}
	if usageReceived == nil {
		t.Fatalf("expected usage to be non-nil")
	}
	if usageReceived.PromptTokens != 15 || usageReceived.CompletionTokens != 30 {
		t.Errorf("unexpected token usage: %+v", usageReceived)
	}
	if usageReceived.CostKopecks <= 0 {
		t.Errorf("expected positive CostKopecks, got %d", usageReceived.CostKopecks)
	}
	if fullText != "Тестовый ответ от ИИ" {
		t.Errorf("unexpected streamed text: %q", fullText)
	}
}
