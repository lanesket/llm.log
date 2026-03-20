package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://openrouter.ai/docs/api/reference/overview

func init() { Register(&openrouterProvider{}) }

// openrouterProvider supports all three API formats on one domain.
type openrouterProvider struct{}

func (o *openrouterProvider) Name() string      { return "openrouter" }
func (o *openrouterProvider) Domains() []string { return []string{"openrouter.ai"} }
func (o *openrouterProvider) Formats() []wire.Format {
	return []wire.Format{wire.AnthropicMessages, wire.Responses, wire.ChatCompletions}
}
