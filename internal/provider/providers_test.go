package provider

import (
	"testing"

	"github.com/lanesket/llm.log/internal/provider/wire"
)

func TestAllProviders_Registration(t *testing.T) {
	tests := []struct {
		domain string
		name   string
	}{
		{"api.openai.com", "openai"},
		{"api.anthropic.com", "anthropic"},
		{"openrouter.ai", "openrouter"},
		{"api.groq.com", "groq"},
		{"api.together.xyz", "together"},
		{"api.fireworks.ai", "fireworks"},
		{"api.deepseek.com", "deepseek"},
		{"api.mistral.ai", "mistral"},
		{"api.perplexity.ai", "perplexity"},
		{"api.x.ai", "xai"},
	}

	for _, tt := range tests {
		p, ok := Lookup(tt.domain)
		if !ok {
			t.Errorf("%s not registered", tt.domain)
			continue
		}
		if p.Name() != tt.name {
			t.Errorf("Lookup(%q).Name() = %q, want %q", tt.domain, p.Name(), tt.name)
		}
	}
}

func TestChatCompletionsProviders_ResolveFormat(t *testing.T) {
	tests := []struct {
		domain string
		paths  []string
	}{
		{"api.groq.com", []string{"/openai/v1/chat/completions", "/v1/chat/completions"}},
		{"api.together.xyz", []string{"/v1/chat/completions"}},
		{"api.fireworks.ai", []string{"/inference/v1/chat/completions", "/v1/chat/completions"}},
		{"api.mistral.ai", []string{"/v1/chat/completions"}},
		{"api.x.ai", []string{"/v1/chat/completions"}},
	}

	for _, tt := range tests {
		p, _ := Lookup(tt.domain)
		for _, path := range tt.paths {
			if f := ResolveFormat(p, path); f != wire.ChatCompletions {
				t.Errorf("%s: ResolveFormat(%q) = %T, want ChatCompletions", tt.domain, path, f)
			}
		}
	}
}

func TestOpenAI_ResolveFormat(t *testing.T) {
	p, ok := Lookup("api.openai.com")
	if !ok {
		t.Fatal("api.openai.com not registered")
	}
	if f := ResolveFormat(p, "/v1/chat/completions"); f != wire.ChatCompletions {
		t.Error("expected ChatCompletions for /v1/chat/completions")
	}
	if f := ResolveFormat(p, "/v1/responses"); f != wire.Responses {
		t.Error("expected Responses for /v1/responses")
	}
}

func TestAnthropic_ResolveFormat(t *testing.T) {
	p, ok := Lookup("api.anthropic.com")
	if !ok {
		t.Fatal("api.anthropic.com not registered")
	}
	if f := ResolveFormat(p, "/v1/messages"); f != wire.AnthropicMessages {
		t.Error("expected AnthropicMessages for /v1/messages")
	}
}

func TestOpenRouter_ResolveFormat(t *testing.T) {
	p, ok := Lookup("openrouter.ai")
	if !ok {
		t.Fatal("openrouter.ai not registered")
	}

	tests := []struct {
		path string
		want wire.Format
	}{
		{"/api/v1/chat/completions", wire.ChatCompletions},
		{"/api/v1/responses", wire.Responses},
		{"/api/v1/messages", wire.AnthropicMessages},
		{"/v1/chat/completions", wire.ChatCompletions},
		{"/v1/responses", wire.Responses},
		{"/v1/messages", wire.AnthropicMessages},
	}
	for _, tt := range tests {
		if got := ResolveFormat(p, tt.path); got != tt.want {
			t.Errorf("ResolveFormat(%q) = %T, want %T", tt.path, got, tt.want)
		}
	}
}

func TestDeepSeek_ResolveFormat(t *testing.T) {
	p, ok := Lookup("api.deepseek.com")
	if !ok {
		t.Fatal("api.deepseek.com not registered")
	}
	if f := ResolveFormat(p, "/chat/completions"); f != wire.DeepSeekChatCompletions {
		t.Errorf("expected DeepSeekChatCompletions, got %T", f)
	}
}

func TestPerplexity_ResolveFormat(t *testing.T) {
	p, ok := Lookup("api.perplexity.ai")
	if !ok {
		t.Fatal("api.perplexity.ai not registered")
	}
	if f := ResolveFormat(p, "/v1/sonar"); f != wire.PerplexitySonar {
		t.Errorf("expected PerplexitySonar for /v1/sonar, got %T", f)
	}
	if f := ResolveFormat(p, "/v1/chat/completions"); f != wire.ChatCompletions {
		t.Errorf("expected ChatCompletions for /v1/chat/completions, got %T", f)
	}
}
