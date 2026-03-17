package provider

func init() { Register(&anthropicProvider{}) }

type anthropicProvider struct{}

func (a *anthropicProvider) Name() string      { return "anthropic" }
func (a *anthropicProvider) Domains() []string { return []string{"api.anthropic.com"} }
func (a *anthropicProvider) Formats() []Format { return []Format{AnthropicMessages} }
