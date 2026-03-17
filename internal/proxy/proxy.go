package proxy

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/provider"
	"github.com/lanesket/llm.log/internal/storage"
)

// Proxy is the MITM proxy server.
type Proxy struct {
	server *http.Server
	store  storage.Store
	price  PriceLookup
}

// PriceLookup calculates cost and normalizes model names. Can be nil.
type PriceLookup interface {
	Cost(providerName, model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) *float64
	Normalize(gateway, model string) string
}

// New creates a new proxy server.
func New(addr, dataDir string, store storage.Store, price PriceLookup) (*Proxy, error) {
	tlsCert, err := LoadOrGenerateCA(dataDir)
	if err != nil {
		return nil, err
	}

	gp := goproxy.NewProxyHttpServer()
	gp.Verbose = false

	// Set CA for MITM cert generation
	goproxy.GoproxyCa = tlsCert

	// MITM for provider domains, passthrough for everything else
	gp.OnRequest().HandleConnectFunc(
		func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			if _, ok := provider.Lookup(hostWithoutPort(host)); ok {
				return goproxy.MitmConnect, host
			}
			return goproxy.OkConnect, host
		},
	)

	p := &Proxy{
		server: &http.Server{Addr: addr, Handler: gp},
		store:  store,
		price:  price,
	}

	isProvider := goproxy.ReqConditionFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		_, ok := provider.Lookup(hostWithoutPort(req.URL.Host))
		return ok
	})

	gp.OnRequest(isProvider).DoFunc(p.onRequest)
	gp.OnResponse(isProvider).DoFunc(p.onResponse)

	return p, nil
}

// ListenAndServe starts the proxy.
func (p *Proxy) ListenAndServe() error {
	log.Printf("proxy listening on %s", p.server.Addr)
	return p.server.ListenAndServe()
}

// Shutdown gracefully stops the proxy.
func (p *Proxy) Shutdown() error {
	return p.server.Close()
}

func (p *Proxy) onRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	prov, ok := provider.Lookup(hostWithoutPort(req.URL.Host))
	if !ok {
		return req, nil
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		log.Printf("error reading request body: %v", err)
		return req, nil
	}

	format := provider.ResolveFormat(prov, req.URL.Path)
	modified, err := format.ModifyRequest(body)
	if err != nil {
		log.Printf("warning: ModifyRequest failed for %s: %v", prov.Name(), err)
		modified = body
	}

	ctx.UserData = &requestState{
		provider:    prov,
		format:      format,
		requestBody: body,
		startTime:   time.Now(),
		endpoint:    req.URL.Path,
		source:      detectSource(req.Header),
	}

	req.Body = io.NopCloser(bytes.NewReader(modified))
	req.ContentLength = int64(len(modified))

	return req, nil
}

func (p *Proxy) onResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	state, ok := ctx.UserData.(*requestState)
	if !ok || state == nil {
		return resp
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		// Tee: client reads streaming chunks in real-time, we accumulate for parsing
		statusCode := resp.StatusCode
		resp.Body = &teeReadCloser{
			rc: resp.Body,
			done: func(raw []byte) {
				events := ParseSSE(raw)
				result, err := state.format.ParseStream(events)
				if err != nil {
					log.Printf("parse error (%s): %v", state.provider.Name(), err)
					return
				}
				p.save(state, statusCode, true, result)
			},
		}
		return resp
	}

	// Non-streaming: read, parse, forward
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Printf("error reading response: %v", err)
		return resp
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	result, err := state.format.Parse(resp.StatusCode, body)
	if err != nil {
		log.Printf("parse error (%s): %v", state.provider.Name(), err)
		return resp
	}
	p.save(state, resp.StatusCode, false, result)

	return resp
}

func (p *Proxy) save(state *requestState, statusCode int, streaming bool, result *provider.Result) {
	if result.Model == "" {
		return
	}
	duration := time.Since(state.startTime)

	model := result.Model
	var cost *float64
	if p.price != nil {
		model = p.price.Normalize(state.provider.Name(), model)
		cost = p.price.Cost(state.provider.Name(), model, result.InputTokens, result.OutputTokens, result.CacheReadTokens, result.CacheWriteTokens)
	}

	rec := &storage.Record{
		Timestamp:        state.startTime,
		Provider:         state.provider.Name(),
		Model:            model,
		Endpoint:         state.endpoint,
		Source:           state.source,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		CacheReadTokens:  result.CacheReadTokens,
		CacheWriteTokens: result.CacheWriteTokens,
		TotalCost:        cost,
		DurationMs:       int(duration.Milliseconds()),
		Streaming:        streaming,
		StatusCode:       statusCode,
		RequestBody:      state.requestBody,
		ResponseBody:     result.ResponseBody,
	}

	if err := p.store.Save(rec); err != nil {
		log.Printf("save error: %v", err)
		return
	}

	costStr := "n/a"
	if cost != nil {
		costStr = format.Cost(*cost)
	}
	log.Printf("%-10s %-25s %6d in / %6d out  %s",
		rec.Provider, rec.Model, rec.InputTokens, rec.OutputTokens, costStr)
}

type requestState struct {
	provider    provider.Provider
	format      provider.Format
	requestBody []byte
	startTime   time.Time
	endpoint    string
	source      string
}

// detectSource identifies the client from the User-Agent header.
//
// Returns:
//
//	"cc:sub" — Claude Code with subscription (OAuth)
//	"cc:key" — Claude Code with API key
//	"copilot" — GitHub Copilot (VS Code / JetBrains)
//	""       — unknown client
func detectSource(h http.Header) string {
	ua := strings.ToLower(h.Get("User-Agent"))

	// Claude Code
	if strings.HasPrefix(ua, "claude-code/") || strings.HasPrefix(ua, "claude-cli/") {
		if h.Get("x-api-key") != "" {
			return "cc:key"
		}
		return "cc:sub"
	}

	// GitHub Copilot (VS Code)
	if strings.HasPrefix(ua, "githubcopilot") {
		return "copilot:key"
	}

	return ""
}

// teeReadCloser copies all bytes read by the client into a buffer.
// When Close is called, it invokes the done callback with accumulated data exactly once.
type teeReadCloser struct {
	rc   io.ReadCloser
	buf  bytes.Buffer
	done func([]byte)
	once sync.Once
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 {
		t.buf.Write(p[:n])
	}
	return n, err
}

func (t *teeReadCloser) Close() error {
	err := t.rc.Close()
	t.once.Do(func() {
		if t.done != nil {
			t.done(t.buf.Bytes())
		}
	})
	return err
}

func hostWithoutPort(host string) string {
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}
