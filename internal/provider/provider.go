package provider

import "github.com/lanesket/llm.log/internal/provider/wire"

// Provider maps a domain to its supported API formats.
// Formats are listed most-specific first; the last one is the default fallback.
type Provider interface {
	Name() string
	Domains() []string
	Formats() []wire.Format
}

// ResolveFormat finds the matching format for a request path.
// Falls back to the provider's last (default) format.
func ResolveFormat(p Provider, path string) wire.Format {
	fmts := p.Formats()
	for _, f := range fmts {
		if f.MatchPath(path) {
			return f
		}
	}
	return fmts[len(fmts)-1]
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
