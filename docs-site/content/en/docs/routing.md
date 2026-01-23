---
title: Routing
weight: 4
---

CC-Relay supports multiple routing strategies to distribute requests across providers. This page explains each strategy and how to configure them.

## Overview

Routing determines how cc-relay chooses which provider handles each request. The right strategy depends on your priorities: availability, cost, latency, or load distribution.

| Strategy | Config Value | Description | Use Case |
|----------|--------------|-------------|----------|
| Round-Robin | `round_robin` | Sequential rotation through providers | Even distribution |
| Weighted Round-Robin | `weighted_round_robin` | Proportional distribution by weight | Capacity-based distribution |
| Shuffle | `shuffle` | Fair random ("dealing cards") | Randomized load balancing |
| Failover | `failover` (default) | Priority-based with automatic retry | High availability |

## Configuration

Configure routing in your `config.yaml`:

```yaml
routing:
  # Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
  strategy: failover

  # Timeout for failover attempts in milliseconds (default: 5000)
  failover_timeout: 5000

  # Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```

**Default:** If `strategy` is not specified, cc-relay uses `failover` as the safest option.

## Strategies

### Round-Robin

Sequential distribution using an atomic counter. Each provider receives one request before any provider receives a second.

```yaml
routing:
  strategy: round_robin
```

**How it works:**

1. Request 1 → Provider A
2. Request 2 → Provider B
3. Request 3 → Provider C
4. Request 4 → Provider A (cycle repeats)

**Best for:** Equal distribution across providers with similar capacity.

### Weighted Round-Robin

Distributes requests proportionally based on provider weights. Uses the Nginx smooth weighted round-robin algorithm for even distribution.

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3  # Receives 3x more requests

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        weight: 1  # Receives 1x requests
```

**How it works:**

With weights 3:1, out of every 4 requests:
- 3 requests → anthropic
- 1 request → zai

**Default weight:** 1 (if not specified)

**Best for:** Distributing load based on provider capacity, rate limits, or cost allocation.

### Shuffle

Fair random distribution using the Fisher-Yates "dealing cards" pattern. Everyone gets one card before anyone gets a second.

```yaml
routing:
  strategy: shuffle
```

**How it works:**

1. All providers start in a "deck"
2. Random provider selected and removed from deck
3. When deck empty, reshuffle all providers
4. Guarantees fair distribution over time

**Best for:** Randomized load balancing while ensuring fairness.

### Failover

Tries providers in priority order. On failure, parallel races remaining providers for the fastest successful response. This is the **default strategy**.

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Tried first (higher = higher priority)

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Fallback
```

**How it works:**

1. Try highest priority provider first
2. If it fails (see [Failover Triggers](#failover-triggers)), launch parallel requests to all remaining providers
3. Return first successful response, cancel others
4. Respects `failover_timeout` for total operation time

**Default priority:** 1 (if not specified)

**Best for:** High availability with automatic fallback.

## Debug Headers

When `routing.debug: true`, cc-relay adds diagnostic headers to responses:

| Header | Value | Description |
|--------|-------|-------------|
| `X-CC-Relay-Strategy` | Strategy name | Which routing strategy was used |
| `X-CC-Relay-Provider` | Provider name | Which provider handled the request |

**Example response headers:**

```
X-CC-Relay-Strategy: failover
X-CC-Relay-Provider: anthropic
```

**Security Warning:** Debug headers expose internal routing decisions. Use only in development or trusted environments. Never enable in production with untrusted clients.

## Failover Triggers

The failover strategy triggers retry on specific error conditions:

| Trigger | Conditions | Description |
|---------|------------|-------------|
| Status Code | `429`, `500`, `502`, `503`, `504` | Rate limit or server errors |
| Timeout | `context.DeadlineExceeded` | Request timeout exceeded |
| Connection | `net.Error` | Network errors, DNS failures, connection refused |

**Important:** Client errors (4xx except 429) do **not** trigger failover. These indicate issues with the request itself, not the provider.

### Status Codes Explained

| Code | Meaning | Failover? |
|------|---------|-----------|
| `429` | Rate Limited | Yes - try another provider |
| `500` | Internal Server Error | Yes - server issue |
| `502` | Bad Gateway | Yes - upstream issue |
| `503` | Service Unavailable | Yes - temporarily down |
| `504` | Gateway Timeout | Yes - upstream timeout |
| `400` | Bad Request | No - fix the request |
| `401` | Unauthorized | No - fix authentication |
| `403` | Forbidden | No - permission issue |

## Examples

### Simple Failover (Recommended for Most Users)

Use the default strategy with prioritized providers:

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1
```

### Load Balanced with Weights

Distribute load based on provider capacity:

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "primary"
    type: "anthropic"
    keys:
      - key: "${PRIMARY_KEY}"
        weight: 3  # 75% of traffic

  - name: "secondary"
    type: "anthropic"
    keys:
      - key: "${SECONDARY_KEY}"
        weight: 1  # 25% of traffic
```

### Development with Debug Headers

Enable debug headers for troubleshooting:

```yaml
routing:
  strategy: round_robin
  debug: true

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
```

### High Availability with Fast Failover

Minimize failover latency:

```yaml
routing:
  strategy: failover
  failover_timeout: 3000  # 3 second timeout

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1
```

## Provider Weight and Priority

Weight and priority are specified in the provider's key configuration:

```yaml
providers:
  - name: "example"
    type: "anthropic"
    keys:
      - key: "${API_KEY}"
        weight: 3      # For weighted-round-robin (higher = more traffic)
        priority: 2    # For failover (higher = tried first)
        rpm_limit: 60  # Rate limit tracking
```

**Note:** Weight and priority are read from the **first key** in the provider's key list.

## Next Steps

- [Configuration reference](/docs/configuration/) - Complete configuration options
- [Architecture overview](/docs/architecture/) - How cc-relay works internally
