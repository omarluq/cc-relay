---
phase: 06-cloud-providers
plan: 01
subsystem: providers
tags: [interface, transformation, config, cloud, bedrock, vertex, azure]

dependency-graph:
  requires: [phase-05]
  provides:
    - Extended Provider interface with transformation methods
    - Shared transformation utilities for cloud providers
    - Cloud-specific configuration fields
  affects: [06-02, 06-03, 06-04, 06-05]

tech-stack:
  added:
    - github.com/tidwall/gjson
    - github.com/tidwall/sjson
  patterns:
    - Interface extension with default implementations
    - JSON manipulation without full parse/serialize

key-files:
  created:
    - internal/providers/transform.go
    - internal/providers/transform_test.go
  modified:
    - internal/providers/provider.go
    - internal/providers/base.go
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/router/weighted_round_robin_test.go
    - internal/proxy/handler_test.go
    - internal/proxy/model_filter_test.go

decisions:
  - name: "Default no-op implementations in BaseProvider"
    rationale: "Existing providers (Anthropic, Z.AI, Ollama) continue to work without modification"
    tradeoffs: "Cloud providers must override 4 methods"
  - name: "nolint:govet for ProviderConfig"
    rationale: "Field order optimized for readability over 16-byte memory savings"
    tradeoffs: "Minor memory overhead acceptable for config struct"

metrics:
  duration: 11 min
  completed: 2026-01-25
---

# Phase 06 Plan 01: Provider Interface Extension Summary

Extended the Provider interface with transformation capabilities and added cloud-specific configuration fields to enable Bedrock, Azure, and Vertex AI provider implementations.

## One-liner

Provider interface extended with TransformRequest/TransformResponse/RequiresBodyTransform/StreamingContentType plus cloud config fields for AWS, GCP, and Azure.

## Tasks Completed

| Task | Name | Commit | Key Changes |
|------|------|--------|-------------|
| 1-2 | Extend Provider interface + BaseProvider defaults | a33c8cd | 4 new interface methods, default implementations |
| 3 | Create shared transformation utilities | b7ec709 | ExtractModel, RemoveModelFromBody, AddAnthropicVersion |
| 4 | Add cloud config fields to ProviderConfig | 92c88a3 | AWS/GCP/Azure fields, ValidateCloudConfig |

## Key Changes

### Provider Interface Extension (internal/providers/provider.go)

Added 4 new methods to the Provider interface:

```go
// TransformRequest modifies request body for cloud providers
TransformRequest(body []byte, endpoint string) (newBody []byte, targetURL string, err error)

// TransformResponse converts response format (e.g., Bedrock Event Stream to SSE)
TransformResponse(resp *http.Response, w http.ResponseWriter) error

// RequiresBodyTransform indicates if provider needs body modification
RequiresBodyTransform() bool

// StreamingContentType returns expected Content-Type for streaming
StreamingContentType() string
```

### Default Implementations (internal/providers/base.go)

BaseProvider provides no-op defaults:
- TransformRequest: Returns body unchanged, constructs standard URL
- TransformResponse: No transformation (standard proxy handling)
- RequiresBodyTransform: Returns false
- StreamingContentType: Returns "text/event-stream"

### Transformation Utilities (internal/providers/transform.go)

Shared functions for cloud provider request transformation:

- `ExtractModel(body)` - Extract model field from JSON
- `RemoveModelFromBody(body)` - Remove model (goes in URL path for cloud)
- `AddAnthropicVersion(body, version)` - Add anthropic_version field
- `TransformBodyForCloudProvider(body, version)` - Combined pipeline

Uses tidwall/gjson and tidwall/sjson for efficient JSON manipulation without full parse/serialize.

### Cloud Configuration (internal/config/config.go)

New fields in ProviderConfig:

| Field | Provider | Required |
|-------|----------|----------|
| AWSRegion | Bedrock | Yes |
| AWSAccessKeyID | Bedrock | No (SDK default chain) |
| AWSSecretAccessKey | Bedrock | No (SDK default chain) |
| GCPProjectID | Vertex | Yes |
| GCPRegion | Vertex | Yes |
| AzureResourceName | Azure | Yes |
| AzureDeploymentID | Azure | No |
| AzureAPIVersion | Azure | No (default: 2024-06-01) |

New methods:
- `GetAzureAPIVersion()` - Returns version with default fallback
- `ValidateCloudConfig()` - Validates required fields per provider type

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Update mock providers in test files**

- **Found during:** Task 2 (commit phase)
- **Issue:** Pre-commit hooks failed - mock providers in test files didn't implement new interface methods
- **Fix:** Added TransformRequest, TransformResponse, RequiresBodyTransform, StreamingContentType to mock providers in:
  - internal/router/weighted_round_robin_test.go
  - internal/proxy/handler_test.go
  - internal/proxy/model_filter_test.go
- **Commit:** a33c8cd

**2. [Rule 1 - Bug] Fix linter warnings for named return parameters**

- **Found during:** Task 2 (commit phase)
- **Issue:** gocritic unnamedResult warning for TransformRequest signature
- **Fix:** Added named return parameters (newBody, targetURL, err)
- **Commit:** a33c8cd

**3. [Rule 1 - Bug] Fix line length and field alignment warnings**

- **Found during:** Task 4 (commit phase)
- **Issue:** lll warning for long line, govet fieldalignment warnings
- **Fix:** Multi-line function signature, nolint directive for ProviderConfig, fixed test struct field order
- **Commit:** 92c88a3

## Verification Results

All verification criteria passed:

- [x] `go build ./...` succeeds
- [x] `go test ./...` passes (14 packages)
- [x] `task lint` passes (0 issues)
- [x] Provider interface has 4 new methods
- [x] BaseProvider has default implementations
- [x] Transform utilities have >80% coverage (86.4%)
- [x] ProviderConfig has all cloud fields with validation

## Test Coverage

| Package | Coverage |
|---------|----------|
| internal/providers | 86.4% |
| internal/config | 90%+ |

Transform utilities:
- ExtractModel: 100%
- RemoveModelFromBody: 100%
- AddAnthropicVersion: 100%
- TransformBodyForCloudProvider: 75%

## Next Phase Readiness

**Ready for 06-02 (Bedrock Provider):**
- TransformRequest interface ready for AWS SigV4 signing
- TransformBodyForCloudProvider handles model removal and version injection
- AWSRegion, AWSAccessKeyID, AWSSecretAccessKey config fields available
- ValidateCloudConfig checks required fields

**Ready for 06-03 (Vertex Provider):**
- TransformRequest interface ready for Google OAuth
- GCPProjectID, GCPRegion config fields available

**Ready for 06-04 (Azure Provider):**
- TransformRequest interface ready for Azure auth
- AzureResourceName, AzureDeploymentID, AzureAPIVersion config fields available

## Dependencies Added

```text
github.com/tidwall/gjson v1.18.0
github.com/tidwall/sjson v1.2.5
```

Used for efficient JSON field manipulation without full parse/serialize overhead.
