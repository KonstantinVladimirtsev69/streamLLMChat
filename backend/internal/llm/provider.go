package llm

import (
	"context"

	"backend/internal/model"
)

// Provider defines the interface for interacting with language model providers.
type Provider interface {
	// GetModels retrieves the list of available models and their current pricing.
	GetModels(ctx context.Context) ([]model.LLMModel, error)

	// StreamChat initiates a streaming completion returning channels for stream events and errors.
	StreamChat(ctx context.Context, req model.ChatCompletionRequest) (<-chan model.StreamEvent, <-chan error)
}
