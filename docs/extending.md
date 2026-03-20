# Extending llm.log

## Architecture

```
provider/           — maps domains to wire formats
  groq.go             "api.groq.com" → wire.ChatCompletions
  deepseek.go         "api.deepseek.com" → wire.DeepSeekChatCompletions
  ...
  wire/             — parses API response formats
    chat_completions.go   OpenAI Chat Completions (shared by many providers)
    anthropic_messages.go Anthropic Messages
    responses.go          OpenAI Responses API
    ...
```

The proxy auto-discovers providers via `init()` → `Register()`. No wiring needed elsewhere.

## Adding a provider

### Scenario 1: OpenAI-compatible provider

Most providers use the standard Chat Completions format. One file, ~13 lines.

1. Check the provider's API docs — confirm it uses `/chat/completions` with standard `usage` fields (`prompt_tokens`, `completion_tokens`)
2. Create `internal/provider/<name>.go` — see `groq.go` as the simplest example
3. Add a test case to `TestAllProviders_Registration` in `providers_test.go`
4. Add the provider to the table in `README.md`

That's it. The `init()` call registers the domain, the proxy picks it up automatically.

### Scenario 2: Different usage fields

Some providers use Chat Completions but report tokens differently. Create a custom `usageMapper` — a function that extracts token counts from raw usage JSON.

1. Create `internal/provider/wire/<name>_chat_completions.go` with a `usageMapper` — see `deepseek_chat_completions.go` as an example
2. Create `internal/provider/<name>.go` pointing to your new format
3. Add tests in `wire/<name>_chat_completions_test.go`
4. Add to `providers_test.go` and `README.md`

The shared `parseCCResponse` / `parseCCStream` in `chat_completions.go` handle all JSON and SSE parsing — you only write the usage mapper.

### Scenario 3: Completely different API format

If the provider uses a non-Chat-Completions API (like Anthropic Messages), implement the full `wire.Format` interface defined in `wire/wire.go`.

See `wire/anthropic_messages.go` or `wire/responses.go` for complete examples.

## Adding a source (client detection)

The proxy identifies which tool made a request via the `User-Agent` header. This is stored as the `source` field and used for filtering in the dashboard and CLI.

Current sources: `cc:sub` (Claude Code subscription), `cc:key` (Claude Code API key), `copilot:key` (GitHub Copilot).

To add a new one, edit `detectSource` in `internal/proxy/proxy.go`. To find the User-Agent of a tool, check the proxy logs while using it.

## Testing

```bash
go test ./internal/provider/ ./internal/provider/wire/ ./internal/proxy/
```

For manual testing, start the proxy and send a request without an API key — the error response should be tracked:

```bash
llm-log start
curl --proxy http://127.0.0.1:9922 --cacert ~/.llm.log/ca.pem \
  https://<domain>/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"<model>","messages":[{"role":"user","content":"hi"}]}'
tail -5 ~/.llm.log/llm-log.log
```

## Conventions

- **Provider files:** `// API docs:` link to the provider's general API reference
- **Wire format files:** `// Spec:` link to the specific format specification
- **Naming:** provider file = provider name (`groq.go`), wire format file = format name (`deepseek_chat_completions.go`)
