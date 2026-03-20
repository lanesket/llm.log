package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://console.groq.com/docs/api-reference#chat-create

func init() { Register(&groqProvider{}) }

type groqProvider struct{}

func (g *groqProvider) Name() string           { return "groq" }
func (g *groqProvider) Domains() []string      { return []string{"api.groq.com"} }
func (g *groqProvider) Formats() []wire.Format { return []wire.Format{wire.ChatCompletions} }
