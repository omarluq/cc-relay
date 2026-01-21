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

- [Die Architektur verstehen](/de/docs/architecture/)
- [API-Referenz](/de/docs/api/)
