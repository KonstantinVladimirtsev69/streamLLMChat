package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/internal/llm"
	"backend/internal/model"
)

// ChatHandler handles LLM catalog and streaming chat requests.
type ChatHandler struct {
	provider   llm.Provider
	modelCache *llm.ModelCache
}

// NewChatHandler constructs a new ChatHandler.
func NewChatHandler(provider llm.Provider, modelCache *llm.ModelCache) *ChatHandler {
	return &ChatHandler{
		provider:   provider,
		modelCache: modelCache,
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

// StreamChat streams chat completion chunks using Server-Sent Events (SSE).
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

	events, errs := h.provider.StreamChat(r.Context(), req)

	for {
		select {
		case <-r.Context().Done():
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
