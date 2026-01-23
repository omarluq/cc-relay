# Requirements: cc-relay

**Defined:** 2026-01-20
**Core Value:** Access all models from all three providers (Anthropic, Z.AI, Ollama) in Claude Code and switch
between them seamlessly.

## v1 Requirements

Requirements for initial comprehensive build. Each maps to roadmap phases.

### API Compatibility

- [ ] **API-01**: Proxy implements exact `/v1/messages` endpoint matching Anthropic API format
- [ ] **API-02**: Proxy accepts and validates `x-api-key` header
- [ ] **API-03**: Proxy forwards all `anthropic-*` headers to backend providers
- [ ] **API-04**: Proxy transforms requests per provider (Bedrock SigV4, Vertex OAuth, Azure headers)
- [ ] **API-05**: Proxy transforms responses to match Anthropic format exactly
- [ ] **API-06**: Proxy preserves `tool_use_id` fields during request/response transformation
- [ ] **API-07**: Proxy handles parallel tool calls atomically (multiple tool blocks in single request)

### Streaming

- [ ] **SSE-01**: Proxy streams responses using Server-Sent Events with correct headers
- [ ] **SSE-02**: Proxy maintains exact event sequence (message_start → content_block_start → delta → stop)
- [ ] **SSE-03**: Proxy flushes each SSE event immediately (no buffering)
- [ ] **SSE-04**: Proxy sets required headers (Content-Type: text/event-stream, Cache-Control: no-cache)
- [ ] **SSE-05**: Proxy handles connection failures mid-stream gracefully
- [ ] **SSE-06**: Proxy supports extended thinking content blocks

### Provider Management

- [ ] **PROV-01**: Proxy connects to Anthropic provider with native API
- [ ] **PROV-02**: Proxy connects to Z.AI provider with Anthropic-compatible API
- [ ] **PROV-03**: Proxy connects to Ollama provider with local API
- [ ] **PROV-04**: Proxy connects to AWS Bedrock with SigV4 signing
- [ ] **PROV-05**: Proxy connects to Azure Foundry with API key or Entra ID
- [ ] **PROV-06**: Proxy connects to Vertex AI with Google OAuth tokens
- [ ] **PROV-07**: Proxy tracks health status per provider (healthy/degraded/down)
- [ ] **PROV-08**: Proxy performs periodic health checks on each provider

### Routing

- [ ] **ROUT-01**: Proxy implements round-robin routing strategy
- [ ] **ROUT-02**: Proxy implements shuffle routing strategy
- [ ] **ROUT-03**: Proxy implements failover routing with fallback chain
- [ ] **ROUT-04**: Proxy implements cost-based routing (cheapest provider wins)
- [ ] **ROUT-05**: Proxy implements latency-based routing (fastest provider wins)
- [ ] **ROUT-06**: Proxy implements model-based routing (route by model name pattern)
- [ ] **ROUT-07**: Proxy selects routing strategy from configuration
- [ ] **ROUT-08**: Proxy routes around unhealthy providers automatically

### Multi-Key Pooling

- [ ] **POOL-01**: Proxy accepts multiple API keys per provider in configuration
- [ ] **POOL-02**: Proxy tracks RPM (requests per minute) limit per key
- [ ] **POOL-03**: Proxy tracks TPM (tokens per minute) limit per key
- [ ] **POOL-04**: Proxy selects key with available capacity using sliding window
- [ ] **POOL-05**: Proxy returns 429 when all keys exhausted
- [ ] **POOL-06**: Proxy distributes load across keys fairly

### Circuit Breaker

- [ ] **CIRC-01**: Proxy implements CLOSED state (normal operation)
- [ ] **CIRC-02**: Proxy implements OPEN state (failing provider bypassed)
- [ ] **CIRC-03**: Proxy implements HALF-OPEN state (recovery probing)
- [ ] **CIRC-04**: Proxy transitions to OPEN after threshold failures (e.g., 5 in 10s)
- [ ] **CIRC-05**: Proxy transitions to HALF-OPEN after cooldown period (e.g., 30s)
- [ ] **CIRC-06**: Proxy transitions to CLOSED after successful recovery probe
- [ ] **CIRC-07**: Proxy tracks failure rate per provider (429s, 5xx, timeouts)

### Authentication

- [ ] **AUTH-01**: Proxy validates incoming `x-api-key` against configured value
- [ ] **AUTH-02**: Proxy returns 401 for missing or invalid API key
- [ ] **AUTH-03**: Proxy never exposes backend provider keys in responses
- [ ] **AUTH-04**: Proxy loads provider credentials from environment variables
- [ ] **AUTH-05**: Proxy supports credential rotation without restart

### Configuration

- [ ] **CONF-01**: Proxy loads configuration from YAML file
- [ ] **CONF-02**: Proxy loads configuration from TOML file
- [ ] **CONF-03**: Proxy expands environment variables in config (${VAR_NAME} syntax)
- [ ] **CONF-04**: Proxy validates configuration on startup
- [ ] **CONF-05**: Proxy supports hot-reload when config file changes
- [ ] **CONF-06**: Proxy fails fast on invalid configuration with clear error messages

### Models Endpoint

- [ ] **MOD-01**: Proxy exposes `/v1/models` endpoint
- [ ] **MOD-02**: Proxy lists all available models from all configured providers
- [ ] **MOD-03**: Proxy includes model metadata (provider, context window, capabilities)
- [ ] **MOD-04**: Proxy formats response matching Anthropic models API format

### Observability

- [ ] **OBS-01**: Proxy logs all requests with structured logging (JSON)
- [ ] **OBS-02**: Proxy includes request ID in all log entries
- [ ] **OBS-03**: Proxy tracks latency per provider
- [ ] **OBS-04**: Proxy exposes Prometheus `/metrics` endpoint
- [ ] **OBS-05**: Proxy emits counters for requests, errors, successes per provider
- [ ] **OBS-06**: Proxy emits histograms for latency per provider
- [ ] **OBS-07**: Proxy emits gauges for provider health status

### gRPC Management API

- [ ] **GRPC-01**: Proxy exposes gRPC service for management
- [ ] **GRPC-02**: Proxy streams real-time stats (requests/sec, latency, health)
- [ ] **GRPC-03**: Proxy accepts provider enable/disable commands
- [ ] **GRPC-04**: Proxy accepts key add/remove commands
- [ ] **GRPC-05**: Proxy accepts config reload commands
- [ ] **GRPC-06**: Proxy returns current configuration via gRPC

### TUI

- [ ] **TUI-01**: TUI connects to proxy daemon via gRPC
- [ ] **TUI-02**: TUI displays real-time request rate and latency per provider
- [ ] **TUI-03**: TUI displays provider health status with visual indicators
- [ ] **TUI-04**: TUI displays active routing strategy
- [ ] **TUI-05**: TUI displays key pool status (available capacity per key)
- [ ] **TUI-06**: TUI supports interactive provider enable/disable
- [ ] **TUI-07**: TUI supports config reload trigger

### CLI

- [ ] **CLI-01**: CLI supports `serve` command to start proxy daemon
- [ ] **CLI-02**: CLI supports `status` command to check proxy health
- [ ] **CLI-03**: CLI supports `tui` command to launch TUI
- [ ] **CLI-04**: CLI supports `config reload` command
- [ ] **CLI-05**: CLI accepts `--config` flag for custom config file path
- [ ] **CLI-06**: CLI accepts `--tui` flag to start TUI with daemon

## v2 Requirements

Deferred to future releases. Tracked but not in current roadmap.

### WebUI

- **WEB-01**: WebUI accessible via browser at http://localhost:8080
- **WEB-02**: WebUI displays real-time stats using grpc-web
- **WEB-03**: WebUI supports provider management
- **WEB-04**: WebUI supports config editing with validation

### Advanced Features

- **ADV-01**: Semantic caching with embedding-based similarity
- **ADV-02**: Request queueing with backpressure
- **ADV-03**: Provider weights and preferences
- **ADV-04**: Cost tracking and attribution per request
- **ADV-05**: Model mapping (generic names to provider-specific IDs)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature                      | Reason                                                                   |
| ---------------------------- | ------------------------------------------------------------------------ |
| Prompt transformation        | Violates transparency, breaks tool use, unpredictable behavior           |
| Response caching (exact)     | Non-deterministic LLM responses, low hit rate, storage bloat             |
| Built-in throttling          | Proxy pools limits, shouldn't create new ones                            |
| Request injection            | Violates transparency, breaks debugging                                  |
| Local model hosting          | Scope creep, Ollama handles this                                         |
| Multi-tenancy                | Different product, adds auth/database complexity                         |
| Response filtering           | Provider responsibility, proxy should pass through unchanged             |
| Model fine-tuning            | Different product category                                               |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status  |
| ----------- | ----- | ------- |
| API-01 | Phase 1 | Pending |
| API-02 | Phase 1 | Pending |
| API-03 | Phase 1 | Pending |
| API-04 | Phase 6 | Pending |
| API-05 | Phase 1 | Pending |
| API-06 | Phase 1 | Pending |
| API-07 | Phase 1 | Pending |
| SSE-01 | Phase 1 | Pending |
| SSE-02 | Phase 1 | Pending |
| SSE-03 | Phase 1 | Pending |
| SSE-04 | Phase 1 | Pending |
| SSE-05 | Phase 1 | Pending |
| SSE-06 | Phase 1 | Pending |
| PROV-01 | Phase 1 | Pending |
| PROV-02 | Phase 5 | Pending |
| PROV-03 | Phase 5 | Pending |
| PROV-04 | Phase 6 | Pending |
| PROV-05 | Phase 6 | Pending |
| PROV-06 | Phase 6 | Pending |
| PROV-07 | Phase 4 | Complete |
| PROV-08 | Phase 4 | Complete |
| ROUT-01 | Phase 3 | Pending |
| ROUT-02 | Phase 3 | Pending |
| ROUT-03 | Phase 3 | Pending |
| ROUT-04 | Phase 11 | Pending |
| ROUT-05 | Phase 11 | Pending |
| ROUT-06 | Phase 11 | Pending |
| ROUT-07 | Phase 3 | Pending |
| ROUT-08 | Phase 4 | Complete |
| POOL-01 | Phase 2 | Complete |
| POOL-02 | Phase 2 | Complete |
| POOL-03 | Phase 2 | Complete |
| POOL-04 | Phase 2 | Complete |
| POOL-05 | Phase 2 | Complete |
| POOL-06 | Phase 2 | Complete |
| CIRC-01 | Phase 4 | Complete |
| CIRC-02 | Phase 4 | Complete |
| CIRC-03 | Phase 4 | Complete |
| CIRC-04 | Phase 4 | Complete |
| CIRC-05 | Phase 4 | Complete |
| CIRC-06 | Phase 4 | Complete |
| CIRC-07 | Phase 4 | Complete |
| AUTH-01 | Phase 1 | Pending |
| AUTH-02 | Phase 1 | Pending |
| AUTH-03 | Phase 1 | Pending |
| AUTH-04 | Phase 2 | Complete |
| AUTH-05 | Phase 2 | Complete |
| CONF-01 | Phase 7 | Pending |
| CONF-02 | Phase 7 | Pending |
| CONF-03 | Phase 7 | Pending |
| CONF-04 | Phase 7 | Pending |
| CONF-05 | Phase 7 | Pending |
| CONF-06 | Phase 7 | Pending |
| MOD-01 | Phase 8 | Pending |
| MOD-02 | Phase 8 | Pending |
| MOD-03 | Phase 8 | Pending |
| MOD-04 | Phase 8 | Pending |
| OBS-01 | Phase 8 | Pending |
| OBS-02 | Phase 8 | Pending |
| OBS-03 | Phase 8 | Pending |
| OBS-04 | Phase 8 | Pending |
| OBS-05 | Phase 8 | Pending |
| OBS-06 | Phase 8 | Pending |
| OBS-07 | Phase 8 | Pending |
| GRPC-01 | Phase 9 | Pending |
| GRPC-02 | Phase 9 | Pending |
| GRPC-03 | Phase 9 | Pending |
| GRPC-04 | Phase 9 | Pending |
| GRPC-05 | Phase 9 | Pending |
| GRPC-06 | Phase 9 | Pending |
| TUI-01 | Phase 10 | Pending |
| TUI-02 | Phase 10 | Pending |
| TUI-03 | Phase 10 | Pending |
| TUI-04 | Phase 10 | Pending |
| TUI-05 | Phase 10 | Pending |
| TUI-06 | Phase 10 | Pending |
| TUI-07 | Phase 10 | Pending |
| CLI-01 | Phase 11 | Pending |
| CLI-02 | Phase 11 | Pending |
| CLI-03 | Phase 11 | Pending |
| CLI-04 | Phase 11 | Pending |
| CLI-05 | Phase 11 | Pending |
| CLI-06 | Phase 11 | Pending |

**Coverage:**

- v1 requirements: 77 total
- Mapped to phases: 77
- Unmapped: 0

---

*Requirements defined: 2026-01-20*
*Last updated: 2026-01-23 after Phase 4 completion*
