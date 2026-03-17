package provider

import (
	"encoding/json"
	"strings"
)

// ChatCompletions parses the OpenAI Chat Completions format.
// Used by: OpenAI (/v1/chat/completions), OpenRouter, and any OpenAI-compatible API.
var ChatCompletions Format = &chatCompletions{}

type chatCompletions struct{}

func (c *chatCompletions) MatchPath(path string) bool {
	return matchPath(path, "/chat/completions")
}

// ModifyRequest injects stream_options.include_usage for streaming requests
// so the final SSE chunk includes token usage.
func (c *chatCompletions) ModifyRequest(body []byte) ([]byte, error) {
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

func (c *chatCompletions) Parse(statusCode int, body []byte) (*Result, error) {
	var resp struct {
		Model string `json:"model"`
		Usage ccUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &Result{
		Model:           resp.Model,
		InputTokens:     resp.Usage.PromptTokens,
		OutputTokens:    resp.Usage.CompletionTokens,
		CacheReadTokens: resp.Usage.PromptTokensDetails.CachedTokens,
		ResponseBody:    body,
	}, nil
}

func (c *chatCompletions) ParseStream(events []SSEEvent) (*Result, error) {
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
			Usage ccUsage `json:"usage"`
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
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			result.InputTokens = chunk.Usage.PromptTokens
			result.OutputTokens = chunk.Usage.CompletionTokens
			result.CacheReadTokens = chunk.Usage.PromptTokensDetails.CachedTokens
		}
	}

	reconstructed, _ := json.Marshal(map[string]any{
		"model":   result.Model,
		"content": content.String(),
	})
	result.ResponseBody = reconstructed
	return &result, nil
}

type ccUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}
