# cc-relay

> A multi-provider proxy for Claude Code that enables simultaneous use of multiple Anthropic-compatible API endpoints, API keys, and models.

## Vision

Claude Code currently connects to one provider at a time. **cc-relay** sits between Claude Code and multiple backends, enabling:

- **Rate limit pooling** across multiple API keys
- **Cost optimization** by routing tasks to appropriate providers
- **Redundancy** via automatic failover
- **Flexibility** to mix cloud providers with local models

```
┌─────────────────────────────────────────────────────────────────┐
│                      Claude Code Client                         │
│           ANTHROPIC_BASE_URL=http://localhost:8787              │
└────────────────────────────┬────────────────────────────────────┘
                             │ Anthropic Messages API
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         cc-relay                                │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    HTTP Proxy Server                     │   │
│  │                  (Anthropic API format)                  │   │
│  └─────────────────────────┬────────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────┬───────────┴───────────┬─────────────────────┐  │
│  │   Router    │      Key Pool         │   Health Tracker    │  │
│  │ (strategy)  │   (per-key usage)     │  (circuit breaker)  │  │
│  └──────┬──────┴───────────┬───────────┴──────────┬──────────┘  │
│         │                  │                      │             │
│  ┌──────▼──────────────────▼──────────────────────▼──────────┐  │
│  │              Provider Transformer Pipeline                │  │
│  │    Request → Adapt Auth/Format → Response Transform       │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│                           │                                     │
│  ┌────────────────────────▼──────────────────────────────────┐  │
│  │                    gRPC Management API                    │  │
│  │         (TUI/WebUI ↔ Daemon communication)                │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌───────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Anthropic   │  │      Z.AI       │  │     Ollama      │
│  (multiple    │  │   (GLM-4.7)     │  │   (local)       │
│   API keys)   │  │                 │  │                 │
└───────────────┘  └─────────────────┘  └─────────────────┘
        │                    │                    │
        ▼                    ▼                    ▼
┌───────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  AWS Bedrock  │  │  Azure Foundry  │  │   Vertex AI     │
│               │  │                 │  │                 │
└───────────────┘  └─────────────────┘  └─────────────────┘
```

## Supported Providers

| Provider          | Base URL                                     | Auth Method                    | Model Format                         | Notes                                                 |
| ----------------- | -------------------------------------------- | ------------------------------ | ------------------------------------ | ----------------------------------------------------- |
| **Anthropic**     | `api.anthropic.com`                          | `x-api-key` header             | `claude-sonnet-4-5-20250929`         | Native, full feature support                          |
| **Z.AI**          | `api.z.ai/api/anthropic`                     | `ANTHROPIC_AUTH_TOKEN`         | `GLM-4.7`, `GLM-4.5-Air`             | Full Anthropic compatibility, ~1/7 cost               |
| **Ollama**        | `localhost:11434/v1/messages`                | None (ignored)                 | Any local model                      | No prompt caching, no extended thinking enforcement   |
| **AWS Bedrock**   | `bedrock-runtime.{region}.amazonaws.com`     | AWS SigV4 / IAM / Bearer Token | `anthropic.claude-sonnet-4-5-*-v1:0` | Model in URL, `anthropic_version: bedrock-2023-05-31` |
| **Azure Foundry** | `{resource}.services.ai.azure.com/anthropic` | `x-api-key` or Entra ID        | `claude-sonnet-4-5`                  | Full compatibility, deployment names as model IDs     |
| **Vertex AI**     | `{region}-aiplatform.googleapis.com`         | Google OAuth                   | `claude-sonnet-4-5@20250929`         | Model in URL, `anthropic_version: vertex-2023-10-16`  |

## Core Features

### 1. Multi-Key Rate Limit Pooling

Pool multiple API keys per provider to maximize throughput:

```yaml
providers:
  - name: anthropic-pool
    type: anthropic
    keys:
      - key: "sk-ant-key1"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "sk-ant-key2"
        rpm_limit: 60
        tpm_limit: 100000
```

The router tracks per-key usage and distributes requests to stay under limits.

### 2. Routing Strategies

| Strategy         | Description                                                         | Use Case              |
| ---------------- | ------------------------------------------------------------------- | --------------------- |
| `simple-shuffle` | Weighted random selection based on available capacity               | Default, low overhead |
| `round-robin`    | Sequential distribution across backends                             | Even distribution     |
| `least-busy`     | Route to backend with fewest in-flight requests                     | Minimize latency      |
| `cost-based`     | Prefer cheaper providers (Z.AI, Ollama) for simple tasks            | Cost optimization     |
| `latency-based`  | Track response times, prefer faster backends                        | Performance critical  |
| `failover`       | Primary → fallback chain with circuit breaker                       | High availability     |
| `model-based`    | Route by model name prefix (`claude-*` → Anthropic, `glm-*` → Z.AI) | Multi-model workflows |

### 3. Automatic Failover

Circuit breaker pattern with three states:

- **CLOSED**: Normal operation
- **OPEN**: Blocking requests after threshold failures (429s, 5xx, timeouts)
- **HALF-OPEN**: Testing recovery after cooldown period

```yaml
failover:
  triggers:
    - rate_limit_errors: 3 # 429 responses
    - timeout_errors: 2 # Request timeouts
    - failure_rate: 0.5 # >50% failure rate
  cooldown_seconds: 60
  recovery_probe_interval: 30
```

### 4. Provider Transformers

Each provider requires specific request/response transformations:

```go
type ProviderTransformer interface {
    // Transform outgoing request
    TransformRequest(req *AnthropicRequest) (*http.Request, error)

    // Transform incoming response
    TransformResponse(resp *http.Response) (*AnthropicResponse, error)

    // Provider-specific auth
    Authenticate(req *http.Request) error

    // Health check
    HealthCheck(ctx context.Context) error
}
```

#### Provider-Specific Requirements

**AWS Bedrock:**

- Model specified in URL path, not request body
- `anthropic_version: "bedrock-2023-05-31"` in body
- Auth: AWS SigV4 signing or Bearer Token (`AWS_BEARER_TOKEN_BEDROCK`)

**Azure Foundry:**

- Endpoint: `https://{resource}.services.ai.azure.com/anthropic/v1/messages`
- Auth: `x-api-key` header (not `api-key`) or Entra ID token
- Model field uses deployment name (user-defined)

**Vertex AI:**

- Endpoint: `https://{region}-aiplatform.googleapis.com/v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:rawPredict`
- `anthropic_version: "vertex-2023-10-16"` in body
- Auth: Google OAuth bearer token (`gcloud auth print-access-token`)

**Ollama:**

- Limitations: No prompt caching, no batches API, no PDF support
- `budget_tokens` for extended thinking accepted but not enforced
- Images must be base64 (no URL support)

## Architecture

### Technology Choice: Go

**Why Go over Crystal:**

| Aspect            | Go                                                  | Crystal                         |
| ----------------- | --------------------------------------------------- | ------------------------------- |
| HTTP Proxy        | `net/http/httputil.ReverseProxy` - production ready | Manual implementation required  |
| SSE Streaming     | Native `http.Flusher` interface                     | Possible but less mature        |
| TUI Framework     | **Bubble Tea** (36k+ stars) - Elm architecture      | Crysterm (135 stars) - immature |
| Cross-compilation | `GOOS=windows GOARCH=amd64 go build`                | Requires target system linking  |
| gRPC Support      | First-class, well-documented                        | Community libraries only        |
| Community         | Massive ecosystem                                   | Small but growing               |

### Component Architecture

```
cc-relay/
├── cmd/
│   └── cc-relay/
│       └── main.go              # Entry point
├── internal/
│   ├── proxy/
│   │   ├── server.go            # HTTP proxy server
│   │   ├── sse.go               # SSE streaming handler
│   │   └── middleware.go        # Logging, metrics, auth
│   ├── router/
│   │   ├── router.go            # Routing logic interface
│   │   ├── strategies/          # Routing strategy implementations
│   │   │   ├── shuffle.go
│   │   │   ├── roundrobin.go
│   │   │   ├── leastbusy.go
│   │   │   ├── costbased.go
│   │   │   ├── latency.go
│   │   │   └── failover.go
│   │   └── keypool.go           # API key pool management
│   ├── providers/
│   │   ├── provider.go          # Provider interface
│   │   ├── anthropic.go         # Direct Anthropic API
│   │   ├── zai.go               # Z.AI / Zhipu GLM
│   │   ├── ollama.go            # Ollama local
│   │   ├── bedrock.go           # AWS Bedrock
│   │   ├── azure.go             # Azure Foundry
│   │   └── vertex.go            # Google Vertex AI
│   ├── health/
│   │   ├── tracker.go           # Health tracking per backend
│   │   └── circuit.go           # Circuit breaker implementation
│   ├── config/
│   │   ├── config.go            # Configuration structs
│   │   ├── loader.go            # YAML/TOML loading
│   │   └── watcher.go           # Hot-reload via fsnotify
│   └── grpc/
│       ├── server.go            # gRPC management server
│       ├── proto/
│       │   └── relay.proto      # Service definitions
│       └── handlers.go          # gRPC service implementations
├── ui/
│   ├── tui/
│   │   ├── app.go               # Bubble Tea application
│   │   ├── views/               # TUI views/components
│   │   └── client.go            # gRPC client for daemon
│   └── web/                     # Optional WebUI (grpc-web)
├── proto/
│   └── relay.proto              # gRPC service definitions
├── config/
│   └── example.yaml             # Example configuration
└── Makefile
```

### gRPC Management API

The daemon exposes a gRPC API for management (TUI, WebUI, CLI):

```protobuf
syntax = "proto3";
package relay;

service RelayManager {
  // Real-time stats streaming
  rpc StreamStats(Empty) returns (stream Stats);

  // Provider management
  rpc ListProviders(Empty) returns (ProviderList);
  rpc GetProviderHealth(ProviderRequest) returns (HealthStatus);
  rpc EnableProvider(ProviderRequest) returns (Result);
  rpc DisableProvider(ProviderRequest) returns (Result);

  // Key management
  rpc ListKeys(ProviderRequest) returns (KeyList);
  rpc AddKey(AddKeyRequest) returns (Result);
  rpc RemoveKey(RemoveKeyRequest) returns (Result);
  rpc GetKeyUsage(KeyRequest) returns (KeyUsage);

  // Configuration
  rpc GetConfig(Empty) returns (Config);
  rpc UpdateConfig(Config) returns (Result);
  rpc ReloadConfig(Empty) returns (Result);

  // Routing
  rpc GetRoutingStrategy(Empty) returns (RoutingStrategy);
  rpc SetRoutingStrategy(RoutingStrategy) returns (Result);
}

message Stats {
  int64 timestamp = 1;
  int64 total_requests = 2;
  int64 active_requests = 3;
  repeated ProviderStats providers = 4;
}

message ProviderStats {
  string name = 1;
  string status = 2;  // healthy, degraded, unhealthy
  int64 requests_total = 3;
  int64 requests_success = 4;
  int64 requests_failed = 5;
  double avg_latency_ms = 6;
  repeated KeyStats keys = 7;
}

message KeyStats {
  string key_id = 1;  // masked key identifier
  int32 rpm_used = 2;
  int32 rpm_limit = 3;
  int64 tpm_used = 4;
  int64 tpm_limit = 5;
}
```

### TUI Design (Bubble Tea)

```
┌─────────────────────────────────────────────────────────────────┐
│  cc-relay v0.1.0                              [q]uit [?]help    │
├─────────────────────────────────────────────────────────────────┤
│  Strategy: simple-shuffle    Active: 3    Requests: 1,247       │
├─────────────────────────────────────────────────────────────────┤
│  PROVIDERS                                                      │
├─────────────────────────────────────────────────────────────────┤
│  ● anthropic     healthy   847 req   avg 234ms   [2 keys]       │
│    └─ sk-ant-...x7f2   42/60 rpm   67,234/100k tpm              │
│    └─ sk-ant-...a3b9   38/60 rpm   54,102/100k tpm              │
│  ● zai           healthy   312 req   avg 189ms   [1 key]        │
│  ○ ollama        degraded   88 req   avg 1.2s    [local]        │
│  ○ bedrock       disabled    0 req                              │
├─────────────────────────────────────────────────────────────────┤
│  RECENT REQUESTS                                                │
├─────────────────────────────────────────────────────────────────┤
│  12:34:56  anthropic  claude-sonnet-4-5  1,234 tok   234ms  ✓   │
│  12:34:55  zai        GLM-4.7            892 tok     189ms  ✓   │
│  12:34:54  anthropic  claude-sonnet-4-5  2,101 tok   312ms  ✓   │
│  12:34:53  ollama     qwen3              445 tok     1.2s   ✓   │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration

### Format: YAML (primary) or TOML

```yaml
# cc-relay.yaml

server:
  listen: "127.0.0.1:8787"
  timeout_ms: 600000 # 10 minutes for long operations

routing:
  strategy: "simple-shuffle"
  # strategy: "failover"
  # strategy: "cost-based"

  # For failover strategy
  failover:
    primary: "anthropic-pool"
    fallbacks:
      - "zai"
      - "ollama"
    cooldown_seconds: 60

providers:
  # Anthropic Direct (multiple keys pooled)
  - name: "anthropic-pool"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY_1}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_2}"
        rpm_limit: 60
        tpm_limit: 100000

  # Z.AI / Zhipu GLM
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"
    keys:
      - key: "${ZAI_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5": "GLM-4.7"
      "claude-haiku-4-5": "GLM-4.5-Air"

  # Ollama (local)
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"
    model_mapping:
      "claude-sonnet-4-5": "qwen3:32b"
      "claude-haiku-4-5": "qwen3:8b"

  # AWS Bedrock
  - name: "bedrock"
    type: "bedrock"
    enabled: false
    region: "us-east-1"
    auth:
      method: "iam" # or "bearer_token"
      # For bearer_token method:
      # token: "${AWS_BEARER_TOKEN_BEDROCK}"
    model_mapping:
      "claude-sonnet-4-5": "anthropic.claude-sonnet-4-5-20250929-v1:0"

  # Azure Foundry
  - name: "azure"
    type: "azure"
    enabled: false
    resource: "my-resource" # {resource}.services.ai.azure.com
    auth:
      method: "api_key" # or "entra_id"
      key: "${AZURE_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5": "claude-sonnet-4-5" # deployment name

  # Google Vertex AI
  - name: "vertex"
    type: "vertex"
    enabled: false
    project_id: "${GOOGLE_CLOUD_PROJECT}"
    region: "us-east5"
    # Auth via GOOGLE_APPLICATION_CREDENTIALS or gcloud CLI
    model_mapping:
      "claude-sonnet-4-5": "claude-sonnet-4-5@20250929"

grpc:
  listen: "127.0.0.1:9090"
  # For WebUI support:
  # web_listen: "127.0.0.1:9091"

logging:
  level: "info"
  format: "json" # or "text"

metrics:
  enabled: true
  prometheus_endpoint: "/metrics"
```

### Hot Reload

Configuration can be reloaded without restart:

- **SIGHUP signal**: `kill -HUP $(pgrep cc-relay)`
- **File watcher**: Automatic reload on config file change (via `fsnotify`)
- **gRPC call**: `ReloadConfig()` from TUI/WebUI

## API Compatibility

### Required Endpoints

| Endpoint       | Method           | Description                |
| -------------- | ---------------- | -------------------------- |
| `/v1/messages` | POST             | Messages API (primary)     |
| `/v1/messages` | POST (streaming) | Streaming messages via SSE |

### SSE Streaming Format

Must maintain exact Anthropic SSE event sequence:

```
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{...}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{...}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{...},"usage":{...}}

event: message_stop
data: {"type":"message_stop"}
```

### Critical Headers

```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

### Tool Use Preservation

Claude Code uses parallel tool calls. The proxy must:

- Preserve `tool_use_id` through transformation
- Handle multiple `tool_use` blocks atomically
- Maintain `input_schema` format (not OpenAI's `parameters`)

## Development Phases

### Phase 1: MVP (v0.1.0)

- [ ] Basic HTTP proxy server
- [ ] Anthropic provider (single key)
- [ ] Z.AI provider
- [ ] Ollama provider
- [ ] Simple-shuffle routing
- [ ] YAML configuration
- [ ] Basic CLI (`cc-relay serve`)

### Phase 2: Multi-Key & Routing (v0.2.0)

- [ ] Multi-key pooling per provider
- [ ] Rate limit tracking
- [ ] Round-robin strategy
- [ ] Failover strategy with circuit breaker
- [ ] Health checking
- [ ] Hot-reload configuration

### Phase 3: Cloud Providers (v0.3.0)

- [ ] AWS Bedrock provider (IAM + Bearer Token auth)
- [ ] Azure Foundry provider (API key + Entra ID)
- [ ] Vertex AI provider (Google OAuth)

### Phase 4: Management Interface (v0.4.0)

- [ ] gRPC management API
- [ ] Bubble Tea TUI
- [ ] Real-time stats streaming
- [ ] Key/provider management commands

### Phase 5: Advanced Features (v0.5.0)

- [ ] Cost-based routing
- [ ] Latency-based routing
- [ ] Model-based routing
- [ ] Prometheus metrics
- [ ] Request logging/audit trail

### Phase 6: WebUI (v0.6.0)

- [ ] grpc-web proxy
- [ ] React/Svelte WebUI
- [ ] Dashboard with live stats

## Usage

### Installation

```bash
# From source
go install github.com/omarish/cc-relay@latest

# Or download binary
curl -sSL https://github.com/omarish/cc-relay/releases/latest/download/cc-relay-$(uname -s)-$(uname -m) -o cc-relay
chmod +x cc-relay
```

### Quick Start

```bash
# Create config
cat > ~/.config/cc-relay/config.yaml << 'EOF'
server:
  listen: "127.0.0.1:8787"
providers:
  - name: anthropic
    type: anthropic
    keys:
      - key: "${ANTHROPIC_API_KEY}"
EOF

# Start daemon
cc-relay serve

# Configure Claude Code
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_API_KEY="proxy-managed"  # Actual keys in cc-relay config

# Use Claude Code normally
claude
```

### TUI

```bash
# Launch TUI (connects to running daemon via gRPC)
cc-relay tui

# Or combined daemon + TUI
cc-relay serve --tui
```

### Commands

```bash
cc-relay serve              # Start proxy daemon
cc-relay tui                # Launch management TUI
cc-relay status             # Show current status
cc-relay config reload      # Hot-reload configuration
cc-relay provider list      # List configured providers
cc-relay provider enable <name>
cc-relay provider disable <name>
cc-relay key add <provider> <key>
cc-relay key remove <provider> <key-id>
```

## Prior Art & Inspiration

| Project                                                                | Language   | Notes                                           |
| ---------------------------------------------------------------------- | ---------- | ----------------------------------------------- |
| [claude-code-router](https://github.com/musistudio/claude-code-router) | TypeScript | Route-based provider selection, React UI        |
| [LiteLLM](https://github.com/BerriAI/litellm)                          | Python     | Comprehensive multi-LLM proxy, 8ms P95 latency  |
| [Bifrost](https://github.com/maximhq/bifrost)                          | Go         | 11μs overhead at 5k RPS - performance benchmark |
| [OpenRouter](https://openrouter.ai)                                    | Service    | Cloud-scale routing with provider preferences   |

## License

MIT

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
