---
title: Konfiguration
weight: 3
---

CC-Relay wird ueber YAML- oder TOML-Dateien konfiguriert. Diese Anleitung behandelt alle Konfigurationsoptionen.

## Speicherort der Konfigurationsdatei

Standard-Speicherorte (in dieser Reihenfolge geprueft):

1. `./config.yaml` oder `./config.toml` (aktuelles Verzeichnis)
2. `~/.config/cc-relay/config.yaml` oder `~/.config/cc-relay/config.toml`
3. Pfad angegeben ueber `--config` Flag

Das Format wird automatisch anhand der Dateierweiterung erkannt (`.yaml`, `.yml` oder `.toml`).

Erstellen Sie eine Standardkonfiguration mit:

```bash
cc-relay config init
```

## Umgebungsvariablen-Erweiterung

CC-Relay unterstuetzt Umgebungsvariablen-Erweiterung mit der `${VAR_NAME}` Syntax in beiden YAML- und TOML-Formaten:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # Wird beim Laden erweitert
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"  # Wird beim Laden erweitert
```
  {{< /tab >}}
{{< /tabs >}}

## Vollstaendige Konfigurationsreferenz

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# ==========================================================================
# Server-Konfiguration
# ==========================================================================
server:
  # Adresse auf der gehorcht wird
  listen: "127.0.0.1:8787"

  # Request-Timeout in Millisekunden (Standard: 600000 = 10 Minuten)
  timeout_ms: 600000

  # Maximale gleichzeitige Anfragen (0 = unbegrenzt)
  max_concurrent: 0

  # HTTP/2 fuer bessere Performance aktivieren
  enable_http2: true

  # Authentifizierungs-Konfiguration
  auth:
    # Spezifischen API-Schluessel fuer Proxy-Zugang erfordern
    api_key: "${PROXY_API_KEY}"

    # Claude Code Abonnement Bearer-Tokens erlauben
    allow_subscription: true

    # Spezifisches Bearer-Token zur Validierung (optional)
    bearer_secret: "${BEARER_SECRET}"

# ==========================================================================
# Provider-Konfigurationen
# ==========================================================================
providers:
  # Anthropic Direct API
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Optional, verwendet Standard

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60       # Anfragen pro Minute
        tpm_limit: 100000   # Tokens pro Minute

    # Optional: Verfuegbare Modelle angeben
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

    # Claude-Modellnamen auf Z.AI-Modelle mappen
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    # Optional: Verfuegbare Modelle angeben
    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"

# ==========================================================================
# Protokollierungs-Konfiguration
# ==========================================================================
logging:
  # Log-Level: debug, info, warn, error
  level: "info"

  # Log-Format: json, text
  format: "text"

  # Farbige Ausgabe aktivieren (fuer Text-Format)
  pretty: true

  # Granulare Debug-Optionen
  debug_options:
    log_request_body: false
    log_response_headers: false
    log_tls_metrics: false
    max_body_log_size: 1000

# ==========================================================================
# Cache-Konfiguration
# ==========================================================================
cache:
  # Cache-Modus: single, ha, disabled
  mode: single

  # Einzelmodus (Ristretto) Konfiguration
  ristretto:
    num_counters: 1000000  # 10x erwartete maximale Elemente
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Aufnahme-Puffergroesse

  # HA-Modus (Olric) Konfiguration
  olric:
    embedded: true                 # Eingebetteten Olric-Knoten ausfuehren
    bind_addr: "0.0.0.0:3320"      # Olric-Client-Port
    dmap_name: "cc-relay"          # Name der verteilten Map
    environment: lan               # local, lan, oder wan
    peers:                         # Memberlist-Adressen (bind_addr + 2)
      - "other-node:3322"
    replica_count: 2               # Kopien pro Schluessel
    read_quorum: 1                 # Min. Lesevorgaenge fuer Erfolg
    write_quorum: 1                # Min. Schreibvorgaenge fuer Erfolg
    member_count_quorum: 2         # Min. Cluster-Mitglieder
    leave_timeout: 5s              # Dauer der Leave-Nachricht

# ==========================================================================
# Routing-Konfiguration
# ==========================================================================
routing:
  # Strategie: round_robin, weighted_round_robin, shuffle, failover (Standard)
  strategy: failover

  # Timeout fuer Failover-Versuche in Millisekunden (Standard: 5000)
  failover_timeout: 5000

  # Debug-Header aktivieren (X-CC-Relay-Strategy, X-CC-Relay-Provider)
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

## Server-Konfiguration

### Listen-Adresse

Das `listen` Feld gibt an, wo der Proxy auf eingehende Anfragen horcht:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"  # Nur lokal (empfohlen)
  # listen: "0.0.0.0:8787"  # Alle Interfaces (mit Vorsicht verwenden)
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"  # Nur lokal (empfohlen)
# listen = "0.0.0.0:8787"  # Alle Interfaces (mit Vorsicht verwenden)
```
  {{< /tab >}}
{{< /tabs >}}

### Authentifizierung

CC-Relay unterstuetzt mehrere Authentifizierungsmethoden:

#### API-Schluessel-Authentifizierung

Clients muessen einen spezifischen API-Schluessel angeben:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
api_key = "${PROXY_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

Clients muessen den Header senden: `x-api-key: <ihr-proxy-schluessel>`

#### Claude Code Abonnement-Durchleitung

Claude Code Abonnenten koennen sich verbinden:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    allow_subscription: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
allow_subscription = true
```
  {{< /tab >}}
{{< /tabs >}}

Dies akzeptiert `Authorization: Bearer` Tokens von Claude Code.

#### Kombinierte Authentifizierung

Sowohl API-Schluessel als auch Abonnement-Authentifizierung erlauben:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
api_key = "${PROXY_API_KEY}"
allow_subscription = true
```
  {{< /tab >}}
{{< /tabs >}}

#### Keine Authentifizierung

Um die Authentifizierung zu deaktivieren (nicht fuer Produktion empfohlen):

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth: {}
  # Oder einfach den auth-Abschnitt weglassen
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
# auth-Abschnitt weglassen oder leer lassen
```
  {{< /tab >}}
{{< /tabs >}}

### HTTP/2-Unterstuetzung

HTTP/2 fuer bessere Performance bei gleichzeitigen Anfragen aktivieren:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  enable_http2: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
enable_http2 = true
```
  {{< /tab >}}
{{< /tabs >}}

## Provider-Konfiguration

### Provider-Typen

CC-Relay unterstuetzt derzeit zwei Provider-Typen:

| Typ | Beschreibung | Standard-Base-URL |
|-----|--------------|-------------------|
| `anthropic` | Anthropic Direct API | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Anthropic Provider

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Optional

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60
tpm_limit = 100000

models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]
```
  {{< /tab >}}
{{< /tabs >}}

### Z.AI Provider

Z.AI bietet Anthropic-kompatible APIs mit GLM-Modellen zu niedrigeren Kosten:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]
```
  {{< /tab >}}
{{< /tabs >}}

### Mehrere API-Schluessel

Mehrere API-Schluessel fuer hoeheren Durchsatz poolen:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_1}"
rpm_limit = 60
tpm_limit = 100000

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_2}"
rpm_limit = 60
tpm_limit = 100000

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_3}"
rpm_limit = 60
tpm_limit = 100000
```
  {{< /tab >}}
{{< /tabs >}}

### Benutzerdefinierte Base-URL

Den Standard-API-Endpunkt ueberschreiben:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic-custom"
type = "anthropic"
base_url = "https://custom-endpoint.example.com"
```
  {{< /tab >}}
{{< /tabs >}}

## Protokollierungs-Konfiguration

### Log-Level

| Level | Beschreibung |
|-------|--------------|
| `debug` | Ausfuehrliche Ausgabe fuer Entwicklung |
| `info` | Normale Betriebsmeldungen |
| `warn` | Warnmeldungen |
| `error` | Nur Fehlermeldungen |

### Log-Format

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
logging:
  format: "text"   # Menschenlesbar (Standard)
  # format: "json" # Maschinenlesbar, fuer Log-Aggregation
```
  {{< /tab >}}
  {{< tab >}}
```toml
[logging]
format = "text"   # Menschenlesbar (Standard)
# format = "json" # Maschinenlesbar, fuer Log-Aggregation
```
  {{< /tab >}}
{{< /tabs >}}

### Debug-Optionen

Feinkoernige Kontrolle ueber Debug-Protokollierung:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # Request-Bodies protokollieren (maskiert)
    log_response_headers: true  # Response-Header protokollieren
    log_tls_metrics: true       # TLS-Verbindungsinfo protokollieren
    max_body_log_size: 1000     # Max. Bytes aus Bodies protokollieren
```
  {{< /tab >}}
  {{< tab >}}
```toml
[logging]
level = "debug"

[logging.debug_options]
log_request_body = true      # Request-Bodies protokollieren (maskiert)
log_response_headers = true  # Response-Header protokollieren
log_tls_metrics = true       # TLS-Verbindungsinfo protokollieren
max_body_log_size = 1000     # Max. Bytes aus Bodies protokollieren
```
  {{< /tab >}}
{{< /tabs >}}

## Cache-Konfiguration

CC-Relay bietet eine einheitliche Caching-Schicht mit mehreren Backend-Optionen fuer verschiedene Einsatzszenarien.

### Cache-Modi

| Modus | Backend | Beschreibung |
|-------|---------|--------------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | Hochleistungs-lokaler In-Memory-Cache (Standard) |
| `ha` | [Olric](https://github.com/buraksezer/olric) | Verteilter Cache fuer Hochverfuegbarkeitsbereitstellungen |
| `disabled` | Noop | Durchleitungsmodus ohne Caching |

### Einzelmodus (Ristretto)

Ristretto ist ein hochleistungsfaehiger, nebenlaeufiger In-Memory-Cache. Dies ist der Standardmodus fuer Einzelinstanz-Bereitstellungen.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x erwartete maximale Elemente
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Aufnahme-Puffergroesse
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "single"

[cache.ristretto]
num_counters = 1000000  # 10x erwartete maximale Elemente
max_cost = 104857600    # 100 MB
buffer_items = 64       # Aufnahme-Puffergroesse
```
  {{< /tab >}}
{{< /tabs >}}

| Feld | Typ | Standard | Beschreibung |
|------|-----|----------|--------------|
| `num_counters` | int64 | 1.000.000 | Anzahl der 4-Bit-Zugriffszaehler. Empfohlen: 10x erwartete maximale Elemente. |
| `max_cost` | int64 | 104.857.600 (100 MB) | Maximaler Speicher in Bytes, den der Cache halten kann. |
| `buffer_items` | int64 | 64 | Anzahl der Schluessel pro Get-Puffer. Steuert die Aufnahme-Puffergroesse. |

### HA-Modus (Olric) - Eingebettet

Fuer Multi-Instanz-Bereitstellungen, die gemeinsamen Cache-Zustand erfordern, verwenden Sie den eingebetteten Olric-Modus, bei dem jede cc-relay-Instanz einen Olric-Knoten ausfuehrt.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "other-node:3322"  # Memberlist-Port = bind_addr + 2
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "ha"

[cache.olric]
embedded = true
bind_addr = "0.0.0.0:3320"
dmap_name = "cc-relay"
environment = "lan"
peers = ["other-node:3322"]  # Memberlist-Port = bind_addr + 2
replica_count = 2
read_quorum = 1
write_quorum = 1
member_count_quorum = 2
leave_timeout = "5s"
```
  {{< /tab >}}
{{< /tabs >}}

| Feld | Typ | Standard | Beschreibung |
|------|-----|----------|--------------|
| `embedded` | bool | false | Eingebetteten Olric-Knoten ausfuehren (true) vs. mit externem Cluster verbinden (false). |
| `bind_addr` | string | erforderlich | Adresse fuer Olric-Client-Verbindungen (z.B. "0.0.0.0:3320"). |
| `dmap_name` | string | "cc-relay" | Name der verteilten Map. Alle Knoten muessen denselben Namen verwenden. |
| `environment` | string | "local" | Memberlist-Preset: "local", "lan" oder "wan". |
| `peers` | []string | - | Memberlist-Adressen fuer Peer-Erkennung. Verwendet Port bind_addr + 2. |
| `replica_count` | int | 1 | Anzahl der Kopien pro Schluessel. 1 = keine Replikation. |
| `read_quorum` | int | 1 | Minimale erfolgreiche Lesevorgaenge fuer Antwort. |
| `write_quorum` | int | 1 | Minimale erfolgreiche Schreibvorgaenge fuer Antwort. |
| `member_count_quorum` | int32 | 1 | Minimale Cluster-Mitglieder erforderlich zum Betrieb. |
| `leave_timeout` | duration | 5s | Zeit zum Senden der Leave-Nachricht vor dem Herunterfahren. |

**Wichtig:** Olric verwendet zwei Ports - den `bind_addr`-Port fuer Client-Verbindungen und `bind_addr + 2` fuer Memberlist-Gossip. Stellen Sie sicher, dass beide Ports in Ihrer Firewall geoeffnet sind.

### HA-Modus (Olric) - Client-Modus

Verbinden Sie sich mit einem externen Olric-Cluster anstatt eingebettete Knoten auszufuehren:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "ha"

[cache.olric]
embedded = false
addresses = ["olric-node-1:3320", "olric-node-2:3320"]
dmap_name = "cc-relay"
```
  {{< /tab >}}
{{< /tabs >}}

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `embedded` | bool | Auf `false` setzen fuer Client-Modus. |
| `addresses` | []string | Externe Olric-Cluster-Adressen. |
| `dmap_name` | string | Name der verteilten Map (muss mit Cluster-Konfiguration uebereinstimmen). |

### Deaktivierter Modus

Caching vollstaendig deaktivieren fuer Debugging oder wenn Caching anderswo behandelt wird:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: disabled
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "disabled"
```
  {{< /tab >}}
{{< /tabs >}}

Fuer umfassende Cache-Dokumentation einschliesslich Cache-Schluessel-Konventionen, Cache-Busting-Strategien, HA-Clustering-Anleitungen und Fehlerbehebung, siehe die [Cache-System-Dokumentation](/de/docs/caching/).

## Routing-Konfiguration

CC-Relay unterstuetzt mehrere Routing-Strategien zur Verteilung von Anfragen auf Provider.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# ==========================================================================
# Routing-Konfiguration
# ==========================================================================
routing:
  # Strategie: round_robin, weighted_round_robin, shuffle, failover (Standard)
  strategy: failover

  # Timeout fuer Failover-Versuche in Millisekunden (Standard: 5000)
  failover_timeout: 5000

  # Debug-Header aktivieren (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# Routing-Konfiguration
# ==========================================================================
[routing]
# Strategie: round_robin, weighted_round_robin, shuffle, failover (Standard)
strategy = "failover"

# Timeout fuer Failover-Versuche in Millisekunden (Standard: 5000)
failover_timeout = 5000

# Debug-Header aktivieren (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

### Routing-Strategien

| Strategie | Beschreibung |
|-----------|--------------|
| `failover` | Provider in Prioritaetsreihenfolge versuchen, bei Fehlschlag Fallback (Standard) |
| `round_robin` | Sequentielle Rotation durch Provider |
| `weighted_round_robin` | Proportionale Verteilung nach Gewichtung |
| `shuffle` | Faire Zufallsverteilung |

### Provider-Gewichtung und -Prioritaet

Gewichtung und Prioritaet werden im ersten Schluessel des Providers konfiguriert:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # Fuer weighted-round-robin (hoeher = mehr Traffic)
        priority: 2    # Fuer failover (hoeher = wird zuerst versucht)
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
weight = 3      # Fuer weighted-round-robin (hoeher = mehr Traffic)
priority = 2    # Fuer failover (hoeher = wird zuerst versucht)
```
  {{< /tab >}}
{{< /tabs >}}

Fuer detaillierte Routing-Konfiguration einschliesslich Strategie-Erklaerungen, Debug-Header und Failover-Ausloeser, siehe die [Routing-Dokumentation](/de/docs/routing/).

## Beispielkonfigurationen

### Minimaler Einzel-Provider

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

### Multi-Provider-Setup

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

### Entwicklung mit Debug-Protokollierung

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

## Konfiguration validieren

Ihre Konfigurationsdatei validieren:

```bash
cc-relay config validate
```

**Tipp**: Validieren Sie Konfigurationsaenderungen immer vor dem Deployment. Hot-Reload wird ungueltige Konfigurationen ablehnen, aber die Validierung erkennt Fehler bevor sie die Produktion erreichen.

## Hot-Reloading

CC-Relay erkennt und wendet Konfigurationsaenderungen automatisch an, ohne dass ein Neustart erforderlich ist. Dies ermoeglicht Konfigurationsaktualisierungen ohne Ausfallzeit.

### Funktionsweise

CC-Relay verwendet [fsnotify](https://github.com/fsnotify/fsnotify) zur Ueberwachung der Konfigurationsdatei:

1. **Dateiueberwachung**: Das uebergeordnete Verzeichnis wird ueberwacht, um atomare Schreibvorgaenge korrekt zu erkennen (Temp-Datei + Umbenennung, wie von den meisten Editoren verwendet)
2. **Entprellung**: Mehrere schnelle Dateiereignisse werden mit einer 100ms Verzoegerung zusammengefasst, um das Speicherverhalten von Editoren zu handhaben
3. **Atomarer Austausch**: Neue Konfiguration wird geladen und atomar mit Go's `sync/atomic.Pointer` ausgetauscht
4. **Erhaltung laufender Anfragen**: Anfragen in Bearbeitung verwenden weiterhin die alte Konfiguration; neue Anfragen verwenden die aktualisierte Konfiguration

### Ereignisse, die ein Neuladen ausloesen

| Ereignis | Loest Neuladen aus |
|----------|-------------------|
| Datei schreiben | Ja |
| Datei erstellen (atomares Umbenennen) | Ja |
| Datei chmod | Nein (ignoriert) |
| Andere Datei im Verzeichnis | Nein (ignoriert) |

### Protokollierung

Bei Hot-Reload sehen Sie Log-Nachrichten:

```
INF config file reloaded path=/path/to/config.yaml
INF config hot-reloaded successfully
```

Bei ungueltiger Konfiguration:

```
ERR failed to reload config path=/path/to/config.yaml error="validation error"
```

Ungueltige Konfigurationen werden abgelehnt und der Proxy laeuft mit der vorherigen gueltigen Konfiguration weiter.

### Einschraenkungen

- **Listen-Adresse**: Aendern von `server.listen` erfordert einen Neustart
- **gRPC-Adresse**: Aendern von `grpc.listen` erfordert einen Neustart

Konfigurationsoptionen, die hot-reloadbar sind:
- Logging-Level und Format
- Routing-Strategie, Failover-Timeout, Gewichtungen und Prioritaeten
- Provider-Aktivierung, Base-URL und Model-Mapping
- Keypool-Strategie, Key-Gewichte und Limits pro Key
- Maximale gleichzeitige Requests und maximale Body-Groesse
- Health-Check-Intervalle und Circuit-Breaker-Schwellenwerte

## Naechste Schritte

- [Routing-Strategien](/de/docs/routing/) - Provider-Auswahl und Failover
- [Die Architektur verstehen](/de/docs/architecture/)
- [API-Referenz](/de/docs/api/)
