# cc-relay

Multi-provider proxy for Claude Code that enables simultaneous use of multiple Anthropic-compatible API endpoints, API keys, and models.

## Why?

Claude Code connects to one provider at a time. But what if you want to:

- **Pool rate limits** across multiple Anthropic API keys?
- **Save money** by routing simple tasks to Z.AI or local Ollama?
- **Never get stuck** with automatic failover between providers?
- **Use your company's Bedrock/Azure/Vertex** alongside personal API keys?

**cc-relay** makes all of this possible.

```
┌─────────────┐     ┌───────────┐     ┌─────────────────────────────┐
│ Claude Code │────▶│ cc-relay  │────▶│ Anthropic (key1, key2, ...) │
└─────────────┘     │           │────▶│ Z.AI                        │
                    │           │────▶│ Ollama (local)              │
                    │           │────▶│ AWS Bedrock                 │
                    │           │────▶│ Azure Foundry               │
                    └───────────┘────▶│ Vertex AI                   │
                                      └─────────────────────────────┘
```

## Features

- 🔄 **Multi-key pooling** - Combine multiple API keys to maximize throughput
- 🚀 **Smart routing** - Shuffle, round-robin, cost-based, latency-based, or failover
- 🔌 **6 providers** - Anthropic, Z.AI, Ollama, Bedrock, Azure, Vertex AI
- 🖥️ **TUI dashboard** - Real-time stats and management via Bubble Tea
- 🔥 **Hot reload** - Change config without restarting
- 📊 **Prometheus metrics** - Built-in observability

## Quick Start

```bash
# Install
go install github.com/omarish/cc-relay@latest

# Create config
mkdir -p ~/.config/cc-relay
cat > ~/.config/cc-relay/config.yaml << 'EOF'
server:
  listen: "127.0.0.1:8787"

providers:
  - name: anthropic
    type: anthropic
    keys:
      - key: "${ANTHROPIC_API_KEY}"
EOF

# Start proxy
cc-relay serve

# Configure Claude Code to use proxy
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_API_KEY="managed-by-cc-relay"

# Use Claude Code normally
claude
```

## CLI Commands

```bash
# Start the proxy server
cc-relay serve [--config path/to/config.yaml]

# Check if server is running
cc-relay status

# Validate configuration file
cc-relay config validate [--config path/to/config.yaml]

# Show version information
cc-relay version

# Get help
cc-relay --help
cc-relay serve --help
```

## API Endpoints

### POST /v1/messages

Proxies requests to the configured backend provider. This is the main endpoint used by Claude Code.

- **Auth**: Required (x-api-key or Bearer token depending on config)
- **Content-Type**: application/json
- **Streaming**: Supports SSE streaming via `stream: true` in request body

### GET /v1/models

Lists all available models from all configured providers. This is a discovery endpoint.

- **Auth**: Not required
- **Response format**: Anthropic-compatible model list

```json
{
  "object": "list",
  "data": [
    {
      "id": "claude-sonnet-4-5-20250514",
      "object": "model",
      "created": 1737500000,
      "owned_by": "anthropic",
      "provider": "anthropic-primary"
    },
    {
      "id": "glm-4-plus",
      "object": "model",
      "created": 1737500000,
      "owned_by": "zhipu",
      "provider": "zai-primary"
    }
  ]
}
```

Configure models in your provider config:

```yaml
providers:
  - name: anthropic-primary
    type: anthropic
    enabled: true
    models:
      - claude-sonnet-4-5-20250514
      - claude-opus-4-5-20250514
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: zai-primary
    type: zai
    enabled: true
    models:
      - glm-4
      - glm-4-plus
    keys:
      - key: "${ZAI_API_KEY}"
```

### GET /health

Health check endpoint for load balancers and monitoring.

- **Auth**: Not required
- **Response**: `{"status":"ok"}`

## Authentication

cc-relay supports multiple authentication methods for different Claude Code user types.

### API Key Users

If you have an Anthropic API key (starts with `sk-ant-...`):

```bash
# Start cc-relay
cc-relay serve

# Configure Claude Code to use proxy
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_API_KEY="sk-ant-..."  # Your actual API key
claude
```

cc-relay configuration:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"  # Optional: require specific proxy auth key
```

### Subscription Users (Claude Code Pro/Team)

If you use Claude Code with a subscription (no API key required):

1. Configure cc-relay to accept subscription tokens:

```yaml
server:
  auth:
    allow_subscription: true  # Accept subscription tokens
```

2. Use Claude Code normally - the token is passed through to Anthropic:

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

**How it works:** Claude Code subscription users authenticate via `Authorization: Bearer` tokens. cc-relay accepts these tokens in passthrough mode and forwards them to Anthropic for validation.

**Security Note:** Subscription tokens are as sensitive as API keys. Never commit them to version control or share them publicly.

### Multiple Auth Methods

You can enable multiple auth methods simultaneously. The proxy tries them in order until one succeeds:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"     # Method 1: x-api-key header
    allow_subscription: true         # Method 2: Bearer token (subscription)
    allow_bearer: true               # Method 3: Generic Bearer token
    bearer_secret: ""                # Empty = accept any bearer token
```

## Configuration

See [example.yaml](example.yaml) for full configuration options.

### Multiple API Keys

```yaml
providers:
  - name: anthropic-pool
    type: anthropic
    keys:
      - key: "${ANTHROPIC_API_KEY_1}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_2}"
        rpm_limit: 60
        tpm_limit: 100000
```

### Failover Chain

```yaml
routing:
  strategy: failover
  failover:
    primary: anthropic
    fallbacks:
      - zai
      - ollama
```

### Cost-Based Routing

```yaml
routing:
  strategy: cost-based
  # Routes to cheapest provider that can handle the request
```

### Z.AI Provider

Z.AI (Zhipu AI) offers GLM models through an Anthropic-compatible API at ~1/7 the cost:

```yaml
providers:
  - name: zai
    type: zai
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"
    keys:
      - key: "${ZAI_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5-20250929": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
      "claude-haiku-4-5-20251001": "GLM-4.5-Air"
      "claude-haiku-4-5": "GLM-4.5-Air"
```

## Supported Providers

| Provider      | Status     | Notes                                   |
| ------------- | ---------- | --------------------------------------- |
| Anthropic     | ✅ Full    | Native support, all features            |
| Z.AI (Zhipu)  | ✅ Full    | Anthropic-compatible, ~1/7 cost         |
| Ollama        | ⚠️ Partial | No prompt caching, no extended thinking |
| AWS Bedrock   | ✅ Full    | IAM or Bearer Token auth                |
| Azure Foundry | ✅ Full    | API Key or Entra ID auth                |
| Vertex AI     | ✅ Full    | Google OAuth auth                       |

## TUI Dashboard

```bash
cc-relay tui
```

```
┌─────────────────────────────────────────────────────────────────┐
│  cc-relay v0.1.0                              [q]uit [?]help    │
├─────────────────────────────────────────────────────────────────┤
│  Strategy: simple-shuffle    Active: 3    Requests: 1,247       │
├─────────────────────────────────────────────────────────────────┤
│  ● anthropic     healthy   847 req   avg 234ms   [2 keys]       │
│  ● zai           healthy   312 req   avg 189ms   [1 key]        │
│  ○ ollama        degraded   88 req   avg 1.2s    [local]        │
└─────────────────────────────────────────────────────────────────┘
```

## Documentation

- [SPEC.md](SPEC.md) - Full technical specification
- [llms.txt](llms.txt) - LLM-friendly project context
- [example.yaml](example.yaml) - Configuration reference

## Development

```bash
# Clone
git clone https://github.com/omarish/cc-relay
cd cc-relay

# Build
go build -o cc-relay ./cmd/cc-relay

# Test
go test ./...

# Run
./cc-relay serve --config example.yaml
```

## Roadmap

- [x] Spec & architecture design
- [ ] Phase 1: MVP (Anthropic, Z.AI, Ollama, simple-shuffle)
- [ ] Phase 2: Multi-key pooling & rate limiting
- [ ] Phase 3: Cloud providers (Bedrock, Azure, Vertex)
- [ ] Phase 4: gRPC management API & TUI
- [ ] Phase 5: Advanced routing strategies
- [ ] Phase 6: WebUI

## License

MIT

## Contributing

Contributions welcome! Please read the spec first and open an issue to discuss before submitting PRs.
