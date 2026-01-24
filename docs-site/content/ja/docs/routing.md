---
title: ルーティング
weight: 4
---

CC-Relay は、プロバイダー間でリクエストを分配するための複数のルーティング戦略をサポートしています。このページでは、各戦略とその設定方法について説明します。

## 概要

ルーティングは、cc-relay がどのプロバイダーに各リクエストを処理させるかを決定します。適切な戦略は、可用性、コスト、レイテンシー、負荷分散など、あなたの優先事項によって異なります。

| 戦略 | 設定値 | 説明 | ユースケース |
|------|--------|------|------------|
| Round-Robin | `round_robin` | プロバイダー間を順番にローテーション | 均等分配 |
| Weighted Round-Robin | `weighted_round_robin` | 重みに基づく比例分配 | 容量ベースの分配 |
| Shuffle | `shuffle` | フェアランダム（「カード配り」パターン） | ランダム化された負荷分散 |
| Failover | `failover`（デフォルト） | 優先度ベースで自動リトライ | 高可用性 |
| Model-Based | `model_based` | モデル名プレフィックスでルーティング | マルチモデルデプロイメント |

## 設定

`config.yaml` でルーティングを設定します：

```yaml
routing:
  # 戦略: round_robin, weighted_round_robin, shuffle, failover（デフォルト）, model_based
  strategy: failover

  # フェイルオーバー試行のタイムアウト（ミリ秒、デフォルト: 5000）
  failover_timeout: 5000

  # デバッグヘッダーを有効化（X-CC-Relay-Strategy, X-CC-Relay-Provider）
  debug: false

  # モデルベースルーティング設定（strategy: model_based の場合のみ使用）
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
  default_provider: anthropic
```

**デフォルト:** `strategy` が指定されていない場合、cc-relay は最も安全なオプションとして `failover` を使用します。

## 戦略

### Round-Robin

アトミックカウンターを使用した順次分配。どのプロバイダーも2回目のリクエストを受ける前に、各プロバイダーが1つのリクエストを受け取ります。

```yaml
routing:
  strategy: round_robin
```

**動作の仕組み:**

1. リクエスト 1 → プロバイダー A
2. リクエスト 2 → プロバイダー B
3. リクエスト 3 → プロバイダー C
4. リクエスト 4 → プロバイダー A（サイクルが繰り返される）

**最適な用途:** 同等の容量を持つプロバイダー間での均等分配。

### Weighted Round-Robin

プロバイダーの重みに基づいてリクエストを比例配分します。均等な分配のために Nginx smooth weighted round-robin アルゴリズムを使用します。

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3  # 3倍のリクエストを受け取る

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        weight: 1  # 1倍のリクエストを受け取る
```

**動作の仕組み:**

重みが 3:1 の場合、4リクエストごとに：
- 3 リクエスト → anthropic
- 1 リクエスト → zai

**デフォルトの重み:** 1（指定されていない場合）

**最適な用途:** プロバイダーの容量、レート制限、またはコスト配分に基づく負荷分散。

### Shuffle

Fisher-Yates「カード配り」パターンを使用したフェアランダム分配。誰かが2枚目のカードを受け取る前に、全員が1枚ずつ受け取ります。

```yaml
routing:
  strategy: shuffle
```

**動作の仕組み:**

1. すべてのプロバイダーが「デッキ」に入る
2. ランダムなプロバイダーが選択されデッキから除外される
3. デッキが空になったら、すべてのプロバイダーを再シャッフル
4. 時間経過で公平な分配を保証

**最適な用途:** 公平性を確保しながらランダム化された負荷分散。

### Failover

優先度順にプロバイダーを試行します。失敗した場合、最も速い成功レスポンスを得るために残りのプロバイダーへ並列リクエストを実行します。これが**デフォルト戦略**です。

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # 最初に試行される（高い = 高優先度）

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # フォールバック
```

**動作の仕組み:**

1. 最高優先度のプロバイダーを最初に試行
2. 失敗した場合（[フェイルオーバートリガー](#フェイルオーバートリガー)参照）、残りのすべてのプロバイダーへ並列リクエストを発行
3. 最初に成功したレスポンスを返し、他をキャンセル
4. 全体の操作時間は `failover_timeout` を尊重

**デフォルトの優先度:** 1（指定されていない場合）

**最適な用途:** 自動フォールバック付きの高可用性。

### Model-Based

リクエスト内のモデル名に基づいてプロバイダーにリクエストをルーティングします。特異性のために最長プレフィックスマッチングを使用します。

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

**動作の仕組み:**

1. リクエストから `model` パラメータを抽出
2. `model_mapping` で最長プレフィックスマッチを見つける
3. 対応するプロバイダーにルーティング
4. マッチが見つからない場合は `default_provider` にフォールバック
5. マッチもデフォルトもない場合はエラーを返す

**プレフィックスマッチングの例:**

| リクエストされたモデル | マッピングエントリ | 選択されたエントリ | プロバイダー |
|----------------------|-------------------|------------------|------------|
| `claude-opus-4` | `claude-opus`, `claude` | `claude-opus` | anthropic |
| `claude-sonnet-3.5` | `claude-sonnet`, `claude` | `claude-sonnet` | anthropic |
| `glm-4-plus` | `glm-4`, `glm` | `glm-4` | zai |
| `qwen-72b` | `qwen`, `claude` | `qwen` | ollama |
| `llama-3.2` | `llama`, `claude` | `llama` | ollama |
| `gpt-4` | `claude`, `llama` | (マッチなし) | default_provider |

**最適な用途:** 異なるモデルを異なるプロバイダーにルーティングする必要があるマルチモデルデプロイメント。

## デバッグヘッダー

`routing.debug: true` の場合、cc-relay はレスポンスに診断ヘッダーを追加します：

| ヘッダー | 値 | 説明 |
|---------|-----|------|
| `X-CC-Relay-Strategy` | 戦略名 | 使用されたルーティング戦略 |
| `X-CC-Relay-Provider` | プロバイダー名 | リクエストを処理したプロバイダー |

**レスポンスヘッダーの例:**

```
X-CC-Relay-Strategy: failover
X-CC-Relay-Provider: anthropic
```

**セキュリティ警告:** デバッグヘッダーは内部のルーティング決定を公開します。開発環境または信頼できる環境でのみ使用してください。信頼できないクライアントがいる本番環境では決して有効にしないでください。

## フェイルオーバートリガー

failover 戦略は特定のエラー条件でリトライをトリガーします：

| トリガー | 条件 | 説明 |
|---------|------|------|
| ステータスコード | `429`, `500`, `502`, `503`, `504` | レート制限またはサーバーエラー |
| タイムアウト | `context.DeadlineExceeded` | リクエストタイムアウト超過 |
| 接続 | `net.Error` | ネットワークエラー、DNS失敗、接続拒否 |

**重要:** クライアントエラー（429を除く4xx）はフェイルオーバーをトリガー**しません**。これらはプロバイダーではなく、リクエスト自体の問題を示しています。

### ステータスコードの説明

| コード | 意味 | フェイルオーバー? |
|--------|------|-----------------|
| `429` | レート制限 | はい - 別のプロバイダーを試す |
| `500` | 内部サーバーエラー | はい - サーバーの問題 |
| `502` | Bad Gateway | はい - アップストリームの問題 |
| `503` | サービス利用不可 | はい - 一時的にダウン |
| `504` | Gateway Timeout | はい - アップストリームタイムアウト |
| `400` | Bad Request | いいえ - リクエストを修正 |
| `401` | Unauthorized | いいえ - 認証を修正 |
| `403` | Forbidden | いいえ - 権限の問題 |

## 例

### シンプルな Failover（ほとんどのユーザーに推奨）

優先度付きプロバイダーでデフォルト戦略を使用：

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

### 重み付きロードバランシング

プロバイダーの容量に基づいて負荷を分散：

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "primary"
    type: "anthropic"
    keys:
      - key: "${PRIMARY_KEY}"
        weight: 3  # トラフィックの 75%

  - name: "secondary"
    type: "anthropic"
    keys:
      - key: "${SECONDARY_KEY}"
        weight: 1  # トラフィックの 25%
```

### デバッグヘッダー付き開発環境

トラブルシューティング用にデバッグヘッダーを有効化：

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

### 高速フェイルオーバーによる高可用性

フェイルオーバーのレイテンシーを最小化：

```yaml
routing:
  strategy: failover
  failover_timeout: 3000  # 3秒タイムアウト

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

### モデルベースルーティングを使用したマルチモデル

異なるモデルを専用プロバイダーにルーティング：

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

この設定により：
- Claudeモデル → Anthropic
- GLMモデル → Z.AI
- Qwen/Llamaモデル → Ollama（ローカル）
- その他のモデル → Anthropic（デフォルト）

## プロバイダーの重みと優先度

重みと優先度はプロバイダーのキー設定で指定します：

```yaml
providers:
  - name: "example"
    type: "anthropic"
    keys:
      - key: "${API_KEY}"
        weight: 3      # weighted-round-robin 用（高い = より多くのトラフィック）
        priority: 2    # failover 用（高い = 最初に試行）
        rpm_limit: 60  # レート制限トラッキング
```

**注:** 重みと優先度はプロバイダーのキーリストの**最初のキー**から読み取られます。

## 次のステップ

- [設定リファレンス](/ja/docs/configuration/) - 完全な設定オプション
- [アーキテクチャ概要](/ja/docs/architecture/) - cc-relay の内部動作
