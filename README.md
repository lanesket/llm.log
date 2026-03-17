<p align="center">
  <h1 align="center">llm.log</h1>
  <p align="center">All your LLM API calls in one place. Tokens, costs, prompts, responses.<br>Runs locally, single binary, zero config.</p>
</p>

<p align="center">
  <a href="#install">Install</a> · <a href="#quick-start">Quick Start</a> · <a href="#dashboard">Dashboard</a> · <a href="#cli">CLI</a> · <a href="#how-it-works">How it Works</a>
</p>

---

<p align="center">
  <img src="assets/demo.gif" alt="llm.log demo" width="600">
</p>

## What is llm.log?

A local proxy that sits between your apps and LLM APIs. It intercepts requests, extracts token usage, calculates costs, and stores everything in SQLite — without changing a single line of code.

- **Zero code changes** — works via `HTTPS_PROXY`, picked up by most apps automatically
- **All providers** — OpenAI, Anthropic, OpenRouter — easy to add more
- **All API formats** — Chat Completions, Responses API, Anthropic Messages
- **Real costs** — auto-updated pricing for 780+ models, cache token breakdowns
- **Claude Code aware** — on a subscription? see what you'd pay without it. On API keys? see your actual spend
- **TUI dashboard** — overview, charts, cost breakdown, request inspector
- **Minimal overhead** — logging is async and never blocks your requests
- **Single binary** — pure Go, no CGO, no dependencies

## Install

```bash
# macOS / Linux (Homebrew)
brew install lanesket/tap/llm-log

# or with Go
go install github.com/lanesket/llm.log/cmd/llm-log@latest

# or from source
git clone https://github.com/lanesket/llm.log.git && cd llm.log && make build
```

Pre-built binaries for macOS and Linux on the [Releases](https://github.com/lanesket/llm.log/releases) page.

## Quick Start

```bash
llm-log setup   # one-time: generate CA cert, trust it, configure shell
llm-log start   # start the proxy
llm-log dash    # open the dashboard
```

After setup, **open a new terminal** (or run `source ~/.zshrc`) — then every LLM API call is logged automatically.

> **Already running apps need a restart** to pick up the proxy.
> On macOS, new apps from Dock pick it up automatically.

> **Note:** llm.log intercepts requests that go directly from your machine to LLM APIs.
> Tools that route through their own servers (Cursor Pro, VS Code Copilot with built-in subscription) won't be logged.
> If the tool supports your own API key, requests go directly to the provider and llm.log captures them.

## Dashboard

```bash
llm-log dashboard   # or: llm-log dash
```

| Tab | What it shows |
|-----|---------------|
| **Overview** | Total spend, request count, cache hit rate, sparkline, top models |
| **Chart** | Cumulative cost, requests, tokens, cache hit rate over time |
| **Cost** | Breakdown by provider/model with percentages, latency, bars |
| **Requests** | Browse requests, inspect full prompt/response JSON |

**Keys:** `1-4` tabs · `p` period · `s` source · `f` provider · `m` model/provider · `j/k` navigate · `enter` detail · `c/p/r` copy · `?` help · `q` quit

## CLI

```bash
llm-log status                    # daemon status + today's summary
llm-log logs                      # recent requests
llm-log logs --id 42              # full detail with prompt/response
llm-log logs -s cc:sub            # filter by source
llm-log stats                     # usage stats by provider
llm-log stats -b model -p week   # by model, last week
llm-log stats --json              # JSON output
```

## How it Works

```
Your app ──HTTPS_PROXY──▸ llm.log (127.0.0.1:9922)
                              │
                              ├─ LLM provider? ──▸ MITM ──▸ parse usage ──▸ SQLite
                              └─ Other?         ──▸ tunnel through (no interception)
```

1. `llm-log setup` generates a CA certificate and adds it to your system trust store
2. `llm-log start` launches a daemon and sets `HTTPS_PROXY` + CA env vars for all major tools
3. The proxy MITMs only known LLM domains — everything else tunnels untouched
4. Streaming responses are tee'd — client gets data in real-time, proxy parses accumulated result
5. Costs are calculated from auto-updated pricing data (780+ models)

### Providers and formats

| Provider | Domain | API formats |
|----------|--------|-------------|
| OpenAI | api.openai.com | Chat Completions, Responses API |
| Anthropic | api.anthropic.com | Anthropic Messages |
| OpenRouter | openrouter.ai | All three |

Both providers and API formats are extensible — each is a single file. See `internal/provider/` for examples.

### Proxy activation

| Mechanism | Scope | Platform |
|-----------|-------|----------|
| `~/.llm.log/env` | New terminal sessions | macOS, Linux |
| `launchctl setenv` | GUI apps from Dock | macOS |
| `systemctl --user` | GUI apps from menu | Linux (systemd) |

CA trust is configured for Node.js, Python, curl, Go, and Ruby via `NODE_EXTRA_CA_CERTS`, `SSL_CERT_FILE`, `REQUESTS_CA_BUNDLE`, and `CURL_CA_BUNDLE`.

### Data storage

Everything in `~/.llm.log/` — SQLite (WAL mode), CA cert, cached pricing, env file, PID. Request/response bodies are gzip-compressed in a separate table.

## License

MIT
