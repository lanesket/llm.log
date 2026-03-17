package pricing

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	sourceURL   = "https://raw.githubusercontent.com/pydantic/genai-prices/main/prices/data.json"
	cacheFile   = "prices.json"
	maxCacheAge = 24 * time.Hour
)

// Price holds per-token pricing for a model.
type Price struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheReadPerMTok  float64
	CacheWritePerMTok float64
}

// DB is the pricing database.
type DB struct {
	mu     sync.RWMutex
	prices map[string]*Price // key: model ID (e.g. "claude-sonnet-4-6")
	dir    string
}

// NewDB creates a pricing DB, loading from cache.
func NewDB(dataDir string) *DB {
	db := &DB{
		prices: make(map[string]*Price),
		dir:    dataDir,
	}
	db.loadCache()
	return db
}

// Normalize returns a canonical model name in "vendor/model" format.
// The vendor is extracted from the model string (e.g. "anthropic/claude-...")
// or falls back to the gateway name for direct API calls.
func (db *DB) Normalize(gateway, model string) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	vendor := gateway
	if i := strings.LastIndex(model, "/"); i >= 0 {
		vendor = model[:i]
	}

	if key := db.lookupKey(model); key != "" {
		return vendor + "/" + key
	}

	bare := model
	if i := strings.LastIndex(model, "/"); i >= 0 {
		bare = model[i+1:]
	}
	return vendor + "/" + bare
}

// Cost calculates the cost for a request. Returns nil if model not found.
func (db *DB) Cost(providerName, model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) *float64 {
	db.mu.RLock()
	defer db.mu.RUnlock()

	key := db.lookupKey(model)
	if key == "" {
		return nil
	}
	p := db.prices[key]

	uncached := float64(max(0, inputTokens-cacheReadTokens-cacheWriteTokens))
	cost := uncached*p.InputPerMTok/1_000_000 +
		float64(outputTokens)*p.OutputPerMTok/1_000_000 +
		float64(cacheReadTokens)*p.CacheReadPerMTok/1_000_000 +
		float64(cacheWriteTokens)*p.CacheWritePerMTok/1_000_000
	return &cost
}

// lookupKey finds the best matching pricing key for a model name.
// Strategies: exact → prefix → token set (handles reordered names like OpenRouter).
func (db *DB) lookupKey(model string) string {
	bare := model
	if i := strings.LastIndex(model, "/"); i >= 0 {
		bare = model[i+1:]
	}

	n := norm(bare)

	// Exact match (normalized)
	for key := range db.prices {
		if norm(key) == n {
			return key
		}
	}

	// Longest prefix match (normalized) for versioned models.
	// The character after the prefix must be a separator to avoid
	// e.g. "gpt-4o" matching "gpt-4" pricing.
	var best string
	var bestLen int
	for key := range db.prices {
		nk := norm(key)
		if len(n) > len(nk) && n[:len(nk)] == nk && n[len(nk)] == '-' && len(nk) > bestLen {
			best = key
			bestLen = len(nk)
		}
	}
	if best != "" {
		return best
	}

	// Token set match: all tokens of pricing key must appear in model tokens.
	// Picks the key with the most tokens to avoid short false matches.
	modelToks := tokenize(n)
	for key := range db.prices {
		keyToks := tokenize(norm(key))
		if len(keyToks) > bestLen && subset(keyToks, modelToks) {
			best = key
			bestLen = len(keyToks)
		}
	}
	return best
}

var normReplacer = strings.NewReplacer(".", "-", "_", "-")

// norm canonicalizes model name separators for fuzzy matching.
func norm(s string) string {
	return normReplacer.Replace(s)
}

func tokenize(s string) map[string]bool {
	m := make(map[string]bool)
	for _, t := range strings.Split(s, "-") {
		if t != "" {
			m[t] = true
		}
	}
	return m
}

func subset(sub, super map[string]bool) bool {
	for k := range sub {
		if !super[k] {
			return false
		}
	}
	return true
}

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
	},
}

// Update fetches fresh pricing data from the source.
func (db *DB) Update() error {
	log.Printf("fetching pricing data from %s", sourceURL)

	resp, err := httpClient.Get(sourceURL)
	if err != nil {
		return fmt.Errorf("fetch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("fetch prices: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read prices: %w", err)
	}

	if err := db.parse(data); err != nil {
		return err
	}

	cachePath := filepath.Join(db.dir, cacheFile)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		log.Printf("warning: failed to cache pricing data: %v", err)
	}

	log.Printf("loaded %d model prices", len(db.prices))
	return nil
}

// UpdateIfStale updates pricing if cache is older than maxCacheAge.
func (db *DB) UpdateIfStale() {
	cachePath := filepath.Join(db.dir, cacheFile)
	info, err := os.Stat(cachePath)
	if err != nil || time.Since(info.ModTime()) > maxCacheAge {
		if err := db.Update(); err != nil {
			log.Printf("warning: price update failed: %v", err)
		}
	}
}

// StartAutoUpdate runs UpdateIfStale periodically in the background.
func (db *DB) StartAutoUpdate() {
	go func() {
		for range time.Tick(time.Hour) {
			db.UpdateIfStale()
		}
	}()
}

func (db *DB) loadCache() {
	cachePath := filepath.Join(db.dir, cacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return
	}
	if err := db.parse(data); err != nil {
		log.Printf("warning: corrupt price cache, will re-fetch: %v", err)
	}
}

func (db *DB) parse(data []byte) error {
	// genai-prices format: array of providers, each with models array
	// prices can be a number or {"base": N, "tiers": [...]}
	var providers []json.RawMessage
	if err := json.Unmarshal(data, &providers); err != nil {
		return fmt.Errorf("parse prices: %w", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.prices = make(map[string]*Price)

	for _, raw := range providers {
		var prov struct {
			Models []struct {
				ID     string `json:"id"`
				Prices struct {
					InputMTok      json.RawMessage `json:"input_mtok"`
					OutputMTok     json.RawMessage `json:"output_mtok"`
					CacheReadMTok  json.RawMessage `json:"cache_read_mtok"`
					CacheWriteMTok json.RawMessage `json:"cache_write_mtok"`
				} `json:"prices"`
			} `json:"models"`
		}
		if json.Unmarshal(raw, &prov) != nil {
			continue
		}

		for _, m := range prov.Models {
			input := extractPrice(m.Prices.InputMTok)
			output := extractPrice(m.Prices.OutputMTok)
			if input == 0 && output == 0 {
				continue
			}
			db.prices[m.ID] = &Price{
				InputPerMTok:      input,
				OutputPerMTok:     output,
				CacheReadPerMTok:  extractPrice(m.Prices.CacheReadMTok),
				CacheWritePerMTok: extractPrice(m.Prices.CacheWriteMTok),
			}
		}
	}
	return nil
}

// extractPrice handles both plain numbers and {"base": N, "tiers": [...]} objects.
func extractPrice(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	// Try plain number first
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	// Try tiered pricing — use base price
	var tiered struct {
		Base float64 `json:"base"`
	}
	if json.Unmarshal(raw, &tiered) == nil {
		return tiered.Base
	}
	return 0
}
