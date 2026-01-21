---
title: API リファレンス
weight: 5
---

CC-Relay は Anthropic Messages API と完全に互換性のある HTTP API を公開しています。

## HTTP プロキシ API

### POST /v1/messages

Anthropic Messages API 形式を使用してメッセージを作成します。これは Claude Code やその他の LLM クライアントが使用する主要なエンドポイントです。

**エンドポイント**: `POST /v1/messages`

**ヘッダー**:
```
Content-Type: application/json
x-api-key: <your-api-key>
anthropic-version: 2023-06-01
```

**リクエストボディ**:
```json
{
  "model": "claude-sonnet-4-5-20250514",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "Hello, Claude!"
    }
  ],
  "temperature": 1.0,
  "stream": false
}
```

**レスポンス**（非ストリーミング）:
```json
{
  "id": "msg_01XYZ...",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "model": "claude-sonnet-4-5-20250514",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 12,
    "output_tokens": 15
  }
}
```

### SSE ストリーミング

リクエストで `"stream": true` を設定すると Server-Sent Events ストリーミングが有効になります。

**イベントシーケンス**:

```mermaid
sequenceDiagram
    participant Client as クライアント
    participant Proxy as プロキシ

    Client->>Proxy: POST /v1/messages (stream=true)

    Proxy-->>Client: event: message_start<br/>data: {"type":"message_start",...}

    Proxy-->>Client: event: content_block_start<br/>data: {"type":"content_block_start",...}

    loop テキスト生成
        Proxy-->>Client: event: content_block_delta<br/>data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}
    end

    Proxy-->>Client: event: content_block_stop<br/>data: {"type":"content_block_stop"}

    Proxy-->>Client: event: message_delta<br/>data: {"type":"message_delta","usage":{...}}

    Proxy-->>Client: event: message_stop<br/>data: {"type":"message_stop"}
```

**ストリーム例**:

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01ABC","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5-20250514","usage":{"input_tokens":12,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}
```

### ツール使用

Claude Code はファイル操作やその他のタスクにツール実行を使用します。CC-Relay は正しい関連付けのために `tool_use_id` を保持します：

**リクエスト**:
```json
{
  "model": "claude-sonnet-4-5-20250514",
  "max_tokens": 1024,
  "tools": [
    {
      "name": "get_weather",
      "description": "Get weather for a location",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        }
      }
    }
  ],
  "messages": [
    {"role": "user", "content": "What's the weather in NYC?"}
  ]
}
```

**レスポンス**:
```json
{
  "content": [
    {
      "type": "text",
      "text": "I'll check the weather for you."
    },
    {
      "type": "tool_use",
      "id": "toolu_01ABC",
      "name": "get_weather",
      "input": {"location": "NYC"}
    }
  ]
}
```

### エラーレスポンス

すべてのエラーは Anthropic API 形式で返されます：

**上流エラー**（502）:
```json
{
  "type": "error",
  "error": {
    "type": "api_error",
    "message": "upstream connection failed"
  }
}
```

**認証エラー**（401）:
```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "missing x-api-key header"
  }
}
```

**無効なリクエスト**（400）:
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Missing required field: messages"
  }
}
```

## GET /v1/models

設定されたすべてのプロバイダーから利用可能なモデルを一覧表示します。

**エンドポイント**: `GET /v1/models`

**ヘッダー**: 不要（認証なし）

**レスポンス**:
```json
{
  "object": "list",
  "data": [
    {
      "id": "claude-sonnet-4-5-20250514",
      "object": "model",
      "owned_by": "anthropic",
      "provider": "anthropic",
      "created": 1737446400
    },
    {
      "id": "claude-opus-4-5-20250514",
      "object": "model",
      "owned_by": "anthropic",
      "provider": "anthropic",
      "created": 1737446400
    },
    {
      "id": "GLM-4.7",
      "object": "model",
      "owned_by": "zhipu",
      "provider": "zai",
      "created": 1737446400
    }
  ]
}
```

## GET /v1/providers

メタデータ付きのアクティブプロバイダーを一覧表示します。

**エンドポイント**: `GET /v1/providers`

**ヘッダー**: 不要（認証なし）

**レスポンス**:
```json
{
  "object": "list",
  "data": [
    {
      "name": "anthropic",
      "type": "anthropic",
      "base_url": "https://api.anthropic.com",
      "models": [
        "claude-sonnet-4-5-20250514",
        "claude-opus-4-5-20250514",
        "claude-haiku-3-5-20241022"
      ],
      "active": true
    },
    {
      "name": "zai",
      "type": "zhipu",
      "base_url": "https://api.z.ai/api/anthropic",
      "models": [
        "GLM-4.7",
        "GLM-4.5-Air",
        "GLM-4-Plus"
      ],
      "active": true
    }
  ]
}
```

## GET /health

監視とロードバランサー用のヘルスチェックエンドポイント。

**エンドポイント**: `GET /health`

**ヘッダー**: 不要（認証なし）

**レスポンス**:
```json
{"status":"ok"}
```

**HTTP ステータスコード**:
- `200 OK`: サーバーは正常
- `503 Service Unavailable`: サーバーが異常（将来の実装）

## 認証

CC-Relay は `/v1/messages` エンドポイントに対して複数の認証方法をサポートしています：

### API キー認証

`x-api-key` ヘッダーを含めます：

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-proxy-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "claude-sonnet-4-5-20250514", ...}'
```

### Bearer トークン認証

`Authorization` ヘッダーを含めます：

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "claude-sonnet-4-5-20250514", ...}'
```

これは Claude Code サブスクリプションユーザーが使用します。

## リクエストヘッダー

CC-Relay はすべての `anthropic-*` ヘッダーをバックエンドプロバイダーに転送します：

| ヘッダー | 説明 |
|--------|------|
| `anthropic-version` | API バージョン（必須） |
| `anthropic-beta` | 有効にするベータ機能 |
| `anthropic-dangerous-direct-browser-access` | ブラウザアクセスフラグ |

## レスポンスヘッダー

CC-Relay はレスポンスに以下のヘッダーを追加します：

| ヘッダー | 説明 |
|--------|------|
| `X-Request-ID` | トレース用の一意なリクエスト識別子 |
| `Content-Type` | `application/json` または `text/event-stream` |
| `Cache-Control` | SSE 用の `no-cache, no-transform` |
| `X-Accel-Buffering` | プロキシバッファリングを無効にする `no` |

## cURL 例

### 非ストリーミングリクエスト

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### ストリーミングリクエスト

```bash
curl -N -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### モデル一覧

```bash
curl http://localhost:8787/v1/models
```

### プロバイダー一覧

```bash
curl http://localhost:8787/v1/providers
```

### ヘルスチェック

```bash
curl http://localhost:8787/health
```

## Python クライアント例

```python
import requests

response = requests.post(
    "http://localhost:8787/v1/messages",
    headers={
        "Content-Type": "application/json",
        "x-api-key": "managed-by-cc-relay",
        "anthropic-version": "2023-06-01",
    },
    json={
        "model": "claude-sonnet-4-5-20250514",
        "max_tokens": 1024,
        "messages": [
            {"role": "user", "content": "Hello!"}
        ],
    },
)

print(response.json())
```

## Python ストリーミング例

```python
import requests

response = requests.post(
    "http://localhost:8787/v1/messages",
    headers={
        "Content-Type": "application/json",
        "x-api-key": "managed-by-cc-relay",
        "anthropic-version": "2023-06-01",
    },
    json={
        "model": "claude-sonnet-4-5-20250514",
        "max_tokens": 1024,
        "messages": [
            {"role": "user", "content": "Hello!"}
        ],
        "stream": True,
    },
    stream=True,
)

for line in response.iter_lines():
    if line:
        print(line.decode('utf-8'))
```

## Go クライアント例

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

func main() {
    body := map[string]interface{}{
        "model":      "claude-sonnet-4-5-20250514",
        "max_tokens": 100,
        "messages": []map[string]string{
            {"role": "user", "content": "Hello!"},
        },
    }

    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", "http://localhost:8787/v1/messages", bytes.NewReader(jsonBody))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", "test")
    req.Header.Set("anthropic-version", "2023-06-01")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    data, _ := io.ReadAll(resp.Body)
    fmt.Println(string(data))
}
```

## 次のステップ

- [設定リファレンス](/ja/docs/configuration/)
- [アーキテクチャ概要](/ja/docs/architecture/)
