package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/provider"
	"github.com/lanesket/llm.log/internal/provider/wire"
	"github.com/lanesket/llm.log/internal/storage"
)

const (
	saveBatchSize    = 50
	saveBatchTimeout = 200 * time.Millisecond
	saveQueueSize    = 512
	maxRetries       = 3
)

// Proxy is the MITM proxy server.
type Proxy struct {
	server       *http.Server
	store        storage.Store
	price        PriceLookup
	saveQueue    chan *storage.Record
	stop         chan struct{}
	stopped      chan struct{}
	batchTimeout time.Duration
}

// PriceLookup calculates cost and normalizes model names. Can be nil.
type PriceLookup interface {
	// Cost calculates cost from parsed usage data and request-level modifiers.
	// multiplier applies to all token costs (e.g. 6.0 for fast mode, 1.1 for data residency).
	// cacheTTL1h switches cache write rate from 1.25x to 2x input price.
	Cost(providerName, model string, result *wire.Result, multiplier float64, cacheTTL1h bool) *float64
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
		server:       &http.Server{Addr: addr, Handler: gp},
		store:        store,
		price:        price,
		saveQueue:    make(chan *storage.Record, saveQueueSize),
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
		batchTimeout: saveBatchTimeout,
	}
	go p.runBatcher()

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

// Shutdown gracefully stops the proxy, flushing any buffered records.
// It waits for in-flight requests to finish before signaling the batcher,
// ensuring no records are lost between server.Shutdown and the drain loop.
func (p *Proxy) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := p.server.Shutdown(ctx)
	close(p.stop)
	<-p.stopped
	return err
}

// runBatcher collects records from saveQueue and writes them to the store
// in batches — either when the batch is full or after a timeout.
// The timer starts only when the first record of a new batch arrives,
// guaranteeing a full batchTimeout window for every batch.
func (p *Proxy) runBatcher() {
	defer close(p.stopped)

	batch := make([]*storage.Record, 0, saveBatchSize)
	retries := 0
	timer := time.NewTimer(p.batchTimeout)
	timer.Stop() // idle until the first record arrives
	var timerC <-chan time.Time

	stopTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerC = nil
	}

	clearBatch := func() {
		for i := range batch {
			batch[i] = nil // release pointers so GC can reclaim RequestBody/ResponseBody
		}
		batch = batch[:0]
		retries = 0
	}

	rearmTimer := func() {
		timer.Reset(p.batchTimeout)
		timerC = timer.C
	}

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := p.store.SaveBatch(batch); err != nil {
			retries++
			if retries >= maxRetries {
				log.Printf("batch save failed after %d retries (%d records dropped): %v", retries, len(batch), err)
				clearBatch()
			} else {
				log.Printf("batch save error (%d records, retry %d/%d): %v", len(batch), retries, maxRetries, err)
				rearmTimer()
			}
			return
		}
		clearBatch()
	}

	for {
		select {
		case rec := <-p.saveQueue:
			batch = append(batch, rec)
			if len(batch) == 1 {
				// Start the window on the first record of a new batch.
				rearmTimer()
			}
			if len(batch) >= saveBatchSize {
				flush()
				if len(batch) == 0 {
					stopTimer()
				}
			}
		case <-timerC:
			flush()
		case <-p.stop:
			stopTimer()
			// Drain any records queued before shutdown.
			for {
				select {
				case rec := <-p.saveQueue:
					batch = append(batch, rec)
				default:
					// Final flush: retry up to maxRetries to avoid silent data loss.
					for attempt := 0; attempt < maxRetries; attempt++ {
						if len(batch) == 0 {
							return
						}
						if err := p.store.SaveBatch(batch); err != nil {
							log.Printf("shutdown flush error (attempt %d/%d, %d records): %v", attempt+1, maxRetries, len(batch), err)
							continue
						}
						clearBatch()
						return
					}
					log.Printf("shutdown: %d records lost after %d retries", len(batch), maxRetries)
					return
				}
			}
		}
	}
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
					p.save(state, statusCode, true, &wire.Result{ResponseBody: raw})
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

	result, err := state.format.Parse(body)
	if err != nil {
		log.Printf("parse error (%s): %v", state.provider.Name(), err)
		p.save(state, resp.StatusCode, false, &wire.Result{ResponseBody: body})
		return resp
	}
	p.save(state, resp.StatusCode, false, result)

	return resp
}

func (p *Proxy) save(state *requestState, statusCode int, streaming bool, result *wire.Result) {
	// Try to recover model from request body when response doesn't include it (e.g. error responses).
	if result.Model == "" {
		result.Model = extractModelFromRequest(state.requestBody)
	}
	if result.Model == "" {
		return
	}

	// Detect request-level pricing modifiers (Anthropic only).
	multiplier := 1.0
	var cacheTTL1h bool
	if state.provider.Name() == "anthropic" {
		multiplier, cacheTTL1h = detectAnthropicModifiers(state, result)
		// Fast mode: detected from response (usage.speed == "fast").
		// Ref: https://platform.claude.com/docs/en/build-with-claude/fast-mode
		if result.FastMode {
			multiplier *= 6.0
		}
	}

	duration := time.Since(state.startTime)

	model := result.Model
	var cost *float64
	if p.price != nil {
		model = p.price.Normalize(state.provider.Name(), model)
		cost = p.price.Cost(state.provider.Name(), model, result, multiplier, cacheTTL1h)
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

	costStr := "n/a"
	if cost != nil {
		costStr = format.Cost(*cost)
	}
	log.Printf("%-10s %-25s %6d in / %6d out  %s",
		rec.Provider, rec.Model, rec.InputTokens, rec.OutputTokens, costStr)

	select {
	case p.saveQueue <- rec:
	default:
		log.Printf("save queue full, record dropped")
	}
}

type requestState struct {
	provider    provider.Provider
	format      wire.Format
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

// detectAnthropicModifiers inspects the Anthropic request to determine pricing modifiers.
// Returns a multiplier (1.0 = none) and whether 1h cache TTL was used.
// Fast mode is detected separately from the response (usage.speed field).
//
// Detected modifiers:
//   - Data residency (1.1x): inference_geo set to "us" in request body
//     Ref: https://platform.claude.com/docs/en/about-claude/pricing#data-residency-pricing
//   - 1-hour cache TTL: any cache_control block in system or messages has ttl: "1h"
//     Ref: https://platform.claude.com/docs/en/about-claude/pricing#prompt-caching
func detectAnthropicModifiers(state *requestState, result *wire.Result) (multiplier float64, cacheTTL1h bool) {
	multiplier = 1.0

	// Data residency and cache TTL: detected from request body.
	// Use lightweight bytes.Contains guard to avoid full JSON parse on most requests.
	body := state.requestBody
	if len(body) > 0 {
		if bytes.Contains(body, []byte(`"inference_geo"`)) {
			var req struct {
				InferenceGeo string `json:"inference_geo"`
			}
			if json.Unmarshal(body, &req) == nil && req.InferenceGeo == "us" {
				multiplier *= 1.1
			}
		}

		// 1h cache TTL: scan for "ttl" field, then confirm via JSON parse.
		if result.CacheWriteTokens > 0 && bytes.Contains(body, []byte(`"ttl"`)) {
			cacheTTL1h = containsCacheTTL1h(body)
		}
	}

	return
}

// containsCacheTTL1h checks whether any cache_control block in the request
// has ttl:"1h". Checks both system and messages arrays.
func containsCacheTTL1h(body []byte) bool {
	type block struct {
		CacheControl struct {
			TTL string `json:"ttl"`
		} `json:"cache_control"`
	}

	var req struct {
		System   json.RawMessage `json:"system"`
		Messages []struct {
			Content json.RawMessage `json:"content"`
			block
		} `json:"messages"`
		Tools []block `json:"tools"`
	}
	if json.Unmarshal(body, &req) != nil {
		return false
	}

	// Check system blocks (can be string or array of content blocks).
	if len(req.System) > 0 {
		var blocks []block
		if json.Unmarshal(req.System, &blocks) == nil {
			for _, b := range blocks {
				if b.CacheControl.TTL == "1h" {
					return true
				}
			}
		}
	}

	// Check message-level cache_control.
	for _, m := range req.Messages {
		if m.CacheControl.TTL == "1h" {
			return true
		}
		var blocks []block
		if json.Unmarshal(m.Content, &blocks) == nil {
			for _, b := range blocks {
				if b.CacheControl.TTL == "1h" {
					return true
				}
			}
		}
	}

	// Check tool-level cache_control.
	for _, tool := range req.Tools {
		if tool.CacheControl.TTL == "1h" {
			return true
		}
	}

	return false
}

// extractModelFromRequest tries to get the model name from the request body.
// All major LLM APIs include "model" as a top-level field in the request JSON.
func extractModelFromRequest(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &req) == nil {
		return req.Model
	}
	return ""
}

func hostWithoutPort(host string) string {
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}
