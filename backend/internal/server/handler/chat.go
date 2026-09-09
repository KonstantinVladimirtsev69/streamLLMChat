package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/internal/llm"
	"backend/internal/model"
	"backend/internal/server/middleware"
	"backend/internal/service"
)

// ChatHandler handles LLM catalog and streaming chat requests.
type ChatHandler struct {
	provider   llm.Provider
	modelCache *llm.ModelCache
	billingSvc service.BillingService
}

// NewChatHandler constructs a new ChatHandler.
func NewChatHandler(provider llm.Provider, modelCache *llm.ModelCache, billingSvc service.BillingService) *ChatHandler {
	return &ChatHandler{
		provider:   provider,
		modelCache: modelCache,
		billingSvc: billingSvc,
	}
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
