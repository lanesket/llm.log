package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://docs.fireworks.ai/api-reference/post-chatcompletions

func init() { Register(&fireworksProvider{}) }

type fireworksProvider struct{}

func (f *fireworksProvider) Name() string           { return "fireworks" }
func (f *fireworksProvider) Domains() []string      { return []string{"api.fireworks.ai"} }
func (f *fireworksProvider) Formats() []wire.Format { return []wire.Format{wire.ChatCompletions} }
