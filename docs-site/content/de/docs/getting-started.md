---
title: Erste Schritte
weight: 2
---

Diese Anleitung fuehrt Sie durch die Installation, Konfiguration und den ersten Start von CC-Relay.

## Voraussetzungen

- **Go 1.21+** zum Bauen aus dem Quellcode
- **API-Schluessel** fuer mindestens einen unterstuetzten Provider (Anthropic oder Z.AI)
- **Claude Code** CLI zum Testen (optional)

## Installation

### Mit Go Install

```bash
go install github.com/omarluq/cc-relay@latest
```

Die Binaerdatei wird unter `$GOPATH/bin/cc-relay` oder `$HOME/go/bin/cc-relay` installiert.

### Aus dem Quellcode bauen

```bash
# Repository klonen
git clone https://github.com/omarluq/cc-relay.git
cd cc-relay

# Mit Task bauen (empfohlen)
task build

# Oder manuell bauen
go build -o cc-relay ./cmd/cc-relay

# Ausfuehren
./cc-relay --help
```

### Vorgefertigte Binaerdateien

Laden Sie vorgefertigte Binaerdateien von der [Releases-Seite](https://github.com/omarluq/cc-relay/releases) herunter.

## Schnellstart

### 1. Konfiguration initialisieren

CC-Relay kann eine Standard-Konfigurationsdatei fuer Sie erstellen:

```bash
cc-relay config init
```

Dies erstellt eine Konfigurationsdatei unter `~/.config/cc-relay/config.yaml` mit sinnvollen Standardwerten.

### 2. Umgebungsvariablen setzen

```bash
export ANTHROPIC_API_KEY="ihr-api-schluessel-hier"

# Optional: Falls Sie Z.AI verwenden
export ZAI_API_KEY="ihr-zai-schluessel-hier"
```

### 3. CC-Relay starten

```bash
cc-relay serve
```

Sie sollten eine Ausgabe wie diese sehen:

```
INF starting cc-relay listen=127.0.0.1:8787
INF using primary provider provider=anthropic-pool type=anthropic
```

### 4. Claude Code konfigurieren

Der einfachste Weg, Claude Code fuer CC-Relay zu konfigurieren:

```bash
cc-relay config cc init
```

Dies aktualisiert automatisch `~/.claude/settings.json` mit der Proxy-Konfiguration.

Alternativ koennen Sie Umgebungsvariablen manuell setzen:

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_AUTH_TOKEN="managed-by-cc-relay"
claude
```

## Funktionspruefung

### Serverstatus pruefen

```bash
cc-relay status
```

Ausgabe:
```
✓ cc-relay is running (127.0.0.1:8787)
```

### Health-Endpunkt testen

```bash
curl http://localhost:8787/health
```

Antwort:
```json
{"status":"ok"}
```

### Verfuegbare Modelle auflisten

```bash
curl http://localhost:8787/v1/models
```

### Anfrage testen

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hallo!"}
    ]
  }'
```

## CLI-Befehle

CC-Relay bietet mehrere CLI-Befehle:

| Befehl | Beschreibung |
|--------|--------------|
| `cc-relay serve` | Proxy-Server starten |
| `cc-relay status` | Pruefen ob Server laeuft |
| `cc-relay config init` | Standard-Konfigurationsdatei erstellen |
| `cc-relay config cc init` | Claude Code fuer cc-relay konfigurieren |
| `cc-relay config cc remove` | cc-relay-Konfiguration aus Claude Code entfernen |
| `cc-relay --version` | Versionsinformationen anzeigen |

### Serve-Befehl-Optionen

```bash
cc-relay serve [flags]

Flags:
  --config string      Pfad zur Konfigurationsdatei (Standard: ~/.config/cc-relay/config.yaml)
  --log-level string   Log-Level (debug, info, warn, error)
  --log-format string  Log-Format (json, text)
  --debug              Debug-Modus aktivieren (ausfuehrliche Protokollierung)
```

## Minimale Konfiguration

Hier ist eine minimale funktionsfaehige Konfiguration:

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

## Naechste Schritte

- [Mehrere Provider konfigurieren](/de/docs/configuration/)
- [Die Architektur verstehen](/de/docs/architecture/)
- [API-Referenz](/de/docs/api/)

## Fehlerbehebung

### Port bereits belegt

Wenn Port 8787 bereits belegt ist, aendern Sie die Listen-Adresse in Ihrer Konfiguration:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8788"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8788"
```
  {{< /tab >}}
{{< /tabs >}}

### Provider antwortet nicht

Pruefen Sie die Server-Logs auf Verbindungsfehler:

```bash
cc-relay serve --log-level debug
```

### Authentifizierungsfehler

Wenn Sie "authentication failed"-Fehler sehen:

1. Ueberpruefen Sie, ob Ihr API-Schluessel korrekt in den Umgebungsvariablen gesetzt ist
2. Pruefen Sie, ob die Konfigurationsdatei die richtige Umgebungsvariable referenziert
3. Stellen Sie sicher, dass der API-Schluessel beim Provider gueltig ist

### Debug-Modus

Aktivieren Sie den Debug-Modus fuer detaillierte Request/Response-Protokollierung:

```bash
cc-relay serve --debug
```

Dies aktiviert:
- Debug-Log-Level
- Request-Body-Protokollierung (sensible Felder werden maskiert)
- Response-Header-Protokollierung
- TLS-Verbindungsmetriken
