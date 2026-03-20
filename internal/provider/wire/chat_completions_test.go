package wire

import (
	"encoding/json"
	"testing"
)

func TestChatCompletions_Parse(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4.1",
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"prompt_tokens_details": {"cached_tokens": 20}
		}
	}`)

	r, err := ChatCompletions.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "gpt-4.1" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 100 {
		t.Errorf("input = %d, want 100", r.InputTokens)
	}
	if r.OutputTokens != 50 {
		t.Errorf("output = %d, want 50", r.OutputTokens)
	}
	if r.CacheReadTokens != 20 {
		t.Errorf("cached = %d, want 20", r.CacheReadTokens)
	}
}

func TestChatCompletions_Parse_ErrorResponse(t *testing.T) {
	// Error responses have no model/usage — Parse returns empty Result, not an error.
	// The proxy layer handles model recovery from the request body.
	body := []byte(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`)

	r, err := ChatCompletions.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "" {
		t.Errorf("model = %q, want empty", r.Model)
	}
	if r.InputTokens != 0 || r.OutputTokens != 0 {
		t.Errorf("tokens = %d/%d, want 0/0", r.InputTokens, r.OutputTokens)
	}
}

func TestChatCompletions_ModifyRequest_InjectsStreamOptions(t *testing.T) {
	body := []byte(`{"model":"gpt-4","stream":true,"messages":[]}`)
	modified, err := ChatCompletions.ModifyRequest(body)
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

func TestChatCompletions_ModifyRequest_PreservesExistingOptions(t *testing.T) {
	body := []byte(`{"model":"gpt-4","stream":true,"stream_options":{"custom":"value"}}`)
	modified, _ := ChatCompletions.ModifyRequest(body)

	var req map[string]any
	json.Unmarshal(modified, &req)
	opts := req["stream_options"].(map[string]any)
	if opts["custom"] != "value" {
		t.Error("existing option lost")
	}
	if opts["include_usage"] != true {
		t.Error("include_usage not set")
	}
}

func TestChatCompletions_ModifyRequest_SkipsNonStreaming(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[]}`)
	result, _ := ChatCompletions.ModifyRequest(body)
	if string(result) != string(body) {
		t.Error("non-streaming request was modified")
	}
}

func TestChatCompletions_ParseStream(t *testing.T) {
	events := []SSEEvent{
		{Data: []byte(`{"model":"gpt-4","choices":[{"delta":{"content":"Hello"}}]}`)},
		{Data: []byte(`{"model":"gpt-4","choices":[{"delta":{"content":" world"}}]}`)},
		{Data: []byte(`{"model":"gpt-4","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":5}}}`)},
		{Data: []byte("[DONE]")},
	}

	r, err := ChatCompletions.ParseStream(events)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "gpt-4" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 10 {
		t.Errorf("input = %d, want 10", r.InputTokens)
	}
	if r.OutputTokens != 2 {
		t.Errorf("output = %d, want 2", r.OutputTokens)
	}
	if r.CacheReadTokens != 5 {
		t.Errorf("cached = %d, want 5", r.CacheReadTokens)
	}

	var body map[string]any
	json.Unmarshal(r.ResponseBody, &body)
	if body["content"] != "Hello world" {
		t.Errorf("content = %q", body["content"])
	}
}
