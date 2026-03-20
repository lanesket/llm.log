package wire

import "encoding/json"

// DeepSeekChatCompletions extends Chat Completions with DeepSeek-specific
// cache token fields (prompt_cache_hit_tokens instead of prompt_tokens_details.cached_tokens).
// Spec: https://api-docs.deepseek.com/api/create-chat-completion
var DeepSeekChatCompletions Format = &deepseekChatCompletions{}

type deepseekChatCompletions struct{}

func (d *deepseekChatCompletions) MatchPath(path string) bool {
	return matchPath(path, "/chat/completions")
}

func (d *deepseekChatCompletions) ModifyRequest(body []byte) ([]byte, error) {
	return injectStreamUsage(body)
}

func (d *deepseekChatCompletions) Parse(body []byte) (*Result, error) {
	return parseCCResponse(body, deepseekUsage)
}

func (d *deepseekChatCompletions) ParseStream(events []SSEEvent) (*Result, error) {
	return parseCCStream(events, deepseekUsage)
}

// deepseekUsage maps DeepSeek's cache fields (prompt_cache_hit_tokens).
// Returns zeros for nil/invalid input — callers handle empty results.
func deepseekUsage(raw json.RawMessage) (input, output, cacheRead, cacheWrite int) {
	var u struct {
		PromptTokens         int `json:"prompt_tokens"`
		CompletionTokens     int `json:"completion_tokens"`
		PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
	}
	json.Unmarshal(raw, &u)
	return u.PromptTokens, u.CompletionTokens, u.PromptCacheHitTokens, 0
}
