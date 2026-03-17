package pricing

import (
	"math"
	"testing"
)

func TestLookupKey_ExactMatch(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60},
	}}
	if db.lookupKey("gpt-4") != "gpt-4" {
		t.Fatal("expected exact match")
	}
}

func TestLookupKey_PrefixMatch(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-sonnet-4-6": {InputPerMTok: 3, OutputPerMTok: 15},
	}}
	if db.lookupKey("claude-sonnet-4-6-20250514") != "claude-sonnet-4-6" {
		t.Fatal("expected prefix match")
	}
}

func TestLookupKey_StripProviderPrefix(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-3-5-haiku": {InputPerMTok: 0.8, OutputPerMTok: 4},
	}}
	if db.lookupKey("anthropic/claude-3-5-haiku") != "claude-3-5-haiku" {
		t.Fatal("expected match after stripping provider prefix")
	}
}

func TestLookupKey_StripPrefixThenPrefixMatch(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4.1-nano": {InputPerMTok: 0.1, OutputPerMTok: 0.4},
	}}
	if db.lookupKey("openai/gpt-4.1-nano-2025-04-14") != "gpt-4.1-nano" {
		t.Fatal("expected match after stripping prefix + prefix match")
	}
}

func TestLookupKey_LongestPrefixWins(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4":        {InputPerMTok: 30},
		"gpt-4.1":      {InputPerMTok: 2},
		"gpt-4.1-nano": {InputPerMTok: 0.1},
	}}
	key := db.lookupKey("gpt-4.1-nano-2025-04-14")
	if key != "gpt-4.1-nano" {
		t.Errorf("got %q, want gpt-4.1-nano (longest match)", key)
	}
}

func TestLookupKey_FuzzySeparators(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4.1": {InputPerMTok: 2},
	}}
	if db.lookupKey("gpt-4-1") != "gpt-4.1" {
		t.Fatal("expected fuzzy separator match")
	}
}

func TestLookupKey_TokenSetMatch(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-opus-4-6":   {InputPerMTok: 15},
		"claude-opus-4-5":   {InputPerMTok: 15},
		"claude-sonnet-4-6": {InputPerMTok: 3},
	}}
	// OpenRouter returns reordered names like "anthropic/claude-4.6-opus-20260205"
	key := db.lookupKey("anthropic/claude-4.6-opus-20260205")
	if key != "claude-opus-4-6" {
		t.Errorf("got %q, want claude-opus-4-6", key)
	}
}

func TestLookupKey_PrefixRequiresSeparator(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60},
	}}
	// "gpt-4o" should NOT match "gpt-4" — the 'o' is not a separator
	if db.lookupKey("gpt-4o") == "gpt-4" {
		t.Fatal("gpt-4o should not match gpt-4 (no separator boundary)")
	}
	// "gpt-4-turbo" SHOULD match "gpt-4" — separated by '-'
	if db.lookupKey("gpt-4-turbo") != "gpt-4" {
		t.Fatal("gpt-4-turbo should match gpt-4 (separator boundary)")
	}
}

func TestLookupKey_NoMatch(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30},
	}}
	if db.lookupKey("totally-unknown-model") != "" {
		t.Error("expected empty for unknown model")
	}
}

func TestNormalize(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-sonnet-4-6": {InputPerMTok: 3},
		"claude-opus-4-6":   {InputPerMTok: 15},
		"gpt-4.1":           {InputPerMTok: 2},
	}}
	tests := []struct {
		gateway, input, want string
	}{
		// Direct calls: gateway = vendor
		{"anthropic", "claude-sonnet-4-6-20250514", "anthropic/claude-sonnet-4-6"},
		{"anthropic", "claude-sonnet-4-6", "anthropic/claude-sonnet-4-6"},
		{"openai", "gpt-4-1-2025-04-14", "openai/gpt-4.1"},
		// OpenRouter: vendor extracted from model prefix
		{"openrouter", "anthropic/claude-sonnet-4-6", "anthropic/claude-sonnet-4-6"},
		{"openrouter", "anthropic/claude-4.6-opus-20260205", "anthropic/claude-opus-4-6"},
		// Unknown model: keep vendor prefix
		{"anthropic", "unknown-model", "anthropic/unknown-model"},
		{"openrouter", "openai/unknown", "openai/unknown"},
	}
	for _, tt := range tests {
		got := db.Normalize(tt.gateway, tt.input)
		if got != tt.want {
			t.Errorf("Normalize(%q, %q) = %q, want %q", tt.gateway, tt.input, got, tt.want)
		}
	}
}

func TestCost(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60, CacheReadPerMTok: 15},
	}}
	// OpenAI: inputTokens=1000 includes cached, cacheRead=200, cacheWrite=0
	cost := db.Cost("openai", "gpt-4", 1000, 500, 200, 0)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// uncached: (1000-200-0) * 30 / 1M = 0.024
	// output: 500 * 60 / 1M = 0.03
	// cache read: 200 * 15 / 1M = 0.003
	expected := 0.057
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_WithCacheWrite(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-sonnet-4-6": {InputPerMTok: 3, OutputPerMTok: 15, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75},
	}}
	// Anthropic: input_tokens=80 (uncached), cache_read=1000, cache_write=200
	// total inputTokens = 80 + 1000 + 200 = 1280
	cost := db.Cost("anthropic", "claude-sonnet-4-6", 1280, 500, 1000, 200)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// uncached: (1280-1000-200) * 3 / 1M = 80 * 3 / 1M = 0.00024
	// output: 500 * 15 / 1M = 0.0075
	// cache read: 1000 * 0.3 / 1M = 0.0003
	// cache write: 200 * 3.75 / 1M = 0.00075
	expected := 0.00024 + 0.0075 + 0.0003 + 0.00075
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_NoCachedTokens(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60},
	}}
	cost := db.Cost("openai", "gpt-4", 1000, 500, 0, 0)
	if cost == nil {
		t.Fatal("expected cost")
	}
	expected := 1000.0*30/1e6 + 500.0*60/1e6
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_UnknownModel(t *testing.T) {
	db := &DB{prices: map[string]*Price{}}
	cost := db.Cost("openai", "unknown", 100, 50, 0, 0)
	if cost != nil {
		t.Error("expected nil for unknown model")
	}
}

func TestParse_PriceData(t *testing.T) {
	data := []byte(`[{"provider":"openai","models":[{"id":"gpt-4","prices":{"input_mtok":30,"output_mtok":60,"cache_read_mtok":15}}]}]`)
	db := &DB{prices: make(map[string]*Price)}
	if err := db.parse(data); err != nil {
		t.Fatal(err)
	}
	p := db.prices["gpt-4"]
	if p == nil {
		t.Fatal("gpt-4 not found")
	}
	if p.InputPerMTok != 30 || p.OutputPerMTok != 60 || p.CacheReadPerMTok != 15 {
		t.Errorf("prices = %+v", p)
	}
}

func TestParse_PriceData_WithCacheWrite(t *testing.T) {
	data := []byte(`[{"provider":"anthropic","models":[{"id":"claude-sonnet-4-6","prices":{"input_mtok":3,"output_mtok":15,"cache_read_mtok":0.3,"cache_write_mtok":3.75}}]}]`)
	db := &DB{prices: make(map[string]*Price)}
	if err := db.parse(data); err != nil {
		t.Fatal(err)
	}
	p := db.prices["claude-sonnet-4-6"]
	if p == nil {
		t.Fatal("claude-sonnet-4-6 not found")
	}
	if p.CacheWritePerMTok != 3.75 {
		t.Errorf("cache write = %f, want 3.75", p.CacheWritePerMTok)
	}
	if p.CacheReadPerMTok != 0.3 {
		t.Errorf("cache read = %f, want 0.3", p.CacheReadPerMTok)
	}
}

func TestParse_TieredPricing(t *testing.T) {
	data := []byte(`[{"provider":"test","models":[{"id":"tiered-model","prices":{"input_mtok":{"base":10,"tiers":[]},"output_mtok":20}}]}]`)
	db := &DB{prices: make(map[string]*Price)}
	if err := db.parse(data); err != nil {
		t.Fatal(err)
	}
	p := db.prices["tiered-model"]
	if p == nil {
		t.Fatal("tiered-model not found")
	}
	if p.InputPerMTok != 10 {
		t.Errorf("input = %f, want 10 (base)", p.InputPerMTok)
	}
	if p.OutputPerMTok != 20 {
		t.Errorf("output = %f, want 20", p.OutputPerMTok)
	}
}
