---
title: Ueber uns
type: about
---

## Ueber CC-Relay

CC-Relay ist ein leistungsstarker HTTP-Proxy, geschrieben in Go, der Claude Code und anderen LLM-Clients die Verbindung zu mehreren Providern ueber einen einzigen Endpunkt ermoeglicht.

### Projektziele

- **Multi-Provider-Zugang vereinfachen** - Ein Proxy, mehrere Backends
- **API-Kompatibilitaet wahren** - Drop-in-Ersatz fuer direkten Anthropic API-Zugang
- **Flexibilitaet ermoeglichen** - Einfacher Provider-Wechsel ohne Client-Aenderungen
- **Claude Code unterstuetzen** - Erstklassige Integration mit Claude Code CLI

### Aktueller Status

CC-Relay befindet sich in aktiver Entwicklung. Folgende Funktionen sind implementiert:

- HTTP-Proxy-Server mit Anthropic API-Kompatibilitaet
- Anthropic und Z.AI Provider-Unterstuetzung
- Vollstaendige SSE-Streaming-Unterstuetzung
- API-Schluessel und Bearer-Token-Authentifizierung
- Mehrere API-Schluessel pro Provider
- Debug-Protokollierung fuer Request/Response-Inspektion
- Claude Code Konfigurationsbefehle

### Geplante Funktionen

- Zusaetzliche Provider (Ollama, AWS Bedrock, Azure, Vertex AI)
- Routing-Strategien (Round-Robin, Failover, kostenbasiert)
- Rate-Limiting pro API-Schluessel
- Circuit Breaker und Health-Tracking
- gRPC-Management-API
- TUI-Dashboard

### Entwickelt mit

- [Go](https://go.dev/) - Programmiersprache
- [Cobra](https://cobra.dev/) - CLI-Framework
- [zerolog](https://github.com/rs/zerolog) - Strukturierte Protokollierung

### Autor

Erstellt von [Omar Alani](https://github.com/omarluq)

### Lizenz

CC-Relay ist Open-Source-Software unter der [AGPL 3 Lizenz](https://github.com/omarluq/cc-relay/blob/main/LICENSE).

### Mitwirken

Beitraege sind willkommen! Besuchen Sie das [GitHub-Repository](https://github.com/omarluq/cc-relay) fuer:

- [Fehler melden](https://github.com/omarluq/cc-relay/issues)
- [Pull Requests einreichen](https://github.com/omarluq/cc-relay/pulls)
- [Diskussionen](https://github.com/omarluq/cc-relay/discussions)
