package provider

import (
	"encoding/json"
	"testing"
)

func TestAnthropicMessages_Parse(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"usage": {
			"input_tokens": 80,
			"output_tokens": 50,
			"cache_read_input_tokens": 20,
			"cache_creation_input_tokens": 10
		}
	}`)

	r, err := AnthropicMessages.Parse(200, body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 110 {
		t.Errorf("input = %d, want 110 (80 uncached + 20 read + 10 write)", r.InputTokens)
	}
	if r.OutputTokens != 50 {
		t.Errorf("output = %d, want 50", r.OutputTokens)
	}
	if r.CacheReadTokens != 20 {
		t.Errorf("cache read = %d, want 20", r.CacheReadTokens)
	}
	if r.CacheWriteTokens != 10 {
		t.Errorf("cache write = %d, want 10", r.CacheWriteTokens)
	}
}

func TestAnthropicMessages_Parse_NoCaching(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-4-5","usage":{"input_tokens":50,"output_tokens":30}}`)
	r, err := AnthropicMessages.Parse(200, body)
	if err != nil {
		t.Fatal(err)
	}
	if r.InputTokens != 50 {
		t.Errorf("input = %d, want 50", r.InputTokens)
	}
	if r.CacheReadTokens != 0 || r.CacheWriteTokens != 0 {
		t.Errorf("cache = %d/%d, want 0/0", r.CacheReadTokens, r.CacheWriteTokens)
	}
}

func TestAnthropicMessages_ParseStream(t *testing.T) {
	events := []SSEEvent{
		{Event: "message_start", Data: []byte(`{"message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":80,"cache_read_input_tokens":20,"cache_creation_input_tokens":5}}}`)},
		{Event: "content_block_delta", Data: []byte(`{"delta":{"type":"text_delta","text":"Hello"}}`)},
		{Event: "content_block_delta", Data: []byte(`{"delta":{"type":"text_delta","text":" world"}}`)},
		{Event: "message_delta", Data: []byte(`{"usage":{"output_tokens":2}}`)},
	}

	r, err := AnthropicMessages.ParseStream(events)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 105 {
		t.Errorf("input = %d, want 105 (80 + 20 + 5)", r.InputTokens)
	}
	if r.OutputTokens != 2 {
		t.Errorf("output = %d, want 2", r.OutputTokens)
	}
	if r.CacheReadTokens != 20 {
		t.Errorf("cache read = %d, want 20", r.CacheReadTokens)
	}
	if r.CacheWriteTokens != 5 {
		t.Errorf("cache write = %d, want 5", r.CacheWriteTokens)
	}

	var body map[string]any
	json.Unmarshal(r.ResponseBody, &body)
	if body["content"] != "Hello world" {
		t.Errorf("content = %q", body["content"])
	}
}

func TestAnthropicProvider_ResolveFormat(t *testing.T) {
	p, ok := Lookup("api.anthropic.com")
	if !ok {
		t.Fatal("api.anthropic.com not registered")
	}
	if f := ResolveFormat(p, "/v1/messages"); f != AnthropicMessages {
		t.Error("expected AnthropicMessages for /v1/messages")
	}
}
