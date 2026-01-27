---
title: Getting Started
weight: 2
---

This guide will walk you through installing, configuring, and running CC-Relay for the first time.

## Prerequisites

- **Go 1.21+** for building from source
- **API keys** for at least one supported provider (Anthropic or Z.AI)
- **Claude Code** CLI for testing (optional)

## Installation

### Using Go Install

```bash
go install github.com/omarluq/cc-relay@latest
```

The binary will be installed to `$GOPATH/bin/cc-relay` or `$HOME/go/bin/cc-relay`.

### Building from Source

```bash
# Clone the repository
git clone https://github.com/omarluq/cc-relay.git
cd cc-relay

# Build using task (recommended)
task build

# Or build manually
go build -o cc-relay ./cmd/cc-relay

# Run
./cc-relay --help
```

### Pre-built Binaries

Download pre-built binaries from the [releases page](https://github.com/omarluq/cc-relay/releases).

## Quick Start

### 1. Initialize Configuration

CC-Relay can generate a default configuration file for you:

```bash
cc-relay config init
```

This creates a config file at `~/.config/cc-relay/config.yaml` with sensible defaults.

### 2. Set Environment Variables

```bash
export ANTHROPIC_API_KEY="your-api-key-here"

# Optional: If using Z.AI
export ZAI_API_KEY="your-zai-key-here"
```

### 3. Run CC-Relay

```bash
cc-relay serve
```

You should see output like:

```
INF starting cc-relay listen=127.0.0.1:8787
INF using primary provider provider=anthropic-pool type=anthropic
```

### 4. Configure Claude Code

The easiest way to configure Claude Code to use CC-Relay:

```bash
cc-relay config cc init
```

This automatically updates `~/.claude/settings.json` with the proxy configuration.

Alternatively, set environment variables manually:

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_AUTH_TOKEN="managed-by-cc-relay"
claude
```

## Verify It's Working

### Check Server Status

```bash
cc-relay status
```

Output:
```
✓ cc-relay is running (127.0.0.1:8787)
```

### Test the Health Endpoint

```bash
curl http://localhost:8787/health
```

Response:
```json
{"status":"ok"}
```

### List Available Models

```bash
curl http://localhost:8787/v1/models
```

### Test a Request

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## CLI Commands

CC-Relay provides several CLI commands:

| Command | Description |
|---------|-------------|
| `cc-relay serve` | Start the proxy server |
| `cc-relay status` | Check if server is running |
| `cc-relay config init` | Generate default config file |
| `cc-relay config cc init` | Configure Claude Code to use cc-relay |
| `cc-relay config cc remove` | Remove cc-relay config from Claude Code |
| `cc-relay --version` | Show version information |

### Serve Command Options

```bash
cc-relay serve [flags]

Flags:
  --config string      Config file path (default: ~/.config/cc-relay/config.yaml)
  --log-level string   Log level (debug, info, warn, error)
  --log-format string  Log format (json, text)
  --debug              Enable debug mode (verbose logging)
```

## Minimal Configuration

Here's a minimal working configuration:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

## Next Steps

- [Configure multiple providers](/docs/configuration/)
- [Understand the architecture](/docs/architecture/)
- [API reference](/docs/api/)

## Troubleshooting

### Port Already in Use

If port 8787 is already in use, change the listen address in your config:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8788"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8788"
```
  {{< /tab >}}
{{< /tabs >}}

### Provider Not Responding

Check the server logs for connection errors:

```bash
cc-relay serve --log-level debug
```

### Authentication Errors

If you see "authentication failed" errors:

1. Verify your API key is correctly set in environment variables
2. Check the config file references the correct environment variable
3. Ensure the API key is valid with the provider

### Debug Mode

Enable debug mode for detailed request/response logging:

```bash
cc-relay serve --debug
```

This enables:
- Debug log level
- Request body logging (redacted for sensitive fields)
- Response header logging
- TLS connection metrics
