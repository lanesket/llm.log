package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://api-docs.deepseek.com/api/create-chat-completion

func init() { Register(&deepseekProvider{}) }

type deepseekProvider struct{}

func (d *deepseekProvider) Name() string      { return "deepseek" }
func (d *deepseekProvider) Domains() []string { return []string{"api.deepseek.com"} }
func (d *deepseekProvider) Formats() []wire.Format {
	return []wire.Format{wire.DeepSeekChatCompletions}
}
