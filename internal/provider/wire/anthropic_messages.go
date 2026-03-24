package wire

import (
	"encoding/json"
	"strings"
)

// AnthropicMessages parses the Anthropic Messages API format.
// Used by: Anthropic (/v1/messages), OpenRouter.
// Anthropic reports input_tokens as the uncached portion only,
// so total = input_tokens + cache_read + cache_creation.
// Spec: https://platform.claude.com/docs/en/api/messages
var AnthropicMessages Format = &anthropicMessages{}

type anthropicMessages struct{}

func (a *anthropicMessages) MatchPath(path string) bool {
	return matchPath(path, "/messages")
}

func (a *anthropicMessages) ModifyRequest(body []byte) ([]byte, error) {
	return body, nil
}

func (a *anthropicMessages) Parse(body []byte) (*Result, error) {
	var resp struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int    `json:"input_tokens"`
			OutputTokens             int    `json:"output_tokens"`
			CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
			Speed                    string `json:"speed"`
			ServerToolUse            struct {
				WebSearchRequests int `json:"web_search_requests"`
			} `json:"server_tool_use"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	u := resp.Usage
	return &Result{
		Model:             resp.Model,
		InputTokens:       u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		OutputTokens:      u.OutputTokens,
		CacheReadTokens:   u.CacheReadInputTokens,
		CacheWriteTokens:  u.CacheCreationInputTokens,
		WebSearchRequests: u.ServerToolUse.WebSearchRequests,
		FastMode:          u.Speed == "fast",
		ResponseBody:      body,
	}, nil
}

// ParseStream extracts usage from accumulated SSE events.
// Anthropic sends:
//   - message_start → model, input_tokens, cache tokens
//   - content_block_delta → text content
//   - message_delta → output_tokens
func (a *anthropicMessages) ParseStream(events []SSEEvent) (*Result, error) {
	var result Result
	var content strings.Builder

	for _, ev := range events {
		switch ev.Event {
		case "message_start":
			var msg struct {
				Message struct {
					Model string `json:"model"`
					Usage struct {
						InputTokens              int    `json:"input_tokens"`
						CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
						CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
						Speed                    string `json:"speed"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(ev.Data, &msg) == nil {
				u := msg.Message.Usage
				result.Model = msg.Message.Model
				result.CacheReadTokens = u.CacheReadInputTokens
				result.CacheWriteTokens = u.CacheCreationInputTokens
				result.InputTokens = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
				if u.Speed == "fast" {
					result.FastMode = true
				}
			}

		case "content_block_delta":
			var delta struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if json.Unmarshal(ev.Data, &delta) == nil && delta.Delta.Type == "text_delta" {
				content.WriteString(delta.Delta.Text)
			}

		case "message_delta":
			var delta struct {
				Usage struct {
					OutputTokens  int    `json:"output_tokens"`
					Speed         string `json:"speed"`
					ServerToolUse struct {
						WebSearchRequests int `json:"web_search_requests"`
					} `json:"server_tool_use"`
				} `json:"usage"`
			}
			if json.Unmarshal(ev.Data, &delta) == nil {
				result.OutputTokens = delta.Usage.OutputTokens
				if delta.Usage.Speed == "fast" {
					result.FastMode = true
				}
				if delta.Usage.ServerToolUse.WebSearchRequests > 0 {
					result.WebSearchRequests = delta.Usage.ServerToolUse.WebSearchRequests
				}
			}
		}
	}

	result.ResponseBody = reconstructStreamBody(result.Model, content.String())
	return &result, nil
}
