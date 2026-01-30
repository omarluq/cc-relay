---
title: 設定
weight: 3
---

CC-Relay は YAML または TOML ファイルで設定されます。このガイドでは、すべての設定オプションについて説明します。

## 設定ファイルの場所

デフォルトの場所（この順序でチェックされます）：

1. `./config.yaml`（カレントディレクトリ）
2. `~/.config/cc-relay/config.yaml`
3. `--config` フラグで指定されたパス

デフォルト設定を生成するには：

```bash
cc-relay config init
```

## 環境変数の展開

CC-Relay は YAML と TOML の両方のフォーマットで `${VAR_NAME}` 構文を使用した環境変数の展開をサポートしています：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # ロード時に展開されます
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"  # ロード時に展開されます
```
  {{< /tab >}}
{{< /tabs >}}

## 完全な設定リファレンス

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# ==========================================================================
# サーバー設定
# ==========================================================================
server:
  # リッスンするアドレス
  listen: "127.0.0.1:8787"

  # リクエストタイムアウト（ミリ秒）（デフォルト: 600000 = 10分）
  timeout_ms: 600000

  # 最大同時リクエスト数（0 = 無制限）
  max_concurrent: 0

  # パフォーマンス向上のため HTTP/2 を有効化
  enable_http2: true

  # 認証設定
  auth:
    # プロキシアクセスに特定の API キーを要求
    api_key: "${PROXY_API_KEY}"

    # Claude Code サブスクリプション Bearer トークンを許可
    allow_subscription: true

    # 検証する特定の Bearer トークン（オプション）
    bearer_secret: "${BEARER_SECRET}"

# ==========================================================================
# プロバイダー設定
# ==========================================================================
providers:
  # Anthropic ダイレクト API
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # オプション、デフォルトを使用

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60       # 1分あたりのリクエスト数
        tpm_limit: 100000   # 1分あたりのトークン数

    # オプション: 利用可能なモデルを指定
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

    # Claude モデル名を Z.AI モデルにマッピング
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    # オプション: 利用可能なモデルを指定
    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"

# ==========================================================================
# ログ設定
# ==========================================================================
logging:
  # ログレベル: debug, info, warn, error
  level: "info"

  # ログ形式: json, text
  format: "text"

  # カラー出力を有効化（text 形式用）
  pretty: true

  # 詳細なデバッグオプション
  debug_options:
    log_request_body: false
    log_response_headers: false
    log_tls_metrics: false
    max_body_log_size: 1000

# ==========================================================================
# キャッシュ設定
# ==========================================================================
cache:
  # キャッシュモード: single, ha, disabled
  mode: single

  # シングルモード (Ristretto) 設定
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size

  # HAモード (Olric) 設定
  olric:
    embedded: true                 # Run embedded Olric node
    bind_addr: "0.0.0.0:3320"      # Olric client port
    dmap_name: "cc-relay"          # Distributed map name
    environment: lan               # local, lan, or wan
    peers:                         # Memberlist addresses (bind_addr + 2)
      - "other-node:3322"
    replica_count: 2               # Copies per key
    read_quorum: 1                 # Min reads for success
    write_quorum: 1                # Min writes for success
    member_count_quorum: 2         # Min cluster members
    leave_timeout: 5s              # Leave broadcast duration
```
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# Server Configuration
# ==========================================================================
[server]
# Address to listen on
listen = "127.0.0.1:8787"

# Request timeout in milliseconds (default: 600000 = 10 minutes)
timeout_ms = 600000

# Maximum concurrent requests (0 = unlimited)
max_concurrent = 0

# Enable HTTP/2 for better performance
enable_http2 = true

# Authentication configuration
[server.auth]
# Require specific API key for proxy access
api_key = "${PROXY_API_KEY}"

# Allow Claude Code subscription Bearer tokens
allow_subscription = true

# Specific Bearer token to validate (optional)
bearer_secret = "${BEARER_SECRET}"

# ==========================================================================
# Provider Configurations
# ==========================================================================

# Anthropic Direct API
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Optional, uses default

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60       # Requests per minute
tpm_limit = 100000   # Tokens per minute

# Optional: Specify available models
models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]

# Z.AI / Zhipu GLM
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

# Map Claude model names to Z.AI models
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

# Optional: Specify available models
models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]

# ==========================================================================
# Logging Configuration
# ==========================================================================
[logging]
# Log level: debug, info, warn, error
level = "info"

# Log format: json, text
format = "text"

# Enable colored output (for text format)
pretty = true

# Granular debug options
[logging.debug_options]
log_request_body = false
log_response_headers = false
log_tls_metrics = false
max_body_log_size = 1000

# ==========================================================================
# Cache Configuration
# ==========================================================================
[cache]
# Cache mode: single, ha, disabled
mode = "single"

# Single mode (Ristretto) configuration
[cache.ristretto]
num_counters = 1000000  # 10x expected max items
max_cost = 104857600    # 100 MB
buffer_items = 64       # Admission buffer size

# HA mode (Olric) configuration
[cache.olric]
embedded = true                 # Run embedded Olric node
bind_addr = "0.0.0.0:3320"      # Olric client port
dmap_name = "cc-relay"          # Distributed map name
environment = "lan"             # local, lan, or wan
peers = ["other-node:3322"]     # Memberlist addresses (bind_addr + 2)
replica_count = 2               # Copies per key
read_quorum = 1                 # Min reads for success
write_quorum = 1                # Min writes for success
member_count_quorum = 2         # Min cluster members
leave_timeout = "5s"            # Leave broadcast duration

# ==========================================================================
# Routing Configuration
# ==========================================================================
[routing]
# Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
strategy = "failover"

# Timeout for failover attempts in milliseconds (default: 5000)
failover_timeout = 5000

# Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

## サーバー設定

### リッスンアドレス

`listen` フィールドは、プロキシが受信リクエストをリッスンする場所を指定します：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"  # ローカルのみ（推奨）
  # listen: "0.0.0.0:8787"  # すべてのインターフェース（注意して使用）
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"  # ローカルのみ（推奨）
# listen = "0.0.0.0:8787"  # すべてのインターフェース（注意して使用）
```
  {{< /tab >}}
{{< /tabs >}}

### 認証

CC-Relay は複数の認証方法をサポートしています：

#### API キー認証

クライアントに特定の API キーの提供を要求：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
api_key = "${PROXY_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

クライアントはヘッダーを含める必要があります: `x-api-key: <your-proxy-key>`

#### Claude Code サブスクリプションパススルー

Claude Code サブスクリプションユーザーの接続を許可：

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

これは Claude Code からの `Authorization: Bearer` トークンを受け入れます。

#### 複合認証

API キーとサブスクリプション認証の両方を許可：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server.auth]
api_key = "${PROXY_API_KEY}"
allow_subscription = true
```
  {{< /tab >}}
{{< /tabs >}}

#### 認証なし

認証を無効にするには（本番環境では非推奨）：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  auth: {}
  # または auth セクションを省略
```
  {{< /tab >}}
  {{< tab >}}
```toml
# auth セクションを省略するか、空のテーブルを使用
# [server.auth]
```
  {{< /tab >}}
{{< /tabs >}}

### HTTP/2 サポート

同時リクエストのパフォーマンス向上のため HTTP/2 を有効化：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  enable_http2: true
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
enable_http2 = true
```
  {{< /tab >}}
{{< /tabs >}}

## プロバイダー設定

### プロバイダータイプ

CC-Relay は現在2つのプロバイダータイプをサポートしています：

| タイプ | 説明 | デフォルト Base URL |
|-------|------|------------------|
| `anthropic` | Anthropic ダイレクト API | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Anthropic プロバイダー

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # オプション

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60
        tpm_limit: 100000

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
base_url = "https://api.anthropic.com"  # オプション

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60
tpm_limit = 100000

models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]
```
  {{< /tab >}}
{{< /tabs >}}

### Z.AI プロバイダー

Z.AI は低コストで GLM モデルを使用した Anthropic 互換 API を提供します：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]
```
  {{< /tab >}}
{{< /tabs >}}

### 複数 API キー

高スループットのために複数の API キーをプール：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_1}"
rpm_limit = 60
tpm_limit = 100000

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_2}"
rpm_limit = 60
tpm_limit = 100000

[[providers.keys]]
key = "${ANTHROPIC_API_KEY_3}"
rpm_limit = 60
tpm_limit = 100000
```
  {{< /tab >}}
{{< /tabs >}}

### カスタム Base URL

デフォルトの API エンドポイントをオーバーライド：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic-custom"
type = "anthropic"
base_url = "https://custom-endpoint.example.com"
```
  {{< /tab >}}
{{< /tabs >}}

## ログ設定

### ログレベル

| レベル | 説明 |
|-------|------|
| `debug` | 開発用の詳細な出力 |
| `info` | 通常の操作メッセージ |
| `warn` | 警告メッセージ |
| `error` | エラーメッセージのみ |

### ログ形式

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
logging:
  format: "text"   # 人間が読みやすい形式（デフォルト）
  # format: "json" # 機械可読、ログ集約用
```
  {{< /tab >}}
  {{< tab >}}
```toml
[logging]
format = "text"   # 人間が読みやすい形式（デフォルト）
# format = "json" # 機械可読、ログ集約用
```
  {{< /tab >}}
{{< /tabs >}}

### デバッグオプション

デバッグログの詳細な制御：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # リクエストボディをログ（編集済み）
    log_response_headers: true  # レスポンスヘッダーをログ
    log_tls_metrics: true       # TLS 接続情報をログ
    max_body_log_size: 1000     # ボディからログする最大バイト数
```
  {{< /tab >}}
  {{< tab >}}
```toml
[logging]
level = "debug"

[logging.debug_options]
log_request_body = true      # リクエストボディをログ（編集済み）
log_response_headers = true  # レスポンスヘッダーをログ
log_tls_metrics = true       # TLS 接続情報をログ
max_body_log_size = 1000     # ボディからログする最大バイト数
```
  {{< /tab >}}
{{< /tabs >}}

## キャッシュ設定

CC-Relay は、さまざまなデプロイメントシナリオに対応する複数のバックエンドオプションを備えた統合キャッシュレイヤーを提供します。

### キャッシュモード

| モード | バックエンド | 用途 |
|--------|---------|----------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | シングルインスタンスデプロイメント、高性能 |
| `ha` | [Olric](https://github.com/buraksezer/olric) | マルチインスタンスデプロイメント、共有状態 |
| `disabled` | Noop | キャッシングなし、パススルー |

### シングルモード (Ristretto)

Ristretto は、高性能で並行処理対応のインメモリキャッシュです。シングルインスタンスデプロイメントのデフォルトモードです。

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "single"

[cache.ristretto]
num_counters = 1000000  # 10x expected max items
max_cost = 104857600    # 100 MB
buffer_items = 64       # Admission buffer size
```
  {{< /tab >}}
{{< /tabs >}}

| フィールド | タイプ | デフォルト | 説明 |
|-------|------|---------|-------------|
| `num_counters` | int64 | 1,000,000 | 4ビットアクセスカウンターの数。推奨: 予想最大アイテム数の10倍。 |
| `max_cost` | int64 | 104,857,600 (100 MB) | キャッシュが保持できる最大メモリ（バイト）。 |
| `buffer_items` | int64 | 64 | Get バッファあたりのキー数。アドミッションバッファサイズを制御。 |

### HAモード (Olric) - 埋め込み

共有キャッシュ状態を必要とするマルチインスタンスデプロイメントには、各 cc-relay インスタンスが Olric ノードを実行する埋め込み Olric モードを使用します。

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "other-node:3322"  # Memberlist port = bind_addr + 2
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "ha"

[cache.olric]
embedded = true
bind_addr = "0.0.0.0:3320"
dmap_name = "cc-relay"
environment = "lan"
peers = ["other-node:3322"]  # Memberlist port = bind_addr + 2
replica_count = 2
read_quorum = 1
write_quorum = 1
member_count_quorum = 2
leave_timeout = "5s"
```
  {{< /tab >}}
{{< /tabs >}}

| フィールド | タイプ | デフォルト | 説明 |
|-------|------|---------|-------------|
| `embedded` | bool | false | 埋め込み Olric ノードを実行 (true) vs. 外部クラスターに接続 (false)。 |
| `bind_addr` | string | 必須 | Olric クライアント接続用アドレス（例: "0.0.0.0:3320"）。 |
| `dmap_name` | string | "cc-relay" | 分散マップの名前。すべてのノードで同じ名前を使用する必要があります。 |
| `environment` | string | "local" | Memberlist プリセット: "local"、"lan"、または "wan"。 |
| `peers` | []string | - | ピア検出用の Memberlist アドレス。bind_addr + 2 のポートを使用。 |
| `replica_count` | int | 1 | キーあたりのコピー数。1 = レプリケーションなし。 |
| `read_quorum` | int | 1 | 応答に必要な最小読み取り成功数。 |
| `write_quorum` | int | 1 | 応答に必要な最小書き込み成功数。 |
| `member_count_quorum` | int32 | 1 | 動作に必要な最小クラスターメンバー数。 |
| `leave_timeout` | duration | 5s | シャットダウン前の離脱メッセージブロードキャスト時間。 |

**重要:** Olric は2つのポートを使用します - クライアント接続用の `bind_addr` ポートと memberlist ゴシップ用の `bind_addr + 2`。ファイアウォールで両方のポートを開いてください。

### HAモード (Olric) - クライアントモード

埋め込みノードを実行する代わりに、外部 Olric クラスターに接続します：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: ha
  olric:
    embedded: false
    addresses:
      - "olric-node-1:3320"
      - "olric-node-2:3320"
    dmap_name: "cc-relay"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "ha"

[cache.olric]
embedded = false
addresses = ["olric-node-1:3320", "olric-node-2:3320"]
dmap_name = "cc-relay"
```
  {{< /tab >}}
{{< /tabs >}}

| フィールド | タイプ | 説明 |
|-------|------|-------------|
| `embedded` | bool | クライアントモードでは `false` に設定。 |
| `addresses` | []string | 外部 Olric クラスターアドレス。 |
| `dmap_name` | string | 分散マップ名（クラスター設定と一致する必要があります）。 |

### 無効モード

デバッグ用または他の場所でキャッシングが処理される場合、キャッシングを完全に無効にします：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
cache:
  mode: disabled
```
  {{< /tab >}}
  {{< tab >}}
```toml
[cache]
mode = "disabled"
```
  {{< /tab >}}
{{< /tabs >}}

HAクラスタリングガイドとトラブルシューティングを含む詳細なキャッシュドキュメントについては、[キャッシング](/ja/docs/caching/)を参照してください。

## ルーティング設定

CC-Relay は、プロバイダー間でリクエストを分配するための複数のルーティング戦略をサポートしています。

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
# ==========================================================================
# ルーティング設定
# ==========================================================================
routing:
  # 戦略: round_robin, weighted_round_robin, shuffle, failover（デフォルト）
  strategy: failover

  # フェイルオーバー試行のタイムアウト（ミリ秒、デフォルト: 5000）
  failover_timeout: 5000

  # デバッグヘッダーを有効化（X-CC-Relay-Strategy, X-CC-Relay-Provider）
  debug: false
```
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# ルーティング設定
# ==========================================================================
[routing]
# 戦略: round_robin, weighted_round_robin, shuffle, failover（デフォルト）
strategy = "failover"

# フェイルオーバー試行のタイムアウト（ミリ秒、デフォルト: 5000）
failover_timeout = 5000

# デバッグヘッダーを有効化（X-CC-Relay-Strategy, X-CC-Relay-Provider）
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

### ルーティング戦略

| 戦略 | 説明 |
|------|------|
| `failover` | 優先度順にプロバイダーを試行、失敗時にフォールバック（デフォルト） |
| `round_robin` | プロバイダー間を順番にローテーション |
| `weighted_round_robin` | 重みに基づいて比例配分 |
| `shuffle` | フェアランダム分配 |

### プロバイダーの重みと優先度

重みと優先度はプロバイダーの最初のキーで設定します：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # weighted-round-robin 用（高い = より多くのトラフィック）
        priority: 2    # failover 用（高い = 最初に試行）
```
  {{< /tab >}}
  {{< tab >}}
```toml
[[providers]]
name = "anthropic"
type = "anthropic"

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
weight = 3      # weighted-round-robin 用（高い = より多くのトラフィック）
priority = 2    # failover 用（高い = 最初に試行）
```
  {{< /tab >}}
{{< /tabs >}}

戦略の説明、デバッグヘッダー、フェイルオーバートリガーを含む詳細なルーティング設定については、[ルーティング](/ja/docs/routing/)を参照してください。

## 設定例

### 最小限のシングルプロバイダー

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

### マルチプロバイダーセットアップ

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[server.auth]
allow_subscription = true

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"
enabled = true

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"

[logging]
level = "info"
format = "text"
```
  {{< /tab >}}
{{< /tabs >}}

### デバッグログを使用した開発

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

logging:
  level: "debug"
  format: "text"
  pretty: true
  debug_options:
    log_request_body: true
    log_response_headers: true
    log_tls_metrics: true
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

[logging]
level = "debug"
format = "text"
pretty = true

[logging.debug_options]
log_request_body = true
log_response_headers = true
log_tls_metrics = true
```
  {{< /tab >}}
{{< /tabs >}}

## 設定の検証

設定ファイルを検証：

```bash
cc-relay config validate
```

**ヒント**: デプロイ前に設定変更を必ず検証してください。ホットリロードは無効な設定を拒否しますが、検証により本番環境に到達する前にエラーを検出できます。

## ホットリロード

CC-Relay は設定変更を自動的に検出して適用し、再起動を必要としません。これによりダウンタイムなしで設定を更新できます。

### 動作の仕組み

CC-Relay は [fsnotify](https://github.com/fsnotify/fsnotify) を使用して設定ファイルを監視します：

1. **ファイル監視**: 親ディレクトリを監視し、アトミック書き込み（ほとんどのエディタが使用する一時ファイル + リネームパターン）を正しく検出
2. **デバウンス**: 複数の高速ファイルイベントは100msの遅延で集約し、エディタの保存動作を処理
3. **アトミックスワップ**: 新しい設定はGoの `sync/atomic.Pointer` を使用してアトミックにロードおよびスワップ
4. **処理中リクエストの保持**: 処理中のリクエストは古い設定を継続使用、新しいリクエストは更新された設定を使用

### リロードをトリガーするイベント

| イベント | リロードをトリガー |
|---------|------------------|
| ファイル書き込み | はい |
| ファイル作成（アトミックリネーム） | はい |
| ファイル chmod | いいえ（無視） |
| ディレクトリ内の他のファイル | いいえ（無視） |

### ログ出力

ホットリロード時には、ログメッセージが表示されます：

```
INF config file reloaded path=/path/to/config.yaml
INF config hot-reloaded successfully
```

新しい設定が無効な場合：

```
ERR failed to reload config path=/path/to/config.yaml error="validation error"
```

無効な設定は拒否され、プロキシは以前の有効な設定で継続動作します。

### 制限事項

- **リッスンアドレス**: `server.listen` の変更には再起動が必要
- **gRPCアドレス**: `grpc.listen` の変更には再起動が必要

ホットリロード可能な設定オプション：
- ログレベルとフォーマット
- ルーティング戦略、フェイルオーバータイムアウト、重みと優先度
- プロバイダーの有効/無効、ベースURL、モデルマッピング
- キープール戦略、キーの重み、キーごとの制限
- 最大同時リクエスト数と最大ボディサイズ
- ヘルスチェック間隔とサーキットブレーカー閾値

## 次のステップ

- [ルーティング戦略](/ja/docs/routing/) - プロバイダー選択とフェイルオーバー
- [アーキテクチャを理解する](/ja/docs/architecture/)
- [API リファレンス](/ja/docs/api/)
