---
status: resolved
trigger: "Z.AI authentication failing with 401 Unauthorized. Transparent auth mode is forwarding client Authorization header but Z.AI returns 401."
created: 2026-01-22T00:00:00Z
updated: 2026-01-22T00:03:00Z
---

## Current Focus

hypothesis: CONFIRMED - Transparent auth forwards Authorization header but NOT x-api-key, and Z.AI requires x-api-key
test: Fix implemented - running verification
expecting: Tests pass, Z.AI requests use configured keys instead of client auth
next_action: Verify all tests pass, archive session

## Symptoms

expected: Z.AI should authenticate successfully and proxy requests to https://api.z.ai/api/anthropic
actual: All requests return 401 Unauthorized despite "authentication succeeded" in proxy logs
errors:
  - WRN -> Unauthorized (XXXms) for every request
  - Logs show: has_authorization=true has_x_api_key=false
reproduction: Any request through cc-relay to Z.AI provider
started: Current implementation - transparent auth was just added in phase 02.2

## Eliminated

(none yet)

## Evidence

- timestamp: 2026-01-22T00:00:00Z
  checked: User-provided logs
  found: "transparent mode: forwarding client auth" and "has_authorization=true has_x_api_key=false"
  implication: The proxy is forwarding the client's Authorization header (Claude Code's token) rather than using ZAI_API_KEY

- timestamp: 2026-01-22T00:01:00Z
  checked: internal/proxy/handler.go lines 62-103 (Rewrite function)
  found: |
    Transparent mode condition (line 66): if clientAuth != "" || clientAPIKey != ""
    When client sends Authorization header but NO x-api-key:
    - Code enters transparent mode (line 67-77)
    - Only forwards anthropic-* headers
    - Does NOT strip Authorization or convert it to x-api-key
    - Does NOT call provider.Authenticate() in transparent mode
  implication: Z.AI receives Authorization header (Claude's bearer token) but expects x-api-key header

- timestamp: 2026-01-22T00:01:00Z
  checked: internal/providers/base.go line 47-48 (Authenticate method)
  found: Provider.Authenticate() sets x-api-key header: req.Header.Set("x-api-key", key)
  implication: This is only called in FALLBACK mode, never in transparent mode

- timestamp: 2026-01-22T00:02:00Z
  checked: All tests after fix
  found: All 17 handler tests pass, including new tests for non-transparent providers
  implication: Fix is working correctly

## Resolution

root_cause: |
  The transparent auth design assumes the client's auth headers are valid for the backend provider.
  When Claude Code sends Authorization: Bearer <anthropic-token>, the proxy forwards it as-is.
  But Z.AI does not accept Anthropic tokens - it needs ZAI_API_KEY in the x-api-key header.

  The fundamental issue: Transparent mode is designed for direct Anthropic -> Anthropic forwarding,
  not for cross-provider routing (Claude auth -> Z.AI backend).

fix: |
  Added SupportsTransparentAuth() method to Provider interface:
  - Anthropic provider returns true (client tokens work directly)
  - All other providers (Z.AI, Ollama, etc.) return false via BaseProvider default

  Modified handler.go to check provider.SupportsTransparentAuth() before entering transparent mode.
  When false, proxy uses configured API keys even if client sends Authorization header.

verification: |
  - All existing tests pass (17 handler tests)
  - Added 2 new tests for non-transparent provider behavior:
    - TestHandler_NonTransparentProviderUsesConfiguredKeys
    - TestHandler_NonTransparentProviderWithKeyPool
  - go test ./... -short passes

files_changed:
  - internal/providers/provider.go - Added SupportsTransparentAuth() to interface
  - internal/providers/base.go - Added SupportsTransparentAuth() returning false
  - internal/providers/anthropic.go - Added SupportsTransparentAuth() returning true
  - internal/proxy/handler.go - Check SupportsTransparentAuth() before transparent mode
  - internal/proxy/handler_test.go - Added SupportsTransparentAuth() to mock, added 2 new tests
