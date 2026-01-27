---
title: はじめに
weight: 2
---

このガイドでは、CC-Relay のインストール、設定、初回実行について説明します。

## 前提条件

- **Go 1.21+** ソースからビルドする場合
- **API キー** サポートされているプロバイダー（Anthropic または Z.AI）のうち少なくとも1つ
- **Claude Code** CLI テスト用（オプション）

## インストール

### Go Install を使用

```bash
go install github.com/omarluq/cc-relay@latest
```

バイナリは `$GOPATH/bin/cc-relay` または `$HOME/go/bin/cc-relay` にインストールされます。

### ソースからビルド

```bash
# リポジトリをクローン
git clone https://github.com/omarluq/cc-relay.git
cd cc-relay

# task を使用してビルド（推奨）
task build

# または手動でビルド
go build -o cc-relay ./cmd/cc-relay

# 実行
./cc-relay --help
```

### ビルド済みバイナリ

[リリースページ](https://github.com/omarluq/cc-relay/releases) からビルド済みバイナリをダウンロードできます。

## クイックスタート

### 1. 設定の初期化

CC-Relay はデフォルトの設定ファイルを生成できます：

```bash
cc-relay config init
```

これにより、適切なデフォルト値を持つ設定ファイルが `~/.config/cc-relay/config.yaml` に作成されます。

### 2. 環境変数の設定

```bash
export ANTHROPIC_API_KEY="your-api-key-here"

# オプション: Z.AI を使用する場合
export ZAI_API_KEY="your-zai-key-here"
```

### 3. CC-Relay の実行

```bash
cc-relay serve
```

以下のような出力が表示されます：

```
INF starting cc-relay listen=127.0.0.1:8787
INF using primary provider provider=anthropic-pool type=anthropic
```

### 4. Claude Code の設定

CC-Relay を使用するように Claude Code を設定する最も簡単な方法：

```bash
cc-relay config cc init
```

これにより、`~/.claude/settings.json` がプロキシ設定で自動的に更新されます。

または、環境変数を手動で設定することもできます：

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_AUTH_TOKEN="managed-by-cc-relay"
claude
```

## 動作確認

### サーバーステータスの確認

```bash
cc-relay status
```

出力：
```
✓ cc-relay is running (127.0.0.1:8787)
```

### ヘルスエンドポイントのテスト

```bash
curl http://localhost:8787/health
```

レスポンス：
```json
{"status":"ok"}
```

### 利用可能なモデルの一覧

```bash
curl http://localhost:8787/v1/models
```

### リクエストのテスト

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## CLI コマンド

CC-Relay はいくつかの CLI コマンドを提供します：

| コマンド | 説明 |
|---------|------|
| `cc-relay serve` | プロキシサーバーを起動 |
| `cc-relay status` | サーバーが実行中か確認 |
| `cc-relay config init` | デフォルト設定ファイルを生成 |
| `cc-relay config cc init` | cc-relay を使用するように Claude Code を設定 |
| `cc-relay config cc remove` | Claude Code から cc-relay 設定を削除 |
| `cc-relay --version` | バージョン情報を表示 |

### Serve コマンドオプション

```bash
cc-relay serve [flags]

Flags:
  --config string      設定ファイルのパス（デフォルト: ~/.config/cc-relay/config.yaml）
  --log-level string   ログレベル（debug, info, warn, error）
  --log-format string  ログ形式（json, text）
  --debug              デバッグモードを有効化（詳細ログ）
```

## 最小設定

動作する最小限の設定：

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

## 次のステップ

- [複数プロバイダーの設定](/ja/docs/configuration/)
- [アーキテクチャを理解する](/ja/docs/architecture/)
- [API リファレンス](/ja/docs/api/)

## トラブルシューティング

### ポートが使用中

ポート 8787 がすでに使用されている場合、設定でリッスンアドレスを変更してください：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8788"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8788"
```
  {{< /tab >}}
{{< /tabs >}}

### プロバイダーが応答しない

接続エラーについてサーバーログを確認してください：

```bash
cc-relay serve --log-level debug
```

### 認証エラー

「authentication failed」エラーが表示される場合：

1. API キーが環境変数に正しく設定されているか確認
2. 設定ファイルが正しい環境変数を参照しているか確認
3. API キーがプロバイダーで有効か確認

### デバッグモード

詳細なリクエスト/レスポンスログのためにデバッグモードを有効化：

```bash
cc-relay serve --debug
```

これにより以下が有効になります：
- デバッグログレベル
- リクエストボディのログ（機密フィールドは編集）
- レスポンスヘッダーのログ
- TLS 接続メトリクス
