package wire

// PerplexitySonar parses the Perplexity Sonar API format.
// Structurally identical to Chat Completions but uses /sonar endpoint path.
// Spec: https://docs.perplexity.ai/api-reference/sonar-post
var PerplexitySonar Format = &perplexitySonar{}

type perplexitySonar struct{}

func (p *perplexitySonar) MatchPath(path string) bool {
	return matchPath(path, "/sonar")
}

func (p *perplexitySonar) ModifyRequest(body []byte) ([]byte, error) {
	return injectStreamUsage(body)
}

func (p *perplexitySonar) Parse(body []byte) (*Result, error) {
	return parseCCResponse(body, openaiUsage)
}

func (p *perplexitySonar) ParseStream(events []SSEEvent) (*Result, error) {
	return parseCCStream(events, openaiUsage)
}
