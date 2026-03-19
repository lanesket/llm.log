# Changelog

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
