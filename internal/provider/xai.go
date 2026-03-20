package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://docs.x.ai/docs/api-reference#chat-completions

func init() { Register(&xaiProvider{}) }

type xaiProvider struct{}

func (x *xaiProvider) Name() string           { return "xai" }
func (x *xaiProvider) Domains() []string      { return []string{"api.x.ai"} }
func (x *xaiProvider) Formats() []wire.Format { return []wire.Format{wire.ChatCompletions} }
