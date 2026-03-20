package proxy

import (
	"testing"
	"time"

	"github.com/lanesket/llm.log/internal/provider"
	"github.com/lanesket/llm.log/internal/provider/wire"
)

func TestExtractModelFromRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"openai request", `{"model":"gpt-4","messages":[]}`, "gpt-4"},
		{"anthropic request", `{"model":"claude-sonnet-4-20250514","max_tokens":1024}`, "claude-sonnet-4-20250514"},
		{"empty body", ``, ""},
		{"no model field", `{"messages":[]}`, ""},
		{"invalid json", `not json`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractModelFromRequest([]byte(tt.body))
			if got != tt.want {
				t.Errorf("extractModelFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSave_ErrorResponse_UsesRequestModel(t *testing.T) {
	store := &mockStore{}
	p := newTestProxy(t, store, 50*time.Millisecond)

	state := &requestState{
		provider:    lookupProvider(t, "api.openai.com"),
		format:      wire.ChatCompletions,
		requestBody: []byte(`{"model":"gpt-4","messages":[]}`),
		startTime:   time.Now(),
		endpoint:    "/v1/chat/completions",
	}

	// Simulate an error response: no model in result, but model in request body.
	p.save(state, 429, false, &wire.Result{
		ResponseBody: []byte(`{"error":{"message":"rate limited"}}`),
	})

	time.Sleep(150 * time.Millisecond)

	if store.count() != 1 {
		t.Fatalf("got %d records, want 1", store.count())
	}

	store.mu.Lock()
	rec := store.saved[0]
	store.mu.Unlock()

	if rec.Model != "gpt-4" {
		t.Errorf("model = %q, want %q", rec.Model, "gpt-4")
	}
	if rec.StatusCode != 429 {
		t.Errorf("status = %d, want 429", rec.StatusCode)
	}
	if rec.InputTokens != 0 {
		t.Errorf("input tokens = %d, want 0", rec.InputTokens)
	}

	close(p.stop)
	<-p.stopped
}

func TestSave_DropWhenNoModelAnywhere(t *testing.T) {
	store := &mockStore{}
	p := newTestProxy(t, store, 50*time.Millisecond)

	state := &requestState{
		provider:    lookupProvider(t, "api.openai.com"),
		format:      wire.ChatCompletions,
		requestBody: []byte(`{}`), // no model in request either
		startTime:   time.Now(),
		endpoint:    "/v1/chat/completions",
	}

	p.save(state, 400, false, &wire.Result{
		ResponseBody: []byte(`{"error":{"message":"bad request"}}`),
	})

	time.Sleep(150 * time.Millisecond)

	if store.count() != 0 {
		t.Errorf("got %d records, want 0 (should be dropped)", store.count())
	}

	close(p.stop)
	<-p.stopped
}

func lookupProvider(t *testing.T, domain string) provider.Provider {
	t.Helper()
	p, ok := provider.Lookup(domain)
	if !ok {
		t.Fatalf("%s not registered", domain)
	}
	return p
}
