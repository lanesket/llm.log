package proxy

import (
	"math"
	"testing"

	"github.com/lanesket/llm.log/internal/provider/wire"
)

func TestDetectAnthropicModifiers_DataResidency(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{"model":"claude-opus-4-6","inference_geo":"us","messages":[]}`),
	}
	multiplier, _ := detectAnthropicModifiers(state, &wire.Result{})

	assertFloat(t, "multiplier", multiplier, 1.1)
}

func TestDetectAnthropicModifiers_CacheTTL1h(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{
			"model":"claude-sonnet-4-6",
			"messages":[{"role":"user","content":"hi"}],
			"system":[{"type":"text","text":"You are helpful","cache_control":{"type":"ephemeral","ttl":"1h"}}]
		}`),
	}
	_, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{CacheWriteTokens: 500})

	if !cacheTTL1h {
		t.Error("cacheTTL1h should be true")
	}
}

func TestDetectAnthropicModifiers_CacheTTL1h_NoCacheWrite(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{
			"model":"claude-sonnet-4-6",
			"system":[{"type":"text","text":"cached","cache_control":{"type":"ephemeral","ttl":"1h"}}]
		}`),
	}
	_, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{CacheReadTokens: 100})

	if cacheTTL1h {
		t.Error("cacheTTL1h should be false when no cache writes")
	}
}

func TestDetectAnthropicModifiers_CacheTTL1h_NoFalsePositive(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{
			"model":"claude-sonnet-4-6",
			"messages":[{"role":"user","content":"Set timer for 1h please"}]
		}`),
	}
	_, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{CacheWriteTokens: 100})

	if cacheTTL1h {
		t.Error("cacheTTL1h should be false — '1h' is in message content, not cache_control")
	}
}

func TestDetectAnthropicModifiers_CacheTTL1h_MessageLevel(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{
			"model":"claude-sonnet-4-6",
			"messages":[
				{"role":"user","content":"hello","cache_control":{"type":"ephemeral","ttl":"1h"}}
			]
		}`),
	}
	_, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{CacheWriteTokens: 100})

	if !cacheTTL1h {
		t.Error("cacheTTL1h should be true for message-level cache_control")
	}
}

func TestDetectAnthropicModifiers_CacheTTL1h_ToolLevel(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{
			"model":"claude-sonnet-4-6",
			"messages":[{"role":"user","content":"use the tool"}],
			"tools":[{"name":"search","description":"search","input_schema":{},"cache_control":{"type":"ephemeral","ttl":"1h"}}]
		}`),
	}
	_, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{CacheWriteTokens: 200})

	if !cacheTTL1h {
		t.Error("cacheTTL1h should be true for tool-level cache_control")
	}
}

func TestDetectAnthropicModifiers_NoModifiers(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{"model":"claude-sonnet-4-6","messages":[]}`),
	}
	multiplier, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{})

	assertFloat(t, "multiplier", multiplier, 1.0)
	if cacheTTL1h {
		t.Error("cacheTTL1h should be false")
	}
}

func TestDetectAnthropicModifiers_InferenceGeoGlobal(t *testing.T) {
	state := &requestState{
		requestBody: []byte(`{"model":"claude-opus-4-6","inference_geo":"global","messages":[]}`),
	}
	multiplier, _ := detectAnthropicModifiers(state, &wire.Result{})

	assertFloat(t, "multiplier", multiplier, 1.0)
}

func TestDetectAnthropicModifiers_EmptyBody(t *testing.T) {
	state := &requestState{
		requestBody: nil,
	}
	multiplier, cacheTTL1h := detectAnthropicModifiers(state, &wire.Result{})

	assertFloat(t, "multiplier", multiplier, 1.0)
	if cacheTTL1h {
		t.Error("cacheTTL1h should be false")
	}
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %f, want %f", name, got, want)
	}
}
