package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://docs.mistral.ai/api/#tag/chat/operation/chat_completion_v1_chat_completions_post

func init() { Register(&mistralProvider{}) }

type mistralProvider struct{}

func (m *mistralProvider) Name() string           { return "mistral" }
func (m *mistralProvider) Domains() []string      { return []string{"api.mistral.ai"} }
func (m *mistralProvider) Formats() []wire.Format { return []wire.Format{wire.ChatCompletions} }
