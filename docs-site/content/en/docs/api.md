---
title: API Reference
weight: 5
---

CC-Relay exposes an HTTP API that is fully compatible with the Anthropic Messages API.

## HTTP Proxy API

### POST /v1/messages

Creates a message using the Anthropic Messages API format. This is the main endpoint used by Claude Code and other LLM clients.

**Endpoint**: `POST /v1/messages`

**Headers**:
```
Content-Type: application/json
x-api-key: <your-api-key>
anthropic-version: 2023-06-01
```

**Request Body**:
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

**Response** (non-streaming):
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

### SSE Streaming

Set `"stream": true` in the request to enable Server-Sent Events streaming.

**Event Sequence**:

```mermaid
sequenceDiagram
    participant Client
    participant Proxy

    Client->>Proxy: POST /v1/messages (stream=true)

    Proxy-->>Client: event: message_start<br/>data: {"type":"message_start",...}

    Proxy-->>Client: event: content_block_start<br/>data: {"type":"content_block_start",...}

    loop Text Generation
        Proxy-->>Client: event: content_block_delta<br/>data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}
    end

    Proxy-->>Client: event: content_block_stop<br/>data: {"type":"content_block_stop"}

    Proxy-->>Client: event: message_delta<br/>data: {"type":"message_delta","usage":{...}}

    Proxy-->>Client: event: message_stop<br/>data: {"type":"message_stop"}
```

**Example Stream**:

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

### Tool Use

Claude Code uses tool execution for file operations and other tasks. CC-Relay preserves `tool_use_id` for correct association:

**Request**:
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

**Response**:
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

### Error Responses

All errors are returned in Anthropic API format:

**Upstream Error** (502):
```json
{
  "type": "error",
  "error": {
    "type": "api_error",
    "message": "upstream connection failed"
  }
}
```

**Authentication Error** (401):
```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "missing x-api-key header"
  }
}
```

**Invalid Request** (400):
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

List available models from all configured providers.

**Endpoint**: `GET /v1/models`

**Headers**: None required (no authentication)

**Response**:
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

List active providers with metadata.

**Endpoint**: `GET /v1/providers`

**Headers**: None required (no authentication)

**Response**:
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

Health check endpoint for monitoring and load balancers.

**Endpoint**: `GET /health`

**Headers**: None required (no authentication)

**Response**:
```json
{"status":"ok"}
```

**HTTP Status Codes**:
- `200 OK`: Server is healthy
- `503 Service Unavailable`: Server is unhealthy (future implementation)

## Authentication

CC-Relay supports multiple authentication methods for the `/v1/messages` endpoint:

### API Key Authentication

Include the `x-api-key` header:

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-proxy-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "claude-sonnet-4-5-20250514", ...}'
```

### Bearer Token Authentication

Include the `Authorization` header:

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "claude-sonnet-4-5-20250514", ...}'
```

This is used by Claude Code subscription users.

## Request Headers

CC-Relay forwards all `anthropic-*` headers to the backend provider:

| Header | Description |
|--------|-------------|
| `anthropic-version` | API version (required) |
| `anthropic-beta` | Beta features to enable |
| `anthropic-dangerous-direct-browser-access` | Browser access flag |

## Response Headers

CC-Relay adds the following headers to responses:

| Header | Description |
|--------|-------------|
| `X-Request-ID` | Unique request identifier for tracing |
| `Content-Type` | `application/json` or `text/event-stream` |
| `Cache-Control` | `no-cache, no-transform` for SSE |
| `X-Accel-Buffering` | `no` to disable proxy buffering |

## cURL Examples

### Non-streaming Request

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

### Streaming Request

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

### List Models

```bash
curl http://localhost:8787/v1/models
```

### List Providers

```bash
curl http://localhost:8787/v1/providers
```

### Health Check

```bash
curl http://localhost:8787/health
```

## Python Client Example

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

## Python Streaming Example

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

## Go Client Example

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

## Next Steps

- [Configuration reference](/docs/configuration/)
- [Architecture overview](/docs/architecture/)
