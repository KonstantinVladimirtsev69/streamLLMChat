package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend/internal/model"
)

var _ Provider = (*RouterAIClient)(nil)

// RouterAIClient implements Provider for the routerai.ru OpenAI-compatible API.
type RouterAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewRouterAIClient instantiates a new RouterAIClient.
func NewRouterAIClient(apiKey, baseURL string, httpClient *http.Client) *RouterAIClient {
	if baseURL == "" {
		baseURL = "https://routerai.ru/api/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 60 * time.Second,
		}
	}
	return &RouterAIClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// openAIModelsResponse matches the /models response payload.
type openAIModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
		Pricing     *struct {
			Prompt     float64 `json:"prompt"`
			Completion float64 `json:"completion"`
		} `json:"pricing,omitempty"`
	} `json:"data"`
}

// GetDefaultStaticModels returns a curated list of popular models and their standard tariffs in RUB per 1M tokens.
func GetDefaultStaticModels() []model.LLMModel {
	return []model.LLMModel{
		{
			ID:                   "deepseek/deepseek-chat",
			Name:                 "DeepSeek V3",
			Description:          "Быстрая и эффективная языковая модель общего назначения",
			PromptPricePer1M:     25.0,
			CompletionPricePer1M: 50.0,
			ContextLength:        64000,
		},
		{
			ID:                   "openai/gpt-4o-mini",
			Name:                 "GPT-4o Mini",
			Description:          "Интеллектуальная легковесная модель от OpenAI",
			PromptPricePer1M:     18.0,
			CompletionPricePer1M: 70.0,
			ContextLength:        128000,
		},
		{
			ID:                   "openai/gpt-4o",
			Name:                 "GPT-4o",
			Description:          "Флагманская мультимодальная модель от OpenAI",
			PromptPricePer1M:     280.0,
			CompletionPricePer1M: 1100.0,
			ContextLength:        128000,
		},
		{
			ID:                   "anthropic/claude-3-5-sonnet",
			Name:                 "Claude 3.5 Sonnet",
			Description:          "Выдающаяся модель от Anthropic для анализа и кода",
			PromptPricePer1M:     320.0,
			CompletionPricePer1M: 1600.0,
			ContextLength:        200000,
		},
		{
			ID:                   "meta-llama/llama-3.1-70b-instruct",
			Name:                 "Llama 3.1 70B",
			Description:          "Открытая производительная модель от Meta",
			PromptPricePer1M:     45.0,
			CompletionPricePer1M: 90.0,
			ContextLength:        128000,
		},
	}
}

// AssignFallbackPricing fills in default pricing if the API doesn't specify prices.
func AssignFallbackPricing(id string) (promptPrice, completionPrice float64) {
	for _, m := range GetDefaultStaticModels() {
		if strings.EqualFold(m.ID, id) {
			return m.PromptPricePer1M, m.CompletionPricePer1M
		}
	}
	idLower := strings.ToLower(id)
	if strings.Contains(idLower, "deepseek") {
		return 25.0, 50.0
	}
	if strings.Contains(idLower, "mini") {
		return 20.0, 70.0
	}
	if strings.Contains(idLower, "claude") {
		return 300.0, 1500.0
	}
	if strings.Contains(idLower, "gpt-4") {
		return 250.0, 1000.0
	}
	return 50.0, 100.0
}

// GetModels queries routerai.ru for available models.
func (c *RouterAIClient) GetModels(ctx context.Context) ([]model.LLMModel, error) {
	reqURL := fmt.Sprintf("%s/models", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build models request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute models request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("models request returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	if len(parsed.Data) == 0 {
		return GetDefaultStaticModels(), nil
	}

	modelsList := make([]model.LLMModel, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		name := item.Name
		if name == "" {
			name = item.ID
		}
		var pPrice, cPrice float64
		if item.Pricing != nil && item.Pricing.Prompt > 0 {
			pPrice = item.Pricing.Prompt
			cPrice = item.Pricing.Completion
		} else {
			pPrice, cPrice = AssignFallbackPricing(item.ID)
		}

		modelsList = append(modelsList, model.LLMModel{
			ID:                   item.ID,
			Name:                 name,
			Description:          item.Description,
			PromptPricePer1M:     pPrice,
			CompletionPricePer1M: cPrice,
		})
	}

	return modelsList, nil
}

// openAIStreamChunk models the OpenAI streaming response packet.
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// StreamChat issues a streaming completion request to routerai.ru.
func (c *RouterAIClient) StreamChat(ctx context.Context, req model.ChatCompletionRequest) (<-chan model.StreamEvent, <-chan error) {
	eventChan := make(chan model.StreamEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		reqBody := map[string]any{
			"model":    req.Model,
			"messages": req.Messages,
			"stream":   true,
			"stream_options": map[string]any{
				"include_usage": true,
			},
		}

		data, err := json.Marshal(reqBody)
		if err != nil {
			errChan <- fmt.Errorf("failed to serialize chat request: %w", err)
			return
		}

		reqURL := fmt.Sprintf("%s/chat/completions", c.baseURL)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(data))
		if err != nil {
			errChan <- fmt.Errorf("failed to create chat request: %w", err)
			return
		}

		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errChan <- fmt.Errorf("upstream chat request failed: %w", err)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			errChan <- fmt.Errorf("upstream returned HTTP %d: %s", resp.StatusCode, string(errBody))
			return
		}

		reader := bufio.NewReader(resp.Body)
		var (
			promptTokens     int
			completionTokens int
			totalTokens      int
			receivedChunks   int
		)

		pPrice, cPrice := AssignFallbackPricing(req.Model)

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				errChan <- fmt.Errorf("error reading stream: %w", err)
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				break
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}

			if chunk.Usage != nil {
				promptTokens = chunk.Usage.PromptTokens
				completionTokens = chunk.Usage.CompletionTokens
				totalTokens = chunk.Usage.TotalTokens
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta.Content
				if delta != "" {
					receivedChunks++
					eventChan <- model.StreamEvent{
						Type:    model.StreamEventDelta,
						Content: delta,
					}
				}
			}
		}

		if totalTokens == 0 {
			for _, m := range req.Messages {
				promptTokens += len(strings.Fields(m.Content)) * 2
			}
			if promptTokens == 0 {
				promptTokens = 10
			}
			completionTokens = receivedChunks * 2
			if completionTokens == 0 {
				completionTokens = 1
			}
			totalTokens = promptTokens + completionTokens
		}

		cost := CalculateTokenCost(promptTokens, completionTokens, pPrice, cPrice)

		eventChan <- model.StreamEvent{
			Type: model.StreamEventDone,
			Usage: &model.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
				CostKopecks:      cost,
			},
		}
	}()

	return eventChan, errChan
}
