---
title: Documentation
weight: 1
---

# CC-Relay Documentation

Welcome to the CC-Relay documentation! This guide will help you set up, configure, and use CC-Relay as a multi-provider proxy for Claude Code and other LLM clients.

## What is CC-Relay?

CC-Relay is a high-performance HTTP proxy written in Go that sits between LLM clients (like Claude Code) and multiple LLM providers. It provides:

- **Multi-provider support**: Anthropic, Z.AI, Ollama, AWS Bedrock, Azure Foundry, Vertex AI
- **Rate limit management**: Pool API keys and distribute requests intelligently
- **Cost optimization**: Route based on cost, latency, or model availability
- **High availability**: Automatic failover with circuit breaker pattern
- **Real-time monitoring**: TUI dashboard and gRPC management API

## Quick Navigation

- [Getting Started](/docs/getting-started/) - Installation and first run
- [Configuration](/docs/configuration/) - Provider setup and routing
- [Architecture](/docs/architecture/) - System design and components
- [API Reference](/docs/api/) - REST and gRPC APIs

## Documentation Sections

### Getting Started
- [Installation](/docs/getting-started/#installation)
- [Quick Start](/docs/getting-started/#quick-start)
- [First Run](/docs/getting-started/#first-run)
- [Testing with Claude Code](/docs/getting-started/#testing-with-claude-code)

### Configuration
- [Provider Setup](/docs/configuration/#provider-setup)
- [Routing Strategies](/docs/configuration/#routing-strategies)
- [Rate Limiting](/docs/configuration/#rate-limiting)
- [Health Tracking](/docs/configuration/#health-tracking)

### Architecture
- [System Overview](/docs/architecture/#system-overview)
- [Core Components](/docs/architecture/#core-components)
- [Request Flow](/docs/architecture/#request-flow)
- [Provider Transformations](/docs/architecture/#provider-transformations)

### API Reference
- [REST Endpoints](/docs/api/#rest-endpoints)
- [gRPC Management API](/docs/api/#grpc-management-api)
- [SSE Streaming](/docs/api/#sse-streaming)

## Need Help?

- [Report an issue](https://github.com/omarluq/cc-relay/issues)
- [Discussions](https://github.com/omarluq/cc-relay/discussions)
- [Examples](https://github.com/omarluq/cc-relay/tree/main/examples)
