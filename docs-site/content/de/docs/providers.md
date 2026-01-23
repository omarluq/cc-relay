---
title: "Anbieter"
description: "Konfigurieren Sie Anthropic, Z.AI und Ollama Anbieter in cc-relay"
weight: 5
---

CC-Relay unterstuetzt mehrere LLM-Anbieter ueber eine einheitliche Schnittstelle. Diese Seite erklaert die Konfiguration jedes Anbieters.

## Ueberblick

CC-Relay fungiert als Proxy zwischen Claude Code und verschiedenen LLM-Backends. Alle Anbieter stellen eine Anthropic-kompatible Messages API bereit, was einen nahtlosen Wechsel zwischen Anbietern ermoeglicht.

| Anbieter | Typ | Beschreibung | Kosten |
|----------|-----|--------------|--------|
| Anthropic | `anthropic` | Direkter Anthropic API Zugang | Standard Anthropic Preise |
| Z.AI | `zai` | Zhipu AI GLM Modelle, Anthropic-kompatibel | ~1/7 der Anthropic Preise |
| Ollama | `ollama` | Lokale LLM Inferenz | Kostenlos (lokale Rechenleistung) |

**Kommt in Phase 6:** AWS Bedrock, Azure Foundry, Google Vertex AI

## Anthropic Anbieter

Der Anthropic Anbieter verbindet sich direkt mit Anthropics API. Dies ist der Standard-Anbieter fuer vollen Claude Modell-Zugang.

### Konfiguration

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Optional, verwendet Standard

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # Anfragen pro Minute
        tpm_limit: 100000    # Tokens pro Minute
        priority: 2          # Hoeher = wird zuerst bei Failover versucht

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### API-Schluessel Einrichtung

1. Erstellen Sie ein Konto unter [console.anthropic.com](https://console.anthropic.com)
2. Navigieren Sie zu Settings > API Keys
3. Erstellen Sie einen neuen API-Schluessel
4. Speichern Sie ihn als Umgebungsvariable: `export ANTHROPIC_API_KEY="sk-ant-..."`

### Transparente Auth Unterstuetzung

Der Anthropic Anbieter unterstuetzt transparente Authentifizierung fuer Claude Code Abonnement-Benutzer. Wenn aktiviert, leitet cc-relay Ihr Abonnement-Token unveraendert weiter:

```yaml
server:
  auth:
    allow_subscription: true
```

```bash
# Ihr Abonnement-Token wird unveraendert weitergeleitet
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

Siehe [Transparente Authentifizierung](/de/docs/configuration/#transparente-authentifizierung) fuer Details.

## Z.AI Anbieter

Z.AI (Zhipu AI) bietet GLM Modelle ueber eine Anthropic-kompatible API. Dies ermoeglicht erhebliche Kosteneinsparungen (~1/7 der Anthropic Preise) bei gleichzeitiger API-Kompatibilitaet.

### Konfiguration

```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # Optional, verwendet Standard

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Niedrigere Prioritaet als Anthropic fuer Failover

    # Claude Modellnamen auf Z.AI Modelle abbilden
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"
      "claude-haiku-3-5": "GLM-4.5-Air"

    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"
```

### API-Schluessel Einrichtung

1. Erstellen Sie ein Konto unter [open.bigmodel.cn](https://open.bigmodel.cn) (Zhipu AI Developer Portal)
2. Navigieren Sie zum API Keys Bereich
3. Erstellen Sie einen neuen API-Schluessel
4. Speichern Sie ihn als Umgebungsvariable: `export ZAI_API_KEY="..."`

### Model Mapping

Model Mapping uebersetzt Anthropic Modellnamen in Z.AI Aequivalente. Wenn Claude Code `claude-sonnet-4-5-20250514` anfordert, leitet cc-relay automatisch zu `GLM-4.7` weiter:

```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7 (Flaggschiff-Modell)
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air (schnell, wirtschaftlich)
  "claude-haiku-3-5-20241022": "GLM-4.5-Air"
  "claude-haiku-3-5": "GLM-4.5-Air"
```

### Kostenvergleich

| Modell | Anthropic (pro 1M Tokens) | Z.AI Aequivalent | Z.AI Kosten |
|--------|---------------------------|------------------|-------------|
| claude-sonnet-4-5 | $3 Input / $15 Output | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 Input / $1.25 Output | GLM-4.5-Air | ~$0.04 / $0.18 |

*Preise sind ungefaehr und koennen sich aendern.*

## Ollama Anbieter

Ollama ermoeglicht lokale LLM Inferenz ueber eine Anthropic-kompatible API (verfuegbar seit Ollama v0.14). Fuehren Sie Modelle lokal aus fuer Datenschutz, keine API-Kosten und Offline-Betrieb.

### Konfiguration

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # Optional, verwendet Standard

    keys:
      - key: "ollama"  # Ollama akzeptiert aber ignoriert API-Schluessel
        priority: 0    # Niedrigste Prioritaet fuer Failover

    # Claude Modellnamen auf lokale Ollama Modelle abbilden
    model_mapping:
      "claude-sonnet-4-5-20250514": "qwen3:32b"
      "claude-sonnet-4-5": "qwen3:32b"
      "claude-haiku-3-5-20241022": "qwen3:8b"
      "claude-haiku-3-5": "qwen3:8b"

    models:
      - "qwen3:32b"
      - "qwen3:8b"
      - "codestral:latest"
```

### Ollama Einrichtung

1. Installieren Sie Ollama von [ollama.com](https://ollama.com)
2. Laden Sie die gewuenschten Modelle herunter:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. Starten Sie Ollama (laeuft automatisch nach Installation)

### Empfohlene Modelle

Fuer Claude Code Workflows waehlen Sie Modelle mit mindestens 32K Kontext:

| Modell | Kontext | Groesse | Optimal fuer |
|--------|---------|---------|--------------|
| `qwen3:32b` | 128K | 32B Parameter | Allgemeines Coding, komplexe Argumentation |
| `qwen3:8b` | 128K | 8B Parameter | Schnelle Iteration, einfachere Aufgaben |
| `codestral:latest` | 32K | 22B Parameter | Code-Generierung, spezialisiertes Coding |
| `llama3.2:3b` | 128K | 3B Parameter | Sehr schnell, grundlegende Aufgaben |

### Funktionseinschraenkungen

Ollamas Anthropic-Kompatibilitaet ist teilweise. Einige Funktionen werden nicht unterstuetzt:

| Funktion | Unterstuetzt | Hinweise |
|----------|--------------|----------|
| Streaming (SSE) | Ja | Gleiche Event-Sequenz wie Anthropic |
| Tool Calling | Ja | Gleiches Format wie Anthropic |
| Extended Thinking | Teilweise | `budget_tokens` akzeptiert aber nicht durchgesetzt |
| Prompt Caching | Nein | `cache_control` Bloecke werden ignoriert |
| PDF Input | Nein | Nicht unterstuetzt |
| Image URLs | Nein | Nur Base64-Kodierung |
| Token Counting | Nein | `/v1/messages/count_tokens` nicht verfuegbar |
| `tool_choice` | Nein | Kann keine spezifische Tool-Nutzung erzwingen |

### Docker Networking

Bei cc-relay in Docker aber Ollama auf dem Host:

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # Dockers Host-Gateway anstelle von localhost verwenden
    base_url: "http://host.docker.internal:11434"
```

Alternativ cc-relay mit `--network host` ausfuehren:

```bash
docker run --network host cc-relay
```

## Model Mapping

Das `model_mapping` Feld uebersetzt eingehende Modellnamen in anbieter-spezifische Modelle:

```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # Format: "eingehendes-modell": "anbieter-modell"
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
```

Wenn Claude Code sendet:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-Relay leitet zu Z.AI weiter mit:
```json
{"model": "GLM-4.7", ...}
```

### Mapping Tipps

1. **Versions-Suffixe einbeziehen**: Sowohl `claude-sonnet-4-5` als auch `claude-sonnet-4-5-20250514` abbilden
2. **Kontextlaenge beruecksichtigen**: Modelle mit aehnlichen Faehigkeiten abgleichen
3. **Qualitaet testen**: Ausgabequalitaet entsprechend Ihren Anforderungen ueberpruefen

## Multi-Provider Setup

Konfigurieren Sie mehrere Anbieter fuer Failover, Kostenoptimierung oder Lastverteilung:

```yaml
providers:
  # Primaer: Anthropic (hoechste Qualitaet)
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Wird zuerst versucht

  # Sekundaer: Z.AI (kosteneffektiv)
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Fallback

  # Tertiaer: Ollama (lokal, kostenlos)
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # Letzte Option

routing:
  strategy: failover  # Anbieter in Prioritaetsreihenfolge versuchen
```

Mit dieser Konfiguration:
1. Anfragen gehen zuerst zu Anthropic (Prioritaet 2)
2. Wenn Anthropic fehlschlaegt (429, 5xx), Z.AI versuchen (Prioritaet 1)
3. Wenn Z.AI fehlschlaegt, Ollama versuchen (Prioritaet 0)

Siehe [Routing Strategien](/de/docs/routing/) fuer weitere Optionen.

## Fehlerbehebung

### Verbindung verweigert (Ollama)

**Symptom:** `connection refused` beim Verbinden mit Ollama

**Ursachen:**
- Ollama laeuft nicht
- Falscher Port
- Docker Networking Problem

**Loesungen:**
```bash
# Pruefen ob Ollama laeuft
ollama list

# Port verifizieren
curl http://localhost:11434/api/version

# Fuer Docker, Host-Gateway verwenden
base_url: "http://host.docker.internal:11434"
```

### Authentifizierung fehlgeschlagen (Z.AI)

**Symptom:** `401 Unauthorized` von Z.AI

**Ursachen:**
- Ungueltiger API-Schluessel
- Umgebungsvariable nicht gesetzt
- Schluessel nicht aktiviert

**Loesungen:**
```bash
# Umgebungsvariable pruefen
echo $ZAI_API_KEY

# Schluessel direkt testen
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### Modell nicht gefunden

**Symptom:** `model not found` Fehler

**Ursachen:**
- Modell nicht in `models` Liste konfiguriert
- Fehlender `model_mapping` Eintrag
- Modell nicht installiert (Ollama)

**Loesungen:**
```yaml
# Sicherstellen dass Modell gelistet ist
models:
  - "GLM-4.7"

# Sicherstellen dass Mapping existiert
model_mapping:
  "claude-sonnet-4-5": "GLM-4.7"
```

Fuer Ollama pruefen ob Modell installiert ist:
```bash
ollama list
ollama pull qwen3:32b
```

### Langsame Antwort (Ollama)

**Symptom:** Sehr langsame Antworten von Ollama

**Ursachen:**
- Modell zu gross fuer Hardware
- GPU wird nicht verwendet
- Ungenuegend RAM

**Loesungen:**
- Kleineres Modell verwenden (`qwen3:8b` anstatt `qwen3:32b`)
- GPU Aktivierung pruefen: `ollama run qwen3:8b --verbose`
- Speichernutzung waehrend der Inferenz pruefen

## Naechste Schritte

- [Konfigurationsreferenz](/de/docs/configuration/) - Vollstaendige Konfigurationsoptionen
- [Routing Strategien](/de/docs/routing/) - Anbieterauswahl und Failover
- [Gesundheitsueberwachung](/de/docs/health/) - Circuit Breaker und Health Checks
