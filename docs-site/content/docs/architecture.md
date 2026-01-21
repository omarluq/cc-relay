---
title: Architecture
weight: 4
---

# Architecture Overview

CC-Relay is designed as a high-performance, multi-provider HTTP proxy with intelligent routing, rate limiting, and health tracking.

## System Overview

```mermaid
graph TB
    subgraph "Client Layer"
        A[Claude Code]
        B[Custom LLM Client]
        C[Other Clients]
    end
    
    subgraph "CC-Relay Proxy"
        D[HTTP Server<br/>:8787]
        E[Middleware Stack]
        F[Router]
        G[Provider Manager]
        H[Health Tracker]
        I[Rate Limiter]
        J[gRPC API<br/>:9090]
    end
    
    subgraph "Provider Layer"
        K[Anthropic]
        L[Z.AI]
        M[Ollama]
        N[AWS Bedrock]
        O[Azure Foundry]
        P[Vertex AI]
    end
    
    A --> D
    B --> D
    C --> D
    
    D --> E
    E --> F
    F --> G
    G --> H
    G --> I
    G --> K
    G --> L
    G --> M
    G --> N
    G --> O
    G --> P
    
    J --> G
    
    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style D fill:#ec4899,stroke:#db2777,color:#fff
    style F fill:#f59e0b,stroke:#d97706,color:#000
    style G fill:#10b981,stroke:#059669,color:#fff
    style K fill:#8b5cf6,stroke:#7c3aed,color:#fff
    style L fill:#3b82f6,stroke:#2563eb,color:#fff
    style M fill:#06b6d4,stroke:#0891b2,color:#fff
    style N fill:#f97316,stroke:#ea580c,color:#fff
    style O fill:#14b8a6,stroke:#0d9488,color:#fff
    style P fill:#f43f5e,stroke:#e11d48,color:#fff
```

## Core Components

### 1. HTTP Proxy Server

**Location**: `internal/proxy/`

The HTTP server implements the Anthropic Messages API (`/v1/messages`) with exact compatibility for Claude Code.

**Features:**
- SSE streaming with proper event sequencing
- Request validation and transformation
- Middleware chain (logging, metrics, auth)
- Context propagation for timeouts and cancellation

**Request Flow:**

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Middleware
    participant Router
    participant Provider
    
    Client->>Proxy: POST /v1/messages
    Proxy->>Middleware: Request
    Middleware->>Middleware: Logging
    Middleware->>Middleware: Metrics
    Middleware->>Middleware: Auth Validation
    Middleware->>Router: Route Request
    Router->>Router: Select Provider
    Router->>Provider: Transform & Forward
    Provider-->>Router: Response
    Router-->>Middleware: Response
    Middleware-->>Proxy: Response
    Proxy-->>Client: Stream SSE Events
```

### 2. Router

**Location**: `internal/router/`

The router selects the optimal provider based on configured strategy.

**Strategies:**
- `shuffle`: Random selection
- `round-robin`: Even distribution
- `failover`: Priority-based fallback chain
- `cost-based`: Cheapest provider meeting threshold
- `latency-based`: Fastest provider (P95 latency)
- `model-based`: Route by model availability

**Decision Tree:**

```mermaid
graph TD
    A[Incoming Request] --> B{Routing Strategy}
    B -->|shuffle| C[Random Provider]
    B -->|round-robin| D[Next in Rotation]
    B -->|failover| E[Try Priority Chain]
    B -->|cost-based| F{Cost < Threshold?}
    B -->|latency-based| G{Latency < Max?}
    B -->|model-based| H{Model Available?}
    
    F -->|Yes| I[Select Cheapest]
    F -->|No| J[Skip Provider]
    
    G -->|Yes| K[Select Fastest]
    G -->|No| L[Skip Provider]
    
    H -->|Yes| M[Select Provider]
    H -->|No| N[Skip Provider]
    
    C --> O[Provider Selected]
    D --> O
    E --> O
    I --> O
    K --> O
    M --> O
    
    J --> P[Try Next]
    L --> P
    N --> P
    
    P --> B
    
    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style B fill:#f59e0b,stroke:#d97706,color:#000
    style O fill:#10b981,stroke:#059669,color:#fff
```

### 3. Provider Manager

**Location**: `internal/providers/`

Each provider implements the `Provider` interface:

```go
type Provider interface {
    Name() string
    Type() string
    TransformRequest(req *Request) (*ProviderRequest, error)
    TransformResponse(resp *ProviderResponse) (*Response, error)
    Authenticate(req *http.Request) error
    HealthCheck(ctx context.Context) error
}
```

**Provider-Specific Transformations:**

```mermaid
graph LR
    A[Standard Request] --> B{Provider Type}
    
    B -->|Anthropic| C[No transformation]
    B -->|Z.AI| D[Model mapping]
    B -->|Ollama| E[Remove caching]
    B -->|Bedrock| F[URL path + SigV4]
    B -->|Azure| G[Deployment name]
    B -->|Vertex| H[URL path + OAuth]
    
    C --> I[Provider Request]
    D --> I
    E --> I
    F --> I
    G --> I
    H --> I
    
    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style B fill:#f59e0b,stroke:#d97706,color:#000
    style I fill:#10b981,stroke:#059669,color:#fff
```

### 4. Health Tracker

**Location**: `internal/health/`

Circuit breaker pattern with three states:

```mermaid
stateDiagram-v2
    [*] --> CLOSED: Initialize
    
    CLOSED --> OPEN: failures >= threshold
    OPEN --> HALF_OPEN: recovery_timeout elapsed
    HALF_OPEN --> CLOSED: successes >= threshold
    HALF_OPEN --> OPEN: any failure
    
    state CLOSED {
        [*] --> Monitoring
        Monitoring --> CountFailure: Request failed
        CountFailure --> Monitoring
    }
    
    state OPEN {
        [*] --> Rejecting
        Rejecting --> StartTimer: All requests rejected
        StartTimer --> Rejecting
    }
    
    state HALF_OPEN {
        [*] --> Probing
        Probing --> CountSuccess: Request succeeded
        Probing --> CountFailure: Request failed
        CountSuccess --> Probing
        CountFailure --> Probing
    }
```

**Failure Detection:**
- HTTP 429 (rate limit exceeded)
- HTTP 5xx (server errors)
- Timeout errors
- Network connection failures

### 5. Rate Limiter

**Location**: `internal/ratelimit/`

Token bucket algorithm per API key:

```mermaid
graph TD
    A[Request Arrives] --> B[Select Provider]
    B --> C{Get Available Key}
    
    C --> D{Key 1: Tokens?}
    D -->|Yes| E[Consume Tokens]
    D -->|No| F{Key 2: Tokens?}
    
    F -->|Yes| E
    F -->|No| G{Key 3: Tokens?}
    
    G -->|Yes| E
    G -->|No| H[All Keys Exhausted]
    
    E --> I[Forward Request]
    H --> J[Return 429]
    
    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style C fill:#f59e0b,stroke:#d97706,color:#000
    style E fill:#10b981,stroke:#059669,color:#fff
    style H fill:#ef4444,stroke:#dc2626,color:#fff
    style I fill:#8b5cf6,stroke:#7c3aed,color:#fff
    style J fill:#f97316,stroke:#ea580c,color:#fff
```

**Per-Key Tracking:**
- Requests per minute (RPM)
- Tokens per minute (TPM)
- Automatic refill at configured rate
- Round-robin key selection

### 6. Configuration Manager

**Location**: `internal/config/`

**Features:**
- YAML/TOML parsing with validation
- Environment variable expansion (`${VAR}`)
- Hot reload via `fsnotify` file watcher
- Schema validation on load

**Hot Reload Flow:**

```mermaid
sequenceDiagram
    participant User
    participant FileSystem
    participant Watcher
    participant ConfigMgr
    participant Proxy
    
    User->>FileSystem: Edit config.yaml
    FileSystem->>Watcher: File modified event
    Watcher->>ConfigMgr: Reload config
    ConfigMgr->>ConfigMgr: Validate
    ConfigMgr->>Proxy: Apply new config
    Proxy->>Proxy: Update providers
    Proxy-->>User: Config reloaded
```

### 7. gRPC Management API

**Location**: `internal/grpc/`, `proto/relay.proto`

Exposes management operations for stats, config, and provider control.

**Service Definition:**

```protobuf
service RelayService {
  rpc GetProviderStats(ProviderStatsRequest) returns (ProviderStatsResponse);
  rpc StreamStats(StreamStatsRequest) returns (stream StatsUpdate);
  rpc UpdateProvider(UpdateProviderRequest) returns (UpdateProviderResponse);
  rpc ReloadConfig(ReloadConfigRequest) returns (ReloadConfigResponse);
  rpc GetProviderHealth(HealthRequest) returns (HealthResponse);
}
```

## Request Flow

### Non-Streaming Request

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Router
    participant RateLimiter
    participant Health
    participant Provider
    
    Client->>Proxy: POST /v1/messages
    Proxy->>Router: Route request
    Router->>Health: Check provider health
    Health-->>Router: Provider healthy
    Router->>RateLimiter: Check rate limit
    RateLimiter-->>Router: Tokens available
    Router->>Provider: Transform & forward
    Provider-->>Router: Response
    Router-->>Proxy: Response
    Proxy-->>Client: JSON response
```

### Streaming Request (SSE)

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Provider
    
    Client->>Proxy: POST /v1/messages (stream=true)
    Proxy->>Provider: Forward request
    
    Provider-->>Proxy: event: message_start
    Proxy-->>Client: event: message_start
    
    Provider-->>Proxy: event: content_block_start
    Proxy-->>Client: event: content_block_start
    
    loop Content Streaming
        Provider-->>Proxy: event: content_block_delta
        Proxy-->>Client: event: content_block_delta
    end
    
    Provider-->>Proxy: event: content_block_stop
    Proxy-->>Client: event: content_block_stop
    
    Provider-->>Proxy: event: message_delta
    Proxy-->>Client: event: message_delta
    
    Provider-->>Proxy: event: message_stop
    Proxy-->>Client: event: message_stop
    
    Note over Client,Proxy: Connection closed
```

## API Compatibility

### Anthropic API Format

CC-Relay implements exact compatibility with the Anthropic Messages API:

**Endpoint**: `POST /v1/messages`

**Headers**:
- `x-api-key`: API key (managed by CC-Relay)
- `anthropic-version`: API version (e.g., `2023-06-01`)
- `content-type`: `application/json`

**Body**:
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "Hello!"}
  ],
  "stream": true
}
```

### Provider Transformations

Different providers require different API formats:

| Provider | Transformation |
|----------|----------------|
| **Anthropic** | None (native format) |
| **Z.AI** | Model name mapping only |
| **Ollama** | Remove `prompt_cache_config`, convert image URLs to base64 |
| **Bedrock** | Model in URL path, `anthropic_version: bedrock-2023-05-31`, AWS SigV4 signing |
| **Azure** | Use `x-api-key` header, deployment name as model ID |
| **Vertex** | Model in URL path, `anthropic_version: vertex-2023-10-16`, OAuth bearer token |

## Performance Considerations

### Connection Pooling

CC-Relay maintains persistent HTTP/2 connections to providers:

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
```

### Concurrency

- Goroutine per request (lightweight)
- Request context for cancellation
- Rate limiter uses sync.Mutex for thread safety
- Circuit breaker uses atomic operations

### Memory Management

- Streaming responses (no buffering)
- Request body size limits
- Connection pooling
- Graceful shutdown with timeout

## Deployment Architecture

### Single Instance

```mermaid
graph TD
    A[Load Balancer] --> B[CC-Relay Instance]
    B --> C[Anthropic]
    B --> D[Z.AI]
    B --> E[Ollama]
    
    style B fill:#ec4899,stroke:#db2777,color:#fff
```

### High Availability

```mermaid
graph TD
    A[Load Balancer] --> B[CC-Relay Instance 1]
    A --> C[CC-Relay Instance 2]
    A --> D[CC-Relay Instance 3]
    
    B --> E[Provider Pool]
    C --> E
    D --> E
    
    E --> F[Anthropic]
    E --> G[Z.AI]
    E --> H[Bedrock]
    
    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style B fill:#ec4899,stroke:#db2777,color:#fff
    style C fill:#ec4899,stroke:#db2777,color:#fff
    style D fill:#ec4899,stroke:#db2777,color:#fff
    style E fill:#10b981,stroke:#059669,color:#fff
```

## Next Steps

- [Configure routing strategies](/docs/configuration/#routing-strategies)
- [Set up health tracking](/docs/configuration/#health-tracking)
- [Use the management API](/docs/api/)
- [Monitor with Prometheus](/docs/monitoring/)
