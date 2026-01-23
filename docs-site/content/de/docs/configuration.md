---
title: Konfiguration
weight: 3
---

CC-Relay wird ueber YAML-Dateien konfiguriert. Diese Anleitung behandelt alle Konfigurationsoptionen.

## Speicherort der Konfigurationsdatei

Standard-Speicherorte (in dieser Reihenfolge geprueft):

1. `./config.yaml` (aktuelles Verzeichnis)
2. `~/.config/cc-relay/config.yaml`
3. Pfad angegeben ueber `--config` Flag

Erstellen Sie eine Standardkonfiguration mit:

```bash
cc-relay config init
```

## Umgebungsvariablen-Erweiterung

CC-Relay unterstuetzt Umgebungsvariablen-Erweiterung mit der `${VAR_NAME}` Syntax:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # Wird beim Laden erweitert
```

## Vollstaendige Konfigurationsreferenz

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

## Server-Konfiguration

### Listen-Adresse

Das `listen` Feld gibt an, wo der Proxy auf eingehende Anfragen horcht:

```yaml
server:
  listen: "127.0.0.1:8787"  # Nur lokal (empfohlen)
  # listen: "0.0.0.0:8787"  # Alle Interfaces (mit Vorsicht verwenden)
```

### Authentifizierung

CC-Relay unterstuetzt mehrere Authentifizierungsmethoden:

#### API-Schluessel-Authentifizierung

Clients muessen einen spezifischen API-Schluessel angeben:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```

Clients muessen den Header senden: `x-api-key: <ihr-proxy-schluessel>`

#### Claude Code Abonnement-Durchleitung

Claude Code Abonnenten koennen sich verbinden:

```yaml
server:
  auth:
    allow_subscription: true
```

Dies akzeptiert `Authorization: Bearer` Tokens von Claude Code.

#### Kombinierte Authentifizierung

Sowohl API-Schluessel als auch Abonnement-Authentifizierung erlauben:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```

#### Keine Authentifizierung

Um die Authentifizierung zu deaktivieren (nicht fuer Produktion empfohlen):

```yaml
server:
  auth: {}
  # Oder einfach den auth-Abschnitt weglassen
```

### HTTP/2-Unterstuetzung

HTTP/2 fuer bessere Performance bei gleichzeitigen Anfragen aktivieren:

```yaml
server:
  enable_http2: true
```

## Provider-Konfiguration

### Provider-Typen

CC-Relay unterstuetzt derzeit zwei Provider-Typen:

| Typ | Beschreibung | Standard-Base-URL |
|-----|--------------|-------------------|
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

Z.AI bietet Anthropic-kompatible APIs mit GLM-Modellen zu niedrigeren Kosten:

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

### Mehrere API-Schluessel

Mehrere API-Schluessel fuer hoeheren Durchsatz poolen:

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

### Benutzerdefinierte Base-URL

Den Standard-API-Endpunkt ueberschreiben:

```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```

## Protokollierungs-Konfiguration

### Log-Level

| Level | Beschreibung |
|-------|--------------|
| `debug` | Ausfuehrliche Ausgabe fuer Entwicklung |
| `info` | Normale Betriebsmeldungen |
| `warn` | Warnmeldungen |
| `error` | Nur Fehlermeldungen |

### Log-Format

```yaml
logging:
  format: "text"   # Menschenlesbar (Standard)
  # format: "json" # Maschinenlesbar, fuer Log-Aggregation
```

### Debug-Optionen

Feinkoernige Kontrolle ueber Debug-Protokollierung:

```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # Request-Bodies protokollieren (maskiert)
    log_response_headers: true  # Response-Header protokollieren
    log_tls_metrics: true       # TLS-Verbindungsinfo protokollieren
    max_body_log_size: 1000     # Max. Bytes aus Bodies protokollieren
```

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

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x erwartete maximale Elemente
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Aufnahme-Puffergroesse
```

| Feld | Typ | Standard | Beschreibung |
|------|-----|----------|--------------|
| `num_counters` | int64 | 1.000.000 | Anzahl der 4-Bit-Zugriffszaehler. Empfohlen: 10x erwartete maximale Elemente. |
| `max_cost` | int64 | 104.857.600 (100 MB) | Maximaler Speicher in Bytes, den der Cache halten kann. |
| `buffer_items` | int64 | 64 | Anzahl der Schluessel pro Get-Puffer. Steuert die Aufnahme-Puffergroesse. |

### HA-Modus (Olric) - Eingebettet

Fuer Multi-Instanz-Bereitstellungen, die gemeinsamen Cache-Zustand erfordern, verwenden Sie den eingebetteten Olric-Modus, bei dem jede cc-relay-Instanz einen Olric-Knoten ausfuehrt.

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

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `embedded` | bool | Auf `false` setzen fuer Client-Modus. |
| `addresses` | []string | Externe Olric-Cluster-Adressen. |
| `dmap_name` | string | Name der verteilten Map (muss mit Cluster-Konfiguration uebereinstimmen). |

### Deaktivierter Modus

Caching vollstaendig deaktivieren fuer Debugging oder wenn Caching anderswo behandelt wird:

```yaml
cache:
  mode: disabled
```

Fuer umfassende Cache-Dokumentation einschliesslich Cache-Schluessel-Konventionen, Cache-Busting-Strategien, HA-Clustering-Anleitungen und Fehlerbehebung, siehe die [Cache-System-Dokumentation](/de/docs/caching/).

## Routing-Konfiguration

CC-Relay unterstuetzt mehrere Routing-Strategien zur Verteilung von Anfragen auf Provider.

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

### Routing-Strategien

| Strategie | Beschreibung |
|-----------|--------------|
| `failover` | Provider in Prioritaetsreihenfolge versuchen, bei Fehlschlag Fallback (Standard) |
| `round_robin` | Sequentielle Rotation durch Provider |
| `weighted_round_robin` | Proportionale Verteilung nach Gewichtung |
| `shuffle` | Faire Zufallsverteilung |

### Provider-Gewichtung und -Prioritaet

Gewichtung und Prioritaet werden im ersten Schluessel des Providers konfiguriert:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # Fuer weighted-round-robin (hoeher = mehr Traffic)
        priority: 2    # Fuer failover (hoeher = wird zuerst versucht)
```

Fuer detaillierte Routing-Konfiguration einschliesslich Strategie-Erklaerungen, Debug-Header und Failover-Ausloeser, siehe die [Routing-Dokumentation](/de/docs/routing/).

## Beispielkonfigurationen

### Minimaler Einzel-Provider

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

### Multi-Provider-Setup

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

### Entwicklung mit Debug-Protokollierung

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

## Konfiguration validieren

Ihre Konfigurationsdatei validieren:

```bash
cc-relay config validate
```

## Hot-Reloading

Konfigurationsaenderungen erfordern einen Server-Neustart. Hot-Reloading ist fuer eine zukuenftige Version geplant.

## Naechste Schritte

- [Routing-Strategien](/de/docs/routing/) - Provider-Auswahl und Failover
- [Die Architektur verstehen](/de/docs/architecture/)
- [API-Referenz](/de/docs/api/)
