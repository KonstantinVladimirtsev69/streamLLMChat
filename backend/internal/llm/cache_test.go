package llm

import (
	"context"
	"testing"
	"time"

	"backend/internal/model"
)

type countingMockProvider struct {
	callCount int
	fail      bool
	models    []model.LLMModel
}

func (p *countingMockProvider) GetModels(ctx context.Context) ([]model.LLMModel, error) {
	p.callCount++
	if p.fail {
		return nil, context.DeadlineExceeded
	}
	return p.models, nil
}

func (p *countingMockProvider) StreamChat(ctx context.Context, req model.ChatCompletionRequest) (<-chan model.StreamEvent, <-chan error) {
	return nil, nil
}

func TestModelCache_TTLAndFallback(t *testing.T) {
	ctx := context.Background()

	provider := &countingMockProvider{
		models: []model.LLMModel{
			{ID: "test-model-1", Name: "Test Model 1"},
		},
	}

	cache := NewModelCache(provider, 50*time.Millisecond)

	// 1. Initial fetch
	models1, err := cache.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models1) != 1 || models1[0].ID != "test-model-1" {
		t.Fatalf("unexpected models: %+v", models1)
	}
	if provider.callCount != 1 {
		t.Errorf("expected callCount 1, got %d", provider.callCount)
	}

	// 2. Fetch within TTL - should use cache without incrementing provider callCount
	models2, err := cache.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models2) != 1 || models2[0].ID != "test-model-1" {
		t.Fatalf("unexpected models: %+v", models2)
	}
	if provider.callCount != 1 {
		t.Errorf("expected callCount still 1, got %d", provider.callCount)
	}

	// 3. Wait for TTL expiry
	time.Sleep(60 * time.Millisecond)

	// Provider fails now - should return stale cache gracefully
	provider.fail = true
	models3, err := cache.GetModels(ctx)
	if err != nil {
		t.Fatalf("expected fallback to stale cache, got error: %v", err)
	}
	if len(models3) != 1 || models3[0].ID != "test-model-1" {
		t.Fatalf("expected stale cache models, got %+v", models3)
	}

	// 4. Test empty cache fallback to static models
	emptyProvider := &countingMockProvider{fail: true}
	freshCache := NewModelCache(emptyProvider, time.Minute)
	staticModels, err := freshCache.GetModels(ctx)
	if err != nil {
		t.Fatalf("expected static fallback, got error: %v", err)
	}
	if len(staticModels) == 0 {
		t.Fatalf("expected non-empty static fallback models")
	}
}
