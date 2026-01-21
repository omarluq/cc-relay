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

## Configuration

See [config/example.yaml](config/example.yaml) for full configuration options.

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
- [config/example.yaml](config/example.yaml) - Configuration reference

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
./cc-relay serve --config config/example.yaml
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
