package llm

import "math"

// CalculateTokenCost calculates the total cost of token usage in kopecks (1 RUB = 100 kopecks).
// Pricing is given in RUB per 1,000,000 tokens for prompt and completion.
// The cost in kopecks is strictly rounded UP to the nearest integer kopeck (math.Ceil)
// to ensure financial solvency and prevent fractional loss.
func CalculateTokenCost(promptTokens, completionTokens int, promptPricePer1M, completionPricePer1M float64) int64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}

	var promptCostRubles float64
	if promptTokens > 0 && promptPricePer1M > 0 {
		promptCostRubles = (float64(promptTokens) * promptPricePer1M) / 1_000_000.0
	}

	var completionCostRubles float64
	if completionTokens > 0 && completionPricePer1M > 0 {
		completionCostRubles = (float64(completionTokens) * completionPricePer1M) / 1_000_000.0
	}

	totalCostRubles := promptCostRubles + completionCostRubles
	if totalCostRubles <= 0 {
		return 0
	}

	// 1 Ruble = 100 Kopecks
	kopecks := math.Ceil(totalCostRubles * 100.0)
	return int64(kopecks)
}
