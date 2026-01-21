---
title: 設定
weight: 3
---

CC-Relay は YAML ファイルで設定されます。このガイドでは、すべての設定オプションについて説明します。

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

CC-Relay は `${VAR_NAME}` 構文を使用した環境変数の展開をサポートしています：

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # ロード時に展開されます
```

## 完全な設定リファレンス

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
```

## サーバー設定

### リッスンアドレス

`listen` フィールドは、プロキシが受信リクエストをリッスンする場所を指定します：

```yaml
server:
  listen: "127.0.0.1:8787"  # ローカルのみ（推奨）
  # listen: "0.0.0.0:8787"  # すべてのインターフェース（注意して使用）
```

### 認証

CC-Relay は複数の認証方法をサポートしています：

#### API キー認証

クライアントに特定の API キーの提供を要求：

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```

クライアントはヘッダーを含める必要があります: `x-api-key: <your-proxy-key>`

#### Claude Code サブスクリプションパススルー

Claude Code サブスクリプションユーザーの接続を許可：

```yaml
server:
  auth:
    allow_subscription: true
```

これは Claude Code からの `Authorization: Bearer` トークンを受け入れます。

#### 複合認証

API キーとサブスクリプション認証の両方を許可：

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```

#### 認証なし

認証を無効にするには（本番環境では非推奨）：

```yaml
server:
  auth: {}
  # または auth セクションを省略
```

### HTTP/2 サポート

同時リクエストのパフォーマンス向上のため HTTP/2 を有効化：

```yaml
server:
  enable_http2: true
```

## プロバイダー設定

### プロバイダータイプ

CC-Relay は現在2つのプロバイダータイプをサポートしています：

| タイプ | 説明 | デフォルト Base URL |
|-------|------|------------------|
| `anthropic` | Anthropic ダイレクト API | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Anthropic プロバイダー

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

### Z.AI プロバイダー

Z.AI は低コストで GLM モデルを使用した Anthropic 互換 API を提供します：

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

### 複数 API キー

高スループットのために複数の API キーをプール：

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

### カスタム Base URL

デフォルトの API エンドポイントをオーバーライド：

```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```

## ログ設定

### ログレベル

| レベル | 説明 |
|-------|------|
| `debug` | 開発用の詳細な出力 |
| `info` | 通常の操作メッセージ |
| `warn` | 警告メッセージ |
| `error` | エラーメッセージのみ |

### ログ形式

```yaml
logging:
  format: "text"   # 人間が読みやすい形式（デフォルト）
  # format: "json" # 機械可読、ログ集約用
```

### デバッグオプション

デバッグログの詳細な制御：

```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # リクエストボディをログ（編集済み）
    log_response_headers: true  # レスポンスヘッダーをログ
    log_tls_metrics: true       # TLS 接続情報をログ
    max_body_log_size: 1000     # ボディからログする最大バイト数
```

## 設定例

### 最小限のシングルプロバイダー

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

### マルチプロバイダーセットアップ

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

### デバッグログを使用した開発

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

## 設定の検証

設定ファイルを検証：

```bash
cc-relay config validate
```

## ホットリロード

設定変更にはサーバーの再起動が必要です。ホットリロードは将来のリリースで予定されています。

## 次のステップ

- [アーキテクチャを理解する](/ja/docs/architecture/)
- [API リファレンス](/ja/docs/api/)
