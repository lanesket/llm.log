# Changelog

## [0.4.2] — 2026-03-27

### Fixed
- **Empty database crash** — dashboard and analytics pages showed black screen when no data existed; nil slices now serialize as `[]` instead of JSON `null`

## [0.4.1] — 2026-03-24

### Added
- **OpenAI audio token pricing** — audio input/output tokens (GPT-4o audio) are now separated from text tokens and priced at their own rates ($100/$200 per MTok)
- **Anthropic web search costs** — `server_tool_use.web_search_requests` parsed from responses, billed at $10/1K searches
- **Anthropic fast mode (6x)** — detected from `usage.speed` in API response, applies 6x multiplier to all token costs
- **Anthropic data residency (1.1x)** — detected from `inference_geo: "us"` in request body
- **Anthropic 1-hour cache TTL** — detected from `cache_control.ttl: "1h"` in request (system, messages, tools), uses 2x input price instead of default 1.25x
- **Integration tests** — real API tests against OpenAI (Chat Completions, Responses, streaming, cache hits) and Anthropic via OpenRouter (Messages, streaming, cache write/read)
- **E2E pricing tests** — verified against live genai-prices data for all pricing scenarios

### Changed
- `Cost()` now takes `*wire.Result` + explicit `multiplier`/`cacheTTL1h` params instead of 6 separate int args
- `wire.Result` is purely response data — request-level modifiers are passed separately through the proxy layer
- Stream body reconstruction extracted to shared `reconstructStreamBody()` helper

### Fixed
- **Fast mode detection in streaming** — `usage.speed` is now read from both `message_start` and `message_delta` SSE events
- **Cache TTL false positives** — `"1h"` in message content no longer triggers 1h pricing; proper JSON parsing of `cache_control.ttl` fields

## [0.4.0] — 2026-03-22

### Added
- **Web UI** — `llm-log ui` opens a browser-based dashboard at `localhost:9923`
  - **Dashboard** — real-time metrics with animated counters, area charts, provider breakdown bars, top models table
  - **Requests** — paginated table with sorting, filtering, search (debounced), page size selector, click-to-open detail dialog
  - **Request Detail** — full-page view with copyable values (model, provider, endpoint, ID, metrics), two-column JSON viewer
  - **Analytics** — tabbed sections (Cost / Tokens / Performance) with summary strips, 10 chart types including heatmap
- **Chameleon mascot** — pixel-art pet that roams the UI, changes color to match the dominant provider, sleeps when proxy is off, reacts to clicks (pet / startle), has a terminal "home"
- **Click-to-copy** — model names, providers, endpoints, request IDs, metric values are all copyable with visual feedback
- **7 new providers** — Groq, Together AI, Fireworks, DeepSeek, Mistral, Perplexity, xAI
  - DeepSeek: custom cache token support (`prompt_cache_hit_tokens`)
  - Perplexity: Sonar API format (`/v1/sonar` endpoint)

### Changed
- **Wire format parsers moved to `provider/wire` subpackage** — clear separation between providers (domain → format mapping) and wire formats (response parsing)
- Deduplicated Chat Completions parsing via `usageMapper` callback — adding new OpenAI-compatible providers requires only a usage mapper function
- Removed unused `statusCode` parameter from `Format.Parse` interface
- **Analytics refactored** — split into 7 sub-methods with proper row scoping
- **Dashboard cache** — replaced unbounded map with single-entry cache
- **`timeFilter` helper** — extracted shared WHERE clause builder to `helpers.go`
- Code splitting — Dashboard and Analytics lazy-loaded (recharts deferred)

### Fixed
- **Error responses (4xx/5xx) are now tracked** — model name is recovered from the request body when the API response doesn't include it
- **Zero-time bug** — `DashboardStats` and `Analytics` now handle all-time queries correctly (IsZero check)
- **NULL cursor pagination** — COALESCE for nullable sort columns prevents skipped records
- **Proxy port** — unified constant, no more magic numbers

## [0.3.0] — 2026-03-19

### Added
- **Export command** — `llm-log export` outputs logged data as CSV, JSON, or JSONL for analysis in Excel, Jupyter, pandas, etc.
  - Flags: `--format`, `--period`, `--from/--to`, `--source`, `--provider`, `--output`, `--with-bodies`
- **Dashboard export** — press `e` to quick-export current filtered view to CSV
- **Prune command** — `llm-log prune --older-than 30d` deletes old request/response bodies while keeping metadata (tokens, costs, timestamps)
  - Flags: `--older-than` (required), `--dry-run`, `--force`
- **Shell completions** — Tab completion for all commands and flags
  - Automatic via Homebrew; `llm-log setup` for manual installs

## [0.2.0] — 2026-03-18

### Added
- Batched proxy saves to reduce SQLite write pressure

### Fixed
- Homebrew formula name in goreleaser config

## [0.1.0] — 2026-03-17

Initial release.

- Local MITM proxy for OpenAI, Anthropic, OpenRouter APIs
- Auto-updated pricing for 780+ models
- TUI dashboard with overview, charts, cost breakdown, request inspector
- CLI commands: `status`, `logs`, `stats`
- Claude Code source detection (`cc:sub`, `cc:key`)
- Gzip-compressed request/response body storage
- One-command setup: `llm-log setup && llm-log start`
