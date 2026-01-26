---
title: "Proveedores"
description: "Configure proveedores Anthropic, Z.AI y Ollama en cc-relay"
weight: 5
---

CC-Relay soporta multiples proveedores de LLM a traves de una interfaz unificada. Esta pagina explica como configurar cada proveedor.

## Descripcion General

CC-Relay actua como un proxy entre Claude Code y varios backends de LLM. Todos los proveedores exponen una API de Messages compatible con Anthropic, permitiendo un cambio fluido entre proveedores.

| Proveedor | Tipo | Descripcion | Costo |
|-----------|------|-------------|-------|
| Anthropic | `anthropic` | Acceso directo a la API de Anthropic | Precios estandar de Anthropic |
| Z.AI | `zai` | Modelos GLM de Zhipu AI, compatible con Anthropic | ~1/7 del precio de Anthropic |
| Ollama | `ollama` | Inferencia LLM local | Gratis (computo local) |
| AWS Bedrock | `bedrock` | Claude via AWS con autenticacion SigV4 | Precios AWS Bedrock |
| Azure AI Foundry | `azure` | Claude via Azure MAAS | Precios Azure AI |
| Google Vertex AI | `vertex` | Claude via Google Cloud | Precios Vertex AI |

## Proveedor Anthropic

El proveedor Anthropic se conecta directamente a la API de Anthropic. Este es el proveedor predeterminado para acceso completo a los modelos Claude.

### Configuracion

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Opcional, usa predeterminado

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # Solicitudes por minuto
        tpm_limit: 100000    # Tokens por minuto
        priority: 2          # Mayor = se intenta primero en failover

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

### Configuracion de API Key

1. Cree una cuenta en [console.anthropic.com](https://console.anthropic.com)
2. Navegue a Settings > API Keys
3. Cree una nueva API key
4. Almacene en variable de entorno: `export ANTHROPIC_API_KEY="sk-ant-..."`

### Soporte de Auth Transparente

El proveedor Anthropic soporta autenticacion transparente para usuarios con suscripcion a Claude Code. Cuando esta habilitado, cc-relay reenvia su token de suscripcion sin cambios:

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
# Su token de suscripcion se reenvia sin cambios
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

Vea [Autenticacion Transparente](/es/docs/configuration/#autenticacion-transparente) para detalles.

## Proveedor Z.AI

Z.AI (Zhipu AI) ofrece modelos GLM a traves de una API compatible con Anthropic. Esto proporciona ahorros significativos (~1/7 del precio de Anthropic) mientras mantiene compatibilidad con la API.

### Configuracion

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # Opcional, usa predeterminado

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Menor prioridad que Anthropic para failover

    # Mapear nombres de modelos Claude a modelos Z.AI
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

### Configuracion de API Key

1. Cree una cuenta en [z.ai/model-api](https://z.ai/model-api)
2. Navegue a la seccion de API Keys
3. Cree una nueva API key
4. Almacene en variable de entorno: `export ZAI_API_KEY="..."`

> **Obtenga 10% de descuento:** Use [este enlace de invitacion](https://z.ai/subscribe?ic=HT5TQVSOZP) al suscribirse — tanto usted como el referente obtienen 10% de descuento.

### Model Mapping

El model mapping traduce nombres de modelos Anthropic a equivalentes de Z.AI. Cuando Claude Code solicita `claude-sonnet-4-5-20250514`, cc-relay automaticamente redirige a `GLM-4.7`:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7 (modelo insignia)
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air (rapido, economico)
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

### Comparacion de Costos

| Modelo | Anthropic (por 1M tokens) | Equivalente Z.AI | Costo Z.AI |
|--------|---------------------------|------------------|------------|
| claude-sonnet-4-5 | $3 entrada / $15 salida | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 entrada / $1.25 salida | GLM-4.5-Air | ~$0.04 / $0.18 |

*Los precios son aproximados y pueden cambiar.*

## Proveedor Ollama

Ollama permite inferencia LLM local a traves de una API compatible con Anthropic (disponible desde Ollama v0.14). Ejecute modelos localmente para privacidad, sin costos de API y operacion offline.

### Configuracion

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # Opcional, usa predeterminado

    keys:
      - key: "ollama"  # Ollama acepta pero ignora API keys
        priority: 0    # Menor prioridad para failover

    # Mapear nombres de modelos Claude a modelos Ollama locales
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

### Configuracion de Ollama

1. Instale Ollama desde [ollama.com](https://ollama.com)
2. Descargue los modelos que desea usar:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. Inicie Ollama (se ejecuta automaticamente al instalar)

### Modelos Recomendados

Para flujos de trabajo de Claude Code, elija modelos con al menos 32K de contexto:

| Modelo | Contexto | Tamano | Mejor Para |
|--------|----------|--------|------------|
| `qwen3:32b` | 128K | 32B params | Codificacion general, razonamiento complejo |
| `qwen3:8b` | 128K | 8B params | Iteracion rapida, tareas simples |
| `codestral:latest` | 32K | 22B params | Generacion de codigo, codificacion especializada |
| `llama3.2:3b` | 128K | 3B params | Muy rapido, tareas basicas |

### Limitaciones de Funciones

La compatibilidad de Ollama con Anthropic es parcial. Algunas funciones no estan soportadas:

| Funcion | Soportado | Notas |
|---------|-----------|-------|
| Streaming (SSE) | Si | Misma secuencia de eventos que Anthropic |
| Tool calling | Si | Mismo formato que Anthropic |
| Extended thinking | Parcial | `budget_tokens` aceptado pero no aplicado |
| Prompt caching | No | bloques `cache_control` ignorados |
| Entrada PDF | No | No soportado |
| URLs de imagen | No | Solo codificacion Base64 |
| Conteo de tokens | No | `/v1/messages/count_tokens` no disponible |
| `tool_choice` | No | No puede forzar uso de herramienta especifica |

### Docker Networking

Al ejecutar cc-relay en Docker pero Ollama en el host:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # Usar gateway del host de Docker en lugar de localhost
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

Alternativamente, ejecute cc-relay con `--network host`:

```bash
docker run --network host cc-relay
```

## Proveedor AWS Bedrock

AWS Bedrock proporciona acceso a Claude a traves de Amazon Web Services con seguridad empresarial y autenticacion SigV4.

### Configuracion

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

## Proveedor Azure AI Foundry

Azure AI Foundry proporciona acceso a Claude a traves de Microsoft Azure con integracion empresarial de Azure.

### Configuracion

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

## Proveedor Google Vertex AI

Vertex AI proporciona acceso a Claude a traves de Google Cloud con integracion GCP nativa.

### Configuracion

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

El campo `model_mapping` traduce nombres de modelos entrantes a modelos especificos del proveedor:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # Formato: "modelo-entrante": "modelo-proveedor"
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

Cuando Claude Code envia:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-Relay redirige a Z.AI con:
```json
{"model": "GLM-4.7", ...}
```

### Consejos de Mapping

1. **Incluir sufijos de version**: Mapear tanto `claude-sonnet-4-5` como `claude-sonnet-4-5-20250514`
2. **Considerar longitud de contexto**: Emparejar modelos con capacidades similares
3. **Probar calidad**: Verificar que la calidad de salida cumple sus necesidades

## Configuracion Multi-Proveedor

Configure multiples proveedores para failover, optimizacion de costos o distribucion de carga:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  # Primario: Anthropic (mayor calidad)
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Se intenta primero

  # Secundario: Z.AI (economico)
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Respaldo

  # Terciario: Ollama (local, gratis)
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # Ultimo recurso

routing:
  strategy: failover  # Intentar proveedores en orden de prioridad
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

Con esta configuracion:
1. Las solicitudes van primero a Anthropic (prioridad 2)
2. Si Anthropic falla (429, 5xx), intentar Z.AI (prioridad 1)
3. Si Z.AI falla, intentar Ollama (prioridad 0)

Vea [Estrategias de Routing](/es/docs/routing/) para mas opciones.

## Solucion de Problemas

### Conexion Rechazada (Ollama)

**Sintoma:** `connection refused` al conectar con Ollama

**Causas:**
- Ollama no esta corriendo
- Puerto incorrecto
- Problema de Docker networking

**Soluciones:**
```bash
# Verificar si Ollama esta corriendo
ollama list

# Verificar puerto
curl http://localhost:11434/api/version

# Para Docker, usar gateway del host
base_url: "http://host.docker.internal:11434"
```

### Autenticacion Fallida (Z.AI)

**Sintoma:** `401 Unauthorized` de Z.AI

**Causas:**
- API key invalida
- Variable de entorno no configurada
- Key no activada

**Soluciones:**
```bash
# Verificar variable de entorno
echo $ZAI_API_KEY

# Probar key directamente
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### Modelo No Encontrado

**Sintoma:** errores `model not found`

**Causas:**
- Modelo no configurado en lista `models`
- Entrada faltante en `model_mapping`
- Modelo no instalado (Ollama)

**Soluciones:**

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# Asegurar que el modelo este listado
models:
  - "GLM-4.7"

# Asegurar que el mapping existe
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

Para Ollama, verificar que el modelo este instalado:
```bash
ollama list
ollama pull qwen3:32b
```

### Respuesta Lenta (Ollama)

**Sintoma:** Respuestas muy lentas de Ollama

**Causas:**
- Modelo muy grande para el hardware
- GPU no esta siendo usada
- RAM insuficiente

**Soluciones:**
- Usar modelo mas pequeno (`qwen3:8b` en lugar de `qwen3:32b`)
- Verificar GPU habilitada: `ollama run qwen3:8b --verbose`
- Verificar uso de memoria durante inferencia

## Siguientes Pasos

- [Referencia de Configuracion](/es/docs/configuration/) - Opciones completas de configuracion
- [Estrategias de Routing](/es/docs/routing/) - Seleccion de proveedor y failover
- [Monitoreo de Salud](/es/docs/health/) - Circuit breakers y health checks
