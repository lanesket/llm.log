package wire

import (
	"encoding/json"
	"strings"
)

// Responses parses the OpenAI Responses API format.
// Used by: OpenAI (/v1/responses), OpenRouter.
// Streaming uses response.output_text.delta for content and
// response.completed for the final response with usage.
// Spec: https://platform.openai.com/docs/api-reference/responses/create
var Responses Format = &responsesFormat{}

type responsesFormat struct{}

func (r *responsesFormat) MatchPath(path string) bool {
	return matchPath(path, "/responses")
}

// ModifyRequest — Responses API includes usage in response.completed by default.
func (r *responsesFormat) ModifyRequest(body []byte) ([]byte, error) {
	return body, nil
}

func (r *responsesFormat) Parse(body []byte) (*Result, error) {
	var resp responsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &Result{
		Model:           resp.Model,
		InputTokens:     resp.Usage.InputTokens,
		OutputTokens:    resp.Usage.OutputTokens,
		CacheReadTokens: resp.Usage.InputTokenDetails.CachedTokens,
		ResponseBody:    body,
	}, nil
}

func (r *responsesFormat) ParseStream(events []SSEEvent) (*Result, error) {
	var result Result
	var content strings.Builder

	for _, ev := range events {
		switch ev.Event {
		case "response.output_text.delta":
			var delta struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(ev.Data, &delta) == nil {
				content.WriteString(delta.Delta)
			}

		case "response.completed":
			var completed struct {
				Response struct {
					Model string         `json:"model"`
					Usage responsesUsage `json:"usage"`
				} `json:"response"`
			}
			if json.Unmarshal(ev.Data, &completed) == nil {
				result.Model = completed.Response.Model
				result.InputTokens = completed.Response.Usage.InputTokens
				result.OutputTokens = completed.Response.Usage.OutputTokens
				result.CacheReadTokens = completed.Response.Usage.InputTokenDetails.CachedTokens
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

type responsesUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	InputTokenDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_token_details"`
}

type responsesResponse struct {
	Model string         `json:"model"`
	Usage responsesUsage `json:"usage"`
}
