// Package wire implements parsers for LLM API wire formats.
//
// Each format (Chat Completions, Responses API, Anthropic Messages, etc.)
// is implemented once and reused across providers.
package wire

import "strings"

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
	ResponseBody     []byte
}

// SSEEvent is a single server-sent event from a streaming response.
type SSEEvent struct {
	Event string
	Data  []byte
}

func matchPath(path, suffix string) bool {
	return strings.HasSuffix(path, suffix) || strings.Contains(path, suffix+"/")
}
