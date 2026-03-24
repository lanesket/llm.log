//go:build e2e

package pricing

import (
	"os"
	"testing"

	"github.com/lanesket/llm.log/internal/provider/wire"
)

func TestE2E_PricingWithRealData(t *testing.T) {
	dir, _ := os.MkdirTemp("", "pricing-e2e")
	defer os.RemoveAll(dir)

	db := NewDB(dir)
	if err := db.Update(); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	t.Logf("Loaded %d model prices", len(db.prices))

	tests := []struct {
		name       string
		provider   string
		model      string
		result     *wire.Result
		multiplier float64
		cacheTTL1h bool
		verify     func(t *testing.T, cost float64)
	}{
		{
			name:     "GPT-4.1-nano basic (1K in, 500 out)",
			provider: "openai", model: "gpt-4.1-nano",
			result: &wire.Result{InputTokens: 1000, OutputTokens: 500},
			// $0.10/MTok in + $0.40/MTok out = 0.0001 + 0.0002 = 0.0003
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.0003, "1K×$0.10/M + 500×$0.40/M") },
		},
		{
			name:     "GPT-4.1-nano with cache (2K in, 1.5K cached)",
			provider: "openai", model: "gpt-4.1-nano",
			result: &wire.Result{InputTokens: 2000, OutputTokens: 100, CacheReadTokens: 1500},
			// uncached: 500×$0.10/M=0.00005, cached: 1500×$0.025/M=0.0000375, out: 100×$0.40/M=0.00004 = $0.0001275
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.0001275, "500×$0.10 + 1500×$0.025 + 100×$0.40") },
		},
		{
			name:     "Sonnet 4.6 basic (1K in, 500 out)",
			provider: "anthropic", model: "claude-sonnet-4-6",
			result: &wire.Result{InputTokens: 1000, OutputTokens: 500},
			// $3/MTok in + $15/MTok out = 0.003 + 0.0075 = 0.0105
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.0105, "1K×$3/M + 500×$15/M") },
		},
		{
			name:     "Sonnet 4.6 cache write 5m",
			provider: "anthropic", model: "claude-sonnet-4-6",
			result: &wire.Result{InputTokens: 5000, OutputTokens: 200, CacheWriteTokens: 4000},
			// uncached: 1000×$3/M=0.003, cw: 4000×$3.75/M=0.015, out: 200×$15/M=0.003
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.021, "1K×$3 + 4K×$3.75 + 200×$15") },
		},
		{
			name:     "Sonnet 4.6 cache write 1h (2x input)",
			provider: "anthropic", model: "claude-sonnet-4-6",
			result:     &wire.Result{InputTokens: 5000, OutputTokens: 200, CacheWriteTokens: 4000},
			cacheTTL1h: true,
			// uncached: 1000×$3/M=0.003, cw 1h: 4000×$6/M=0.024, out: 200×$15/M=0.003
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.030, "1K×$3 + 4K×$6 + 200×$15") },
		},
		{
			name:     "Sonnet 4.6 cache read",
			provider: "anthropic", model: "claude-sonnet-4-6",
			result: &wire.Result{InputTokens: 5000, OutputTokens: 200, CacheReadTokens: 4000},
			// uncached: 1000×$3/M=0.003, cr: 4000×$0.30/M=0.0012, out: 200×$15/M=0.003
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.0072, "1K×$3 + 4K×$0.30 + 200×$15") },
		},
		{
			name:     "Opus 4.6 fast mode (6x)",
			provider: "anthropic", model: "claude-opus-4-6",
			result:     &wire.Result{InputTokens: 10000, OutputTokens: 5000},
			multiplier: 6.0,
			// base: 10K×$5/M=0.05 + 5K×$25/M=0.125 = 0.175, ×6 = 1.05
			verify: func(t *testing.T, c float64) { expectCost(t, c, 1.05, "(10K×$5 + 5K×$25)×6") },
		},
		{
			name:     "Sonnet 4.6 + 5 web searches",
			provider: "anthropic", model: "claude-sonnet-4-6",
			result: &wire.Result{InputTokens: 2000, OutputTokens: 1000, WebSearchRequests: 5},
			// tokens: 2K×$3/M + 1K×$15/M = 0.006+0.015=0.021, web: 5×$0.01=0.05
			verify: func(t *testing.T, c float64) { expectCost(t, c, 0.071, "2K×$3 + 1K×$15 + 5×$0.01") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.multiplier == 0 {
				tt.multiplier = 1.0
			}
			cost := db.Cost(tt.provider, tt.model, tt.result, tt.multiplier, tt.cacheTTL1h)
			if cost == nil {
				t.Fatal("model not found in pricing DB")
			}
			t.Logf("cost = $%.6f", *cost)
			if tt.verify != nil {
				tt.verify(t, *cost)
			}
		})
	}
}

func expectCost(t *testing.T, got, want float64, breakdown string) {
	t.Helper()
	tolerance := want * 0.01 // 1% tolerance for genai-prices rounding
	if tolerance < 0.000001 {
		tolerance = 0.000001
	}
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Errorf("cost = $%.6f, want ~$%.6f (%s)\n  diff = $%.6f (%.1f%%)", got, want, breakdown, diff, diff/want*100)
	}
}
