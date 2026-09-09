package llm

import (
	"context"
	"testing"
	"time"

	"backend/internal/model"
)

func TestLLMProvider_Mock(t *testing.T) {
	mock := NewMockLLMProvider()
	ctx := context.Background()

	// 1. Test GetModels
	models, err := mock.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error getting models: %v", err)
	}
	if len(models) == 0 {
		t.Fatalf("expected non-empty models list")
	}

	// 2. Test StreamChat
	mock.SetChunkDelay(1 * time.Millisecond)
	mock.SetDefaultReply("Раз два три")
	mock.SetSimulatedUsage(10, 20)

	req := model.ChatCompletionRequest{
		Model: "deepseek/deepseek-chat",
		Messages: []model.ChatMessage{
			{Role: "user", Content: "Тест"},
		},
		Stream: true,
	}

	events, errs := mock.StreamChat(ctx, req)

	var receivedText string
	var doneReceived bool
	var usageReceived *model.TokenUsage

	for event := range events {
		switch event.Type {
		case model.StreamEventDelta:
			receivedText += event.Content
		case model.StreamEventDone:
			doneReceived = true
			usageReceived = event.Usage
		}
	}

	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	if !doneReceived {
		t.Errorf("expected done event")
	}
	if usageReceived == nil {
		t.Fatalf("expected non-nil usage")
	}
	if usageReceived.PromptTokens != 10 || usageReceived.CompletionTokens != 20 {
		t.Errorf("unexpected tokens: %+v", usageReceived)
	}
	if usageReceived.CostKopecks <= 0 {
		t.Errorf("expected positive cost in kopecks, got %d", usageReceived.CostKopecks)
	}
	if receivedText != "Раз два три" {
		t.Errorf("unexpected full text: %q", receivedText)
	}
}
