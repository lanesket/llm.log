package provider

func init() { Register(&openrouterProvider{}) }

// OpenRouter supports all three API formats on one domain.
type openrouterProvider struct{}

func (o *openrouterProvider) Name() string      { return "openrouter" }
func (o *openrouterProvider) Domains() []string { return []string{"openrouter.ai"} }
func (o *openrouterProvider) Formats() []Format { return []Format{AnthropicMessages, Responses, ChatCompletions} }
