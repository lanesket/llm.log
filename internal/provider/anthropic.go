package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://docs.anthropic.com/en/api/messages

func init() { Register(&anthropicProvider{}) }

type anthropicProvider struct{}

func (a *anthropicProvider) Name() string           { return "anthropic" }
func (a *anthropicProvider) Domains() []string      { return []string{"api.anthropic.com"} }
func (a *anthropicProvider) Formats() []wire.Format { return []wire.Format{wire.AnthropicMessages} }
