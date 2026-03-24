// Package wire implements parsers for LLM API wire formats.
//
// Each format (Chat Completions, Responses API, Anthropic Messages, etc.)
// is implemented once and reused across providers.
package wire

import (
	"encoding/json"
	"strings"
)

// Format handles parsing for a specific LLM API wire format.
type Format interface {
	MatchPath(path string) bool
	ModifyRequest(body []byte) ([]byte, error)
	Parse(body []byte) (*Result, error)
	ParseStream(events []SSEEvent) (*Result, error)
}

// Result holds parsed usage data from an API response.
// InputTokens is the total input tokens INCLUDING all cache tokens.
type Result struct {
	Model            string
	InputTokens      int // total: uncached + cache read + cache write
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int

	// AudioInputTokens and AudioOutputTokens are subsets of InputTokens and
	// OutputTokens that were audio (OpenAI only). Priced at a different rate.
	AudioInputTokens  int
	AudioOutputTokens int

	// WebSearchRequests is the number of server-side web searches performed
	// (Anthropic only). Billed at a flat per-search rate.
	WebSearchRequests int

	// FastMode indicates the Anthropic response was served in fast mode (6x pricing).
	// Detected from usage.speed == "fast" in the API response.
	// Ref: https://platform.claude.com/docs/en/build-with-claude/fast-mode
	FastMode bool

	ResponseBody []byte
}

// SSEEvent is a single server-sent event from a streaming response.
type SSEEvent struct {
	Event string
	Data  []byte
}

// reconstructStreamBody builds a minimal JSON response body from streaming data.
// Used by all stream parsers to reconstruct a storable response.
func reconstructStreamBody(model, content string) []byte {
	b, _ := json.Marshal(map[string]any{"model": model, "content": content})
	return b
}

func matchPath(path, suffix string) bool {
	return strings.HasSuffix(path, suffix) || strings.Contains(path, suffix+"/")
}
