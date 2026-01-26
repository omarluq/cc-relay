---
title: "プロバイダー"
description: "cc-relayでAnthropic、Z.AI、Ollamaプロバイダーを設定する"
weight: 5
---

CC-Relayは統一されたインターフェースを通じて複数のLLMプロバイダーをサポートしています。このページでは各プロバイダーの設定方法について説明します。

## 概要

CC-RelayはClaude Codeと様々なLLMバックエンド間のプロキシとして機能します。すべてのプロバイダーはAnthropic互換のMessages APIを公開しており、プロバイダー間のシームレスな切り替えが可能です。

| プロバイダー | タイプ | 説明 | コスト |
|-------------|--------|------|--------|
| Anthropic | `anthropic` | 直接Anthropic APIアクセス | 標準Anthropic料金 |
| Z.AI | `zai` | Zhipu AI GLMモデル、Anthropic互換 | Anthropicの約1/7の料金 |
| Ollama | `ollama` | ローカルLLM推論 | 無料（ローカルコンピューティング） |
| AWS Bedrock | `bedrock` | SigV4認証によるAWS経由のClaude | AWS Bedrock料金 |
| Azure AI Foundry | `azure` | Azure MAAS経由のClaude | Azure AI料金 |
| Google Vertex AI | `vertex` | Google Cloud経由のClaude | Vertex AI料金 |

## Anthropicプロバイダー

Anthropicプロバイダーは直接AnthropicのAPIに接続します。これはClaudeモデルへの完全なアクセスのためのデフォルトプロバイダーです。

### 設定

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # オプション、デフォルトを使用

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # 毎分リクエスト数
        tpm_limit: 100000    # 毎分トークン数
        priority: 2          # 高い = フェイルオーバーで最初に試行

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

### APIキーの設定

1. [console.anthropic.com](https://console.anthropic.com)でアカウントを作成
2. Settings > API Keysに移動
3. 新しいAPIキーを作成
4. 環境変数に保存: `export ANTHROPIC_API_KEY="sk-ant-..."`

### 透過的認証サポート

Anthropicプロバイダーは、Claude Codeサブスクリプションユーザーの透過的認証をサポートしています。有効にすると、cc-relayはサブスクリプショントークンをそのまま転送します:

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
# サブスクリプショントークンはそのまま転送されます
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

詳細は[透過的認証](/ja/docs/configuration/#透過的認証)を参照してください。

## Z.AIプロバイダー

Z.AI（Zhipu AI）はAnthropic互換APIを通じてGLMモデルを提供します。これにより、APIの互換性を維持しながら大幅なコスト削減（Anthropicの約1/7）が可能です。

### 設定

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # オプション、デフォルトを使用

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # フェイルオーバー時Anthropicより低い優先度

    # Claudeモデル名をZ.AIモデルにマッピング
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

### APIキーの設定

1. [z.ai/model-api](https://z.ai/model-api)でアカウントを作成
2. API Keysセクションに移動
3. 新しいAPIキーを作成
4. 環境変数に保存: `export ZAI_API_KEY="..."`

> **10%割引:** 登録時に[この招待リンク](https://z.ai/subscribe?ic=HT5TQVSOZP)を使用すると、あなたと紹介者の両方が10%割引を受けられます。

### Model Mapping

Model MappingはAnthropicモデル名をZ.AIの同等品に変換します。Claude Codeが`claude-sonnet-4-5-20250514`をリクエストすると、cc-relayは自動的に`GLM-4.7`にルーティングします:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7（フラッグシップモデル）
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air（高速、経済的）
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

### コスト比較

| モデル | Anthropic（100万トークンあたり） | Z.AI同等品 | Z.AIコスト |
|--------|--------------------------------|-----------|-----------|
| claude-sonnet-4-5 | $3 入力 / $15 出力 | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 入力 / $1.25 出力 | GLM-4.5-Air | ~$0.04 / $0.18 |

*価格は概算であり、変更される可能性があります。*

## Ollamaプロバイダー

OllamaはAnthropic互換API（Ollama v0.14以降で利用可能）を通じてローカルLLM推論を可能にします。プライバシー、APIコストゼロ、オフライン操作のためにモデルをローカルで実行できます。

### 設定

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # オプション、デフォルトを使用

    keys:
      - key: "ollama"  # OllamaはAPIキーを受け入れるが無視する
        priority: 0    # フェイルオーバーの最低優先度

    # Claudeモデル名をローカルOllamaモデルにマッピング
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

### Ollamaのセットアップ

1. [ollama.com](https://ollama.com)からOllamaをインストール
2. 使用したいモデルをプル:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. Ollamaを起動（インストール時に自動的に実行）

### 推奨モデル

Claude Codeワークフローには、少なくとも32Kコンテキストを持つモデルを選択してください:

| モデル | コンテキスト | サイズ | 最適な用途 |
|--------|------------|--------|-----------|
| `qwen3:32b` | 128K | 32Bパラメータ | 一般的なコーディング、複雑な推論 |
| `qwen3:8b` | 128K | 8Bパラメータ | 高速イテレーション、シンプルなタスク |
| `codestral:latest` | 32K | 22Bパラメータ | コード生成、専門的なコーディング |
| `llama3.2:3b` | 128K | 3Bパラメータ | 非常に高速、基本的なタスク |

### 機能制限

OllamaのAnthropic互換性は部分的です。一部の機能はサポートされていません:

| 機能 | サポート | 備考 |
|------|---------|------|
| Streaming（SSE） | はい | Anthropicと同じイベントシーケンス |
| Tool calling | はい | Anthropicと同じ形式 |
| Extended thinking | 部分的 | `budget_tokens`は受け入れられるが適用されない |
| Prompt caching | いいえ | `cache_control`ブロックは無視される |
| PDF入力 | いいえ | サポートされていない |
| 画像URL | いいえ | Base64エンコーディングのみ |
| トークンカウント | いいえ | `/v1/messages/count_tokens`は利用不可 |
| `tool_choice` | いいえ | 特定のツール使用を強制できない |

### Dockerネットワーキング

cc-relayをDockerで実行し、Ollamaをホストで実行する場合:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # localhostの代わりにDockerのホストゲートウェイを使用
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

または、cc-relayを`--network host`で実行:

```bash
docker run --network host cc-relay
```

## AWS Bedrockプロバイダー

AWS Bedrockは、エンタープライズセキュリティとSigV4認証によるAmazon Web Servicesを通じてClaudeへのアクセスを提供します。

### 設定

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

### AWSセットアップ

1. **Bedrockアクセスを有効化**: AWS ConsoleでBedrock > Model accessに移動してClaudeモデルを有効化
2. **認証情報を設定**: 以下の方法のいずれかを使用:
   - **環境変数**: `export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...`
   - **AWS CLI**: `aws configure`
   - **IAMロール**: EC2/ECS/LambdaロールにBedrockアクセスポリシーをアタッチ

### Bedrockモデル ID

**注:** AWS Bedrockが新しいClaudeバージョンを追加するにつれてモデルIDは頻繁に変更されます。デプロイ前に[AWS Bedrockモデルアクセスドキュメント](https://docs.aws.amazon.com/bedrock/latest/userguide/models-supported.html)で現在のリストを確認してください。

Bedrockは特定のモデルID形式を使用します: `anthropic.{model}-v{version}:{minor}`

| Claudeモデル | BedrockモデルID |
|--------------|------------------|
| claude-sonnet-4-5-20250514 | `anthropic.claude-sonnet-4-5-20250514-v1:0` |
| claude-opus-4-5-20250514 | `anthropic.claude-opus-4-5-20250514-v1:0` |
| claude-haiku-3-5-20241022 | `anthropic.claude-haiku-3-5-20241022-v1:0` |

### イベントストリーム変換

BedrockはAWS Event Stream形式でレスポンスを返します。CC-RelayはこれをClaude Code互換性のためにSSE形式に自動変換します。追加の設定は不要です。

## Azure AI Foundryプロバイダー

Azure AI Foundryは、エンタープライズAzure統合によるMicrosoft Azureを通じてClaudeへのアクセスを提供します。

### 設定

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

### Azureセットアップ

1. **Azure AIリソースを作成**: Azure PortalでAzure AI Foundryリソースを作成
2. **Claudeモデルをデプロイ**: AI Foundryワークスペースでクモデルをデプロイ
3. **APIキーを取得**: Keys and EndpointセクションからAPIキーをコピー
4. **リソース名を確認**: URLは `https://{resource_name}.services.ai.azure.com`

### デプロイ名

AzureはモデルIDとしてデプロイ名を使用します。Azure AI Foundryでデプロイメントを作成してからマッピングしてください:

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

## Google Vertex AIプロバイダー

Vertex AIは、シームレスなGCP統合によるGoogle Cloudを通じてClaudeへのアクセスを提供します。

### 設定

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

### GCPセットアップ

1. **Vertex AI APIを有効化**: GCP ConsoleでVertex AI APIを有効化
2. **Claudeアクセスをリクエスト**: Vertex AI Model GardenからClaudeモデルへのアクセスをリクエスト
3. **認証を設定**: 以下の方法のいずれかを使用:
   - **Application Default Credentials**: `gcloud auth application-default login`
   - **サービスアカウント**: `GOOGLE_APPLICATION_CREDENTIALS`環境変数を設定
   - **GCE/GKE**: アタッチされたサービスアカウントを自動的に使用

### Vertex AIモデルID

Vertex AIは `{model}@{version}` 形式を使用します:

| Claudeモデル | Vertex AIモデルID |
|--------------|-------------------|
| claude-sonnet-4-5-20250514 | `claude-sonnet-4-5@20250514` |
| claude-opus-4-5-20250514 | `claude-opus-4-5@20250514` |
| claude-haiku-3-5-20241022 | `claude-haiku-3-5@20241022` |

### リージョン

Vertex AIでClaudeが利用可能なリージョン（完全な最新リストは[Google Cloudドキュメント](https://cloud.google.com/vertex-ai/docs/general/locations)を確認してください）:
- `us-east5`（デフォルト）
- `us-central1`
- `europe-west1`

## クラウドプロバイダー比較

| 機能 | Bedrock | Azure | Vertex AI |
|---------|---------|-------|-----------|
| 認証 | SigV4（AWS） | APIキー | OAuth2（GCP） |
| ストリーミング形式 | Event Stream | SSE | SSE |
| ボディ変換 | あり | なし | あり |
| URLにモデル | あり | なし | あり |
| エンタープライズSSO | AWS IAM | Entra ID | GCP IAM |
| リージョン | US, EU, APAC | グローバル | US, EU |

## Model Mapping

`model_mapping`フィールドは、入力されるモデル名をプロバイダー固有のモデルに変換します:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # 形式: "入力モデル": "プロバイダーモデル"
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

Claude Codeが送信した場合:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-RelayはZ.AIに以下でルーティング:
```json
{"model": "GLM-4.7", ...}
```

### マッピングのヒント

1. **バージョンサフィックスを含める**: `claude-sonnet-4-5`と`claude-sonnet-4-5-20250514`の両方をマッピング
2. **コンテキスト長を考慮**: 同様の機能を持つモデルをマッチング
3. **品質をテスト**: 出力品質がニーズに合っているか確認

## マルチプロバイダー設定

フェイルオーバー、コスト最適化、または負荷分散のために複数のプロバイダーを設定:

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  # プライマリ: Anthropic（最高品質）
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # 最初に試行

  # セカンダリ: Z.AI（コスト効率的）
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # フォールバック

  # ターシャリ: Ollama（ローカル、無料）
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # 最後の手段

routing:
  strategy: failover  # 優先順位でプロバイダーを試行
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

この設定では:
1. リクエストは最初にAnthropicへ（優先度2）
2. Anthropicが失敗した場合（429、5xx）、Z.AIを試行（優先度1）
3. Z.AIが失敗した場合、Ollamaを試行（優先度0）

詳細は[ルーティング戦略](/ja/docs/routing/)を参照してください。

## トラブルシューティング

### 接続拒否（Ollama）

**症状:** Ollamaへの接続時に`connection refused`

**原因:**
- Ollamaが実行されていない
- ポートが間違っている
- Dockerネットワーキングの問題

**解決策:**
```bash
# Ollamaが実行中か確認
ollama list

# ポートを確認
curl http://localhost:11434/api/version

# Dockerの場合、ホストゲートウェイを使用
base_url: "http://host.docker.internal:11434"
```

### 認証失敗（Z.AI）

**症状:** Z.AIから`401 Unauthorized`

**原因:**
- 無効なAPIキー
- 環境変数が設定されていない
- キーがアクティベートされていない

**解決策:**
```bash
# 環境変数を確認
echo $ZAI_API_KEY

# キーを直接テスト
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### モデルが見つからない

**症状:** `model not found`エラー

**原因:**
- モデルが`models`リストに設定されていない
- `model_mapping`エントリがない
- モデルがインストールされていない（Ollama）

**解決策:**

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# モデルがリストされていることを確認
models:
  - "GLM-4.7"

# マッピングが存在することを確認
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

Ollamaの場合、モデルがインストールされているか確認:
```bash
ollama list
ollama pull qwen3:32b
```

### 応答が遅い（Ollama）

**症状:** Ollamaからの応答が非常に遅い

**原因:**
- ハードウェアに対してモデルが大きすぎる
- GPUが使用されていない
- RAMが不足

**解決策:**
- より小さいモデルを使用（`qwen3:32b`の代わりに`qwen3:8b`）
- GPU有効化を確認: `ollama run qwen3:8b --verbose`
- 推論中のメモリ使用量を確認

## 次のステップ

- [設定リファレンス](/ja/docs/configuration/) - 完全な設定オプション
- [ルーティング戦略](/ja/docs/routing/) - プロバイダー選択とフェイルオーバー
- [ヘルスモニタリング](/ja/docs/health/) - サーキットブレーカーとヘルスチェック
