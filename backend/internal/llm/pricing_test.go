package llm

import (
	"testing"
)

func TestCalculateTokenCost(t *testing.T) {
	tests := []struct {
		name                 string
		promptTokens         int
		completionTokens     int
		promptPricePer1M     float64
		completionPricePer1M float64
		expectedKopecks      int64
	}{
		{
			name:                 "zero tokens",
			promptTokens:         0,
			completionTokens:     0,
			promptPricePer1M:     100.0,
			completionPricePer1M: 200.0,
			expectedKopecks:      0,
		},
		{
			name:                 "negative tokens handled safely",
			promptTokens:         -10,
			completionTokens:     -5,
			promptPricePer1M:     100.0,
			completionPricePer1M: 200.0,
			expectedKopecks:      0,
		},
		{
			name:                 "single token ceiling rounding up to 1 kopeck",
			promptTokens:         1,
			completionTokens:     0,
			promptPricePer1M:     50.0, // 1 token * 50 / 1M = 0.00005 rub = 0.005 kopecks -> Ceil -> 1 kop
			completionPricePer1M: 100.0,
			expectedKopecks:      1,
		},
		{
			name:                 "exact kopecks calculation",
			promptTokens:         1000,
			completionTokens:     500,
			promptPricePer1M:     100.0, // 1000 * 100 / 1M = 0.10 rub = 10 kopecks
			completionPricePer1M: 200.0, // 500 * 200 / 1M = 0.10 rub = 10 kopecks
			expectedKopecks:      20,    // 20 kopecks exactly
		},
		{
			name:                 "fractional kopeck rounds up strictly",
			promptTokens:         1000,
			completionTokens:     501,
			promptPricePer1M:     100.0, // 10 kopecks
			completionPricePer1M: 200.0, // 501 * 200 / 1M = 0.1002 rub = 10.02 kopecks
			expectedKopecks:      21,    // 10 + 10.02 = 20.02 -> 21 kopecks
		},
		{
			name:                 "free model with zero price",
			promptTokens:         100000,
			completionTokens:     50000,
			promptPricePer1M:     0.0,
			completionPricePer1M: 0.0,
			expectedKopecks:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateTokenCost(tc.promptTokens, tc.completionTokens, tc.promptPricePer1M, tc.completionPricePer1M)
			if got != tc.expectedKopecks {
				t.Fatalf("expected %d kopecks, got %d", tc.expectedKopecks, got)
			}
		})
	}
}
