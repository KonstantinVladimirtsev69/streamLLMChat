package model

// LLMModel represents a language model available in the system with its pricing per 1M tokens in rubles.
type LLMModel struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Description          string  `json:"description,omitempty"`
	PromptPricePer1M     float64 `json:"prompt_price_per_1m"`     // Price for 1M prompt tokens in RUB
	CompletionPricePer1M float64 `json:"completion_price_per_1m"` // Price for 1M completion tokens in RUB
	ContextLength        int     `json:"context_length,omitempty"`
}

// ChatMessage represents a single message in an LLM conversation.
type ChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// ChatCompletionRequest is the payload sent to initiate a completion.
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// StreamEventType defines the type of event in an SSE stream.
type StreamEventType string

const (
	// StreamEventDelta indicates an incremental text chunk from the assistant.
	StreamEventDelta StreamEventType = "delta"
	// StreamEventDone indicates completion of generation and contains token/billing usage.
	StreamEventDone StreamEventType = "done"
	// StreamEventError indicates a streaming error.
	StreamEventError StreamEventType = "error"
)

// TokenUsage reports token metrics and the final cost in kopecks.
type TokenUsage struct {
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	TotalTokens      int   `json:"total_tokens"`
	CostKopecks      int64 `json:"cost_kopecks"`
}

// StreamEvent represents a single normalized SSE payload sent to the client.
type StreamEvent struct {
	Type    StreamEventType `json:"type"`
	Content string          `json:"content,omitempty"`
	Usage   *TokenUsage     `json:"usage,omitempty"`
	Error   string          `json:"error,omitempty"`
}
