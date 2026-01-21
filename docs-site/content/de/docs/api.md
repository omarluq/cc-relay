---
title: API-Referenz
weight: 5
---

CC-Relay bietet eine HTTP-API, die vollstaendig kompatibel mit der Anthropic Messages API ist.

## HTTP Proxy API

### POST /v1/messages

Erstellt eine Nachricht im Anthropic Messages API-Format. Dies ist der Hauptendpunkt, der von Claude Code und anderen LLM-Clients verwendet wird.

**Endpunkt**: `POST /v1/messages`

**Header**:
```
Content-Type: application/json
x-api-key: <ihr-api-schluessel>
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
      "content": "Hallo, Claude!"
    }
  ],
  "temperature": 1.0,
  "stream": false
}
```

**Response** (nicht-streaming):
```json
{
  "id": "msg_01XYZ...",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hallo! Wie kann ich Ihnen heute helfen?"
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

### SSE-Streaming

Setzen Sie `"stream": true` im Request, um Server-Sent Events Streaming zu aktivieren.

**Event-Sequenz**:

```mermaid
sequenceDiagram
    participant Client
    participant Proxy

    Client->>Proxy: POST /v1/messages (stream=true)

    Proxy-->>Client: event: message_start<br/>data: {"type":"message_start",...}

    Proxy-->>Client: event: content_block_start<br/>data: {"type":"content_block_start",...}

    loop Textgenerierung
        Proxy-->>Client: event: content_block_delta<br/>data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}
    end

    Proxy-->>Client: event: content_block_stop<br/>data: {"type":"content_block_stop"}

    Proxy-->>Client: event: message_delta<br/>data: {"type":"message_delta","usage":{...}}

    Proxy-->>Client: event: message_stop<br/>data: {"type":"message_stop"}
```

**Beispiel-Stream**:

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01ABC","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5-20250514","usage":{"input_tokens":12,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hallo"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}
```

### Tool-Verwendung

Claude Code verwendet Tool-Ausfuehrung fuer Dateioperationen und andere Aufgaben. CC-Relay bewahrt die `tool_use_id` fuer korrekte Zuordnung:

**Request**:
```json
{
  "model": "claude-sonnet-4-5-20250514",
  "max_tokens": 1024,
  "tools": [
    {
      "name": "get_weather",
      "description": "Wetter fuer einen Ort abrufen",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        }
      }
    }
  ],
  "messages": [
    {"role": "user", "content": "Wie ist das Wetter in Berlin?"}
  ]
}
```

**Response**:
```json
{
  "content": [
    {
      "type": "text",
      "text": "Ich pruefe das Wetter fuer Sie."
    },
    {
      "type": "tool_use",
      "id": "toolu_01ABC",
      "name": "get_weather",
      "input": {"location": "Berlin"}
    }
  ]
}
```

### Fehler-Responses

Alle Fehler werden im Anthropic API-Format zurueckgegeben:

**Upstream-Fehler** (502):
```json
{
  "type": "error",
  "error": {
    "type": "api_error",
    "message": "upstream connection failed"
  }
}
```

**Authentifizierungsfehler** (401):
```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "missing x-api-key header"
  }
}
```

**Ungueltiger Request** (400):
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

Listet verfuegbare Modelle aller konfigurierten Provider auf.

**Endpunkt**: `GET /v1/models`

**Header**: Keine erforderlich (keine Authentifizierung)

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

Listet aktive Provider mit Metadaten auf.

**Endpunkt**: `GET /v1/providers`

**Header**: Keine erforderlich (keine Authentifizierung)

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

Health-Check-Endpunkt fuer Monitoring und Load-Balancer.

**Endpunkt**: `GET /health`

**Header**: Keine erforderlich (keine Authentifizierung)

**Response**:
```json
{"status":"ok"}
```

**HTTP-Statuscodes**:
- `200 OK`: Server ist gesund
- `503 Service Unavailable`: Server ist nicht gesund (zukuenftige Implementierung)

## Authentifizierung

CC-Relay unterstuetzt mehrere Authentifizierungsmethoden fuer den `/v1/messages` Endpunkt:

### API-Schluessel-Authentifizierung

Den `x-api-key` Header mitsenden:

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: ihr-proxy-schluessel" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "claude-sonnet-4-5-20250514", ...}'
```

### Bearer-Token-Authentifizierung

Den `Authorization` Header mitsenden:

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ihr-token" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model": "claude-sonnet-4-5-20250514", ...}'
```

Dies wird von Claude Code Abonnenten verwendet.

## Request-Header

CC-Relay leitet alle `anthropic-*` Header an den Backend-Provider weiter:

| Header | Beschreibung |
|--------|--------------|
| `anthropic-version` | API-Version (erforderlich) |
| `anthropic-beta` | Zu aktivierende Beta-Features |
| `anthropic-dangerous-direct-browser-access` | Browser-Zugangs-Flag |

## Response-Header

CC-Relay fuegt folgende Header zu Responses hinzu:

| Header | Beschreibung |
|--------|--------------|
| `X-Request-ID` | Eindeutige Request-ID fuer Tracing |
| `Content-Type` | `application/json` oder `text/event-stream` |
| `Cache-Control` | `no-cache, no-transform` fuer SSE |
| `X-Accel-Buffering` | `no` um Proxy-Pufferung zu deaktivieren |

## cURL-Beispiele

### Nicht-Streaming Request

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hallo!"}]
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
    "messages": [{"role": "user", "content": "Hallo!"}],
    "stream": true
  }'
```

### Modelle auflisten

```bash
curl http://localhost:8787/v1/models
```

### Provider auflisten

```bash
curl http://localhost:8787/v1/providers
```

### Health Check

```bash
curl http://localhost:8787/health
```

## Python-Client-Beispiel

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
            {"role": "user", "content": "Hallo!"}
        ],
    },
)

print(response.json())
```

## Python-Streaming-Beispiel

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
            {"role": "user", "content": "Hallo!"}
        ],
        "stream": True,
    },
    stream=True,
)

for line in response.iter_lines():
    if line:
        print(line.decode('utf-8'))
```

## Go-Client-Beispiel

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
            {"role": "user", "content": "Hallo!"},
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

## Naechste Schritte

- [Konfigurationsreferenz](/de/docs/configuration/)
- [Architekturuebersicht](/de/docs/architecture/)
