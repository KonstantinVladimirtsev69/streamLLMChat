package llm

import (
	"context"
	"sync"
	"time"

	"backend/internal/model"
)

// ModelCache provides thread-safe in-memory caching of the models catalog with TTL.
type ModelCache struct {
	provider     Provider
	ttl          time.Duration
	cachedModels []model.LLMModel
	lastUpdated  time.Time
	mu           sync.RWMutex
}

// NewModelCache creates a new ModelCache wrapping a Provider.
func NewModelCache(provider Provider, ttl time.Duration) *ModelCache {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &ModelCache{
		provider: provider,
		ttl:      ttl,
	}
}

// GetModels returns cached models if fresh, or fetches and caches new ones from provider.
// If the provider fails, it gracefully returns existing cache or default static models.
func (c *ModelCache) GetModels(ctx context.Context) ([]model.LLMModel, error) {
	c.mu.RLock()
	if len(c.cachedModels) > 0 && time.Since(c.lastUpdated) < c.ttl {
		res := make([]model.LLMModel, len(c.cachedModels))
		copy(res, c.cachedModels)
		c.mu.RUnlock()
		return res, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check under write lock
	if len(c.cachedModels) > 0 && time.Since(c.lastUpdated) < c.ttl {
		res := make([]model.LLMModel, len(c.cachedModels))
		copy(res, c.cachedModels)
		return res, nil
	}

	models, err := c.provider.GetModels(ctx)
	if err != nil {
		// Fallback: if we already have stale cache, return it
		if len(c.cachedModels) > 0 {
			res := make([]model.LLMModel, len(c.cachedModels))
			copy(res, c.cachedModels)
			return res, nil
		}
		// Fallback: static fallback models
		static := GetDefaultStaticModels()
		return static, nil
	}

	c.cachedModels = make([]model.LLMModel, len(models))
	copy(c.cachedModels, models)
	c.lastUpdated = time.Now()

	res := make([]model.LLMModel, len(models))
	copy(res, models)
	return res, nil
}
