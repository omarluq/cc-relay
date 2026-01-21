# Phase 1: Core Proxy (MVP) - Context

**Gathered:** 2026-01-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish working proxy that accepts Claude Code requests, routes to Anthropic, preserves tool_use_id, handles SSE streaming correctly, and validates API keys. This is the foundational MVP - multi-provider routing, pooling, and advanced features come in later phases.

</domain>

<decisions>
## Implementation Decisions

### SSE Streaming
- **Flush after every event** - Guarantees real-time delivery with no buffering delays visible to user
- Event sequence must match Anthropic's exactly: message_start → content_block_start → content_block_delta → content_block_stop → message_delta → message_stop
- Use `http.Flusher` interface to flush immediately after writing each SSE event

### Error Handling & Responses
- **Return exact Anthropic error format** - Proxy is transparent, clients shouldn't know they're not talking to Anthropic
- **Retry on 5xx errors only** - Server errors get retried, client errors (4xx) do not
- Status codes: 401 (invalid auth), 429 (rate limit), 504 (timeout), 5xx (backend errors)
- Error responses match Anthropic's JSON structure with `type`, `error.type`, `error.message`

### Claude's Discretion
- **Connection handling**: How to handle SSE connection breaks mid-stream (close cleanly vs error signal)
- **Event ordering**: Whether to validate SSE event sequence from upstream or trust provider
- **Extended thinking blocks**: Pass through unchanged vs strip if unsupported
- **Timeout status codes**: Choose between 504 Gateway Timeout vs 408 Request Timeout
- **Partial streaming failures**: Log and close vs inject error event into SSE stream
- **tool_use_id preservation**: Pure passthrough (never touch/validate) vs format validation
- **Header forwarding**: Allowlist specific headers vs forward all client headers
- **Request body validation**: No validation (let upstream handle) vs basic field checks
- **anthropic_version handling**: Forward client value vs override with proxy default
- **API key validation timing**: Before routing (fail fast) vs during provider selection
- **Placeholder keys**: Accept any client key vs require specific placeholder value
- **Upstream auth strategy**: Use configured keys from config vs forward client key
- **Auth error messages**: Generic 401 vs descriptive error explaining what's wrong

</decisions>

<specifics>
## Specific Ideas

- Success criteria explicitly requires: "Parallel tool calls preserve tool_use_id correctly (no orphan tool_result errors)"
- Success criteria: "SSE streaming works with real-time event delivery (no buffering delays visible to user)"
- Success criteria: "Invalid API keys return 401 errors before hitting backend providers"
- SSE headers required: `Content-Type: text/event-stream`, `Cache-Control: no-cache, no-transform`, `X-Accel-Buffering: no`, `Connection: keep-alive`
- Proxy must be transparent - Claude Code shouldn't know it's not talking to Anthropic

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope

</deferred>

---

*Phase: 01-core-proxy*
*Context gathered: 2026-01-20*
