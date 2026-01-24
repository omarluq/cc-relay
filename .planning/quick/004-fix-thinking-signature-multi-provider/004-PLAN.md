---
phase: quick
plan: 004
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/proxy/signature_cache.go
  - internal/proxy/signature_cache_test.go
  - internal/proxy/thinking.go
  - internal/proxy/thinking_test.go
  - internal/proxy/handler.go
  - internal/proxy/sse.go
autonomous: true

must_haves:
  truths:
    - "Signatures are cached by modelGroup + SHA256(thinkingText)[:16] with 3-hour sliding TTL"
    - "Cached signature takes priority over client-provided signature"
    - "Unsigned thinking blocks are dropped to prevent 400 errors"
    - "Tool use inherits signature from preceding thinking block in same message"
    - "Signatures emitted to client include modelGroup prefix: {group}#{sig}"
    - "Block reordering ensures thinking blocks precede other content"
    - "Detection uses bytes.Contains for speed (no JSON parsing on hot path)"
  artifacts:
    - path: "internal/proxy/signature_cache.go"
      provides: "Thread-safe signature caching with model groups and TTL"
      exports: ["SignatureCache", "NewSignatureCache", "GetModelGroup"]
    - path: "internal/proxy/thinking.go"
      provides: "Thinking block detection, manipulation, signature lookup"
      exports: ["HasThinkingBlocks", "ProcessRequestThinking", "ProcessResponseSignature"]
    - path: "internal/proxy/handler.go"
      provides: "Integration of signature processing into request/response flow"
  key_links:
    - from: "internal/proxy/handler.go"
      to: "internal/proxy/thinking.go"
      via: "ProcessRequestThinking call in ServeHTTP"
      pattern: "ProcessRequestThinking.*Request"
    - from: "internal/proxy/sse.go"
      to: "internal/proxy/thinking.go"
      via: "ProcessResponseSignature call on signature_delta events"
      pattern: "ProcessResponseSignature.*event"
---

<objective>
Fix thinking block signature invalidation when requests cross providers in multi-provider mode.

**Problem:** Extended thinking signatures are provider-specific. Round-robin routing causes Provider A's signature to be sent to Provider B, which rejects it with "Invalid signature in thinking block" (400).

**Solution (CLIProxyAPI approach, optimized for cc-relay):**
1. Cache signatures by `{modelGroup}:{SHA256(text)[:16]}` with 3-hour sliding TTL
2. On request: lookup cached signature → use if found → drop unsigned blocks
3. On response: cache signature when `signature_delta` arrives
4. Tool use inherits signature from preceding thinking block
5. Emit signatures with modelGroup prefix: `claude#abc123...`

**Optimizations for cc-relay:**
- Use existing `internal/cache.Cache` interface (Ristretto-backed)
- bytes.Contains detection (10-100x faster than JSON parsing)
- Zero-copy signature extraction with gjson
- Integrated with existing SSE streaming infrastructure
</objective>

<execution_context>
@./.claude/get-shit-done/workflows/execute-plan.md
@./.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@internal/proxy/handler.go
@internal/proxy/sse.go
@internal/cache/cache.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create signature cache module</name>
  <files>internal/proxy/signature_cache.go, internal/proxy/signature_cache_test.go</files>
  <action>
Create `internal/proxy/signature_cache.go` with optimized signature caching.

**Constants:**
```go
const (
    SignatureCacheTTL    = 3 * time.Hour  // CLIProxyAPI uses 3 hours
    SignatureHashLen     = 16             // First 16 hex chars of SHA256
    MinSignatureLen      = 50             // Minimum valid signature length
)
```

**Model Group Mapping:**
```go
// GetModelGroup returns the model group for signature sharing.
// Models in the same group share signatures (e.g., claude-sonnet-4, claude-3-opus → "claude").
func GetModelGroup(modelName string) string {
    switch {
    case strings.Contains(modelName, "claude"):
        return "claude"
    case strings.Contains(modelName, "gpt"):
        return "gpt"
    case strings.Contains(modelName, "gemini"):
        return "gemini"
    default:
        return modelName // Fallback to exact model name
    }
}
```

**SignatureCache struct:**
```go
// SignatureCache provides thread-safe caching of thinking block signatures.
// Uses cc-relay's cache.Cache interface for storage.
type SignatureCache struct {
    cache cache.Cache
}

// NewSignatureCache creates a new signature cache using the provided cache backend.
func NewSignatureCache(c cache.Cache) *SignatureCache

// cacheKey builds the cache key: "sig:{modelGroup}:{textHash}"
func (sc *SignatureCache) cacheKey(modelGroup, text string) string {
    h := sha256.Sum256([]byte(text))
    textHash := hex.EncodeToString(h[:])[:SignatureHashLen]
    return fmt.Sprintf("sig:%s:%s", modelGroup, textHash)
}

// Get retrieves a cached signature for the given model and text.
// Returns empty string on cache miss.
func (sc *SignatureCache) Get(ctx context.Context, modelName, text string) string

// Set caches a signature for the given model and text.
// Skips caching if signature is too short.
func (sc *SignatureCache) Set(ctx context.Context, modelName, text, signature string)

// IsValid checks if a signature is valid (non-empty and long enough).
// Special case: "skip_thought_signature_validator" is valid for gemini models.
func IsValidSignature(modelName, signature string) bool
```

**Test file with:**
- Test GetModelGroup mapping (claude-*, gpt-*, gemini-*, unknown)
- Test cache key generation (deterministic, correct format)
- Test Get/Set round-trip
- Test TTL expiration (mock or short TTL)
- Test IsValidSignature (valid, too short, empty, gemini sentinel)
  </action>
  <verify>
`go test ./internal/proxy/... -run SignatureCache -v` passes

Test cases cover:
- Model group detection
- Cache hit/miss
- Signature validation
- TTL behavior
  </verify>
  <done>
signature_cache.go exists with SignatureCache, GetModelGroup, IsValidSignature
signature_cache_test.go exists with 5+ test cases
All tests pass
  </done>
</task>

<task type="auto">
  <name>Task 2: Create thinking block processor</name>
  <files>internal/proxy/thinking.go, internal/proxy/thinking_test.go</files>
  <action>
Create `internal/proxy/thinking.go` with thinking block detection and manipulation.

**Fast Detection (hot path):**
```go
// Detection markers for thinking blocks
var (
    thinkingTypeMarker = []byte(`"type":"thinking"`)
    thinkingFieldMarker = []byte(`"thinking":`)
    signatureMarker     = []byte(`"signature":`)
)

// HasThinkingBlocks performs fast detection without JSON parsing.
// Returns true if the body likely contains thinking blocks with signatures.
// Uses bytes.Contains which is 10-100x faster than JSON parsing.
func HasThinkingBlocks(body []byte) bool {
    return bytes.Contains(body, thinkingTypeMarker) &&
           bytes.Contains(body, thinkingFieldMarker) &&
           bytes.Contains(body, signatureMarker)
}
```

**Request Processing:**
```go
// ThinkingContext holds state for processing thinking blocks in a message.
type ThinkingContext struct {
    CurrentSignature string // Signature from most recent thinking block (for tool_use inheritance)
    DroppedBlocks    int    // Count of unsigned blocks dropped
    ReorderedBlocks  bool   // Whether blocks were reordered
}

// ProcessRequestThinking processes thinking blocks in the request body:
// 1. Looks up cached signatures for thinking blocks
// 2. Falls back to client-provided signature (validating format)
// 3. Drops unsigned thinking blocks
// 4. Tracks signature for tool_use inheritance
// 5. Reorders blocks so thinking comes first
//
// Returns modified body and context, or error.
func ProcessRequestThinking(
    ctx context.Context,
    body []byte,
    modelName string,
    cache *SignatureCache,
) ([]byte, *ThinkingContext, error)
```

**Response Processing:**
```go
// ProcessResponseSignature handles signature_delta events from upstream.
// Extracts signature, caches it, and transforms to include modelGroup prefix.
// Returns the modified event data with prefixed signature.
func ProcessResponseSignature(
    ctx context.Context,
    eventData []byte,
    thinkingText string,
    modelName string,
    cache *SignatureCache,
) []byte
```

**Block Reordering:**
```go
// reorderAssistantBlocks ensures thinking blocks come before text/tool_use.
// This is required by some providers (Gemini/Vertex via Antigravity).
// Returns reordered content array.
func reorderAssistantBlocks(content []gjson.Result) []gjson.Result
```

**Signature Format Helpers:**
```go
// FormatSignature adds modelGroup prefix: "claude#abc123..."
func FormatSignature(modelName, signature string) string {
    return fmt.Sprintf("%s#%s", GetModelGroup(modelName), signature)
}

// ParseSignature extracts modelGroup and raw signature from prefixed format.
// Returns modelGroup, signature, ok.
func ParseSignature(prefixed string) (modelGroup, signature string, ok bool)
```

**Tool Use Handling:**
```go
// ProcessToolUse applies signature to tool_use block.
// Uses inherited signature from preceding thinking block, or sentinel.
func ProcessToolUse(
    toolBlock gjson.Result,
    inheritedSignature string,
    modelName string,
) (json.RawMessage, error)
```

**Test file with:**
- Test HasThinkingBlocks (positive, negative, partial matches)
- Test ProcessRequestThinking (cached sig, client sig, no sig → drop)
- Test block reordering (thinking after text → thinking first)
- Test tool use signature inheritance
- Test signature format/parse roundtrip
- Benchmark HasThinkingBlocks vs JSON parsing
  </action>
  <verify>
`go test ./internal/proxy/... -run Thinking -v` passes

Benchmark shows bytes.Contains is faster than JSON parsing:
`go test ./internal/proxy/... -bench BenchmarkThinking -benchmem`
  </verify>
  <done>
thinking.go exists with all functions
thinking_test.go exists with 8+ test cases
All tests pass
Benchmark confirms speed improvement
  </done>
</task>

<task type="auto">
  <name>Task 3: Integrate signature processing into handler</name>
  <files>internal/proxy/handler.go</files>
  <action>
Modify `internal/proxy/handler.go` to integrate thinking signature processing.

**Add SignatureCache to Handler:**
```go
type Handler struct {
    // ... existing fields ...
    signatureCache *SignatureCache // Thinking signature cache
}

// In NewHandler:
func NewHandler(..., sigCache *SignatureCache, ...) (*Handler, error) {
    h := &Handler{
        // ... existing ...
        signatureCache: sigCache,
    }
    // ...
}
```

**Request Processing in ServeHTTP:**
```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ... existing setup code ...

    // Read body for thinking detection
    body, err := io.ReadAll(r.Body)
    if err != nil {
        // handle error
    }
    r.Body = io.NopCloser(bytes.NewReader(body))

    // Fast path: check if request has thinking blocks
    if h.signatureCache != nil && HasThinkingBlocks(body) {
        // Extract model from body
        modelName := gjson.GetBytes(body, "model").String()

        // Process thinking blocks
        modifiedBody, thinkingCtx, err := ProcessRequestThinking(
            r.Context(), body, modelName, h.signatureCache,
        )
        if err != nil {
            // log warning, continue with original body
        } else {
            body = modifiedBody
            r.Body = io.NopCloser(bytes.NewReader(body))
            r.ContentLength = int64(len(body))

            // Store context for response processing
            ctx := context.WithValue(r.Context(), thinkingContextKey, thinkingCtx)
            r = r.WithContext(ctx)
        }
    }

    // ... continue with existing proxy logic ...
}
```

**Store model name in context for response processing:**
```go
const modelNameContextKey contextKey = "modelName"

// In ServeHTTP after model extraction:
ctx := context.WithValue(r.Context(), modelNameContextKey, modelName)
r = r.WithContext(ctx)
```
  </action>
  <verify>
`go test ./internal/proxy/... -v` passes

Verify handler passes modelName and thinkingCtx in context
  </verify>
  <done>
Handler struct has signatureCache field
ServeHTTP calls HasThinkingBlocks and ProcessRequestThinking
Model name stored in context
All tests pass
  </done>
</task>

<task type="auto">
  <name>Task 4: Integrate signature caching into SSE response handling</name>
  <files>internal/proxy/sse.go</files>
  <action>
Modify SSE response handling to cache signatures from `signature_delta` events.

**Track thinking text during streaming:**
```go
// In SSE forwarding code, track current thinking text
var currentThinkingText strings.Builder

// When processing content_block_delta with type "thinking_delta":
if delta.Type == "thinking_delta" {
    currentThinkingText.WriteString(delta.Thinking)
}
```

**Cache and transform signature_delta:**
```go
// When processing content_block_delta with type "signature_delta":
if delta.Type == "signature_delta" && sigCache != nil {
    // Get model name from context
    modelName, _ := r.Context().Value(modelNameContextKey).(string)

    // Cache the signature
    sigCache.Set(r.Context(), modelName, currentThinkingText.String(), delta.Signature)
    currentThinkingText.Reset()

    // Transform signature to include modelGroup prefix
    delta.Signature = FormatSignature(modelName, delta.Signature)

    // Re-encode the modified event
    modifiedData := encodeSignatureDelta(delta)
    // Write modified event to client
}
```

**Handle non-streaming responses:**
If non-streaming mode, process complete response body:
```go
func processNonStreamingResponse(body []byte, modelName string, cache *SignatureCache) []byte {
    // Find thinking blocks in response
    // Extract and cache signatures
    // Add modelGroup prefix to signatures
    return modifiedBody
}
```
  </action>
  <verify>
`go test ./internal/proxy/... -run SSE -v` passes

Integration test:
1. Send request with thinking enabled
2. Verify signature cached from response
3. Send follow-up request
4. Verify cached signature used
  </verify>
  <done>
SSE handler caches signatures from signature_delta events
Signatures are transformed with modelGroup prefix
Thinking text accumulated for cache key
All tests pass
  </done>
</task>

<task type="auto">
  <name>Task 5: Add handler integration tests</name>
  <files>internal/proxy/handler_thinking_test.go</files>
  <action>
Create comprehensive integration tests for thinking signature handling.

**Test Cases:**

1. `TestHandler_ThinkingSignature_CacheHit`:
   - Pre-populate cache with signature
   - Send request with thinking block (same text)
   - Verify cached signature is used

2. `TestHandler_ThinkingSignature_CacheMiss_ClientSignature`:
   - Empty cache
   - Send request with valid client-provided signature
   - Verify client signature is used

3. `TestHandler_ThinkingSignature_UnsignedBlock_Dropped`:
   - Empty cache
   - Send request with thinking block, no signature
   - Verify thinking block is dropped from request

4. `TestHandler_ThinkingSignature_ToolUseInheritance`:
   - Send request with thinking block followed by tool_use
   - Verify tool_use inherits thinking's signature

5. `TestHandler_ThinkingSignature_BlockReordering`:
   - Send request with [text, thinking] order
   - Verify output has [thinking, text] order

6. `TestHandler_ThinkingSignature_ResponseCaching`:
   - Send request, get response with signature_delta
   - Verify signature is cached
   - Send follow-up request with same thinking text
   - Verify cached signature is used

7. `TestHandler_ThinkingSignature_ModelGroupSharing`:
   - Cache signature with claude-sonnet-4
   - Request with claude-3-opus (same thinking text)
   - Verify signature is shared (same "claude" group)

8. `TestHandler_ThinkingSignature_CrossProviderRouting`:
   - Configure round-robin with 2+ providers
   - Send conversation with thinking
   - Verify no "Invalid signature" errors across turns
  </action>
  <verify>
`go test ./internal/proxy/... -run ThinkingSignature -v` passes

All 8 test cases pass
  </verify>
  <done>
handler_thinking_test.go exists with 8 test cases
All tests pass
Cross-provider routing test confirms fix
  </done>
</task>

</tasks>

<verification>
1. All tests pass: `task test`
2. Linters pass: `task lint`
3. Benchmarks show improvement: `go test ./internal/proxy/... -bench BenchmarkThinking`
4. Manual test with multiple providers and extended thinking enabled
5. No "Invalid signature in thinking block" errors in multi-turn conversations
</verification>

<success_criteria>
- Signatures are cached and reused across requests
- Unsigned thinking blocks are dropped (prevents 400 errors)
- Tool use inherits signature from preceding thinking block
- Signature format includes modelGroup prefix
- Detection is fast (bytes.Contains, not JSON parsing)
- Works with existing cc-relay cache infrastructure
- All existing tests continue to pass
- No regressions in single-provider mode
</success_criteria>

<output>
After completion, create `.planning/quick/004-fix-thinking-signature-multi-provider/004-SUMMARY.md`
</output>
