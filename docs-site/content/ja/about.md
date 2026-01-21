---
title: CC-Relay について
type: about
---

## CC-Relay について

CC-Relay は Go で書かれた高性能な HTTP プロキシで、Claude Code やその他の LLM クライアントが単一のエンドポイントを通じて複数のプロバイダーに接続することを可能にします。

### プロジェクトの目標

- **マルチプロバイダーアクセスの簡素化** - 1つのプロキシで複数のバックエンド
- **API 互換性の維持** - Anthropic API への直接アクセスをそのまま置き換え可能
- **柔軟性の実現** - クライアントの変更なしでプロバイダーを簡単に切り替え
- **Claude Code のサポート** - Claude Code CLI とのファーストクラス統合

### 現在のステータス

CC-Relay は活発に開発中です。以下の機能が実装されています：

- Anthropic API 互換の HTTP プロキシサーバー
- Anthropic と Z.AI プロバイダーのサポート
- 完全な SSE ストリーミングサポート
- API キーと Bearer トークン認証
- プロバイダーごとの複数 API キー
- リクエスト/レスポンス検査のためのデバッグログ
- Claude Code 設定コマンド

### 計画中の機能

- 追加プロバイダー（Ollama、AWS Bedrock、Azure、Vertex AI）
- ルーティング戦略（ラウンドロビン、フェイルオーバー、コストベース）
- API キーごとのレート制限
- サーキットブレーカーとヘルスト ラッキング
- gRPC 管理 API
- TUI ダッシュボード

### 使用技術

- [Go](https://go.dev/) - プログラミング言語
- [Cobra](https://cobra.dev/) - CLI フレームワーク
- [zerolog](https://github.com/rs/zerolog) - 構造化ログ

### 作者

[Omar Alani](https://github.com/omarluq) により作成

### ライセンス

CC-Relay は [AGPL 3 ライセンス](https://github.com/omarluq/cc-relay/blob/main/LICENSE) の下でライセンスされたオープンソースソフトウェアです。

### コントリビューション

コントリビューションを歓迎します！以下については [GitHub リポジトリ](https://github.com/omarluq/cc-relay) をご覧ください：

- [Issue の報告](https://github.com/omarluq/cc-relay/issues)
- [プルリクエストの送信](https://github.com/omarluq/cc-relay/pulls)
- [ディスカッション](https://github.com/omarluq/cc-relay/discussions)
