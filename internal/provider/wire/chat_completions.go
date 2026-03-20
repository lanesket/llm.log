package wire

import (
	"encoding/json"
	"strings"
)

// ChatCompletions parses the OpenAI Chat Completions format.
// Used by: OpenAI (/v1/chat/completions), OpenRouter, and any OpenAI-compatible API.
// Spec: https://platform.openai.com/docs/api-reference/chat/create
var ChatCompletions Format = &chatCompletions{}

type chatCompletions struct{}

func (c *chatCompletions) MatchPath(path string) bool {
	return matchPath(path, "/chat/completions")
}

func (c *chatCompletions) ModifyRequest(body []byte) ([]byte, error) {
	return injectStreamUsage(body)
}

func (c *chatCompletions) Parse(body []byte) (*Result, error) {
	return parseCCResponse(body, openaiUsage)
}

func (c *chatCompletions) ParseStream(events []SSEEvent) (*Result, error) {
	return parseCCStream(events, openaiUsage)
}

// openaiUsage maps the standard OpenAI usage fields.
// Returns zeros for nil/invalid input — callers handle empty results.
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

// usageMapper extracts token counts from a raw usage JSON object.
// Each wire format provides its own mapper to handle provider-specific field names.
type usageMapper func(raw json.RawMessage) (input, output, cacheRead, cacheWrite int)

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

	for _, ev := range events {
		if string(ev.Data) == "[DONE]" {
			continue
		}
		var chunk struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage json.RawMessage `json:"usage"`
		}
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
// Shared by ChatCompletions and its derivatives.
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
