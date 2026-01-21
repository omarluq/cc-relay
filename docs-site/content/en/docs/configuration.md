---
title: Configuration
weight: 3
---

CC-Relay is configured via YAML files. This guide covers all configuration options.

## Configuration File Location

Default locations (checked in order):

1. `./config.yaml` (current directory)
2. `~/.config/cc-relay/config.yaml`
3. Path specified via `--config` flag

Generate a default config with:

```bash
cc-relay config init
```

## Environment Variable Expansion

CC-Relay supports environment variable expansion using `${VAR_NAME}` syntax:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # Expanded at load time
```

## Complete Configuration Reference

```yaml
# ==========================================================================
# Server Configuration
# ==========================================================================
server:
  # Address to listen on
  listen: "127.0.0.1:8787"

  # Request timeout in milliseconds (default: 600000 = 10 minutes)
  timeout_ms: 600000

  # Maximum concurrent requests (0 = unlimited)
  max_concurrent: 0

  # Enable HTTP/2 for better performance
  enable_http2: true

  # Authentication configuration
  auth:
    # Require specific API key for proxy access
    api_key: "${PROXY_API_KEY}"

    # Allow Claude Code subscription Bearer tokens
    allow_subscription: true

    # Specific Bearer token to validate (optional)
    bearer_secret: "${BEARER_SECRET}"

# ==========================================================================
# Provider Configurations
# ==========================================================================
providers:
  # Anthropic Direct API
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Optional, uses default

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60       # Requests per minute
        tpm_limit: 100000   # Tokens per minute

    # Optional: Specify available models
    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"

  # Z.AI / Zhipu GLM
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    # Map Claude model names to Z.AI models
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    # Optional: Specify available models
    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"

# ==========================================================================
# Logging Configuration
# ==========================================================================
logging:
  # Log level: debug, info, warn, error
  level: "info"

  # Log format: json, text
  format: "text"

  # Enable colored output (for text format)
  pretty: true

  # Granular debug options
  debug_options:
    log_request_body: false
    log_response_headers: false
    log_tls_metrics: false
    max_body_log_size: 1000
```

## Server Configuration

### Listen Address

The `listen` field specifies where the proxy listens for incoming requests:

```yaml
server:
  listen: "127.0.0.1:8787"  # Local only (recommended)
  # listen: "0.0.0.0:8787"  # All interfaces (use with caution)
```

### Authentication

CC-Relay supports multiple authentication methods:

#### API Key Authentication

Require clients to provide a specific API key:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```

Clients must include the header: `x-api-key: <your-proxy-key>`

#### Claude Code Subscription Passthrough

Allow Claude Code subscription users to connect:

```yaml
server:
  auth:
    allow_subscription: true
```

This accepts `Authorization: Bearer` tokens from Claude Code.

#### Combined Authentication

Allow both API key and subscription authentication:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```

#### No Authentication

To disable authentication (not recommended for production):

```yaml
server:
  auth: {}
  # Or simply omit the auth section
```

### HTTP/2 Support

Enable HTTP/2 for better performance with concurrent requests:

```yaml
server:
  enable_http2: true
```

## Provider Configuration

### Provider Types

CC-Relay currently supports two provider types:

| Type | Description | Default Base URL |
|------|-------------|------------------|
| `anthropic` | Anthropic Direct API | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Anthropic Provider

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Optional

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60
        tpm_limit: 100000

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### Z.AI Provider

Z.AI offers Anthropic-compatible APIs with GLM models at lower cost:

```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"
```

### Multiple API Keys

Pool multiple API keys for higher throughput:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true

    keys:
      - key: "${ANTHROPIC_API_KEY_1}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_2}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_3}"
        rpm_limit: 60
        tpm_limit: 100000
```

### Custom Base URL

Override the default API endpoint:

```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```

## Logging Configuration

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Verbose output for development |
| `info` | Normal operation messages |
| `warn` | Warning messages |
| `error` | Error messages only |

### Log Format

```yaml
logging:
  format: "text"   # Human-readable (default)
  # format: "json" # Machine-readable, for log aggregation
```

### Debug Options

Fine-grained control over debug logging:

```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # Log request bodies (redacted)
    log_response_headers: true  # Log response headers
    log_tls_metrics: true       # Log TLS connection info
    max_body_log_size: 1000     # Max bytes to log from bodies
```

## Cache Configuration

CC-Relay provides a unified caching layer with multiple backend options for different deployment scenarios.

### Cache Modes

| Mode | Backend | Use Case |
|------|---------|----------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | Single-instance deployments, high performance |
| `ha` | [Olric](https://github.com/buraksezer/olric) | Multi-instance deployments, shared state |
| `disabled` | Noop | No caching, passthrough |

### Single Mode (Ristretto)

Ristretto is a high-performance, concurrent in-memory cache. This is the default mode for single-instance deployments.

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `num_counters` | int64 | 1,000,000 | Number of 4-bit access counters. Recommended: 10x expected max items. |
| `max_cost` | int64 | 104,857,600 (100 MB) | Maximum memory in bytes the cache can hold. |
| `buffer_items` | int64 | 64 | Number of keys per Get buffer. Controls admission buffer size. |

### HA Mode (Olric) - Embedded

For multi-instance deployments requiring shared cache state, use embedded Olric mode where each cc-relay instance runs an Olric node.

```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "other-node:3322"  # Memberlist port = bind_addr + 2
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `embedded` | bool | false | Run embedded Olric node (true) vs. connect to external cluster (false). |
| `bind_addr` | string | required | Address for Olric client connections (e.g., "0.0.0.0:3320"). |
| `dmap_name` | string | "cc-relay" | Name of the distributed map. All nodes must use the same name. |
| `environment` | string | "local" | Memberlist preset: "local", "lan", or "wan". |
| `peers` | []string | - | Memberlist addresses for peer discovery. Uses port bind_addr + 2. |
| `replica_count` | int | 1 | Number of copies per key. 1 = no replication. |
| `read_quorum` | int | 1 | Minimum successful reads for response. |
| `write_quorum` | int | 1 | Minimum successful writes for response. |
| `member_count_quorum` | int32 | 1 | Minimum cluster members required to operate. |
| `leave_timeout` | duration | 5s | Time to broadcast leave message before shutdown. |

**Important:** Olric uses two ports - the `bind_addr` port for client connections and `bind_addr + 2` for memberlist gossip. Ensure both ports are open in your firewall.

### HA Mode (Olric) - Client Mode

Connect to an external Olric cluster instead of running embedded nodes:

```yaml
cache:
  mode: ha
  olric:
    embedded: false
    addresses:
      - "olric-node-1:3320"
      - "olric-node-2:3320"
    dmap_name: "cc-relay"
```

| Field | Type | Description |
|-------|------|-------------|
| `embedded` | bool | Set to `false` for client mode. |
| `addresses` | []string | External Olric cluster addresses. |
| `dmap_name` | string | Distributed map name (must match cluster configuration). |

### Disabled Mode

Disable caching entirely for debugging or when caching is handled elsewhere:

```yaml
cache:
  mode: disabled
```

For detailed cache configuration including cache key conventions, cache busting strategies, HA clustering guides, and troubleshooting, see the [Cache System documentation](/docs/cache/).

## Example Configurations

### Minimal Single Provider

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

### Multi-Provider Setup

```yaml
server:
  listen: "127.0.0.1:8787"
  auth:
    allow_subscription: true

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"

logging:
  level: "info"
  format: "text"
```

### Development with Debug Logging

```yaml
server:
  listen: "127.0.0.1:8787"

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"

logging:
  level: "debug"
  format: "text"
  pretty: true
  debug_options:
    log_request_body: true
    log_response_headers: true
    log_tls_metrics: true
```

## Validating Configuration

Validate your configuration file:

```bash
cc-relay config validate
```

## Hot Reloading

Configuration changes require a server restart. Hot-reloading is planned for a future release.

## Next Steps

- [Understanding the architecture](/docs/architecture/)
- [API reference](/docs/api/)
