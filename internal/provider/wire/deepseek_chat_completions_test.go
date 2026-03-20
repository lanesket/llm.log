package wire

import (
	"encoding/json"
	"testing"
)

func TestDeepSeekChatCompletions_Parse(t *testing.T) {
	body := []byte(`{
		"model": "deepseek-chat",
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"prompt_cache_hit_tokens": 80,
			"prompt_cache_miss_tokens": 20
		}
	}`)

	r, err := DeepSeekChatCompletions.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "deepseek-chat" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 100 {
		t.Errorf("input = %d, want 100", r.InputTokens)
	}
	if r.OutputTokens != 50 {
		t.Errorf("output = %d, want 50", r.OutputTokens)
	}
	if r.CacheReadTokens != 80 {
		t.Errorf("cache read = %d, want 80", r.CacheReadTokens)
	}
}

func TestDeepSeekChatCompletions_ParseStream(t *testing.T) {
	events := []SSEEvent{
		{Data: []byte(`{"model":"deepseek-chat","choices":[{"delta":{"content":"Hi"}}]}`)},
		{Data: []byte(`{"model":"deepseek-chat","choices":[],"usage":{"prompt_tokens":50,"completion_tokens":10,"prompt_cache_hit_tokens":30,"prompt_cache_miss_tokens":20}}`)},
		{Data: []byte("[DONE]")},
	}

	r, err := DeepSeekChatCompletions.ParseStream(events)
	if err != nil {
		t.Fatal(err)
	}
	if r.InputTokens != 50 {
		t.Errorf("input = %d, want 50", r.InputTokens)
	}
	if r.OutputTokens != 10 {
		t.Errorf("output = %d, want 10", r.OutputTokens)
	}
	if r.CacheReadTokens != 30 {
		t.Errorf("cache read = %d, want 30", r.CacheReadTokens)
	}

	var body map[string]any
	json.Unmarshal(r.ResponseBody, &body)
	if body["content"] != "Hi" {
		t.Errorf("content = %q", body["content"])
	}
}

func TestDeepSeekChatCompletions_ModifyRequest(t *testing.T) {
	body := []byte(`{"model":"deepseek-chat","stream":true,"messages":[]}`)
	modified, err := DeepSeekChatCompletions.ModifyRequest(body)
	if err != nil {
		t.Fatal(err)
	}

	var req map[string]any
	json.Unmarshal(modified, &req)
	opts, ok := req["stream_options"].(map[string]any)
	if !ok {
		t.Fatal("stream_options not set")
	}
	if opts["include_usage"] != true {
		t.Error("include_usage not true")
	}
}
