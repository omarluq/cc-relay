# Bug Analysis: Dynamic Provider Routing

Generated: 2026-01-23

## Symptom

When multiple providers are configured, ALL requests are sent to the FIRST provider's URL, even when the router selects a different provider. The model mapping correctly uses the selected provider, but the HTTP request goes to the wrong backend.

**Example failure scenario:**
1. Providers configured: Anthropic (first), Z.AI, Ollama
2. Ollama has `model_mapping: claude-opus-4-5-20251101: qwen3:8b`
3. Router selects Ollama
4. Model is rewritten to `qwen3:8b` (correct)
5. Request is sent to Anthropic's URL (incorrect)
6. Anthropic returns 404 for `qwen3:8b`

## Erotetic Analysis

**X (symptom):** Request goes to wrong provider despite router selection

**Q (questions to resolve):**
1. Where is the target URL set? (Answered: Line 61, NewHandler)
2. Where is the selected provider determined? (Answered: Line 253, ServeHTTP)
3. Why doesn't the URL update with provider selection? (Answered: Static closure capture)
4. What components correctly use selected provider? (Answered: Model mapping, logging, context)
5. What components incorrectly use static provider? (Answered: URL, auth, headers in Rewrite)

## Root Cause

**Location:** `/home/omarluq/sandbox/go/cc-relay/internal/proxy/handler.go`

### Static URL Problem (Line 61)

```go
// Line 61: URL parsed ONCE at handler creation from FIRST provider
targetURL, err := url.Parse(provider.BaseURL())
```

This `targetURL` is captured in the `Rewrite` closure and used for ALL requests:

```go
// Line 80: Static URL used for EVERY request
r.SetURL(targetURL)
```

### Static Provider Problem (Lines 91, 118, 123)

The `Rewrite` closure references `h.provider` (the handler's static first provider):

```go
// Line 91: Transparent auth check uses static provider
if hasClientAuth && h.provider.SupportsTransparentAuth() {

// Line 118: Authentication uses static provider  
h.provider.Authenticate(r.Out, selectedKey)

// Line 123: Header forwarding uses static provider
forwardHeaders := h.provider.ForwardHeaders(r.In.Header)
```

### Correct Usage in ServeHTTP (Lines 253-299)

The `ServeHTTP` method correctly selects and uses the dynamic provider:

```go
// Line 253: Provider correctly selected via router
selectedProviderInfo, err := h.selectProvider(r.Context())
selectedProvider := selectedProviderInfo.Provider

// Line 266: Provider name stored in context (for circuit breaker)
r = r.WithContext(context.WithValue(r.Context(), providerNameContextKey, selectedProvider.Name()))

// Line 295: Model mapping correctly uses selected provider
if mapping := selectedProvider.GetModelMapping(); len(mapping) > 0 {
    rewriter := NewModelRewriter(mapping)
    // ...
}
```

But then on line 309, it calls the shared proxy which uses the static provider:

```go
// Line 309: Proxy uses static targetURL and h.provider
h.proxy.ServeHTTP(w, r)
```

## Investigation Trail

| Step | Action | Finding |
|------|--------|---------|
| 1 | Read handler.go lines 1-100 | Found static `targetURL` on line 61 |
| 2 | Read handler.go lines 100-200 | Found static `h.provider` usage in Rewrite closure |
| 3 | Read handler.go lines 200-300 | Found correct `selectedProvider` usage in ServeHTTP |
| 4 | Read handler.go lines 300-400 | Confirmed proxy invocation on line 309 |
| 5 | Read router/router.go | Confirmed ProviderInfo structure and Select interface |
| 6 | Read providers/provider.go | Confirmed Provider interface methods |

## Evidence Summary

### Finding 1: Static URL Capture

- **Location:** `/home/omarluq/sandbox/go/cc-relay/internal/proxy/handler.go:61`
- **Observation:** `targetURL` is parsed once from `provider.BaseURL()` and captured in closure
- **Relevance:** This is the root cause - URL never changes per-request

### Finding 2: Static Provider in Rewrite Closure

- **Location:** `/home/omarluq/sandbox/go/cc-relay/internal/proxy/handler.go:78-127`
- **Observation:** `Rewrite` closure uses `h.provider` for auth, headers, transparent auth check
- **Relevance:** Even if URL was dynamic, auth would use wrong provider

### Finding 3: Correct Provider Selection Not Propagated

- **Location:** `/home/omarluq/sandbox/go/cc-relay/internal/proxy/handler.go:253-259`
- **Observation:** `selectedProvider` is correctly obtained but only stored for logging/context
- **Relevance:** The fix must propagate this to the Rewrite closure

### Finding 4: Existing Context Pattern

- **Location:** `/home/omarluq/sandbox/go/cc-relay/internal/proxy/handler.go:27-30`
- **Observation:** Context keys already exist: `keyIDContextKey`, `providerNameContextKey`
- **Relevance:** Same pattern can be used for selected provider

## Confidence

**Confidence:** High

The bug is clearly a design oversight where the proxy was implemented for single-provider mode but extended for multi-provider routing without updating the `Rewrite` closure to use dynamic provider selection.

## Recommended Fix

### Files to Modify

1. `/home/omarluq/sandbox/go/cc-relay/internal/proxy/handler.go`

### Code Changes

#### 1. Add new context key (line ~30)

```go
const (
    keyIDContextKey           contextKey = "keyID"
    providerNameContextKey    contextKey = "providerName"
    selectedProviderContextKey contextKey = "selectedProvider"  // ADD THIS
)
```

#### 2. Store selected provider in context (after line 266)

```go
// Store provider name in context for modifyResponse to report to circuit breaker
r = r.WithContext(context.WithValue(r.Context(), providerNameContextKey, selectedProvider.Name()))

// ADD: Store selected provider in context for Rewrite closure
r = r.WithContext(context.WithValue(r.Context(), selectedProviderContextKey, selectedProvider))
```

#### 3. Update Rewrite closure to use dynamic provider (lines 78-127)

Replace the static `Rewrite` function to get provider from context:

```go
Rewrite: func(r *httputil.ProxyRequest) {
    // Get selected provider from context (set in ServeHTTP)
    selectedProvider, ok := r.In.Context().Value(selectedProviderContextKey).(providers.Provider)
    if !ok {
        selectedProvider = h.provider // Fallback to static provider
    }
    
    // Parse and set backend URL dynamically
    targetURL, err := url.Parse(selectedProvider.BaseURL())
    if err != nil {
        // Log error and fall back to static URL
        // Note: This shouldn't happen since BaseURL is validated at config time
        targetURL, _ = url.Parse(h.provider.BaseURL())
    }
    r.SetURL(targetURL)
    r.SetXForwarded()

    // Check if client provided auth headers
    clientAuth := r.In.Header.Get("Authorization")
    clientAPIKey := r.In.Header.Get("x-api-key")
    hasClientAuth := clientAuth != "" || clientAPIKey != ""

    // Use SELECTED provider for transparent auth check (not static h.provider)
    if hasClientAuth && selectedProvider.SupportsTransparentAuth() {
        // TRANSPARENT MODE: Forward client auth unchanged
        // ... existing transparent mode code ...
    } else {
        // CONFIGURED KEY MODE: Use our configured keys
        r.Out.Header.Del("Authorization")
        r.Out.Header.Del("x-api-key")

        selectedKey := r.In.Header.Get("X-Selected-Key")
        if selectedKey == "" {
            selectedKey = h.apiKey
        }

        if selectedKey != "" {
            // Use SELECTED provider for authentication (not static h.provider)
            selectedProvider.Authenticate(r.Out, selectedKey)
        }

        // Use SELECTED provider for header forwarding (not static h.provider)
        forwardHeaders := selectedProvider.ForwardHeaders(r.In.Header)
        // ... rest of header forwarding ...
    }
},
```

#### 4. Also update handleAuthAndKeySelection (line 332)

This method also uses `h.provider.SupportsTransparentAuth()` which should use the selected provider:

```go
// Line 332: Currently uses static provider
useTransparentAuth := hasClientAuth && h.provider.SupportsTransparentAuth()

// Should be updated to accept selected provider as parameter
```

## Prevention

1. **Add integration test** for multi-provider routing that verifies:
   - Request URL matches selected provider
   - Auth headers match selected provider's format
   - Model mapping from selected provider is applied

2. **Add request tracing** to log the actual destination URL in debug mode

3. **Consider architectural change**: Create per-provider proxy instances (as done in christianfaust/cc-relay fork) for cleaner separation

## Summary

The bug is a closure capture issue where `targetURL` and `h.provider` are captured once at handler creation and never updated per-request. The fix requires:

1. Store `selectedProvider` in request context (already done for provider name)
2. Read `selectedProvider` from context in `Rewrite` closure
3. Dynamically parse URL from `selectedProvider.BaseURL()`
4. Use `selectedProvider` for `SupportsTransparentAuth()`, `Authenticate()`, and `ForwardHeaders()`
