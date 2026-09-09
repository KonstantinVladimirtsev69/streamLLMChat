package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/llm"
	"backend/internal/model"
)

func TestRouterAIClient_GetModels_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := map[string]any{
			"data": []map[string]any{
				{
					"id":          "custom-model",
					"name":        "Custom Model Pro",
					"description": "Custom model with explicit pricing",
					"pricing": map[string]float64{
						"prompt":     55.0,
						"completion": 110.0,
					},
				},
				{
					"id":          "openai/gpt-4o-mini",
					"name":        "",
					"description": "Mini model fallback pricing",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := llm.NewRouterAIClient("test-api-key", ts.URL, ts.Client())
	ctx := context.Background()

	models, err := client.GetModels(ctx)
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	if models[0].ID != "custom-model" || models[0].PromptPricePer1M != 55.0 || models[0].CompletionPricePer1M != 110.0 {
		t.Errorf("unexpected model 0: %+v", models[0])
	}

	// Second model should get fallback pricing for mini
	if models[1].ID != "openai/gpt-4o-mini" || models[1].Name != "openai/gpt-4o-mini" {
		t.Errorf("unexpected model 1 ID or Name fallback: %+v", models[1])
	}
	if models[1].PromptPricePer1M <= 0 || models[1].CompletionPricePer1M <= 0 {
		t.Errorf("expected positive fallback pricing, got prompt=%f, completion=%f",
			models[1].PromptPricePer1M, models[1].CompletionPricePer1M)
	}
}

func TestRouterAIClient_GetModels_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := llm.NewRouterAIClient("test-key", ts.URL, ts.Client())
	_, err := client.GetModels(context.Background())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

func TestRouterAIClient_StreamChat_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		chunks := []string{
			`: ping comment` + "\n\n",
			`data: {"choices":[{"delta":{"content":"Hello"}}], "usage":null}` + "\n\n",
			`data: {"choices":[{"delta":{"content":" World"}}], "usage":null}` + "\n\n",
			`data: {"choices":[], "usage":{"prompt_tokens":15,"completion_tokens":5,"total_tokens":20}}` + "\n\n",
			`data: [DONE]` + "\n\n",
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer ts.Close()

	client := llm.NewRouterAIClient("test-api-key", ts.URL, ts.Client())
	ctx := context.Background()

	req := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Say Hello World"},
		},
	}

	eventChan, errChan := client.StreamChat(ctx, req)

	var textReceived string
	var doneEventReceived bool
	var usage *model.TokenUsage

	for event := range eventChan {
		switch event.Type {
		case model.StreamEventDelta:
			textReceived += event.Content
		case model.StreamEventDone:
			doneEventReceived = true
			usage = event.Usage
		}
	}

	for err := range errChan {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	if textReceived != "Hello World" {
		t.Errorf("expected text 'Hello World', got %q", textReceived)
	}
	if !doneEventReceived {
		t.Fatal("expected StreamEventDone event")
	}
	if usage == nil {
		t.Fatal("expected usage in done event")
	}
	if usage.PromptTokens != 15 || usage.CompletionTokens != 5 || usage.TotalTokens != 20 {
		t.Errorf("unexpected usage: %+v", usage)
	}
	if usage.CostKopecks <= 0 {
		t.Errorf("expected cost in kopecks > 0, got %d", usage.CostKopecks)
	}
}

func TestRouterAIClient_StreamChat_UpstreamError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	}))
	defer ts.Close()

	client := llm.NewRouterAIClient("test-api-key", ts.URL, ts.Client())
	ctx := context.Background()

	req := model.ChatCompletionRequest{
		Model: "openai/gpt-4o-mini",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
	}

	eventChan, errChan := client.StreamChat(ctx, req)
	// Drain events
	for range eventChan {
	}

	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		t.Fatal("expected error from upstream HTTP 429, got none")
	}
}

func TestRouterAIClient_StreamChat_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"start\"}}]}\n\n")
			f.Flush()
			// Hang to simulate long generation
			time.Sleep(1 * time.Second)
		}
	}))
	defer ts.Close()

	client := llm.NewRouterAIClient("test-key", ts.URL, ts.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "hang"},
		},
	}

	eventChan, errChan := client.StreamChat(ctx, req)
	for range eventChan {
	}

	var hasCancelErr bool
	for err := range errChan {
		if err != nil {
			hasCancelErr = true
		}
	}

	if !hasCancelErr {
		t.Error("expected error due to context cancellation")
	}
}
