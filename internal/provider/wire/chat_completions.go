package wire

import (
	"encoding/json"
	"strings"
)

// ChatCompletions parses the OpenAI Chat Completions format.
// Used by: OpenAI (/v1/chat/completions), OpenRouter, and any OpenAI-compatible API.
// Spec: https://platform.openai.com/docs/api-reference/chat/create
var ChatCompletions Format = NewCCFormat("/chat/completions", openaiUsage)

// DeepSeekChatCompletions extends Chat Completions with DeepSeek-specific
// cache token fields (prompt_cache_hit_tokens instead of prompt_tokens_details.cached_tokens).
// Spec: https://api-docs.deepseek.com/api/create-chat-completion
var DeepSeekChatCompletions Format = NewCCFormat("/chat/completions", deepseekUsage)

// PerplexitySonar parses the Perplexity Sonar API format.
// Structurally identical to Chat Completions but uses /sonar endpoint path.
// Spec: https://docs.perplexity.ai/api-reference/sonar-post
var PerplexitySonar Format = NewCCFormat("/sonar", openaiUsage)

// NewCCFormat creates a Chat Completions-compatible Format with a custom
// path suffix and usage mapper. This is the extension point for adding
// new providers that use Chat Completions with different usage fields.
func NewCCFormat(pathSuffix string, mapUsage usageMapper) Format {
	return &ccFormat{pathSuffix: pathSuffix, mapUsage: mapUsage}
}

type ccFormat struct {
	pathSuffix string
	mapUsage   usageMapper
}

func (f *ccFormat) MatchPath(path string) bool {
	return matchPath(path, f.pathSuffix)
}

func (f *ccFormat) ModifyRequest(body []byte) ([]byte, error) {
	return injectStreamUsage(body)
}

func (f *ccFormat) Parse(body []byte) (*Result, error) {
	return parseCCResponse(body, f.mapUsage)
}

func (f *ccFormat) ParseStream(events []SSEEvent) (*Result, error) {
	return parseCCStream(events, f.mapUsage)
}

// usageMapper extracts token counts from a raw usage JSON object.
// Each wire format provides its own mapper to handle provider-specific field names.
// Returns zeros for nil/invalid input — callers handle empty results.
type usageMapper func(raw json.RawMessage) (input, output, cacheRead, cacheWrite int)

// openaiUsage maps the standard OpenAI usage fields.
func openaiUsage(raw json.RawMessage) (input, output, cacheRead, cacheWrite int) {
	var u struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
	json.Unmarshal(raw, &u)
	return u.PromptTokens, u.CompletionTokens, u.PromptTokensDetails.CachedTokens, 0
}

// deepseekUsage maps DeepSeek's cache fields (prompt_cache_hit_tokens).
func deepseekUsage(raw json.RawMessage) (input, output, cacheRead, cacheWrite int) {
	var u struct {
		PromptTokens         int `json:"prompt_tokens"`
		CompletionTokens     int `json:"completion_tokens"`
		PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
	}
	json.Unmarshal(raw, &u)
	return u.PromptTokens, u.CompletionTokens, u.PromptCacheHitTokens, 0
}

// parseCCResponse parses a non-streaming Chat Completions-style response.
func parseCCResponse(body []byte, mapUsage usageMapper) (*Result, error) {
	var resp struct {
		Model string          `json:"model"`
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	input, output, cacheRead, cacheWrite := mapUsage(resp.Usage)
	return &Result{
		Model:            resp.Model,
		InputTokens:      input,
		OutputTokens:     output,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
		ResponseBody:     body,
	}, nil
}

// parseCCStream parses a streaming Chat Completions-style response.
func parseCCStream(events []SSEEvent, mapUsage usageMapper) (*Result, error) {
	var result Result
	var content strings.Builder
	var chunk struct {
		Model   string `json:"model"`
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage json.RawMessage `json:"usage"`
	}

	for _, ev := range events {
		if string(ev.Data) == "[DONE]" {
			continue
		}
		chunk.Model = ""
		chunk.Choices = chunk.Choices[:0]
		chunk.Usage = chunk.Usage[:0]
		if json.Unmarshal(ev.Data, &chunk) != nil {
			continue
		}
		if result.Model == "" && chunk.Model != "" {
			result.Model = chunk.Model
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}
		if len(chunk.Usage) > 0 {
			input, output, cacheRead, cacheWrite := mapUsage(chunk.Usage)
			if input > 0 || output > 0 {
				result.InputTokens = input
				result.OutputTokens = output
				result.CacheReadTokens = cacheRead
				result.CacheWriteTokens = cacheWrite
			}
		}
	}

	reconstructed, _ := json.Marshal(map[string]any{
		"model":   result.Model,
		"content": content.String(),
	})
	result.ResponseBody = reconstructed
	return &result, nil
}

// injectStreamUsage adds stream_options.include_usage to streaming requests.
// Shared by all Chat Completions-compatible formats.
func injectStreamUsage(body []byte) ([]byte, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return body, nil
	}
	if stream, ok := req["stream"].(bool); !ok || !stream {
		return body, nil
	}
	opts, _ := req["stream_options"].(map[string]any)
	if opts == nil {
		opts = map[string]any{}
	}
	opts["include_usage"] = true
	req["stream_options"] = opts
	modified, err := json.Marshal(req)
	if err != nil {
		return body, nil
	}
	return modified, nil
}
