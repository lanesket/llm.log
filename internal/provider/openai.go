package provider

func init() { Register(&openaiProvider{}) }

type openaiProvider struct{}

func (o *openaiProvider) Name() string      { return "openai" }
func (o *openaiProvider) Domains() []string { return []string{"api.openai.com"} }
func (o *openaiProvider) Formats() []Format { return []Format{Responses, ChatCompletions} }
