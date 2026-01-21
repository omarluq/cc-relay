---
title: About
type: about
---

## About CC-Relay

CC-Relay is a high-performance HTTP proxy written in Go that enables Claude Code and other LLM clients to connect to multiple providers through a single endpoint.

### Project Goals

- **Simplify multi-provider access** - One proxy, multiple backends
- **Maintain API compatibility** - Drop-in replacement for direct Anthropic API access
- **Enable flexibility** - Easily switch providers without client changes
- **Support Claude Code** - First-class integration with Claude Code CLI

### Current Status

CC-Relay is in active development. The following features are implemented:

- HTTP proxy server with Anthropic API compatibility
- Anthropic and Z.AI provider support
- Full SSE streaming support
- API key and Bearer token authentication
- Multiple API keys per provider
- Debug logging for request/response inspection
- Claude Code configuration commands

### Planned Features

- Additional providers (Ollama, AWS Bedrock, Azure, Vertex AI)
- Routing strategies (round-robin, failover, cost-based)
- Rate limiting per API key
- Circuit breaker and health tracking
- gRPC management API
- TUI dashboard

### Built With

- [Go](https://go.dev/) - Programming language
- [Cobra](https://cobra.dev/) - CLI framework
- [zerolog](https://github.com/rs/zerolog) - Structured logging

### Author

Created by [Omar Alani](https://github.com/omarluq)

### License

CC-Relay is open source software licensed under the [AGPL 3 License](https://github.com/omarluq/cc-relay/blob/main/LICENSE).

### Contributing

Contributions are welcome! Please see the [GitHub repository](https://github.com/omarluq/cc-relay) for:

- [Reporting issues](https://github.com/omarluq/cc-relay/issues)
- [Submitting pull requests](https://github.com/omarluq/cc-relay/pulls)
- [Discussions](https://github.com/omarluq/cc-relay/discussions)
