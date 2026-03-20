package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://docs.together.ai/reference/chat-completions

func init() { Register(&togetherProvider{}) }

type togetherProvider struct{}

func (t *togetherProvider) Name() string           { return "together" }
func (t *togetherProvider) Domains() []string      { return []string{"api.together.xyz"} }
func (t *togetherProvider) Formats() []wire.Format { return []wire.Format{wire.ChatCompletions} }
