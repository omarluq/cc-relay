---
title: Configuration
weight: 3
---

CC-Relay is configured via YAML or TOML files. This guide covers all configuration options.

## Configuration File Location

Default locations (checked in order):

1. `./config.yaml` or `./config.toml` (current directory)
2. `~/.config/cc-relay/config.yaml` or `~/.config/cc-relay/config.toml`
3. Path specified via `--config` flag

The format is automatically detected from the file extension (`.yaml`, `.yml`, or `.toml`).

Generate a default config with:

```bash
cc-relay config init
```

## Environment Variable Expansion

CC-Relay supports environment variable expansion using `${VAR_NAME}` syntax in both YAML and TOML formats:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # Expanded at load time
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"  # Expanded at load time
```
  {{< /tab >}}
{{< /tabs >}}

## Complete Configuration Reference

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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

# ==========================================================================
# Cache Configuration
# ==========================================================================
cache:
  # Cache mode: single, ha, disabled
  mode: single

  # Single mode (Ristretto) configuration
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size

  # HA mode (Olric) configuration
  olric:
    embedded: true                 # Run embedded Olric node
    bind_addr: "0.0.0.0:3320"      # Olric client port
    dmap_name: "cc-relay"          # Distributed map name
    environment: lan               # local, lan, or wan
    peers:                         # Memberlist addresses (bind_addr + 2)
      - "other-node:3322"
    replica_count: 2               # Copies per key
    read_quorum: 1                 # Min reads for success
    write_quorum: 1                # Min writes for success
    member_count_quorum: 2         # Min cluster members
    leave_timeout: 5s              # Leave broadcast duration

# ==========================================================================
# Routing Configuration
# ==========================================================================
routing:
  # Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
  strategy: failover

  # Timeout for failover attempts in milliseconds (default: 5000)
  failover_timeout: 5000

  # Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# Server Configuration
# ==========================================================================
[server]
# Address to listen on
listen = "127.0.0.1:8787"

# Request timeout in milliseconds (default: 600000 = 10 minutes)
timeout_ms = 600000

# Maximum concurrent requests (0 = unlimited)
max_concurrent = 0

# Enable HTTP/2 for better performance
enable_http2 = true

# Authentication configuration
[server.auth]
# Require specific API key for proxy access
api_key = "${PROXY_API_KEY}"

# Allow Claude Code subscription Bearer tokens
allow_subscription = true

# Specific Bearer token to validate (optional)
bearer_secret = "${BEARER_SECRET}"

# ==========================================================================
# Provider Configurations
# ==========================================================================

# Anthropic Direct API
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Optional, uses default

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60       # Requests per minute
tpm_limit = 100000   # Tokens per minute

# Optional: Specify available models
models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]

# Z.AI / Zhipu GLM
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

# Map Claude model names to Z.AI models
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

# Optional: Specify available models
models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]

# ==========================================================================
# Logging Configuration
# ==========================================================================
[logging]
# Log level: debug, info, warn, error
level = "info"

# Log format: json, text
format = "text"

# Enable colored output (for text format)
pretty = true

# Granular debug options
[logging.debug_options]
log_request_body = false
log_response_headers = false
log_tls_metrics = false
max_body_log_size = 1000

# ==========================================================================
# Cache Configuration
# ==========================================================================
[cache]
# Cache mode: single, ha, disabled
mode = "single"

# Single mode (Ristretto) configuration
[cache.ristretto]
num_counters = 1000000  # 10x expected max items
max_cost = 104857600    # 100 MB
buffer_items = 64       # Admission buffer size

# HA mode (Olric) configuration
[cache.olric]
embedded = true                 # Run embedded Olric node
bind_addr = "0.0.0.0:3320"      # Olric client port
dmap_name = "cc-relay"          # Distributed map name
environment = "lan"             # local, lan, or wan
peers = ["other-node:3322"]     # Memberlist addresses (bind_addr + 2)
replica_count = 2               # Copies per key
read_quorum = 1                 # Min reads for success
write_quorum = 1                # Min writes for success
member_count_quorum = 2         # Min cluster members
leave_timeout = "5s"            # Leave broadcast duration

# ==========================================================================
# Routing Configuration
# ==========================================================================
[routing]
# Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
strategy = "failover"

# Timeout for failover attempts in milliseconds (default: 5000)
failover_timeout = 5000

# Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

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

## Transparent Authentication

cc-relay automatically detects how to handle authentication based on what the client sends:

### How It Works

| Client Sends | cc-relay Behavior | Use Case |
|--------------|-------------------|----------|
| `Authorization: Bearer <token>` | Forward unchanged | Claude Code subscription users |
| `x-api-key: <key>` | Forward unchanged | Direct API key users |
| No auth headers | Use configured provider keys | Enterprise/team deployments |

### Claude Code Subscription Users

If you have a Claude Code subscription (Max/Team/Enterprise plan), you can use cc-relay as a transparent proxy:

```bash
# Set cc-relay as your API endpoint
export ANTHROPIC_BASE_URL="http://localhost:8787"

# Your subscription token flows through unchanged
# ANTHROPIC_AUTH_TOKEN is already set by Claude Code
claude
```

**No API key required** - cc-relay forwards your subscription token to Anthropic.

### Enterprise/Team Deployments

For centralized API key management, don't provide client auth - cc-relay uses configured keys:

```yaml
# config.yaml
providers:
  - name: anthropic
    type: anthropic
    base_url: https://api.anthropic.com
    enabled: true
    keys:
      - key: ${ANTHROPIC_API_KEY}
        rpm_limit: 50
```

```bash
# Client has no auth - uses configured keys
export ANTHROPIC_BASE_URL="http://localhost:8787"
unset ANTHROPIC_AUTH_TOKEN
unset ANTHROPIC_API_KEY
claude
```

### Mixed Mode

You can run both modes simultaneously:
- Subscription users: Their auth flows through (no key pool overhead)
- Team users: Use configured keys with rate limit pooling

Rate limiting and key pooling only apply when using configured keys, not client-provided auth.

### Key Points

1. **Auto-detection**: No configuration needed - behavior determined by client headers
2. **Subscription passthrough**: `Authorization: Bearer` forwarded unchanged
3. **Fallback keys**: Used only when client has no auth
4. **Key pool efficiency**: Only tracks usage of YOUR keys, not client subscriptions

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

## Routing Configuration

CC-Relay supports multiple routing strategies for distributing requests across providers.

```yaml
# ==========================================================================
# Routing Configuration
# ==========================================================================
routing:
  # Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
  strategy: failover

  # Timeout for failover attempts in milliseconds (default: 5000)
  failover_timeout: 5000

  # Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```

### Routing Strategies

| Strategy | Description |
|----------|-------------|
| `failover` | Try providers in priority order, fallback on failure (default) |
| `round_robin` | Sequential rotation through providers |
| `weighted_round_robin` | Distribute proportionally by weight |
| `shuffle` | Fair random distribution |

### Provider Weight and Priority

Weight and priority are configured in the provider's first key:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # For weighted-round-robin (higher = more traffic)
        priority: 2    # For failover (higher = tried first)
```

For detailed routing configuration including strategy explanations, debug headers, and failover triggers, see the [Routing documentation](/docs/routing/).

## Example Configurations

### Minimal Single Provider

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

### Multi-Provider Setup

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[server.auth]
allow_subscription = true

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"
enabled = true

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"

[logging]
level = "info"
format = "text"
```
  {{< /tab >}}
{{< /tabs >}}

### Development with Debug Logging

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

logging:
  level: "debug"
  format: "text"
  pretty: true
  debug_options:
    log_request_body: true
    log_response_headers: true
    log_tls_metrics: true
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

[logging]
level = "debug"
format = "text"
pretty = true

[logging.debug_options]
log_request_body = true
log_response_headers = true
log_tls_metrics = true
```
  {{< /tab >}}
{{< /tabs >}}

## Validating Configuration

Validate your configuration file:

```bash
cc-relay config validate
```

**Tip**: Always validate configuration changes before deploying. Hot-reload will reject invalid configurations, but validation catches errors before they reach production.

## Hot Reloading

CC-Relay automatically detects and applies configuration changes without requiring a restart. This enables zero-downtime configuration updates.

### How It Works

CC-Relay uses [fsnotify](https://github.com/fsnotify/fsnotify) to monitor the config file for changes:

1. **File watching**: The parent directory is monitored to properly detect atomic writes (temp file + rename pattern used by most editors)
2. **Debouncing**: Multiple rapid file events are coalesced with a 100ms debounce delay to handle editor save behavior
3. **Atomic swap**: New configuration is loaded and swapped atomically using Go's `sync/atomic.Pointer`
4. **In-flight preservation**: Requests in progress continue with the old configuration; new requests use the updated configuration

### Events That Trigger Reload

| Event | Triggers Reload |
|-------|-----------------|
| File write | Yes |
| File create (atomic rename) | Yes |
| File chmod | No (ignored) |
| Other file in directory | No (ignored) |

### Logging

When hot-reload occurs, you'll see log messages:

```
INF config file reloaded path=/path/to/config.yaml
INF config hot-reloaded successfully
```

If the new configuration is invalid:

```
ERR failed to reload config path=/path/to/config.yaml error="validation error"
```

Invalid configurations are rejected and the proxy continues with the previous valid configuration.

### Limitations

- **Provider changes**: Adding or removing providers requires a restart (routing infrastructure is initialized at startup)
- **Listen address**: Changing `server.listen` requires a restart
- **gRPC address**: Changing gRPC management API address requires a restart

Configuration options that can be hot-reloaded:
- Logging level and format
- Rate limits on existing keys
- Health check intervals
- Routing strategy weights and priorities

## Next Steps

- [Routing strategies](/docs/routing/) - Provider selection and failover
- [Understanding the architecture](/docs/architecture/)
- [API reference](/docs/api/)
