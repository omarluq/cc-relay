---
phase: 06-cloud-providers
plan: 04
subsystem: providers
tags: [bedrock, aws, sigv4, event-stream, sse, streaming]
dependency-graph:
  requires: [06-01]
  provides: [BedrockProvider, EventStreamToSSE, ContentTypeEventStream]
  affects: [06-05, handler-integration]
tech-stack:
  added: [aws-sdk-go-v2, aws-sdk-go-v2/config, aws-sdk-go-v2/aws/signer/v4]
  patterns: [sigv4-signing, binary-protocol-parsing, stream-conversion]
key-files:
  created:
    - internal/providers/bedrock.go
    - internal/providers/bedrock_test.go
    - internal/providers/eventstream.go
    - internal/providers/eventstream_test.go
  modified:
    - internal/providers/base.go
    - internal/proxy/debug.go
    - internal/proxy/middleware.go
    - internal/proxy/sse.go
    - internal/proxy/sse_stream.go
    - internal/proxy/provider_proxy.go
    - go.mod
    - go.sum
decisions:
  - id: bedrock-url-path-escape
    choice: Use url.PathEscape for model ID, colons allowed in paths
    reason: AWS Bedrock accepts colons in model IDs without encoding
  - id: event-stream-crc32c
    choice: CRC32-C (Castagnoli) polynomial for AWS Event Stream
    reason: AWS Event Stream format uses CRC32-C, not CRC32-IEEE
  - id: content-type-constant
    choice: Add ContentTypeSSE constant in providers package
    reason: Reduce string duplication across proxy and providers
metrics:
  duration: 19 min
  completed: 2026-01-25
---

# Phase 6 Plan 4: Bedrock Provider Summary

BedrockProvider with SigV4 authentication and Event Stream to SSE conversion for Claude Code compatibility.

## Objective Achievement

Implemented AWS Bedrock provider with the most complex authentication (SigV4) and streaming format (Event Stream binary protocol). This validates the full transformer architecture works for cloud providers.

## Implementation Summary

### Event Stream to SSE Converter (Task 2)

Created `eventstream.go` with:
- `ParseEventStreamMessage`: Parses AWS Event Stream binary format
  - Prelude parsing (total length, headers length, prelude CRC)
  - CRC32-C validation for both prelude and message
  - Header parsing supporting multiple types (string, bool, int, bytes, etc.)
  - Payload extraction

- `EventStreamToSSE`: Converts streaming response to SSE format
  - Sets SSE headers (Content-Type, Cache-Control, X-Accel-Buffering)
  - Reads Event Stream messages from response body
  - Maps Bedrock event types to Anthropic SSE event types
  - Handles exception events as error SSE events
  - Flushes each SSE event immediately

### Bedrock Provider (Task 3)

Created `bedrock.go` implementing Provider interface:

- **Constructor**: `NewBedrockProvider` (default credentials) and `NewBedrockProviderWithCredentials` (testable)
- **Authentication**: SigV4 signing via aws-sdk-go-v2/signer/v4
  - Reads body to compute SHA256 hash
  - Signs request with credentials
  - Preserves body for actual request
- **TransformRequest**: Uses shared `TransformBodyForCloudProvider`
  - Removes model from body
  - Adds `anthropic_version: "bedrock-2023-05-31"` to body
  - Constructs URL: `/model/{model}/invoke-with-response-stream`
- **TransformResponse**: Delegates to `EventStreamToSSE` for streaming
- **StreamingContentType**: Returns `application/vnd.amazon.eventstream`

### Key Links Established

| From | To | Via |
|------|----|----|
| bedrock.go | transform.go | `TransformBodyForCloudProvider` |
| bedrock.go | eventstream.go | `EventStreamToSSE` for streaming conversion |
| bedrock.go | aws-sdk-go-v2/signer/v4 | SigV4 signing |

## Test Coverage

| File | Coverage | Key Tests |
|------|----------|-----------|
| bedrock.go | 80%+ | SigV4 headers, body preservation, URL construction, model mapping |
| eventstream.go | 70%+ | Message parsing, CRC validation, SSE conversion, exception handling |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ContentTypeSSE constant duplication**
- **Found during:** Task 2 commit
- **Issue:** golangci-lint flagged `text/event-stream` string appearing 4+ times
- **Fix:** Added `ContentTypeSSE` constant in `providers/base.go`, updated all usages in proxy package
- **Files modified:** base.go, debug.go, middleware.go, sse.go, sse_stream.go, provider_proxy.go
- **Commit:** 412e7d7

## Dependencies Added

```
github.com/aws/aws-sdk-go-v2 v1.41.1
github.com/aws/aws-sdk-go-v2/config v1.32.7
github.com/aws/aws-sdk-go-v2/credentials v1.19.7 (indirect)
+ several other aws-sdk-go-v2 subpackages
```

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 412e7d7 | feat | Event Stream to SSE converter with ContentTypeSSE constant |
| dd02c2d | feat | BedrockProvider with SigV4 authentication |
| f198a20 | test | Comprehensive unit tests for BedrockProvider |

## Next Phase Readiness

- BedrockProvider fully implements Provider interface
- Ready for handler integration (06-05)
- Event Stream conversion handles all Anthropic streaming event types
- TransformResponse method ready to be called by proxy handler

### Integration Points for 06-05

1. Handler must check `provider.StreamingContentType()` to detect Event Stream
2. For Bedrock, call `provider.TransformResponse()` instead of direct SSE proxy
3. Non-streaming requests handled normally (JSON response)
