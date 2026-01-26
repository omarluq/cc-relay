---
title: Routing
weight: 4
---

CC-Relay unterstuetzt mehrere Routing-Strategien zur Verteilung von Anfragen auf verschiedene Provider. Diese Seite erklaert jede Strategie und wie sie konfiguriert wird.

## Ueberblick

Routing bestimmt, wie cc-relay entscheidet, welcher Provider jede Anfrage bearbeitet. Die richtige Strategie haengt von Ihren Prioritaeten ab: Verfuegbarkeit, Kosten, Latenz oder Lastverteilung.

| Strategie | Konfigurationswert | Beschreibung | Anwendungsfall |
|-----------|-------------------|--------------|----------------|
| Round-Robin | `round_robin` | Sequentielle Rotation durch Provider | Gleichmaessige Verteilung |
| Weighted Round-Robin | `weighted_round_robin` | Proportionale Verteilung nach Gewichtung | Kapazitaetsbasierte Verteilung |
| Shuffle | `shuffle` | Faire Zufallsverteilung ("Karten austeilen") | Randomisierter Lastausgleich |
| Failover | `failover` (Standard) | Prioritaetsbasiert mit automatischem Retry | Hohe Verfuegbarkeit |
| Model-Based | `model_based` | Routing nach Modellname-Praefix | Multi-Modell-Deployments |

## Konfiguration

Konfigurieren Sie das Routing in Ihrer Konfigurationsdatei:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  # Strategie: round_robin, weighted_round_robin, shuffle, failover (Standard), model_based
  strategy: failover

  # Timeout fuer Failover-Versuche in Millisekunden (Standard: 5000)
  failover_timeout: 5000

  # Debug-Header aktivieren (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false

  # Model-based Routing Konfiguration (nur verwendet wenn strategy: model_based)
  model_mapping:
    claude-opus: anthropic    # claude-opus-* models → anthropic provider
    claude-sonnet: anthropic  # claude-sonnet-* models → anthropic provider
    glm-4: zai                # glm-4* models → zai provider
    qwen: ollama              # qwen* models → ollama provider

  # Default provider when no model mapping matches
  default_provider: anthropic
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
# Strategy: round_robin, weighted_round_robin, shuffle, failover (default), model_based
strategy = "failover"

# Timeout for failover attempts in milliseconds (default: 5000)
failover_timeout = 5000

# Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false

# Default provider when no model mapping matches
default_provider = "anthropic"

# Model-based routing configuration (only used when strategy: model_based)
[routing.model_mapping]
claude-opus = "anthropic"    # claude-opus-* models → anthropic provider
claude-sonnet = "anthropic"  # claude-sonnet-* models → anthropic provider
glm-4 = "zai"                # glm-4* models → zai provider
qwen = "ollama"              # qwen* models → ollama provider
```
  {{< /tab >}}
{{< /tabs >}}

**Standard:** Wenn `strategy` nicht angegeben ist, verwendet cc-relay `failover` als sicherste Option.

## Strategien

### Round-Robin

Sequentielle Verteilung mit einem atomaren Zaehler. Jeder Provider erhaelt eine Anfrage, bevor ein Provider eine zweite erhaelt.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: round_robin
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "round_robin"
```
  {{< /tab >}}
{{< /tabs >}}

**Funktionsweise:**

1. Anfrage 1 → Provider A
2. Anfrage 2 → Provider B
3. Anfrage 3 → Provider C
4. Anfrage 4 → Provider A (Zyklus wiederholt sich)

**Optimal fuer:** Gleichmaessige Verteilung auf Provider mit aehnlicher Kapazitaet.

### Weighted Round-Robin

Verteilt Anfragen proportional basierend auf Provider-Gewichtungen. Verwendet den Nginx Smooth Weighted Round-Robin Algorithmus fuer gleichmaessige Verteilung.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3  # Erhaelt 3x mehr Anfragen

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        weight: 1  # Erhaelt 1x Anfragen
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "weighted_round_robin"

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
weight = 3  # Receives 3x more requests

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"
weight = 1  # Receives 1x requests
```
  {{< /tab >}}
{{< /tabs >}}

**Funktionsweise:**

Mit Gewichtungen 3:1, von je 4 Anfragen:
- 3 Anfragen → anthropic
- 1 Anfrage → zai

**Standardgewichtung:** 1 (wenn nicht angegeben)

**Optimal fuer:** Lastverteilung basierend auf Provider-Kapazitaet, Rate-Limits oder Kostenzuweisung.

### Shuffle

Faire Zufallsverteilung mit dem Fisher-Yates "Karten austeilen" Muster. Jeder erhaelt eine Karte, bevor jemand eine zweite erhaelt.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: shuffle
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "shuffle"
```
  {{< /tab >}}
{{< /tabs >}}

**Funktionsweise:**

1. Alle Provider beginnen in einem "Kartenstapel"
2. Zufaelliger Provider wird ausgewaehlt und aus dem Stapel entfernt
3. Wenn der Stapel leer ist, alle Provider neu mischen
4. Garantiert faire Verteilung ueber Zeit

**Optimal fuer:** Randomisierter Lastausgleich bei gleichzeitiger Gewaehrleistung von Fairness.

### Failover

Versucht Provider in Prioritaetsreihenfolge. Bei Fehlschlag werden parallele Rennen mit den verbleibenden Providern gestartet fuer die schnellste erfolgreiche Antwort. Dies ist die **Standardstrategie**.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Wird zuerst versucht (hoeher = hoehere Prioritaet)

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Fallback
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "failover"

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
priority = 2  # Tried first (higher = higher priority)

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"
priority = 1  # Fallback
```
  {{< /tab >}}
{{< /tabs >}}

**Funktionsweise:**

1. Versucht zuerst den Provider mit hoechster Prioritaet
2. Bei Fehlschlag (siehe [Failover-Ausloeser](#failover-ausloeser)), parallele Anfragen an alle verbleibenden Provider starten
3. Erste erfolgreiche Antwort zurueckgeben, andere abbrechen
4. Beachtet `failover_timeout` fuer die Gesamtoperationsdauer

**Standardprioritaet:** 1 (wenn nicht angegeben)

**Optimal fuer:** Hohe Verfuegbarkeit mit automatischem Fallback.

### Model-Based

Leitet Anfragen basierend auf dem Modellnamen in der Anfrage an Provider weiter. Verwendet Longest-Prefix-Matching fuer Spezifitaet.

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: model_based

  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
    llama: ollama

  default_provider: anthropic

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"

  - name: "ollama"
    type: "ollama"
    base_url: "http://localhost:11434"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "model_based"
default_provider = "anthropic"

[routing.model_mapping]
claude-opus = "anthropic"
claude-sonnet = "anthropic"
glm-4 = "zai"
qwen = "ollama"
llama = "ollama"

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"

[[providers]]
name = "ollama"
type = "ollama"
base_url = "http://localhost:11434"
```
  {{< /tab >}}
{{< /tabs >}}

**Funktionsweise:**

1. Extrahiert den `model` Parameter aus der Anfrage
2. Versucht das laengste Praefix-Match in `model_mapping` zu finden
3. Leitet an den entsprechenden Provider weiter
4. Faellt auf `default_provider` zurueck, wenn kein Match gefunden wird
5. Wenn weder Match noch Standard existiert, werden alle Provider beibehalten (kein Filtering / Graceful Fallback)

**Beispiele fuer Praefix-Matching:**

| Angefordertes Modell | Mapping-Eintraege | Ausgewaehlter Eintrag | Provider |
|---------------------|-------------------|---------------------|----------|
| `claude-opus-4` | `claude-opus`, `claude` | `claude-opus` | anthropic |
| `claude-sonnet-3.5` | `claude-sonnet`, `claude` | `claude-sonnet` | anthropic |
| `glm-4-plus` | `glm-4`, `glm` | `glm-4` | zai |
| `qwen-72b` | `qwen`, `claude` | `qwen` | ollama |
| `llama-3.2` | `llama`, `claude` | `llama` | ollama |
| `gpt-4` | `claude`, `llama` | (kein Match) | default_provider |

**Optimal fuer:** Multi-Modell-Deployments, bei denen verschiedene Modelle an unterschiedliche Provider weitergeleitet werden muessen.

## Debug-Header

Wenn `routing.debug: true`, fuegt cc-relay Diagnose-Header zu Antworten hinzu:

| Header | Wert | Beschreibung |
|--------|------|--------------|
| `X-CC-Relay-Strategy` | Strategiename | Welche Routing-Strategie verwendet wurde |
| `X-CC-Relay-Provider` | Provider-Name | Welcher Provider die Anfrage bearbeitet hat |

**Beispiel-Antwort-Header:**

```
X-CC-Relay-Strategy: failover
X-CC-Relay-Provider: anthropic
```

**Sicherheitswarnung:** Debug-Header offenbaren interne Routing-Entscheidungen. Nur in Entwicklungs- oder vertrauenswuerdigen Umgebungen verwenden. Niemals in Produktion mit nicht vertrauenswuerdigen Clients aktivieren.

## Failover-Ausloeser

Die Failover-Strategie loest einen Retry bei bestimmten Fehlerbedingungen aus:

| Ausloeser | Bedingungen | Beschreibung |
|-----------|-------------|--------------|
| Statuscode | `429`, `500`, `502`, `503`, `504` | Rate-Limit oder Server-Fehler |
| Timeout | `context.DeadlineExceeded` | Anfrage-Timeout ueberschritten |
| Verbindung | `net.Error` | Netzwerkfehler, DNS-Fehler, Verbindung abgelehnt |

**Wichtig:** Client-Fehler (4xx ausser 429) loesen **keinen** Failover aus. Diese weisen auf Probleme mit der Anfrage selbst hin, nicht mit dem Provider.

### Statuscodes erklaert

| Code | Bedeutung | Failover? |
|------|-----------|-----------|
| `429` | Rate-Limit erreicht | Ja - anderen Provider versuchen |
| `500` | Interner Serverfehler | Ja - Serverproblem |
| `502` | Bad Gateway | Ja - Upstream-Problem |
| `503` | Service nicht verfuegbar | Ja - voruebergehend nicht erreichbar |
| `504` | Gateway Timeout | Ja - Upstream-Timeout |
| `400` | Ungueltige Anfrage | Nein - Anfrage korrigieren |
| `401` | Nicht autorisiert | Nein - Authentifizierung korrigieren |
| `403` | Verboten | Nein - Berechtigungsproblem |

## Beispiele

### Einfacher Failover (Empfohlen fuer die meisten Benutzer)

Verwenden Sie die Standardstrategie mit priorisierten Providern:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "failover"

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
priority = 2

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"
priority = 1
```
  {{< /tab >}}
{{< /tabs >}}

### Lastverteilung mit Gewichtungen

Last basierend auf Provider-Kapazitaet verteilen:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "primary"
    type: "anthropic"
    keys:
      - key: "${PRIMARY_KEY}"
        weight: 3  # 75% des Traffics

  - name: "secondary"
    type: "anthropic"
    keys:
      - key: "${SECONDARY_KEY}"
        weight: 1  # 25% des Traffics
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "weighted_round_robin"

[[providers]]
name = "primary"
type = "anthropic"

[[providers.keys]]
key = "${PRIMARY_KEY}"
weight = 3  # 75% of traffic

[[providers]]
name = "secondary"
type = "anthropic"

[[providers.keys]]
key = "${SECONDARY_KEY}"
weight = 1  # 25% of traffic
```
  {{< /tab >}}
{{< /tabs >}}

### Entwicklung mit Debug-Headern

Debug-Header fuer Fehlerbehebung aktivieren:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "round_robin"
debug = true

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

### Hohe Verfuegbarkeit mit schnellem Failover

Failover-Latenz minimieren:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: failover
  failover_timeout: 3000  # 3 Sekunden Timeout

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
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "failover"
failover_timeout = 3000  # 3 second timeout

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
priority = 2

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"
priority = 1
```
  {{< /tab >}}
{{< /tabs >}}

### Multi-Modell mit Model-Based Routing

Verschiedene Modelle an spezialisierte Provider weiterleiten:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
routing:
  strategy: model_based

  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    claude-haiku: anthropic
    glm-4: zai
    glm-3: zai
    qwen: ollama
    llama: ollama

  default_provider: anthropic

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"

  - name: "ollama"
    type: "ollama"
    base_url: "http://localhost:11434"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[routing]
strategy = "model_based"
default_provider = "anthropic"

[routing.model_mapping]
claude-opus = "anthropic"
claude-sonnet = "anthropic"
claude-haiku = "anthropic"
glm-4 = "zai"
glm-3 = "zai"
qwen = "ollama"
llama = "ollama"

[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"

[[providers.keys]]
key = "${ZAI_API_KEY}"

[[providers]]
name = "ollama"
type = "ollama"
base_url = "http://localhost:11434"
```
  {{< /tab >}}
{{< /tabs >}}

Mit dieser Konfiguration:
- Claude Modelle → Anthropic
- GLM Modelle → Z.AI
- Qwen/Llama Modelle → Ollama (lokal)
- Andere Modelle → Anthropic (Standard)

## Provider-Gewichtung und -Prioritaet

Gewichtung und Prioritaet werden in der Schluessel-Konfiguration des Providers angegeben:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "example"
    type: "anthropic"
    keys:
      - key: "${API_KEY}"
        weight: 3      # Fuer weighted-round-robin (hoeher = mehr Traffic)
        priority: 2    # Fuer failover (hoeher = wird zuerst versucht)
        rpm_limit: 60  # Rate-Limit-Tracking
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "example"
type = "anthropic"

[[providers.keys]]
key = "${API_KEY}"
weight = 3      # For weighted-round-robin (higher = more traffic)
priority = 2    # For failover (higher = tried first)
rpm_limit = 60  # Rate limit tracking
```
  {{< /tab >}}
{{< /tabs >}}

**Hinweis:** Gewichtung und Prioritaet werden vom **ersten Schluessel** in der Schluesselliste des Providers gelesen.

## Naechste Schritte

- [Konfigurationsreferenz](/de/docs/configuration/) - Vollstaendige Konfigurationsoptionen
- [Architektur-Uebersicht](/de/docs/architecture/) - Wie cc-relay intern funktioniert
