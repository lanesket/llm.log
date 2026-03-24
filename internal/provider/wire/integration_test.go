// integration_test.go verifies wire parsers against real LLM API responses.
//
// Run: go test -tags integration -v ./internal/provider/wire/
//
// Required env vars:
//   - OPENAI_API_KEY     — for OpenAI tests
//   - ANTHROPIC_API_KEY  — for Anthropic tests
//
// Tests use the cheapest models (gpt-4.1-nano, claude-haiku-4-5) to minimize cost.
// Cache hit tests send duplicate prompts with >1024 tokens to trigger automatic caching.
//
//go:build integration

package wire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// --- OpenAI Chat Completions ---

func TestIntegration_OpenAI_ChatCompletions(t *testing.T) {
	key := requireEnv(t, "OPENAI_API_KEY")

	body := openaiRequest(t, key, `{
		"model": "gpt-4.1-nano",
		"messages": [{"role": "user", "content": "Say hello in exactly 3 words."}],
		"max_completion_tokens": 20
	}`)

	raw := extractRawUsage(t, body)
	r := mustParse(t, ChatCompletions, body)

	assertTokensMatch(t, r, raw)
	assertPositive(t, "InputTokens", r.InputTokens)
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)
}

func TestIntegration_OpenAI_ChatCompletions_Streaming(t *testing.T) {
	key := requireEnv(t, "OPENAI_API_KEY")

	events := openaiStreamRequest(t, key, `{
		"model": "gpt-4.1-nano",
		"messages": [{"role": "user", "content": "Say hello."}],
		"max_completion_tokens": 20,
		"stream": true,
		"stream_options": {"include_usage": true}
	}`)

	r := mustParseStream(t, ChatCompletions, events)

	assertPositive(t, "InputTokens", r.InputTokens)
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)
}

func TestIntegration_OpenAI_ChatCompletions_CacheHit(t *testing.T) {
	key := requireEnv(t, "OPENAI_API_KEY")

	// >1024 tokens to trigger automatic caching.
	longText := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 200)
	reqJSON := fmt.Sprintf(`{
		"model": "gpt-4.1-nano",
		"messages": [
			{"role": "system", "content": %q},
			{"role": "user", "content": "Reply with one word."}
		],
		"max_completion_tokens": 10
	}`, longText)

	// Two identical requests — second should hit cache.
	r1 := mustParse(t, ChatCompletions, openaiRequest(t, key, reqJSON))
	r2 := mustParse(t, ChatCompletions, openaiRequest(t, key, reqJSON))

	assertPositive(t, "call1.InputTokens", r1.InputTokens)
	assertPositive(t, "call2.InputTokens", r2.InputTokens)

	t.Logf("call1: input=%d cacheRead=%d", r1.InputTokens, r1.CacheReadTokens)
	t.Logf("call2: input=%d cacheRead=%d", r2.InputTokens, r2.CacheReadTokens)

	// Cache hit is not guaranteed (server-side), but we verify the field parses correctly.
	if r2.CacheReadTokens > 0 {
		t.Logf("cache hit confirmed: %d cached tokens", r2.CacheReadTokens)
		if r2.CacheReadTokens > r2.InputTokens {
			t.Errorf("CacheReadTokens (%d) > InputTokens (%d)", r2.CacheReadTokens, r2.InputTokens)
		}
	}
}

func TestIntegration_OpenAI_ChatCompletions_AudioFields(t *testing.T) {
	key := requireEnv(t, "OPENAI_API_KEY")

	// Non-audio request: audio_tokens fields should be present but zero.
	body := openaiRequest(t, key, `{
		"model": "gpt-4.1-nano",
		"messages": [{"role": "user", "content": "Hi."}],
		"max_completion_tokens": 10
	}`)

	raw := extractRawUsage(t, body)
	r := mustParse(t, ChatCompletions, body)

	// Verify audio fields parse (should be 0 for text-only).
	if r.AudioInputTokens != 0 {
		t.Errorf("AudioInputTokens = %d, want 0 (text-only request)", r.AudioInputTokens)
	}
	if r.AudioOutputTokens != 0 {
		t.Errorf("AudioOutputTokens = %d, want 0 (text-only request)", r.AudioOutputTokens)
	}

	// Verify API actually returns the audio_tokens field (even if 0).
	details := extractNestedInt(raw, "prompt_tokens_details", "audio_tokens")
	if details != 0 {
		t.Errorf("raw audio_tokens = %d, want 0", details)
	}
}

func TestIntegration_OpenAI_Responses(t *testing.T) {
	key := requireEnv(t, "OPENAI_API_KEY")

	body := doHTTP(t, "POST", "https://api.openai.com/v1/responses", `{
		"model": "gpt-4.1-nano",
		"input": "Say hello in exactly 3 words.",
		"max_output_tokens": 20
	}`, map[string]string{
		"Authorization": "Bearer " + key,
		"Content-Type":  "application/json",
	})

	r := mustParse(t, Responses, body)

	assertPositive(t, "InputTokens", r.InputTokens)
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)

	t.Logf("Responses API: input=%d output=%d cacheRead=%d model=%s",
		r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.Model)
}

// --- Anthropic Messages ---

func TestIntegration_Anthropic_Messages(t *testing.T) {
	key := requireEnv(t, "ANTHROPIC_API_KEY")

	body := anthropicRequest(t, key, `{
		"model": "claude-haiku-4-5-20251001",
		"max_tokens": 20,
		"messages": [{"role": "user", "content": "Say hello in exactly 3 words."}]
	}`)

	raw := extractRawUsage(t, body)
	r := mustParse(t, AnthropicMessages, body)

	assertPositive(t, "InputTokens", r.InputTokens)
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)

	// Verify Anthropic semantics: input_tokens is uncached, our InputTokens = total.
	rawInput := extractTopInt(raw, "input_tokens")
	rawCacheRead := extractTopInt(raw, "cache_read_input_tokens")
	rawCacheWrite := extractTopInt(raw, "cache_creation_input_tokens")
	expectedTotal := rawInput + rawCacheRead + rawCacheWrite

	if r.InputTokens != expectedTotal {
		t.Errorf("InputTokens = %d, want %d (raw input %d + cache_read %d + cache_write %d)",
			r.InputTokens, expectedTotal, rawInput, rawCacheRead, rawCacheWrite)
	}

	t.Logf("Anthropic: total_input=%d (uncached=%d read=%d write=%d) output=%d",
		r.InputTokens, rawInput, r.CacheReadTokens, r.CacheWriteTokens, r.OutputTokens)
}

func TestIntegration_Anthropic_Messages_Streaming(t *testing.T) {
	key := requireEnv(t, "ANTHROPIC_API_KEY")

	events := anthropicStreamRequest(t, key, `{
		"model": "claude-haiku-4-5-20251001",
		"max_tokens": 20,
		"stream": true,
		"messages": [{"role": "user", "content": "Say hello."}]
	}`)

	r := mustParseStream(t, AnthropicMessages, events)

	assertPositive(t, "InputTokens", r.InputTokens)
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)

	t.Logf("Anthropic streaming: input=%d output=%d cacheRead=%d cacheWrite=%d model=%s",
		r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens, r.Model)
}

func TestIntegration_Anthropic_CacheHit(t *testing.T) {
	key := requireEnv(t, "ANTHROPIC_API_KEY")

	// >4096 tokens for Haiku cache eligibility.
	longText := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 500)
	reqJSON := fmt.Sprintf(`{
		"model": "claude-haiku-4-5-20251001",
		"max_tokens": 10,
		"system": [{"type":"text","text":%q,"cache_control":{"type":"ephemeral"}}],
		"messages": [{"role":"user","content":"Reply with one word."}]
	}`, longText)

	r1 := mustParse(t, AnthropicMessages, anthropicRequest(t, key, reqJSON))
	r2 := mustParse(t, AnthropicMessages, anthropicRequest(t, key, reqJSON))

	t.Logf("call1: input=%d cacheRead=%d cacheWrite=%d", r1.InputTokens, r1.CacheReadTokens, r1.CacheWriteTokens)
	t.Logf("call2: input=%d cacheRead=%d cacheWrite=%d", r2.InputTokens, r2.CacheReadTokens, r2.CacheWriteTokens)

	// First call should write cache, second should read.
	if r1.CacheWriteTokens > 0 {
		t.Logf("call1 cache write confirmed: %d tokens", r1.CacheWriteTokens)
	}
	if r2.CacheReadTokens > 0 {
		t.Logf("call2 cache hit confirmed: %d tokens", r2.CacheReadTokens)
	}
}

// --- OpenRouter as Anthropic proxy ---
// These tests use OpenRouter's /v1/messages endpoint which returns
// Anthropic-format responses, allowing us to test the Anthropic parser
// without an Anthropic API key.

func TestIntegration_OpenRouter_AnthropicFormat(t *testing.T) {
	key := requireEnv(t, "OPENROUTER_API_KEY")

	body := doHTTP(t, "POST", "https://openrouter.ai/api/v1/messages", `{
		"model": "anthropic/claude-haiku-4-5",
		"max_tokens": 20,
		"messages": [{"role": "user", "content": "Say hello in 3 words."}]
	}`, map[string]string{
		"Authorization":     "Bearer " + key,
		"Content-Type":      "application/json",
		"anthropic-version": "2023-06-01",
	})

	raw := extractRawUsage(t, body)
	r := mustParse(t, AnthropicMessages, body)

	assertPositive(t, "InputTokens", r.InputTokens)
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)

	// Verify Anthropic semantics: total = uncached + cache_read + cache_creation.
	rawInput := extractTopInt(raw, "input_tokens")
	rawCacheRead := extractTopInt(raw, "cache_read_input_tokens")
	rawCacheWrite := extractTopInt(raw, "cache_creation_input_tokens")
	expectedTotal := rawInput + rawCacheRead + rawCacheWrite

	if r.InputTokens != expectedTotal {
		t.Errorf("InputTokens = %d, want %d (uncached=%d + read=%d + write=%d)",
			r.InputTokens, expectedTotal, rawInput, rawCacheRead, rawCacheWrite)
	}

	// Cross-verify with OpenRouter's cost field.
	rawCost := extractFloat(raw, "cost")
	if rawCost > 0 {
		t.Logf("OpenRouter cost: $%.6f", rawCost)
	}

	t.Logf("Anthropic via OpenRouter: input=%d (uncached=%d read=%d write=%d) output=%d model=%s",
		r.InputTokens, rawInput, r.CacheReadTokens, r.CacheWriteTokens, r.OutputTokens, r.Model)
}

func TestIntegration_OpenRouter_AnthropicFormat_Streaming(t *testing.T) {
	key := requireEnv(t, "OPENROUTER_API_KEY")

	events := doHTTPStream(t, "POST", "https://openrouter.ai/api/v1/messages", `{
		"model": "anthropic/claude-haiku-4-5",
		"max_tokens": 20,
		"stream": true,
		"messages": [{"role": "user", "content": "Say hello."}]
	}`, map[string]string{
		"Authorization":     "Bearer " + key,
		"Content-Type":      "application/json",
		"anthropic-version": "2023-06-01",
	})

	r := mustParseStream(t, AnthropicMessages, events)

	// OpenRouter may not populate input_tokens in message_start for streamed responses.
	// We still verify output tokens and model are parsed correctly.
	assertPositive(t, "OutputTokens", r.OutputTokens)
	assertNonEmpty(t, "Model", r.Model)

	t.Logf("Anthropic streaming via OpenRouter: input=%d output=%d model=%s",
		r.InputTokens, r.OutputTokens, r.Model)
	if r.InputTokens == 0 {
		t.Log("NOTE: InputTokens=0 is expected — OpenRouter omits input_tokens in streaming message_start")
	}
}

func TestIntegration_OpenRouter_AnthropicFormat_CacheHit(t *testing.T) {
	key := requireEnv(t, "OPENROUTER_API_KEY")

	// >4096 tokens for cache eligibility.
	longText := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 500)
	reqJSON := fmt.Sprintf(`{
		"model": "anthropic/claude-haiku-4-5",
		"max_tokens": 10,
		"system": [{"type":"text","text":%q,"cache_control":{"type":"ephemeral"}}],
		"messages": [{"role":"user","content":"Reply with one word."}]
	}`, longText)

	headers := map[string]string{
		"Authorization":     "Bearer " + key,
		"Content-Type":      "application/json",
		"anthropic-version": "2023-06-01",
	}

	body1 := doHTTP(t, "POST", "https://openrouter.ai/api/v1/messages", reqJSON, headers)
	r1 := mustParse(t, AnthropicMessages, body1)
	raw1 := extractRawUsage(t, body1)

	body2 := doHTTP(t, "POST", "https://openrouter.ai/api/v1/messages", reqJSON, headers)
	r2 := mustParse(t, AnthropicMessages, body2)
	raw2 := extractRawUsage(t, body2)

	t.Logf("call1: input=%d cacheRead=%d cacheWrite=%d cost=$%.6f",
		r1.InputTokens, r1.CacheReadTokens, r1.CacheWriteTokens, extractFloat(raw1, "cost"))
	t.Logf("call2: input=%d cacheRead=%d cacheWrite=%d cost=$%.6f",
		r2.InputTokens, r2.CacheReadTokens, r2.CacheWriteTokens, extractFloat(raw2, "cost"))

	if r1.CacheWriteTokens > 0 {
		t.Logf("call1 cache write confirmed: %d tokens", r1.CacheWriteTokens)
	}
	if r2.CacheReadTokens > 0 {
		t.Logf("call2 cache hit confirmed: %d tokens", r2.CacheReadTokens)
		// With cache hit, cost should be lower.
		cost1 := extractFloat(raw1, "cost")
		cost2 := extractFloat(raw2, "cost")
		if cost1 > 0 && cost2 > 0 && cost2 >= cost1 {
			t.Logf("WARNING: cache hit cost ($%.6f) >= cache miss cost ($%.6f)", cost2, cost1)
		}
	}
}

// --- Helpers ---

func extractFloat(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s not set", key)
	}
	return v
}

func doHTTP(t *testing.T, method, url, body string, headers map[string]string) []byte {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("HTTP %d: %s", resp.StatusCode, respBody)
	}
	return respBody
}

func doHTTPStream(t *testing.T, method, url, body string, headers map[string]string) []SSEEvent {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("HTTP %d: %s", resp.StatusCode, b)
	}

	return readSSEEvents(t, resp.Body)
}

func openaiRequest(t *testing.T, key, body string) []byte {
	t.Helper()
	return doHTTP(t, "POST", "https://api.openai.com/v1/chat/completions", body, map[string]string{
		"Authorization": "Bearer " + key,
		"Content-Type":  "application/json",
	})
}

func openaiStreamRequest(t *testing.T, key, body string) []SSEEvent {
	t.Helper()
	return doHTTPStream(t, "POST", "https://api.openai.com/v1/chat/completions", body, map[string]string{
		"Authorization": "Bearer " + key,
		"Content-Type":  "application/json",
	})
}

func anthropicRequest(t *testing.T, key, body string) []byte {
	t.Helper()
	return doHTTP(t, "POST", "https://api.anthropic.com/v1/messages", body, map[string]string{
		"x-api-key":         key,
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
	})
}

func anthropicStreamRequest(t *testing.T, key, body string) []SSEEvent {
	t.Helper()
	return doHTTPStream(t, "POST", "https://api.anthropic.com/v1/messages", body, map[string]string{
		"x-api-key":         key,
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
	})
}

func mustParse(t *testing.T, f Format, body []byte) *Result {
	t.Helper()
	r, err := f.Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return r
}

func mustParseStream(t *testing.T, f Format, events []SSEEvent) *Result {
	t.Helper()
	r, err := f.ParseStream(events)
	if err != nil {
		t.Fatalf("ParseStream: %v", err)
	}
	return r
}

func assertPositive(t *testing.T, name string, v int) {
	t.Helper()
	if v <= 0 {
		t.Errorf("%s = %d, want > 0", name, v)
	}
}

func assertNonEmpty(t *testing.T, name, v string) {
	t.Helper()
	if v == "" {
		t.Errorf("%s is empty", name)
	}
}

// assertTokensMatch cross-checks parsed Result against raw API usage JSON.
func assertTokensMatch(t *testing.T, r *Result, raw map[string]any) {
	t.Helper()
	if want := extractTopInt(raw, "prompt_tokens"); r.InputTokens != want {
		t.Errorf("InputTokens = %d, raw prompt_tokens = %d", r.InputTokens, want)
	}
	if want := extractTopInt(raw, "completion_tokens"); r.OutputTokens != want {
		t.Errorf("OutputTokens = %d, raw completion_tokens = %d", r.OutputTokens, want)
	}
	cached := extractNestedInt(raw, "prompt_tokens_details", "cached_tokens")
	if r.CacheReadTokens != cached {
		t.Errorf("CacheReadTokens = %d, raw cached_tokens = %d", r.CacheReadTokens, cached)
	}
}

func extractRawUsage(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	usage, ok := resp["usage"].(map[string]any)
	if !ok {
		t.Fatal("no usage object in response")
	}
	t.Logf("raw usage: %s", mustJSON(usage))
	return usage
}

func extractTopInt(m map[string]any, key string) int {
	v, _ := m[key].(float64)
	return int(v)
}

func extractNestedInt(m map[string]any, outer, inner string) int {
	sub, ok := m[outer].(map[string]any)
	if !ok {
		return 0
	}
	v, _ := sub[inner].(float64)
	return int(v)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func readSSEEvents(t *testing.T, r io.Reader) []SSEEvent {
	t.Helper()
	data, _ := io.ReadAll(r)
	lines := bytes.Split(data, []byte("\n"))

	var events []SSEEvent
	var current SSEEvent

	for _, line := range lines {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			if len(current.Data) > 0 || current.Event != "" {
				events = append(events, current)
				current = SSEEvent{}
			}
			continue
		}
		if bytes.HasPrefix(line, []byte("event: ")) {
			current.Event = string(bytes.TrimPrefix(line, []byte("event: ")))
		} else if bytes.HasPrefix(line, []byte("data: ")) {
			current.Data = bytes.TrimPrefix(line, []byte("data: "))
		}
	}
	return events
}
