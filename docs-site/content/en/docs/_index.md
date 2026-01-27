---
title: Documentation
weight: 1
---

Welcome to the CC-Relay documentation! This guide will help you set up, configure, and use CC-Relay as a multi-provider proxy for Claude Code and other LLM clients.

## What is CC-Relay?

CC-Relay is a high-performance HTTP proxy written in Go that sits between LLM clients (like Claude Code) and LLM providers. It provides:

- **Multi-provider support**: Anthropic and Z.AI (with more providers planned)
- **Anthropic API compatible**: Drop-in replacement for direct API access
- **SSE streaming**: Full support for streaming responses
- **Multiple authentication methods**: API key and Bearer token support
- **Claude Code integration**: Easy setup with built-in configuration command

## Current Status

CC-Relay is in active development. Currently implemented features:

| Feature | Status |
|---------|--------|
| HTTP Proxy Server | Implemented |
| Anthropic Provider | Implemented |
| Z.AI Provider | Implemented |
| SSE Streaming | Implemented |
| API Key Authentication | Implemented |
| Bearer Token (Subscription) Auth | Implemented |
| Claude Code Configuration | Implemented |
| Multiple API Keys | Implemented |
| Debug Logging | Implemented |

**Planned features:**
- Routing strategies (round-robin, failover, cost-based)
- Rate limiting per API key
- Circuit breaker and health tracking
- gRPC management API
- TUI dashboard
- Additional providers (Ollama, Bedrock, Azure, Vertex)

## Quick Start

```bash
# Install
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest

# Initialize config
cc-relay config init

# Set your API key
export ANTHROPIC_API_KEY="your-key-here"

# Start the proxy
cc-relay serve

# Configure Claude Code (in another terminal)
cc-relay config cc init
```

## Quick Navigation

- [Getting Started](/docs/getting-started/) - Installation and first run
- [Configuration](/docs/configuration/) - Provider setup and options
- [Architecture](/docs/architecture/) - System design and components
- [API Reference](/docs/api/) - HTTP endpoints and examples

## Documentation Sections

### Getting Started
- [Installation](/docs/getting-started/#installation)
- [Quick Start](/docs/getting-started/#quick-start)
- [CLI Commands](/docs/getting-started/#cli-commands)
- [Testing with Claude Code](/docs/getting-started/#testing-with-claude-code)
- [Troubleshooting](/docs/getting-started/#troubleshooting)

### Configuration
- [Server Configuration](/docs/configuration/#server-configuration)
- [Provider Configuration](/docs/configuration/#provider-configuration)
- [Authentication](/docs/configuration/#authentication)
- [Logging Configuration](/docs/configuration/#logging-configuration)
- [Example Configurations](/docs/configuration/#example-configurations)

### Architecture
- [System Overview](/docs/architecture/#system-overview)
- [Core Components](/docs/architecture/#core-components)
- [Request Flow](/docs/architecture/#request-flow)
- [SSE Streaming](/docs/architecture/#sse-streaming)
- [Authentication Flow](/docs/architecture/#authentication-flow)

### API Reference
- [POST /v1/messages](/docs/api/#post-v1messages)
- [GET /v1/models](/docs/api/#get-v1models)
- [GET /v1/providers](/docs/api/#get-v1providers)
- [GET /health](/docs/api/#get-health)
- [Client Examples](/docs/api/#curl-examples)

## Need Help?

- [Report an issue](https://github.com/omarluq/cc-relay/issues)
- [Discussions](https://github.com/omarluq/cc-relay/discussions)
