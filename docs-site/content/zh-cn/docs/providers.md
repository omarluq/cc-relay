---
title: "供应商"
description: "在 cc-relay 中配置 Anthropic、Z.AI 和 Ollama 供应商"
weight: 5
---

CC-Relay 通过统一接口支持多个 LLM 供应商。本页介绍如何配置每个供应商。

## 概述

CC-Relay 作为 Claude Code 和各种 LLM 后端之间的代理。所有供应商都公开 Anthropic 兼容的 Messages API，实现供应商之间的无缝切换。

| 供应商 | 类型 | 描述 | 成本 |
|--------|------|------|------|
| Anthropic | `anthropic` | 直接访问 Anthropic API | 标准 Anthropic 定价 |
| Z.AI | `zai` | Zhipu AI GLM 模型，Anthropic 兼容 | 约为 Anthropic 定价的 1/7 |
| Ollama | `ollama` | 本地 LLM 推理 | 免费（本地计算） |
| AWS Bedrock | `bedrock` | 通过 AWS 使用 SigV4 认证访问 Claude | AWS Bedrock 定价 |
| Azure AI Foundry | `azure` | 通过 Azure MAAS 访问 Claude | Azure AI 定价 |
| Google Vertex AI | `vertex` | 通过 Google Cloud 访问 Claude | Vertex AI 定价 |

## Anthropic 供应商

Anthropic 供应商直接连接到 Anthropic 的 API。这是完整访问 Claude 模型的默认供应商。

### 配置

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # 可选，使用默认值

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # 每分钟请求数
        tpm_limit: 100000    # 每分钟令牌数
        priority: 2          # 更高 = 在故障转移中首先尝试

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

### API 密钥设置

1. 在 [console.anthropic.com](https://console.anthropic.com) 创建账户
2. 导航到 Settings > API Keys
3. 创建新的 API 密钥
4. 存储在环境变量中: `export ANTHROPIC_API_KEY="sk-ant-..."`

### 透明认证支持

Anthropic 供应商支持 Claude Code 订阅用户的透明认证。启用后，cc-relay 会原样转发您的订阅令牌:

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
# 您的订阅令牌将原样转发
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

详情请参阅[透明认证](/zh-cn/docs/configuration/#透明认证)。

## Z.AI 供应商

Z.AI（智谱 AI）通过 Anthropic 兼容 API 提供 GLM 模型。这在保持 API 兼容性的同时提供显著的成本节省（约为 Anthropic 定价的 1/7）。

### 配置

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # 可选，使用默认值

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # 故障转移时优先级低于 Anthropic

    # 将 Claude 模型名称映射到 Z.AI 模型
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

### API 密钥设置

1. 在 [z.ai/model-api](https://z.ai/model-api) 创建账户
2. 导航到 API Keys 部分
3. 创建新的 API 密钥
4. 存储在环境变量中: `export ZAI_API_KEY="..."`

> **享受10%折扣:** 订阅时使用[此邀请链接](https://z.ai/subscribe?ic=HT5TQVSOZP) — 您和推荐人都可获得10%折扣。

### Model Mapping

Model Mapping 将 Anthropic 模型名称转换为 Z.AI 等效模型。当 Claude Code 请求 `claude-sonnet-4-5-20250514` 时，cc-relay 会自动路由到 `GLM-4.7`:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7（旗舰模型）
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air（快速、经济）
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

### 成本比较

| 模型 | Anthropic（每百万令牌） | Z.AI 等效 | Z.AI 成本 |
|------|------------------------|----------|----------|
| claude-sonnet-4-5 | $3 输入 / $15 输出 | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 输入 / $1.25 输出 | GLM-4.5-Air | ~$0.04 / $0.18 |

*价格为近似值，可能会有变动。*

## Ollama 供应商

Ollama 通过 Anthropic 兼容 API（Ollama v0.14 以来可用）实现本地 LLM 推理。在本地运行模型以保护隐私、零 API 成本和离线操作。

### 配置

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # 可选，使用默认值

    keys:
      - key: "ollama"  # Ollama 接受但忽略 API 密钥
        priority: 0    # 故障转移的最低优先级

    # 将 Claude 模型名称映射到本地 Ollama 模型
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

### Ollama 设置

1. 从 [ollama.com](https://ollama.com) 安装 Ollama
2. 拉取您想使用的模型:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. 启动 Ollama（安装时自动运行）

### 推荐模型

对于 Claude Code 工作流，选择至少 32K 上下文的模型:

| 模型 | 上下文 | 大小 | 最佳用途 |
|------|--------|------|---------|
| `qwen3:32b` | 128K | 32B 参数 | 通用编码、复杂推理 |
| `qwen3:8b` | 128K | 8B 参数 | 快速迭代、简单任务 |
| `codestral:latest` | 32K | 22B 参数 | 代码生成、专业编码 |
| `llama3.2:3b` | 128K | 3B 参数 | 非常快、基础任务 |

### 功能限制

Ollama 的 Anthropic 兼容性是部分的。某些功能不支持:

| 功能 | 支持 | 备注 |
|------|------|------|
| Streaming（SSE） | 是 | 与 Anthropic 相同的事件序列 |
| Tool calling | 是 | 与 Anthropic 相同的格式 |
| Extended thinking | 部分 | `budget_tokens` 被接受但不强制执行 |
| Prompt caching | 否 | `cache_control` 块被忽略 |
| PDF 输入 | 否 | 不支持 |
| 图片 URL | 否 | 仅支持 Base64 编码 |
| 令牌计数 | 否 | `/v1/messages/count_tokens` 不可用 |
| `tool_choice` | 否 | 无法强制使用特定工具 |

### Docker 网络

在 Docker 中运行 cc-relay 但 Ollama 在主机上时:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # 使用 Docker 的主机网关代替 localhost
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

或者使用 `--network host` 运行 cc-relay:

```bash
docker run --network host cc-relay
```

## AWS Bedrock 供应商

AWS Bedrock 通过 Amazon Web Services 提供 Claude 访问，具有企业级安全性和 SigV4 认证。

### 配置

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

## Azure AI Foundry 供应商

Azure AI Foundry 通过 Microsoft Azure 提供 Claude 访问，具有企业级 Azure 集成。

### 配置

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

## Google Vertex AI 供应商

Vertex AI 通过 Google Cloud 提供 Claude 访问，具有无缝 GCP 集成。

### 配置

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

`model_mapping` 字段将传入的模型名称转换为供应商特定的模型:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # 格式: "传入模型": "供应商模型"
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

当 Claude Code 发送:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-Relay 路由到 Z.AI:
```json
{"model": "GLM-4.7", ...}
```

### 映射技巧

1. **包含版本后缀**: 同时映射 `claude-sonnet-4-5` 和 `claude-sonnet-4-5-20250514`
2. **考虑上下文长度**: 匹配具有类似能力的模型
3. **测试质量**: 验证输出质量满足您的需求

## 多供应商设置

为故障转移、成本优化或负载分配配置多个供应商:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  # 主要: Anthropic（最高质量）
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # 首先尝试

  # 次要: Z.AI（成本效益）
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # 后备

  # 第三: Ollama（本地、免费）
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # 最后手段

routing:
  strategy: failover  # 按优先级顺序尝试供应商
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

使用此配置:
1. 请求首先发送到 Anthropic（优先级 2）
2. 如果 Anthropic 失败（429、5xx），尝试 Z.AI（优先级 1）
3. 如果 Z.AI 失败，尝试 Ollama（优先级 0）

更多选项请参阅[路由策略](/zh-cn/docs/routing/)。

## 故障排除

### 连接被拒绝（Ollama）

**症状:** 连接 Ollama 时 `connection refused`

**原因:**
- Ollama 未运行
- 端口错误
- Docker 网络问题

**解决方案:**
```bash
# 检查 Ollama 是否正在运行
ollama list

# 验证端口
curl http://localhost:11434/api/version

# 对于 Docker，使用主机网关
base_url: "http://host.docker.internal:11434"
```

### 认证失败（Z.AI）

**症状:** 从 Z.AI 收到 `401 Unauthorized`

**原因:**
- 无效的 API 密钥
- 环境变量未设置
- 密钥未激活

**解决方案:**
```bash
# 验证环境变量已设置
echo $ZAI_API_KEY

# 直接测试密钥
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### 模型未找到

**症状:** `model not found` 错误

**原因:**
- 模型未在 `models` 列表中配置
- 缺少 `model_mapping` 条目
- 模型未安装（Ollama）

**解决方案:**

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# 确保模型已列出
models:
  - "GLM-4.7"

# 确保映射存在
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

对于 Ollama，验证模型已安装:
```bash
ollama list
ollama pull qwen3:32b
```

### 响应缓慢（Ollama）

**症状:** Ollama 响应非常慢

**原因:**
- 模型对硬件来说太大
- 未使用 GPU
- RAM 不足

**解决方案:**
- 使用更小的模型（用 `qwen3:8b` 代替 `qwen3:32b`）
- 验证 GPU 已启用: `ollama run qwen3:8b --verbose`
- 在推理期间检查内存使用情况

## 后续步骤

- [配置参考](/zh-cn/docs/configuration/) - 完整配置选项
- [路由策略](/zh-cn/docs/routing/) - 供应商选择和故障转移
- [健康监控](/zh-cn/docs/health/) - 熔断器和健康检查
