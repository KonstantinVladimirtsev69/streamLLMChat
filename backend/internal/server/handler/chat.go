package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/llm"
	"backend/internal/model"
	"backend/internal/server/middleware"
	"backend/internal/service"

	chi "github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ChatHandler handles LLM catalog, chat management, and streaming chat requests.
type ChatHandler struct {
	provider   llm.Provider
	modelCache *llm.ModelCache
	billingSvc service.BillingService
	chatSvc    service.ChatService
}

// NewChatHandler constructs a new ChatHandler.
func NewChatHandler(provider llm.Provider, modelCache *llm.ModelCache, billingSvc service.BillingService, chatSvc service.ChatService) *ChatHandler {
	return &ChatHandler{
		provider:   provider,
		modelCache: modelCache,
		billingSvc: billingSvc,
		chatSvc:    chatSvc,
	}
}

// CreateChatRequest is the request body for creating a new chat session.
type CreateChatRequest struct {
	Title string `json:"title"`
	Model string `json:"model"`
}

// UpdateTitleRequest is the request body for updating a chat title.
type UpdateTitleRequest struct {
	Title string `json:"title"`
}

// CreateChat creates a new chat session for the authenticated user.
func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var req CreateChatRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
			return
		}
	}

	chat, err := h.chatSvc.CreateChat(r.Context(), userID, req.Title, req.Model)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to create chat"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"chat": chat})
}

// ListChats lists all chats belonging to the authenticated user.
func (h *ChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	limit := int64(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.ParseInt(l, 10, 64); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	offset := int64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.ParseInt(o, 10, 64); err == nil && val >= 0 {
			offset = val
		}
	}

	chats, err := h.chatSvc.ListChats(r.Context(), userID, limit, offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list chats"})
		return
	}

	if chats == nil {
		chats = []model.Chat{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"chats": chats})
}

// GetChat returns the details of a specific chat session.
func (h *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	chatID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid chat id"})
		return
	}

	chat, err := h.chatSvc.GetChat(r.Context(), chatID, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "chat not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to get chat"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"chat": chat})
}

// UpdateChatTitle updates the title of a chat session.
func (h *ChatHandler) UpdateChatTitle(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	chatID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid chat id"})
		return
	}

	var req UpdateTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "title cannot be empty"})
		return
	}

	chat, err := h.chatSvc.UpdateChatTitle(r.Context(), chatID, userID, req.Title)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "chat not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to update chat title"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"chat": chat})
}

// DeleteChat removes a chat session and cascades deletion to all its messages.
func (h *ChatHandler) DeleteChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	chatID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid chat id"})
		return
	}

	if err := h.chatSvc.DeleteChat(r.Context(), chatID, userID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "chat not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete chat"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMessages returns chronological messages for a chat session.
func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	chatID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid chat id"})
		return
	}

	limit := int64(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.ParseInt(l, 10, 64); err == nil && val > 0 && val <= 200 {
			limit = val
		}
	}

	offset := int64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.ParseInt(o, 10, 64); err == nil && val >= 0 {
			offset = val
		}
	}

	messages, err := h.chatSvc.ListMessages(r.Context(), chatID, userID, limit, offset)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "chat not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list messages"})
		return
	}

	if messages == nil {
		messages = []model.Message{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"messages": messages})
}

// GetModels returns the list of available LLM models with pricing.
func (h *ChatHandler) GetModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.modelCache.GetModels(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch models"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"models": models,
	})
}

// StreamChat streams chat completion chunks using Server-Sent Events (SSE) with pre-generation balance check and billing.
func (h *ChatHandler) StreamChat(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "streaming unsupported"})
		return
	}

	var req model.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if req.Model == "" || len(req.Messages) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "model and non-empty messages are required"})
		return
	}

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	// 1. Pre-generation balance check: block with HTTP 402 if balance <= 0 without invoking upstream LLM
	canGen, balance, err := h.billingSvc.CanGenerate(r.Context(), userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to check balance"})
		return
	}
	if !canGen {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":           "insufficient_balance",
			"balance_kopecks": balance,
		})
		return
	}

	req.Stream = true

	// D-03: Disable write deadline on the underlying connection to allow arbitrary-length streaming
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	// Set required SSE headers and flush them immediately
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	targetModel, _ := h.modelCache.GetModelByID(r.Context(), req.Model)

	promptChars := 0
	for _, m := range req.Messages {
		promptChars += len(m.Content)
	}

	events, errs := h.provider.StreamChat(r.Context(), req)

	var lastUsage *model.TokenUsage
	var generatedChars int
	var billed bool

	chargeDeltas := func(ctx context.Context) {
		if billed {
			return
		}
		billed = true
		if lastUsage == nil {
			promptTokens := promptChars / 4
			if promptTokens <= 0 {
				promptTokens = 1
			}
			compTokens := generatedChars / 4
			if compTokens <= 0 {
				compTokens = 1
			}
			var cost int64 = 1
			if targetModel != nil {
				cost = llm.CalculateTokenCost(promptTokens, compTokens, targetModel.PromptPricePer1M, targetModel.CompletionPricePer1M)
			}
			lastUsage = &model.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: compTokens,
				TotalTokens:      promptTokens + compTokens,
				CostKopecks:      cost,
			}
		}
		_, _ = h.billingSvc.ChargeTokens(ctx, userID, *lastUsage, req.Model, nil)
	}

	for {
		select {
		case <-r.Context().Done():
			// D-06: Client disconnected mid-stream; bill generated tokens using detached timeout context
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			chargeDeltas(cleanupCtx)
			return
		case err, ok := <-errs:
			if ok && err != nil {
				errEvent := model.StreamEvent{
					Type:  model.StreamEventError,
					Error: err.Error(),
				}
				data, _ := json.Marshal(errEvent)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
				return
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			switch event.Type {
			case model.StreamEventDelta:
				generatedChars += len(event.Content)
			case model.StreamEventDone:
				if event.Usage != nil {
					lastUsage = event.Usage
					if lastUsage.CostKopecks <= 0 && targetModel != nil {
						lastUsage.CostKopecks = llm.CalculateTokenCost(lastUsage.PromptTokens, lastUsage.CompletionTokens, targetModel.PromptPricePer1M, targetModel.CompletionPricePer1M)
					}
				}
				chargeDeltas(r.Context())
				if lastUsage != nil {
					event.Usage = lastUsage
				}
			}

			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if event.Type == model.StreamEventDone {
				return
			}
		}
	}
}
