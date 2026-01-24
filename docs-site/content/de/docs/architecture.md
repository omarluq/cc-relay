---
title: Architektur
weight: 4
---

CC-Relay ist ein hochperformanter, Multi-Provider HTTP-Proxy, der für LLM-Anwendungen entwickelt wurde. Er bietet intelligentes Routing, Caching von Thinking-Signaturen und nahtloses Failover zwischen Providern.

## Systemübersicht

```mermaid
graph TB
    subgraph "Client-Schicht"
        A[Claude Code]
        B[Eigener LLM-Client]
    end

    subgraph "CC-Relay Proxy"
        D[HTTP Server<br/>:8787]
        E[Middleware-Stack]
        F[Handler]
        G[Router]
        H[Signatur-Cache]
    end

    subgraph "Provider-Proxies"
        I[ProviderProxy<br/>Anthropic]
        J[ProviderProxy<br/>Z.AI]
        K[ProviderProxy<br/>Ollama]
    end

    subgraph "Backend-Provider"
        L[Anthropic API]
        M[Z.AI API]
        N[Ollama API]
    end

    A --> D
    B --> D
    D --> E
    E --> F
    F --> G
    F <--> H
    G --> I
    G --> J
    G --> K
    I --> L
    J --> M
    K --> N

    style A fill:#6366f1,stroke:#4f46e5,color:#fff
    style D fill:#ec4899,stroke:#db2777,color:#fff
    style F fill:#f59e0b,stroke:#d97706,color:#000
    style G fill:#10b981,stroke:#059669,color:#fff
    style H fill:#8b5cf6,stroke:#7c3aed,color:#fff
```

## Kernkomponenten

### 1. Handler

**Speicherort**: `internal/proxy/handler.go`

Der Handler ist der zentrale Koordinator für die Anfrageverarbeitung:

```go
type Handler struct {
    providerProxies map[string]*ProviderProxy  // Reverse-Proxies pro Provider
    defaultProvider providers.Provider          // Fallback für Single-Provider-Modus
    router          router.ProviderRouter       // Implementierung der Routing-Strategie
    healthTracker   *health.Tracker             // Circuit-Breaker-Tracking
    signatureCache  *SignatureCache             // Thinking-Signatur-Cache
    routingConfig   *config.RoutingConfig       // Modellbasierte Routing-Konfiguration
    providers       []router.ProviderInfo       // Verfügbare Provider
}
```

**Verantwortlichkeiten:**
- Extrahieren des Modellnamens aus dem Request-Body
- Erkennen von Thinking-Signaturen für Provider-Affinität
- Auswählen des Providers über den Router
- Delegieren an den entsprechenden ProviderProxy
- Verarbeiten von Thinking-Blöcken und Cachen von Signaturen

### 2. ProviderProxy

**Speicherort**: `internal/proxy/provider_proxy.go`

Jeder Provider erhält einen dedizierten Reverse-Proxy mit vorkonfigurierter URL und Authentifizierung:

```go
type ProviderProxy struct {
    Provider           providers.Provider
    Proxy              *httputil.ReverseProxy
    KeyPool            *keypool.KeyPool  // Für Multi-Key-Rotation
    APIKey             string            // Fallback-Einzelschlüssel
    targetURL          *url.URL          // Basis-URL des Providers
    modifyResponseHook ModifyResponseFunc
}
```

**Hauptmerkmale:**
- URL-Parsing erfolgt einmalig bei der Initialisierung (nicht pro Anfrage)
- Unterstützt transparente Authentifizierung (Weiterleitung von Client-Credentials) oder konfigurierte Authentifizierung
- Automatische SSE-Header-Injektion für Streaming-Antworten
- Key-Pool-Integration für Rate-Limit-Verteilung

### 3. Router

**Speicherort**: `internal/router/`

Der Router wählt aus, welcher Provider jede Anfrage bearbeitet:

| Strategie | Beschreibung |
|----------|-------------|
| `failover` | Prioritätsbasiert mit automatischem Retry (Standard) |
| `round_robin` | Sequentielle Rotation |
| `weighted_round_robin` | Proportional nach Gewichtung |
| `shuffle` | Faire Zufallsverteilung |
| `model_based` | Routing nach Modellnamen-Präfix |

### 4. Signatur-Cache

**Speicherort**: `internal/proxy/signature_cache.go`

Cached Thinking-Block-Signaturen für Cross-Provider-Kompatibilität:

```go
type SignatureCache struct {
    cache cache.Cache  // Ristretto-gestützter Cache
}

// Cache-Schlüsselformat: "sig:{modelGroup}:{textHash}"
// TTL: 3 Stunden (entspricht Claude API)
```

## Anfragefluss

### Multi-Provider-Routing

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Router
    participant ModelFilter
    participant ProviderProxy
    participant Backend

    Client->>Handler: HTTP-Anfrage (mit model-Feld)
    Handler->>Handler: Modell aus Body extrahieren
    Handler->>Handler: Thinking-Signatur-Präsenz prüfen
    Handler->>ModelFilter: FilterProvidersByModel(model, providers, mapping)
    ModelFilter->>ModelFilter: Längste-Präfix-Übereinstimmung gegen modelMapping
    ModelFilter->>Router: Gefilterte Provider-Liste zurückgeben
    Handler->>Router: Provider auswählen (Failover/Round-Robin auf gefilterter Liste)
    Router->>Router: Routing-Strategie auf gefilterte Provider anwenden
    Router->>Handler: Ausgewählte ProviderInfo zurückgeben

    Handler->>Handler: ProviderProxy für ausgewählten Provider abrufen
    Handler->>ProviderProxy: Anfrage mit Auth/Headers vorbereiten
    ProviderProxy->>ProviderProxy: Transparenten vs. konfigurierten Auth-Modus bestimmen
    ProviderProxy->>Backend: Anfrage an Provider-Ziel-URL weiterleiten
    Backend->>ProviderProxy: Antwort (mit Signatur-Headern)
    ProviderProxy->>Handler: Antwort mit Signatur-Info
    Handler->>Handler: Signatur cachen wenn Thinking vorhanden
    Handler->>Client: Antwort
```

### Verarbeitung von Thinking-Signaturen

Wenn erweitertes Thinking aktiviert ist, geben Provider signierte Thinking-Blöcke zurück. Diese Signaturen müssen vom selben Provider bei nachfolgenden Turns validiert werden. CC-Relay löst Cross-Provider-Signatur-Probleme durch Caching:

```mermaid
sequenceDiagram
    participant Request as Anfrage
    participant Handler
    participant SignatureCache as Signatur-Cache
    participant Backend
    participant ResponseStream as Antwort-Stream (SSE)

    Request->>Handler: HTTP mit Thinking-Blöcken
    Handler->>Handler: HasThinkingSignature-Prüfung
    Handler->>Handler: ProcessRequestThinking
    Handler->>SignatureCache: Get(modelGroup, thinkingText)
    SignatureCache-->>Handler: Gecachte Signatur oder leer
    Handler->>Handler: Unsignierte Blöcke entfernen / Gecachte Signatur anwenden
    Handler->>Backend: Bereinigte Anfrage weiterleiten

    Backend->>ResponseStream: Streaming-Antwort (thinking_delta-Events)
    ResponseStream->>Handler: thinking_delta-Event
    Handler->>Handler: Thinking-Text akkumulieren
    ResponseStream->>Handler: signature_delta-Event
    Handler->>SignatureCache: Set(modelGroup, thinking_text, signature)
    SignatureCache-->>Handler: Gecacht
    Handler->>ResponseStream: Signatur mit modelGroup-Präfix transformieren
    ResponseStream->>Request: SSE-Event mit präfixierter Signatur zurückgeben
```

**Modellgruppen für Signatur-Sharing:**

| Modell-Pattern | Gruppe | Signaturen geteilt |
|--------------|-------|-------------------|
| `claude-*` | `claude` | Ja, über alle Claude-Modelle |
| `gpt-*` | `gpt` | Ja, über alle GPT-Modelle |
| `gemini-*` | `gemini` | Ja, verwendet Sentinel-Wert |
| Andere | Exakter Name | Kein Sharing |

### SSE-Streaming-Ablauf

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Provider

    Client->>Proxy: POST /v1/messages (stream=true)
    Proxy->>Provider: Anfrage weiterleiten

    Provider-->>Proxy: event: message_start
    Proxy-->>Client: event: message_start

    Provider-->>Proxy: event: content_block_start
    Proxy-->>Client: event: content_block_start

    loop Content-Streaming
        Provider-->>Proxy: event: content_block_delta
        Proxy-->>Client: event: content_block_delta
    end

    Provider-->>Proxy: event: content_block_stop
    Proxy-->>Client: event: content_block_stop

    Provider-->>Proxy: event: message_delta
    Proxy-->>Client: event: message_delta

    Provider-->>Proxy: event: message_stop
    Proxy-->>Client: event: message_stop
```

**Erforderliche SSE-Header:**
```text
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

## Middleware-Stack

**Speicherort**: `internal/proxy/middleware.go`

| Middleware | Zweck |
|------------|-------|
| `RequestIDMiddleware` | Generiert/extrahiert X-Request-ID für Tracing |
| `LoggingMiddleware` | Protokolliert Anfrage/Antwort mit Timing |
| `AuthMiddleware` | Validiert x-api-key-Header |
| `MultiAuthMiddleware` | Unterstützt API-Key- und Bearer-Token-Auth |

## Provider-Schnittstelle

**Speicherort**: `internal/providers/provider.go`

```go
type Provider interface {
    Name() string
    BaseURL() string
    Owner() string
    Authenticate(req *http.Request, key string) error
    ForwardHeaders(originalHeaders http.Header) http.Header
    SupportsStreaming() bool
    SupportsTransparentAuth() bool
    ListModels() []Model
    GetModelMapping() map[string]string
    MapModel(requestModel string) string
}
```

**Implementierte Provider:**

| Provider | Typ | Funktionen |
|----------|------|----------|
| `AnthropicProvider` | `anthropic` | Natives Format, volle Funktionsunterstützung |
| `ZAIProvider` | `zai` | Anthropic-kompatibel, GLM-Modelle |
| `OllamaProvider` | `ollama` | Lokale Modelle, kein Prompt-Caching |

## Authentifizierungsmodi

### Transparente Authentifizierung
Wenn der Client Credentials bereitstellt und der Provider es unterstützt:
- `Authorization`- oder `x-api-key`-Header des Clients werden unverändert weitergeleitet
- CC-Relay agiert als reiner Proxy

### Konfigurierte Authentifizierung
Bei Verwendung von CC-Relays verwalteten Schlüsseln:
- Client-Credentials werden entfernt
- CC-Relay injiziert konfigurierten API-Schlüssel
- Unterstützt Key-Pool-Rotation für Rate-Limit-Verteilung

```mermaid
graph TD
    A[Anfrage eingehend] --> B{Hat Client Auth?}
    B -->|Ja| C{Provider unterstützt<br/>transparente Auth?}
    B -->|Nein| D[Konfigurierten Key verwenden]
    C -->|Ja| E[Client Auth weiterleiten]
    C -->|Nein| D
    D --> F{Key Pool verfügbar?}
    F -->|Ja| G[Key aus Pool wählen]
    F -->|Nein| H[Einzelnen API Key verwenden]
    E --> I[An Provider weiterleiten]
    G --> I
    H --> I
```

## Health-Tracking & Circuit Breaker

**Speicherort**: `internal/health/`

CC-Relay verfolgt Provider-Gesundheit und implementiert Circuit-Breaker-Muster:

| Status | Verhalten |
|--------|----------|
| CLOSED | Normalbetrieb, Anfragen fließen durch |
| OPEN | Provider als ungesund markiert, Anfragen scheitern schnell |
| HALF-OPEN | Prüfung mit begrenzten Anfragen nach Abkühlung |

**Auslöser für OPEN-Status:**
- HTTP 429 (Rate-limitiert)
- HTTP 5xx (Server-Fehler)
- Verbindungs-Timeouts
- Aufeinanderfolgende Fehler überschreiten Schwellenwert

## Verzeichnisstruktur

```text
cc-relay/
├── cmd/cc-relay/           # CLI-Einstiegspunkt
│   ├── main.go             # Root-Befehl
│   ├── serve.go            # Serve-Befehl
│   └── di/                 # Dependency Injection
│       └── providers.go    # Service-Verdrahtung
├── internal/
│   ├── config/             # Konfiguration laden
│   ├── providers/          # Provider-Implementierungen
│   │   ├── provider.go     # Provider-Schnittstelle
│   │   ├── base.go         # Basis-Provider
│   │   ├── anthropic.go    # Anthropic-Provider
│   │   ├── zai.go          # Z.AI-Provider
│   │   └── ollama.go       # Ollama-Provider
│   ├── proxy/              # HTTP-Proxy-Server
│   │   ├── handler.go      # Haupt-Request-Handler
│   │   ├── provider_proxy.go # Pro-Provider-Proxy
│   │   ├── thinking.go     # Thinking-Block-Verarbeitung
│   │   ├── signature_cache.go # Signatur-Caching
│   │   ├── sse.go          # SSE-Hilfsfunktionen
│   │   └── middleware.go   # Middleware-Kette
│   ├── router/             # Routing-Strategien
│   │   ├── router.go       # Router-Schnittstelle
│   │   ├── failover.go     # Failover-Strategie
│   │   ├── round_robin.go  # Round-Robin-Strategie
│   │   └── model_filter.go # Modellbasierte Filterung
│   ├── health/             # Health-Tracking
│   │   └── tracker.go      # Circuit Breaker
│   ├── keypool/            # API-Key-Pooling
│   │   └── keypool.go      # Key-Rotation
│   └── cache/              # Caching-Schicht
│       └── cache.go        # Ristretto-Wrapper
└── docs-site/              # Dokumentation
```

## Leistungsüberlegungen

### Verbindungshandling
- **Connection Pooling**: HTTP-Verbindungen zu Backends werden wiederverwendet
- **HTTP/2-Unterstützung**: Multiplexed Requests wo unterstützt
- **Sofortiges Flushing**: SSE-Events werden ohne Pufferung geflusht

### Nebenläufigkeit
- **Goroutine pro Anfrage**: Leichtgewichtige Go-Nebenläufigkeit
- **Context-Propagierung**: Korrektes Timeout und Abbruch
- **Thread-sicheres Caching**: Ristretto bietet nebenläufigen Zugriff

### Speicher
- **Streaming-Antworten**: Keine Pufferung von Response-Bodies
- **Signatur-Cache**: Begrenzte Größe mit LRU-Eviction
- **Request-Body-Wiederherstellung**: Effizientes Body-Neulesen

## Nächste Schritte

- [Konfigurationsreferenz](/docs/configuration/)
- [Routing-Strategien](/docs/routing/)
- [Provider-Einrichtung](/docs/providers/)
