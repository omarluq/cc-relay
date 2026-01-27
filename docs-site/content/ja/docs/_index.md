---
title: ドキュメント
weight: 1
---

CC-Relay ドキュメントへようこそ！このガイドでは、CC-Relay を Claude Code やその他の LLM クライアント用のマルチプロバイダープロキシとしてセットアップ、設定、使用する方法を説明します。

## CC-Relay とは？

CC-Relay は Go で書かれた高性能な HTTP プロキシで、LLM クライアント（Claude Code など）と LLM プロバイダーの間に位置します。以下を提供します：

- **マルチプロバイダーサポート**: Anthropic と Z.AI（他のプロバイダーも予定）
- **Anthropic API 互換**: 直接 API アクセスをそのまま置き換え可能
- **SSE ストリーミング**: ストリーミングレスポンスを完全サポート
- **複数の認証方法**: API キーと Bearer トークンをサポート
- **Claude Code 統合**: 組み込みの設定コマンドで簡単セットアップ

## 現在のステータス

CC-Relay は活発に開発中です。現在実装されている機能：

| 機能 | ステータス |
|------|--------|
| HTTP プロキシサーバー | 実装済み |
| Anthropic プロバイダー | 実装済み |
| Z.AI プロバイダー | 実装済み |
| SSE ストリーミング | 実装済み |
| API キー認証 | 実装済み |
| Bearer トークン（サブスクリプション）認証 | 実装済み |
| Claude Code 設定 | 実装済み |
| 複数 API キー | 実装済み |
| デバッグログ | 実装済み |

**計画中の機能:**
- ルーティング戦略（ラウンドロビン、フェイルオーバー、コストベース）
- API キーごとのレート制限
- サーキットブレーカーとヘルストラッキング
- gRPC 管理 API
- TUI ダッシュボード
- 追加プロバイダー（Ollama、Bedrock、Azure、Vertex）

## クイックスタート

```bash
# インストール
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest

# 設定を初期化
cc-relay config init

# API キーを設定
export ANTHROPIC_API_KEY="your-key-here"

# プロキシを起動
cc-relay serve

# Claude Code を設定（別のターミナルで）
cc-relay config cc init
```

## クイックナビゲーション

- [はじめに](/ja/docs/getting-started/) - インストールと初回実行
- [設定](/ja/docs/configuration/) - プロバイダーのセットアップとオプション
- [アーキテクチャ](/ja/docs/architecture/) - システム設計とコンポーネント
- [API リファレンス](/ja/docs/api/) - HTTP エンドポイントと例

## ドキュメントセクション

### はじめに
- [インストール](/ja/docs/getting-started/#インストール)
- [クイックスタート](/ja/docs/getting-started/#クイックスタート)
- [CLI コマンド](/ja/docs/getting-started/#cli-コマンド)
- [Claude Code でのテスト](/ja/docs/getting-started/#claude-code-でのテスト)
- [トラブルシューティング](/ja/docs/getting-started/#トラブルシューティング)

### 設定
- [サーバー設定](/ja/docs/configuration/#サーバー設定)
- [プロバイダー設定](/ja/docs/configuration/#プロバイダー設定)
- [認証](/ja/docs/configuration/#認証)
- [ログ設定](/ja/docs/configuration/#ログ設定)
- [設定例](/ja/docs/configuration/#設定例)

### アーキテクチャ
- [システム概要](/ja/docs/architecture/#システム概要)
- [コアコンポーネント](/ja/docs/architecture/#コアコンポーネント)
- [リクエストフロー](/ja/docs/architecture/#リクエストフロー)
- [SSE ストリーミング](/ja/docs/architecture/#sse-ストリーミング)
- [認証フロー](/ja/docs/architecture/#認証フロー)

### API リファレンス
- [POST /v1/messages](/ja/docs/api/#post-v1messages)
- [GET /v1/models](/ja/docs/api/#get-v1models)
- [GET /v1/providers](/ja/docs/api/#get-v1providers)
- [GET /health](/ja/docs/api/#get-health)
- [クライアント例](/ja/docs/api/#curl-例)

## お困りですか？

- [Issue を報告](https://github.com/omarluq/cc-relay/issues)
- [ディスカッション](https://github.com/omarluq/cc-relay/discussions)
