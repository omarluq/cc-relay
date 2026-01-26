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
| AWS Bedrock | `bedrock` | Claude ueber AWS mit SigV4 Authentifizierung | AWS Bedrock Preise |
| Azure AI Foundry | `azure` | Claude ueber Azure MAAS | Azure AI Preise |
| Google Vertex AI | `vertex` | Claude ueber Google Cloud | Vertex AI Preise |

## Anthropic Anbieter

Der Anthropic Anbieter verbindet sich direkt mit Anthropics API. Dies ist der Standard-Anbieter fuer vollen Claude Modell-Zugang.

### Konfiguration

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Optional, uses default

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60        # Requests per minute
tpm_limit = 100000    # Tokens per minute
priority = 2          # Higher = tried first in failover

models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]
```
  {{< /tab >}}
{{< /tabs >}}

### API-Schluessel Einrichtung

1. Erstellen Sie ein Konto unter [console.anthropic.com](https://console.anthropic.com)
2. Navigieren Sie zu Settings > API Keys
3. Erstellen Sie einen neuen API-Schluessel
4. Speichern Sie ihn als Umgebungsvariable: `export ANTHROPIC_API_KEY="sk-ant-..."`

### Transparente Auth Unterstuetzung

Der Anthropic Anbieter unterstuetzt transparente Authentifizierung fuer Claude Code Abonnement-Benutzer. Wenn aktiviert, leitet cc-relay Ihr Abonnement-Token unveraendert weiter:

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

```bash
# Ihr Abonnement-Token wird unveraendert weitergeleitet
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

Siehe [Transparente Authentifizierung](/de/docs/configuration/#transparente-authentifizierung) fuer Details.

## Z.AI Anbieter

Z.AI (Zhipu AI) bietet GLM Modelle ueber eine Anthropic-kompatible API. Dies ermoeglicht erhebliche Kosteneinsparungen (~1/7 der Anthropic Preise) bei gleichzeitiger API-Kompatibilitaet.

### Konfiguration

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"  # Optional, uses default

[[providers.keys]]
key = "${ZAI_API_KEY}"
priority = 1  # Lower priority than Anthropic for failover

# Map Claude model names to Z.AI models
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-sonnet-4-5" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"
"claude-haiku-3-5" = "GLM-4.5-Air"

models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]
```
  {{< /tab >}}
{{< /tabs >}}

### API-Schluessel Einrichtung

1. Erstellen Sie ein Konto unter [z.ai/model-api](https://z.ai/model-api)
2. Navigieren Sie zum API Keys Bereich
3. Erstellen Sie einen neuen API-Schluessel
4. Speichern Sie ihn als Umgebungsvariable: `export ZAI_API_KEY="..."`

> **10% Rabatt:** Verwenden Sie [diesen Einladungslink](https://z.ai/subscribe?ic=HT5TQVSOZP) bei der Anmeldung — sowohl Sie als auch der Empfehlende erhalten 10% Rabatt.

### Model Mapping

Model Mapping uebersetzt Anthropic Modellnamen in Z.AI Aequivalente. Wenn Claude Code `claude-sonnet-4-5-20250514` anfordert, leitet cc-relay automatisch zu `GLM-4.7` weiter:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7 (Flaggschiff-Modell)
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air (schnell, wirtschaftlich)
  "claude-haiku-3-5-20241022": "GLM-4.5-Air"
  "claude-haiku-3-5": "GLM-4.5-Air"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[model_mapping]
# Claude Sonnet -> GLM-4.7 (flagship model)
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-sonnet-4-5" = "GLM-4.7"

# Claude Haiku -> GLM-4.5-Air (fast, economical)
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"
"claude-haiku-3-5" = "GLM-4.5-Air"
```
  {{< /tab >}}
{{< /tabs >}}

### Kostenvergleich

| Modell | Anthropic (pro 1M Tokens) | Z.AI Aequivalent | Z.AI Kosten |
|--------|---------------------------|------------------|-------------|
| claude-sonnet-4-5 | $3 Input / $15 Output | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 Input / $1.25 Output | GLM-4.5-Air | ~$0.04 / $0.18 |

*Preise sind ungefaehr und koennen sich aendern.*

## Ollama Anbieter

Ollama ermoeglicht lokale LLM Inferenz ueber eine Anthropic-kompatible API (verfuegbar seit Ollama v0.14). Fuehren Sie Modelle lokal aus fuer Datenschutz, keine API-Kosten und Offline-Betrieb.

### Konfiguration

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "ollama"
type = "ollama"
enabled = true
base_url = "http://localhost:11434"  # Optional, uses default

[[providers.keys]]
key = "ollama"  # Ollama accepts but ignores API keys
priority = 0    # Lowest priority for failover

# Map Claude model names to local Ollama models
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "qwen3:32b"
"claude-sonnet-4-5" = "qwen3:32b"
"claude-haiku-3-5-20241022" = "qwen3:8b"
"claude-haiku-3-5" = "qwen3:8b"

models = [
  "qwen3:32b",
  "qwen3:8b",
  "codestral:latest"
]
```
  {{< /tab >}}
{{< /tabs >}}

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

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # Dockers Host-Gateway anstelle von localhost verwenden
    base_url: "http://host.docker.internal:11434"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "ollama"
type = "ollama"
# Use Docker's host gateway instead of localhost
base_url = "http://host.docker.internal:11434"
```
  {{< /tab >}}
{{< /tabs >}}

Alternativ cc-relay mit `--network host` ausfuehren:

```bash
docker run --network host cc-relay
```

## AWS Bedrock Anbieter

AWS Bedrock bietet Claude Zugang ueber Amazon Web Services mit Enterprise-Sicherheit und SigV4 Authentifizierung.

### Konfiguration

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "bedrock"
    type: "bedrock"
    enabled: true

    # AWS region (required)
    aws_region: "us-east-1"

    # Explicit AWS credentials (optional)
    # If not set, uses AWS SDK default credential chain:
    # 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
    # 2. Shared credentials file (~/.aws/credentials)
    # 3. IAM role (EC2, ECS, Lambda)
    aws_access_key_id: "${AWS_ACCESS_KEY_ID}"
    aws_secret_access_key: "${AWS_SECRET_ACCESS_KEY}"

    # Map Claude model names to Bedrock model IDs
    model_mapping:
      "claude-sonnet-4-5-20250514": "anthropic.claude-sonnet-4-5-20250514-v1:0"
      "claude-sonnet-4-5": "anthropic.claude-sonnet-4-5-20250514-v1:0"
      "claude-haiku-3-5-20241022": "anthropic.claude-haiku-3-5-20241022-v1:0"

    keys:
      - key: "bedrock-internal"  # Internal key for cc-relay auth
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "bedrock"
type = "bedrock"
enabled = true

# AWS region (required)
aws_region = "us-east-1"

# Explicit AWS credentials (optional)
# If not set, uses AWS SDK default credential chain:
# 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
# 2. Shared credentials file (~/.aws/credentials)
# 3. IAM role (EC2, ECS, Lambda)
aws_access_key_id = "${AWS_ACCESS_KEY_ID}"
aws_secret_access_key = "${AWS_SECRET_ACCESS_KEY}"

# Map Claude model names to Bedrock model IDs
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "anthropic.claude-sonnet-4-5-20250514-v1:0"
"claude-sonnet-4-5" = "anthropic.claude-sonnet-4-5-20250514-v1:0"
"claude-haiku-3-5-20241022" = "anthropic.claude-haiku-3-5-20241022-v1:0"

[[providers.keys]]
key = "bedrock-internal"  # Internal key for cc-relay auth
```
  {{< /tab >}}
{{< /tabs >}}

### AWS Setup

1. **Enable Bedrock Access**: In AWS Console, navigate to Bedrock > Model access and enable Claude models
2. **Configure Credentials**: Use one of these methods:
   - **Environment Variables**: `export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...`
   - **AWS CLI**: `aws configure`
   - **IAM Role**: Attach Bedrock access policy to EC2/ECS/Lambda role

### Bedrock Model IDs

**Note:** Model IDs change frequently as AWS Bedrock adds new Claude versions. Verify the current list in [AWS Bedrock model access documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/models-supported.html) before deploying.

Bedrock uses a specific model ID format: `anthropic.{model}-v{version}:{minor}`

| Claude Model | Bedrock Model ID |
|--------------|------------------|
| claude-sonnet-4-5-20250514 | `anthropic.claude-sonnet-4-5-20250514-v1:0` |
| claude-opus-4-5-20250514 | `anthropic.claude-opus-4-5-20250514-v1:0` |
| claude-haiku-3-5-20241022 | `anthropic.claude-haiku-3-5-20241022-v1:0` |

### Event Stream Conversion

Bedrock returns responses in AWS Event Stream format. CC-Relay automatically converts this to SSE format for Claude Code compatibility. No additional configuration is needed.

## Azure AI Foundry Anbieter

Azure AI Foundry bietet Claude Zugang ueber Microsoft Azure mit Enterprise Azure Integration.

### Konfiguration

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "azure"
    type: "azure"
    enabled: true

    # Your Azure resource name (appears in URL: {name}.services.ai.azure.com)
    azure_resource_name: "my-azure-resource"

    # Azure API version (default: 2024-06-01)
    azure_api_version: "2024-06-01"

    # Azure uses x-api-key authentication (Anthropic-compatible)
    keys:
      - key: "${AZURE_API_KEY}"

    # Map Claude model names to Azure deployment names
    model_mapping:
      "claude-sonnet-4-5-20250514": "claude-sonnet-4-5"
      "claude-sonnet-4-5": "claude-sonnet-4-5"
      "claude-haiku-3-5": "claude-haiku-3-5"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "azure"
type = "azure"
enabled = true

# Your Azure resource name (appears in URL: {name}.services.ai.azure.com)
azure_resource_name = "my-azure-resource"

# Azure API version (default: 2024-06-01)
azure_api_version = "2024-06-01"

# Azure uses x-api-key authentication (Anthropic-compatible)
[[providers.keys]]
key = "${AZURE_API_KEY}"

# Map Claude model names to Azure deployment names
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "claude-sonnet-4-5"
"claude-sonnet-4-5" = "claude-sonnet-4-5"
"claude-haiku-3-5" = "claude-haiku-3-5"
```
  {{< /tab >}}
{{< /tabs >}}

### Azure Setup

1. **Create Azure AI Resource**: In Azure Portal, create an Azure AI Foundry resource
2. **Deploy Claude Model**: Deploy a Claude model in your AI Foundry workspace
3. **Get API Key**: Copy the API key from Keys and Endpoint section
4. **Note Resource Name**: Your URL is `https://{resource_name}.services.ai.azure.com`

### Deployment Names

Azure uses deployment names as model identifiers. Create deployments in Azure AI Foundry, then map them:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
model_mapping:
  "claude-sonnet-4-5": "my-sonnet-deployment"  # Your deployment name
```
  {{< /tab >}}
  {{< tab >}}
```toml
[model_mapping]
"claude-sonnet-4-5" = "my-sonnet-deployment"  # Your deployment name
```
  {{< /tab >}}
{{< /tabs >}}

## Google Vertex AI Anbieter

Vertex AI bietet Claude Zugang ueber Google Cloud mit nahtloser GCP Integration.

### Konfiguration

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "vertex"
    type: "vertex"
    enabled: true

    # Google Cloud project ID (required)
    gcp_project_id: "${GOOGLE_CLOUD_PROJECT}"

    # Google Cloud region (required)
    gcp_region: "us-east5"

    # Map Claude model names to Vertex AI model IDs
    model_mapping:
      "claude-sonnet-4-5-20250514": "claude-sonnet-4-5@20250514"
      "claude-sonnet-4-5": "claude-sonnet-4-5@20250514"
      "claude-haiku-3-5-20241022": "claude-haiku-3-5@20241022"

    keys:
      - key: "vertex-internal"  # Internal key for cc-relay auth
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "vertex"
type = "vertex"
enabled = true

# Google Cloud project ID (required)
gcp_project_id = "${GOOGLE_CLOUD_PROJECT}"

# Google Cloud region (required)
gcp_region = "us-east5"

# Map Claude model names to Vertex AI model IDs
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "claude-sonnet-4-5@20250514"
"claude-sonnet-4-5" = "claude-sonnet-4-5@20250514"
"claude-haiku-3-5-20241022" = "claude-haiku-3-5@20241022"

[[providers.keys]]
key = "vertex-internal"  # Internal key for cc-relay auth
```
  {{< /tab >}}
{{< /tabs >}}

### GCP Setup

1. **Enable Vertex AI API**: In GCP Console, enable the Vertex AI API
2. **Request Claude Access**: Request access to Claude models through Vertex AI Model Garden
3. **Configure Authentication**: Use one of these methods:
   - **Application Default Credentials**: `gcloud auth application-default login`
   - **Service Account**: Set `GOOGLE_APPLICATION_CREDENTIALS` environment variable
   - **GCE/GKE**: Uses attached service account automatically

### Vertex AI Model IDs

Vertex AI uses `{model}@{version}` format:

| Claude Model | Vertex AI Model ID |
|--------------|-------------------|
| claude-sonnet-4-5-20250514 | `claude-sonnet-4-5@20250514` |
| claude-opus-4-5-20250514 | `claude-opus-4-5@20250514` |
| claude-haiku-3-5-20241022 | `claude-haiku-3-5@20241022` |

### Regions

Available regions for Claude on Vertex AI (check [Google Cloud documentation](https://cloud.google.com/vertex-ai/docs/general/locations) for the complete current list):
- `us-east5` (default)
- `us-central1`
- `europe-west1`

## Cloud Provider Comparison

| Feature | Bedrock | Azure | Vertex AI |
|---------|---------|-------|-----------|
| Authentication | SigV4 (AWS) | API Key | OAuth2 (GCP) |
| Streaming Format | Event Stream | SSE | SSE |
| Body Transform | Yes | No | Yes |
| Model in URL | Yes | No | Yes |
| Enterprise SSO | AWS IAM | Entra ID | GCP IAM |
| Regions | US, EU, APAC | Global | US, EU |

## Model Mapping

Das `model_mapping` Feld uebersetzt eingehende Modellnamen in anbieter-spezifische Modelle:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # Format: "eingehendes-modell": "anbieter-modell"
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "zai"
type = "zai"

[providers.model_mapping]
# Format: "incoming-model" = "provider-model"
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-sonnet-4-5" = "GLM-4.7"
```
  {{< /tab >}}
{{< /tabs >}}

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

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
# Primary: Anthropic (highest quality)
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
priority = 2  # Tried first

# Secondary: Z.AI (cost-effective)
[[providers]]
name = "zai"
type = "zai"
enabled = true

[[providers.keys]]
key = "${ZAI_API_KEY}"
priority = 1  # Fallback

# Tertiary: Ollama (local, free)
[[providers]]
name = "ollama"
type = "ollama"
enabled = true

[[providers.keys]]
key = "ollama"
priority = 0  # Last resort

[routing]
strategy = "failover"  # Try providers in priority order
```
  {{< /tab >}}
{{< /tabs >}}

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

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# Sicherstellen dass Modell gelistet ist
models:
  - "GLM-4.7"

# Sicherstellen dass Mapping existiert
model_mapping:
  "claude-sonnet-4-5": "GLM-4.7"
```
  {{< /tab >}}
  {{< tab >}}
```toml
# Ensure model is listed
models = ["GLM-4.7"]

# Ensure mapping exists
[model_mapping]
"claude-sonnet-4-5" = "GLM-4.7"
```
  {{< /tab >}}
{{< /tabs >}}

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
