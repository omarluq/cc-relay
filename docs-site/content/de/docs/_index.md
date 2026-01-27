---
title: Dokumentation
weight: 1
---

Willkommen zur CC-Relay-Dokumentation! Diese Anleitung hilft Ihnen bei der Einrichtung, Konfiguration und Nutzung von CC-Relay als Multi-Provider-Proxy fuer Claude Code und andere LLM-Clients.

## Was ist CC-Relay?

CC-Relay ist ein leistungsstarker HTTP-Proxy, geschrieben in Go, der zwischen LLM-Clients (wie Claude Code) und LLM-Providern vermittelt. Er bietet:

- **Multi-Provider-Unterstuetzung**: Anthropic und Z.AI (weitere Provider geplant)
- **Anthropic API-kompatibel**: Drop-in-Ersatz fuer direkten API-Zugang
- **SSE-Streaming**: Vollstaendige Unterstuetzung fuer Streaming-Antworten
- **Mehrere Authentifizierungsmethoden**: API-Schluessel und Bearer-Token-Unterstuetzung
- **Claude Code Integration**: Einfache Einrichtung mit integriertem Konfigurationsbefehl

## Aktueller Status

CC-Relay befindet sich in aktiver Entwicklung. Aktuell implementierte Funktionen:

| Funktion | Status |
|----------|--------|
| HTTP-Proxy-Server | Implementiert |
| Anthropic Provider | Implementiert |
| Z.AI Provider | Implementiert |
| SSE-Streaming | Implementiert |
| API-Schluessel-Authentifizierung | Implementiert |
| Bearer-Token (Abonnement) Auth | Implementiert |
| Claude Code Konfiguration | Implementiert |
| Mehrere API-Schluessel | Implementiert |
| Debug-Protokollierung | Implementiert |

**Geplante Funktionen:**
- Routing-Strategien (Round-Robin, Failover, kostenbasiert)
- Rate-Limiting pro API-Schluessel
- Circuit Breaker und Health-Tracking
- gRPC-Management-API
- TUI-Dashboard
- Zusaetzliche Provider (Ollama, Bedrock, Azure, Vertex)

## Schnellstart

```bash
# Installieren
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest

# Konfiguration initialisieren
cc-relay config init

# API-Schluessel setzen
export ANTHROPIC_API_KEY="ihr-schluessel-hier"

# Proxy starten
cc-relay serve

# Claude Code konfigurieren (in einem anderen Terminal)
cc-relay config cc init
```

## Schnellnavigation

- [Erste Schritte](/de/docs/getting-started/) - Installation und erster Start
- [Konfiguration](/de/docs/configuration/) - Provider-Einrichtung und Optionen
- [Architektur](/de/docs/architecture/) - Systemdesign und Komponenten
- [API-Referenz](/de/docs/api/) - HTTP-Endpunkte und Beispiele

## Dokumentationsabschnitte

### Erste Schritte
- [Installation](/de/docs/getting-started/#installation)
- [Schnellstart](/de/docs/getting-started/#schnellstart)
- [CLI-Befehle](/de/docs/getting-started/#cli-befehle)
- [Testen mit Claude Code](/de/docs/getting-started/#testen-mit-claude-code)
- [Fehlerbehebung](/de/docs/getting-started/#fehlerbehebung)

### Konfiguration
- [Server-Konfiguration](/de/docs/configuration/#server-konfiguration)
- [Provider-Konfiguration](/de/docs/configuration/#provider-konfiguration)
- [Authentifizierung](/de/docs/configuration/#authentifizierung)
- [Protokollierungs-Konfiguration](/de/docs/configuration/#protokollierungs-konfiguration)
- [Beispielkonfigurationen](/de/docs/configuration/#beispielkonfigurationen)

### Architektur
- [Systemuebersicht](/de/docs/architecture/#systemuebersicht)
- [Kernkomponenten](/de/docs/architecture/#kernkomponenten)
- [Request-Ablauf](/de/docs/architecture/#request-ablauf)
- [SSE-Streaming](/de/docs/architecture/#sse-streaming)
- [Authentifizierungsablauf](/de/docs/architecture/#authentifizierungsablauf)

### API-Referenz
- [POST /v1/messages](/de/docs/api/#post-v1messages)
- [GET /v1/models](/de/docs/api/#get-v1models)
- [GET /v1/providers](/de/docs/api/#get-v1providers)
- [GET /health](/de/docs/api/#get-health)
- [Client-Beispiele](/de/docs/api/#curl-beispiele)

## Brauchen Sie Hilfe?

- [Problem melden](https://github.com/omarluq/cc-relay/issues)
- [Diskussionen](https://github.com/omarluq/cc-relay/discussions)
