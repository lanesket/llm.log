package provider

import "testing"

func TestOpenRouter_FormatFor(t *testing.T) {
	p, ok := Lookup("openrouter.ai")
	if !ok {
		t.Fatal("openrouter.ai not registered")
	}

	tests := []struct {
		path string
		want Format
	}{
		{"/api/v1/chat/completions", ChatCompletions},
		{"/api/v1/responses", Responses},
		{"/api/v1/messages", AnthropicMessages},
		{"/v1/chat/completions", ChatCompletions},
		{"/v1/responses", Responses},
		{"/v1/messages", AnthropicMessages},
	}
	for _, tt := range tests {
		if got := ResolveFormat(p, tt.path); got != tt.want {
			t.Errorf("ResolveFormat(%q) = %T, want %T", tt.path, got, tt.want)
		}
	}
}

func TestOpenRouter_ChatCompletions(t *testing.T) {
	body := []byte(`{
		"model": "anthropic/claude-3-5-haiku",
		"usage": {
			"prompt_tokens": 50,
			"completion_tokens": 20,
			"prompt_tokens_details": {"cached_tokens": 10}
		}
	}`)

	r, err := ChatCompletions.Parse(200, body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "anthropic/claude-3-5-haiku" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 50 || r.OutputTokens != 20 || r.CacheReadTokens != 10 {
		t.Errorf("tokens = %d/%d/%d, want 50/20/10", r.InputTokens, r.OutputTokens, r.CacheReadTokens)
	}
}
