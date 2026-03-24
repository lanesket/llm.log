package pricing

import (
	"math"
	"testing"

	"github.com/lanesket/llm.log/internal/provider/wire"
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
	cost := db.Cost("openai", "gpt-4", &wire.Result{
		InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200,
	}, 1.0, false)
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
	cost := db.Cost("anthropic", "claude-sonnet-4-6", &wire.Result{
		InputTokens: 1280, OutputTokens: 500, CacheReadTokens: 1000, CacheWriteTokens: 200,
	}, 1.0, false)
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
	cost := db.Cost("openai", "gpt-4", &wire.Result{
		InputTokens: 1000, OutputTokens: 500,
	}, 1.0, false)
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
	cost := db.Cost("openai", "unknown", &wire.Result{
		InputTokens: 100, OutputTokens: 50,
	}, 1.0, false)
	if cost != nil {
		t.Error("expected nil for unknown model")
	}
}

func TestCost_WithAudioTokens(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4o-audio": {
			InputPerMTok:       2.5,
			OutputPerMTok:      10,
			AudioInputPerMTok:  100,
			AudioOutputPerMTok: 200,
		},
	}}
	// 1000 total input, 800 audio input, 200 text input
	// 500 total output, 300 audio output, 200 text output
	cost := db.Cost("openai", "gpt-4o-audio", &wire.Result{
		InputTokens:       1000,
		OutputTokens:      500,
		AudioInputTokens:  800,
		AudioOutputTokens: 300,
	}, 1.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// text input: (1000-800) * 2.5 / 1M = 0.0005
	// text output: (500-300) * 10 / 1M = 0.002
	// audio input: 800 * 100 / 1M = 0.08
	// audio output: 300 * 200 / 1M = 0.06
	expected := 0.0005 + 0.002 + 0.08 + 0.06
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_WithWebSearch(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-sonnet-4-6": {InputPerMTok: 3, OutputPerMTok: 15},
	}}
	cost := db.Cost("anthropic", "claude-sonnet-4-6", &wire.Result{
		InputTokens:       1000,
		OutputTokens:      500,
		WebSearchRequests: 3,
	}, 1.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// text: 1000 * 3 / 1M + 500 * 15 / 1M = 0.003 + 0.0075 = 0.0105
	// web search: 3 * 0.01 = 0.03
	expected := 0.0105 + 0.03
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_NoAudioPriceIgnoresAudioTokens(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60},
	}}
	// Model has no audio pricing — audio tokens treated as regular text
	cost := db.Cost("openai", "gpt-4", &wire.Result{
		InputTokens:      1000,
		OutputTokens:     500,
		AudioInputTokens: 200,
	}, 1.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// No audio price, so all 1000 input at text rate
	expected := 1000.0*30/1e6 + 500.0*60/1e6
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_CacheTTL1h(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-sonnet-4-6": {InputPerMTok: 3, OutputPerMTok: 15, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75},
	}}
	// With 1h TTL: cache write = 2x input = $6/MTok instead of $3.75/MTok
	cost := db.Cost("anthropic", "claude-sonnet-4-6", &wire.Result{
		InputTokens:      1200,
		OutputTokens:     500,
		CacheReadTokens:  0,
		CacheWriteTokens: 1000,
	}, 1.0, true)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// uncached: (1200-0-1000) * 3 / 1M = 0.0006
	// output: 500 * 15 / 1M = 0.0075
	// cache write 1h: 1000 * (3 * 2.0) / 1M = 1000 * 6 / 1M = 0.006
	expected := 0.0006 + 0.0075 + 0.006
	if math.Abs(*cost-expected) > 0.000001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}

	// Compare with 5-min TTL (default)
	cost5m := db.Cost("anthropic", "claude-sonnet-4-6", &wire.Result{
		InputTokens:      1200,
		OutputTokens:     500,
		CacheWriteTokens: 1000,
	}, 1.0, false)
	// cache write 5m: 1000 * 3.75 / 1M = 0.00375
	expected5m := 0.0006 + 0.0075 + 0.00375
	if math.Abs(*cost5m-expected5m) > 0.000001 {
		t.Errorf("cost5m = %f, want %f", *cost5m, expected5m)
	}
	if *cost <= *cost5m {
		t.Errorf("1h cost (%f) should be greater than 5m cost (%f)", *cost, *cost5m)
	}
}

func TestCost_FastMode(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-opus-4-6": {InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25},
	}}
	// Fast mode: 6x multiplier on all token costs
	cost := db.Cost("anthropic", "claude-opus-4-6", &wire.Result{
		InputTokens:  1000,
		OutputTokens: 500,
	}, 6.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// base: 1000*5/1M + 500*25/1M = 0.005 + 0.0125 = 0.0175
	// × 6 = 0.105
	expected := 0.105
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_DataResidency(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-opus-4-6": {InputPerMTok: 5, OutputPerMTok: 25},
	}}
	// Data residency: 1.1x multiplier
	cost := db.Cost("anthropic", "claude-opus-4-6", &wire.Result{
		InputTokens:  1000,
		OutputTokens: 500,
	}, 1.1, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// base: 1000*5/1M + 500*25/1M = 0.005 + 0.0125 = 0.0175
	// × 1.1 = 0.01925
	expected := 0.01925
	if math.Abs(*cost-expected) > 0.00001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_FastModeWithDataResidency(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-opus-4-6": {InputPerMTok: 5, OutputPerMTok: 25},
	}}
	// Stacked: fast mode 6x × data residency 1.1x = 6.6x
	cost := db.Cost("anthropic", "claude-opus-4-6", &wire.Result{
		InputTokens:  1000,
		OutputTokens: 500,
	}, 6.0*1.1, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// base: 0.0175 × 6.6 = 0.1155
	expected := 0.0175 * 6.6
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_WebSearchNotAffectedByMultiplier(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"claude-sonnet-4-6": {InputPerMTok: 3, OutputPerMTok: 15},
	}}
	// Web search cost should NOT be multiplied by fast mode
	cost := db.Cost("anthropic", "claude-sonnet-4-6", &wire.Result{
		InputTokens:       1000,
		OutputTokens:      500,
		WebSearchRequests: 2,
	}, 6.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// token cost: (1000*3/1M + 500*15/1M) * 6.0 = 0.0105 * 6.0 = 0.063
	// web search: 2 * 0.01 = 0.02 (NOT multiplied)
	expected := 0.063 + 0.02
	if math.Abs(*cost-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
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

func TestParse_PriceData_WithAudio(t *testing.T) {
	data := []byte(`[{"provider":"openai","models":[{"id":"gpt-4o-audio","prices":{"input_mtok":2.5,"output_mtok":10,"input_audio_mtok":100,"output_audio_mtok":200}}]}]`)
	db := &DB{prices: make(map[string]*Price)}
	if err := db.parse(data); err != nil {
		t.Fatal(err)
	}
	p := db.prices["gpt-4o-audio"]
	if p == nil {
		t.Fatal("gpt-4o-audio not found")
	}
	if p.AudioInputPerMTok != 100 {
		t.Errorf("audio input = %f, want 100", p.AudioInputPerMTok)
	}
	if p.AudioOutputPerMTok != 200 {
		t.Errorf("audio output = %f, want 200", p.AudioOutputPerMTok)
	}
}

func TestCost_AudioWithCache(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4o-audio": {
			InputPerMTok:       2.5,
			OutputPerMTok:      10,
			CacheReadPerMTok:   1.25,
			AudioInputPerMTok:  100,
			AudioOutputPerMTok: 200,
		},
	}}
	// 1000 total input, 300 cached, 400 audio (disjoint from cached).
	// uncached = 1000 - 300 = 700
	// text input = uncached - audio = 700 - 400 = 300
	cost := db.Cost("openai", "gpt-4o-audio", &wire.Result{
		InputTokens:      1000,
		OutputTokens:     200,
		CacheReadTokens:  300,
		AudioInputTokens: 400,
	}, 1.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// text input: 300 * 2.5 / 1M = 0.00075
	// output: 200 * 10 / 1M = 0.002
	// cache read: 300 * 1.25 / 1M = 0.000375
	// audio input: 400 * 100 / 1M = 0.04
	expected := 0.00075 + 0.002 + 0.000375 + 0.04
	if math.Abs(*cost-expected) > 0.000001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_AudioExceedsUncached(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4o-audio": {
			InputPerMTok:      2.5,
			OutputPerMTok:     10,
			CacheReadPerMTok:  1.25,
			AudioInputPerMTok: 100,
		},
	}}
	// Edge case: audio tokens > uncached portion (audio overlaps cached).
	// 1000 total, 600 cached, 800 audio → uncached=400, textInput=max(0, 400-800)=0
	cost := db.Cost("openai", "gpt-4o-audio", &wire.Result{
		InputTokens:      1000,
		OutputTokens:     100,
		CacheReadTokens:  600,
		AudioInputTokens: 800,
	}, 1.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	// text input: max(0, 400-800) = 0
	// output: 100 * 10 / 1M = 0.001
	// cache read: 600 * 1.25 / 1M = 0.00075
	// audio input: 800 * 100 / 1M = 0.08
	expected := 0.0 + 0.001 + 0.00075 + 0.08
	if math.Abs(*cost-expected) > 0.000001 {
		t.Errorf("cost = %f, want %f", *cost, expected)
	}
}

func TestCost_ZeroTokens(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60},
	}}
	cost := db.Cost("openai", "gpt-4", &wire.Result{}, 1.0, false)
	if cost == nil {
		t.Fatal("expected cost")
	}
	if *cost != 0 {
		t.Errorf("cost = %f, want 0", *cost)
	}
}

func TestCost_MultiplierIdentity(t *testing.T) {
	db := &DB{prices: map[string]*Price{
		"gpt-4": {InputPerMTok: 30, OutputPerMTok: 60},
	}}
	// multiplier=1.0 should produce same result as no multiplier
	cost1 := db.Cost("openai", "gpt-4", &wire.Result{
		InputTokens: 1000, OutputTokens: 500,
	}, 1.0, false)
	if cost1 == nil {
		t.Fatal("expected cost")
	}
	expected := 1000.0*30/1e6 + 500.0*60/1e6
	if math.Abs(*cost1-expected) > 0.0001 {
		t.Errorf("cost = %f, want %f", *cost1, expected)
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
