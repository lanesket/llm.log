package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// API docs: https://docs.perplexity.ai/api-reference/sonar-post

func init() { Register(&perplexityProvider{}) }

type perplexityProvider struct{}

func (p *perplexityProvider) Name() string      { return "perplexity" }
func (p *perplexityProvider) Domains() []string { return []string{"api.perplexity.ai"} }
func (p *perplexityProvider) Formats() []wire.Format {
	return []wire.Format{wire.PerplexitySonar, wire.ChatCompletions}
}
