package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://platform.openai.com/docs/api-reference/chat

func init() { Register(&openaiProvider{}) }

type openaiProvider struct{}

func (o *openaiProvider) Name() string      { return "openai" }
func (o *openaiProvider) Domains() []string { return []string{"api.openai.com"} }
func (o *openaiProvider) Formats() []wire.Format {
	return []wire.Format{wire.Responses, wire.ChatCompletions}
}
