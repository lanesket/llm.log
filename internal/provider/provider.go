package provider

import "strings"

// Format handles parsing for a specific LLM API wire format.
// Each format (Chat Completions, Responses API, Anthropic Messages)
// is implemented once and reused across providers.
type Format interface {
	MatchPath(path string) bool
	ModifyRequest(body []byte) ([]byte, error)
	Parse(statusCode int, body []byte) (*Result, error)
	ParseStream(events []SSEEvent) (*Result, error)
}

// Provider maps a domain to its supported API formats.
// Formats are listed most-specific first; the last one is the default fallback.
type Provider interface {
	Name() string
	Domains() []string
	Formats() []Format
}

// ResolveFormat finds the matching format for a request path.
// Falls back to the provider's last (default) format.
func ResolveFormat(p Provider, path string) Format {
	for _, f := range p.Formats() {
		if f.MatchPath(path) {
			return f
		}
	}
	fmts := p.Formats()
	return fmts[len(fmts)-1]
}

// Result holds parsed usage data from an API response.
// InputTokens is the total input tokens INCLUDING all cache tokens.
type Result struct {
	Model            string
	InputTokens      int // total: uncached + cache read + cache write
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ResponseBody     []byte
}

// SSEEvent is a single server-sent event from a streaming response.
type SSEEvent struct {
	Event string
	Data  []byte
}

var providers = map[string]Provider{}

func Register(p Provider) {
	for _, d := range p.Domains() {
		providers[d] = p
	}
}

func Lookup(domain string) (Provider, bool) {
	p, ok := providers[domain]
	return p, ok
}

func matchPath(path, suffix string) bool {
	return strings.HasSuffix(path, suffix) || strings.Contains(path, suffix+"/")
}
