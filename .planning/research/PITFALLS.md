# Pitfalls Research

**Domain:** Multi-provider LLM proxy for Claude Code
**Researched:** 2026-01-20
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: SSE Streaming Buffering

**What goes wrong:**
Proxy buffers SSE events instead of flushing them immediately, causing Claude Code to hang waiting for responses. Chunks arrive all at once after completion instead of streaming incrementally, breaking the interactive experience.

**Why it happens:**
- Default HTTP proxies and reverse proxies (nginx, Cloudflare, Azure App Gateway) buffer responses
- Go's `http.ResponseWriter` doesn't automatically flush after each write
- Missing critical headers that disable buffering
- Platform-specific behaviors (Vercel, Azure App Service) enable buffering by default

**How to avoid:**
1. Set required headers on SSE responses:
   ```go
   w.Header().Set("Content-Type", "text/event-stream")
   w.Header().Set("Cache-Control", "no-cache, no-transform")
   w.Header().Set("X-Accel-Buffering", "no")  // Critical for nginx/Cloudflare
   w.Header().Set("Connection", "keep-alive")
   ```
2. Use `http.Flusher` interface to flush after each SSE event:
   ```go
   flusher, ok := w.(http.Flusher)
   if ok {
       flusher.Flush()
   }
   ```
3. Test with real Claude Code, not just curl (curl may hide buffering issues)

**Warning signs:**
- Claude Code shows "waiting for response" spinner indefinitely
- Responses arrive all at once after long delay
- Works in local dev but fails when deployed behind nginx/CDN
- Network inspector shows response body only after stream completes

**Phase to address:**
Phase 1 (MVP) - Core proxy implementation must get streaming right from the start. Retrofitting correct streaming behavior is extremely difficult.

---

### Pitfall 2: Tool Use ID Preservation Failure

**What goes wrong:**
Proxy fails to preserve `tool_use_id` when handling parallel tool calls, causing Claude Code to reject responses with "orphan tool_result blocks" errors. This breaks agent workflows that spawn multiple concurrent operations.

**Why it happens:**
- Naive proxy implementations transform requests/responses without preserving all fields
- JSON marshaling/unmarshaling drops unknown fields if not using `map[string]interface{}`
- Tool use blocks are treated individually instead of atomically
- Provider API differences cause field mappings to lose IDs during transformation

**How to avoid:**
1. Preserve ALL fields when transforming requests:
   ```go
   // BAD: Struct marshaling drops unknown fields
   type Message struct {
       Role    string `json:"role"`
       Content string `json:"content"`
   }

   // GOOD: Use map to preserve all fields
   var message map[string]interface{}
   json.Unmarshal(body, &message)
   ```
2. Handle multiple `tool_use` blocks atomically in a single message
3. Validate tool IDs match between request and response in integration tests
4. Test specifically with parallel tool calls (Read + Bash + Grep simultaneously)

**Warning signs:**
- "API Error: 400 - orphan tool_result blocks" in Claude Code
- Parallel tool operations fail while sequential operations work
- Errors only occur with 3+ simultaneous tools, not simple cases
- Integration tests pass but real Claude Code usage fails

**Phase to address:**
Phase 1 (MVP) - Must be correct from the start. This is a hard API compatibility requirement, not an enhancement.

---

### Pitfall 3: Weak Authentication and Public Exposure

**What goes wrong:**
Proxy deployed without robust authentication becomes an access broker for attackers to consume your paid API credits. Between Oct 2025 and Jan 2026, over 91,000 attack sessions targeted misconfigured LLM proxies.

**Why it happens:**
- Developers test with `SKIP_AUTH=true` and forget to disable in production
- Assuming network-level security is sufficient (it's not - proxies get discovered)
- Using weak API keys or relying solely on IP allowlisting
- Forgetting that attackers now scan for LLM endpoints like any other infrastructure

**How to avoid:**
1. Never deploy with authentication disabled, even "temporarily"
2. Implement proper API key validation:
   ```go
   // Validate incoming key against allowed keys
   if !isValidAPIKey(req.Header.Get("x-api-key")) {
       http.Error(w, "Unauthorized", http.StatusUnauthorized)
       return
   }
   ```
3. Add rate limiting per API key (not just per IP)
4. Log all authentication failures and alert on suspicious patterns
5. Use separate API keys for the proxy (not the same keys as backend providers)
6. Consider mutual TLS for internal deployments

**Warning signs:**
- Unexpected spike in API usage/costs
- High rate of 401/403 errors in logs
- Traffic from unexpected geographic regions
- Usage patterns inconsistent with known clients

**Phase to address:**
Phase 1 (MVP) - Authentication must be present from day one. Add advanced features (mutual TLS, OAuth) in Phase 2.

---

### Pitfall 4: Rate Limit Bypass via Key Pool Mismanagement

**What goes wrong:**
Rate limiting implementation can be bypassed because the proxy identifies requests by IP instead of by API key, or fails to track limits correctly across multiple backend keys in the pool.

**Why it happens:**
- Using IP-based rate limiting when clients can use proxies/VPNs
- Not tracking per-key limits separately for each backend provider key
- Sharing rate limit counters across unrelated API keys
- Forgetting that provider rate limits are per-key, not per-proxy

**How to avoid:**
1. Track rate limits per incoming API key AND per backend provider key:
   ```go
   type RateLimiter struct {
       incomingKeyLimits map[string]*TokenBucket  // Proxy API key
       providerKeyLimits map[string]*TokenBucket  // Backend API key
   }
   ```
2. Don't use IP address as the primary rate limit key
3. Respect provider-specific rate limits (RPM, TPM, RPD)
4. Implement token bucket or sliding window (not fixed window) for burst handling
5. Return proper `Retry-After` headers when rate limited

**Warning signs:**
- Clients bypass rate limits by rotating IPs
- Backend providers return 429 despite proxy "enforcing" limits
- Rate limits fail to prevent abuse during traffic spikes
- Different API keys share the same quota unexpectedly

**Phase to address:**
Phase 2 (Multi-key pooling) - Implement comprehensive rate tracking. Phase 1 can have simple global limits, but Phase 2 must get per-key tracking correct.

---

### Pitfall 5: Circuit Breaker Anti-Patterns

**What goes wrong:**
Circuit breaker implementation treats all failures equally, opening the circuit for recoverable errors (like 400 Bad Request), or uses original expensive requests for health probing instead of dedicated health checks.

**Why it happens:**
- Not distinguishing between client errors (4xx) and server errors (5xx)
- Using the half-open state to retry the same failed operation
- Treating partial failures as complete system failure
- Cascading failures when circuit breaker logic itself becomes a bottleneck

**How to avoid:**
1. Only count server errors (5xx, timeouts, connection failures) as circuit breaker failures:
   ```go
   func shouldCountAsFailure(statusCode int, err error) bool {
       if err != nil && (isTimeout(err) || isConnectionError(err)) {
           return true
       }
       return statusCode >= 500
   }
   ```
2. Use dedicated health check endpoints in half-open state, not original requests
3. Implement per-provider circuit breakers (don't share state across providers)
4. Set appropriate thresholds (e.g., 5 consecutive failures, not 1)
5. Add alerting when circuit opens to enable manual intervention

**Warning signs:**
- Circuit opens on client errors (400, 404) that shouldn't affect health
- Health probes trigger expensive operations (inference requests as health checks)
- All providers marked unhealthy when only one actually failed
- Circuit breaker causes more downtime than it prevents

**Phase to address:**
Phase 2 (Health tracking and failover) - Circuit breakers are complex, don't rush them in Phase 1.

---

### Pitfall 6: Cost Attribution Blindness

**What goes wrong:**
Proxy obscures which models, users, or projects are driving API costs. You discover a $10,000 bill but can't determine who or what caused it. Teams unknowingly route to expensive models.

**Why it happens:**
- Not capturing metadata (user, project, environment) at request time
- Relying on user-provided metadata that can be spoofed
- Logging requests without tokenization costs
- No breakdown by model, provider, or routing strategy

**How to avoid:**
1. Capture cost attribution metadata on every request:
   ```go
   type RequestMetadata struct {
       APIKey      string  // Who made the request
       Model       string  // Which model
       Provider    string  // Which backend
       InputTokens int
       OutputTokens int
       Cost        float64 // Computed from provider pricing
       Timestamp   time.Time
   }
   ```
2. Log to structured format (JSON) for later analysis
3. Export metrics to Prometheus/Datadog with labels (model, provider, key)
4. Implement cost budgets per API key with alerts
5. Don't trust user-provided `user` parameter for billing (validate against API key)

**Warning signs:**
- Can't explain cost increases
- No visibility into which models are being used
- Can't allocate costs to teams or projects
- Cost optimization attempts fail due to lack of data

**Phase to address:**
Phase 2 (Metrics and monitoring) - Basic logging in Phase 1, comprehensive cost tracking in Phase 2.

---

### Pitfall 7: Provider-Specific Compatibility Ignored

**What goes wrong:**
Assuming all providers are "Anthropic-compatible" leads to subtle bugs. Bedrock requires inference profiles, Ollama doesn't support prompt caching, Azure uses different auth headers.

**Why it happens:**
- Reading marketing materials ("OpenAI-compatible!") instead of actual API docs
- Testing only with Anthropic provider
- Copy-pasting provider implementations without understanding differences
- Not validating provider-specific constraints

**How to avoid:**
1. Create comprehensive provider compatibility matrix:
   - Bedrock: Requires inference profiles, not direct model IDs
   - Vertex: Model in URL path, `anthropic_version: "vertex-2023-10-16"`
   - Ollama: No prompt caching, images must be base64
   - Azure: Different auth headers (`x-api-key` vs `api-key`)
2. Implement provider-specific request validation
3. Test each provider integration independently
4. Document limitations clearly (e.g., "Ollama: prompt caching silently ignored")
5. Return clear error messages for unsupported features

**Warning signs:**
- Works with Anthropic but fails with Bedrock/Vertex
- Prompt caching silently not working on some providers
- Authentication fails intermittently based on provider
- Model selection errors specific to certain backends

**Phase to address:**
Phase 3 (Cloud providers) - Each provider needs dedicated implementation and testing.

---

### Pitfall 8: Missing Header and Feature Forwarding

**What goes wrong:**
Proxy doesn't forward critical headers like `anthropic-beta`, `anthropic-version`, or new feature flags, silently disabling features like extended thinking, prompt caching, or programmatic tool calling.

**Why it happens:**
- Hardcoding which headers to forward instead of allowlisting
- Not staying current with API updates
- Testing with basic requests that don't use advanced features
- Assuming all important data is in the request body

**How to avoid:**
1. Forward ALL `anthropic-*` headers by default:
   ```go
   for key, values := range req.Header {
       if strings.HasPrefix(key, "Anthropic-") {
           backendReq.Header[key] = values
       }
   }
   ```
2. Subscribe to Anthropic API changelog and test new features
3. Test with beta features enabled (extended thinking, programmatic tools)
4. Log when unknown headers are encountered (helps catch new features)

**Warning signs:**
- Prompt caching not working despite correct request format
- Extended thinking blocks missing in responses
- Beta features work with direct API but not through proxy
- Users report "proxy doesn't support X" when X is actually an Anthropic feature

**Phase to address:**
Phase 1 (MVP) - Header forwarding is simple and critical for compatibility.

---

### Pitfall 9: Credential Rotation Causing Downtime

**What goes wrong:**
Rotating API keys causes request failures during the rotation window. Config reloads don't handle graceful transition, causing 401 errors for in-flight requests.

**Why it happens:**
- Revoking old keys before new keys are active in the proxy
- Config reload replaces keys atomically instead of gracefully
- No overlap period for key transitions
- Not testing rotation procedures before production

**How to avoid:**
1. Support multiple active keys per provider during rotation:
   ```go
   type ProviderConfig struct {
       ActiveKeys     []string  // Currently valid keys
       DeprecatedKeys []string  // Valid for 24h during rotation
   }
   ```
2. Implement graceful config reload (don't drop in-flight requests)
3. Follow zero-downtime rotation procedure:
   - Add new key to config
   - Wait for config reload
   - Verify new key works
   - Mark old key as deprecated
   - Wait 24 hours (or max request duration)
   - Remove old key
4. Use secrets manager (AWS Secrets Manager, Vault) for rotation automation

**Warning signs:**
- 401 errors during scheduled maintenance windows
- Requests fail immediately after config reload
- No way to test key validity before deploying
- Manual rotation requires service restart

**Phase to address:**
Phase 2 (Multi-key pooling) - Proper key management needs multi-key support. Phase 1 can tolerate brief downtime during rotation.

---

### Pitfall 10: AWS Bedrock Inference Profile Confusion

**What goes wrong:**
Using direct model IDs (e.g., `anthropic.claude-sonnet-4-5-20250929-v1:0`) with Bedrock causes "on-demand throughput not supported" errors. Teams waste time debugging before discovering inference profiles are required.

**Why it happens:**
- Bedrock's requirement for inference profiles is non-obvious
- Marketing materials show model IDs without explaining inference profiles
- Error messages are cryptic
- Other providers don't have this concept

**How to avoid:**
1. Document Bedrock-specific setup prominently:
   ```yaml
   providers:
     - type: bedrock
       region: us-west-2
       # IMPORTANT: Use inference profile, not direct model ID
       models:
         - id: "us.anthropic.claude-sonnet-4-5-v2:0"  # Inference profile
           name: "claude-sonnet-4.5"
   ```
2. Validate Bedrock model IDs against expected pattern (inference profiles start with region)
3. Provide clear error messages: "Bedrock requires inference profiles. See docs/bedrock.md"
4. Include working examples in config templates

**Warning signs:**
- Bedrock requests fail with "on-demand not supported"
- Works in AWS console but not through proxy
- Model ID validation doesn't catch invalid IDs
- Users copy model IDs from AWS docs without understanding inference profiles

**Phase to address:**
Phase 3 (Cloud providers) - Bedrock has multiple gotchas, allocate time for proper implementation.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single global rate limiter | Simple implementation | Can't track per-key limits, easy to bypass | Phase 1 MVP only |
| IP-based client identification | Easy to implement | Fails with proxies/NAT, easy to bypass | Never in production |
| Hardcoded model-to-cost mapping | No external dependencies | Stale pricing, inaccurate cost tracking | Never - fetch pricing dynamically or config |
| Synchronous health checks | Simple logic | Blocks request processing, slows responses | Never - always async |
| Fixed-window rate limiting | Simple algorithm | Allows burst attacks at window boundaries | Phase 1 only, upgrade to token bucket |
| Sharing circuit breaker across providers | Less state to manage | One provider failure affects all providers | Never - defeats the purpose |
| Logging sensitive prompts | Helps debugging | PII/secrets leakage, compliance violations | Never - sanitize or disable |
| Using same API keys for proxy and backend | Fewer credentials to manage | Can't distinguish proxy vs direct usage in billing | Never - use separate keys |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| **AWS Bedrock** | Using direct model IDs instead of inference profiles | Use region-prefixed inference profile IDs (e.g., `us.anthropic.claude-*`) |
| **Vertex AI** | Sending model in request body like Anthropic | Put model in URL path: `/v1/projects/{project}/locations/{location}/publishers/anthropic/models/{model}:streamRawPredict` |
| **Ollama** | Expecting prompt caching to work | Document that prompt caching is silently ignored, don't rely on it |
| **Azure** | Using `api-key` header | Azure uses `x-api-key` (note the `x-` prefix), same as Anthropic |
| **Z.AI** | Assuming model names match Anthropic's | Map GLM models: `glm-4-plus` → Claude Sonnet equivalent in docs |
| **Anthropic** | Not handling `thinking` content blocks | Extended thinking adds new content block type, handle it in SSE stream |
| **All Providers** | Assuming OAuth tokens never expire | Implement token refresh logic (especially Vertex with 1-hour token lifetime) |
| **All Providers** | Not handling 429 with `retry-after` | Parse `retry-after` header and backoff appropriately, don't hammer |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| In-memory request logging | Memory grows unbounded | Use log rotation and bounded buffers | 1K concurrent requests |
| Synchronous cost calculation | Response latency increases | Calculate costs async in background | 100 req/sec |
| Full request/response body logging | Disk fills up, I/O bottleneck | Log only metadata, sample bodies | 1K requests/min |
| Single global mutex for routing | Lock contention, serialized routing | Use per-provider or per-key locks | 50 concurrent requests |
| Not pooling HTTP clients | Connection exhaustion, high latency | Use `http.Client` with connection pooling | 100 concurrent backend connections |
| Blocking health checks | Health check delays block requests | Run health checks in background goroutines | 10 providers |
| JSON marshal/unmarshal on every request | CPU-bound at high throughput | Use `jsoniter` or minimize marshaling | 500 req/sec |
| No request timeout | Stuck requests hold resources forever | Set reasonable timeouts (30-120s for streaming) | First stuck request |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Logging full request/response bodies | PII, API keys, secrets in logs | Sanitize or disable body logging, log only metadata |
| Forwarding all incoming headers to backend | Proxy becomes an authorization bypass | Allowlist safe headers (`anthropic-*`), blocklist dangerous ones (`authorization`, `cookie`) |
| Not validating model names | Prompt injection in model field → log injection | Validate model names against known list |
| Allowing arbitrary provider URLs in config | SSRF - attacker can hit internal services | Restrict to known provider domains, validate URLs |
| Using user-provided metadata for billing | Users can spoof `user` field to avoid tracking | Validate metadata against authenticated API key |
| Not rate limiting by API key | Single malicious key can DoS proxy | Implement per-key rate limits, not just global |
| Exposing provider API keys in error messages | Credentials leak in 500 errors | Sanitize error messages, log full errors server-side only |
| No request size limits | Memory exhaustion via huge prompts | Enforce max request body size (e.g., 10MB) |
| Trusting `X-Forwarded-For` for rate limiting | IP spoofing bypasses limits | Use authenticated API key as rate limit key |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Not returning standard Anthropic error format | Claude Code can't parse errors | Match exact error response schema: `{"type":"error","error":{"type":"...","message":"..."}}` |
| Silent feature downgrades | Prompt caching doesn't work, no warning | Log warnings or return errors when features are unsupported on chosen provider |
| No visibility into routing decisions | Users can't tell which provider/key was used | Add optional `X-CC-Relay-Provider` response header with routing info |
| Cryptic error messages | "Internal server error" doesn't help debugging | Include request ID, which provider failed, why it failed |
| No health status endpoint | Can't tell if proxy is working before sending requests | Expose `/health` and `/status` endpoints |
| Rate limit errors without `Retry-After` | Clients don't know when to retry | Always include `Retry-After` header on 429 responses |
| Breaking changes in config format | Existing configs stop working on update | Version config schema, auto-migrate old formats |
| No configuration validation | Errors only appear at runtime | Validate config on load, fail fast with clear error messages |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **SSE Streaming:** Did you test with real Claude Code, not just curl? (Curl may hide buffering issues)
- [ ] **SSE Streaming:** Did you verify `X-Accel-Buffering: no` is set? (Required for nginx/Cloudflare)
- [ ] **Tool Use:** Did you test with parallel tool calls (3+ simultaneous)? (Single tool calls can hide ID preservation bugs)
- [ ] **Provider Integration:** Did you test each provider independently? (Don't assume "compatible" means identical)
- [ ] **Rate Limiting:** Did you test limit bypass with multiple API keys/IPs? (Simple tests miss bypass vulnerabilities)
- [ ] **Circuit Breaker:** Did you test with partial failures, not just total outages? (Should not treat 400s as circuit breaker failures)
- [ ] **Authentication:** Did you test with invalid API keys and missing auth headers? (Don't just test happy path)
- [ ] **Error Handling:** Do error responses match Anthropic's exact schema? (Claude Code expects specific format)
- [ ] **Config Reload:** Did you test reload without restarting the server? (In-flight requests should complete gracefully)
- [ ] **Cost Tracking:** Can you attribute costs to specific users/projects? (Logging tokens isn't enough without metadata)
- [ ] **Header Forwarding:** Did you test with `anthropic-beta` features enabled? (Missing beta headers silently disable features)
- [ ] **Credential Rotation:** Did you test rotating keys without downtime? (Zero-downtime rotation requires multi-key support)

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| **SSE buffering breaks streaming** | LOW | Add missing headers (`X-Accel-Buffering: no`), deploy, verify with Claude Code |
| **Tool use IDs lost** | MEDIUM | Add integration tests for parallel tools, fix field preservation, redeploy |
| **Weak authentication exploited** | HIGH | Revoke compromised keys, add proper auth, audit access logs, alert customers |
| **Rate limits bypassed** | MEDIUM | Switch to per-key limiting, deploy, communicate limits to users |
| **Cost tracking gaps** | LOW | Add metadata capture going forward, backfill costs from provider bills if needed |
| **Circuit breaker opens unnecessarily** | LOW | Adjust thresholds, exempt client errors, redeploy |
| **Provider compatibility broken** | MEDIUM | Implement provider-specific transformations, add integration tests, redeploy |
| **Missing header forwarding** | LOW | Update header allowlist, deploy, test beta features |
| **Credential rotation downtime** | MEDIUM | Implement multi-key support, test rotation procedure, re-rotate correctly |
| **Bedrock inference profile confusion** | LOW | Update docs and examples, validate model IDs in config loader |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| SSE buffering | Phase 1 (MVP) | Test with real Claude Code, verify streaming works |
| Tool use ID preservation | Phase 1 (MVP) | Integration tests with parallel tool calls pass |
| Weak authentication | Phase 1 (MVP) | Security scan shows no public unauthenticated access |
| Rate limit bypass | Phase 2 (Multi-key) | Load tests with multiple keys respect per-key limits |
| Circuit breaker anti-patterns | Phase 2 (Health tracking) | Circuit doesn't open on 4xx errors, health checks are lightweight |
| Cost attribution blindness | Phase 2 (Metrics) | Can generate cost report by API key, model, and provider |
| Provider-specific compatibility | Phase 3 (Cloud providers) | Each provider has integration tests covering unique quirks |
| Missing header forwarding | Phase 1 (MVP) | Test with `anthropic-beta` features enabled, verify they work |
| Credential rotation downtime | Phase 2 (Multi-key) | Zero-downtime rotation tested in staging |
| Bedrock inference profiles | Phase 3 (Cloud providers) | Config validation rejects direct model IDs for Bedrock |

## Sources

**SSE Streaming & Buffering:**
- [Fixing Slow SSE (Server-Sent Events) Streaming in Next.js and Vercel](https://medium.com/@oyetoketoby80/fixing-slow-sse-server-sent-events-streaming-in-next-js-and-vercel-99f42fbdb996)
- [Issues with SSE (server side events) on Azure App Service](https://learn.microsoft.com/en-us/answers/questions/5573038/issues-with-sse-(server-side-events)-on-azure-app)
- [Using Server Sent Events (SSE) with Cloudflare Proxy](https://community.cloudflare.com/t/using-server-sent-events-sse-with-cloudflare-proxy/656279)

**Tool Use & Parallel Calls:**
- [API Error: 400 due to tool use concurrency issues](https://github.com/badrisnarayanan/antigravity-claude-proxy/issues/91)
- [Programmatic tool calling - Claude Docs](https://platform.claude.com/docs/en/agents-and-tools/tool-use/programmatic-tool-calling)

**Security & Authentication:**
- [Hackers scan misconfigured proxies for paid LLM services](https://anavem.com/cybersecurity/hackers-scan-misconfigured-proxies-paid-llm-services)
- [How API Gateways Proxy LLM Requests](https://api7.ai/learning-center/api-gateway-guide/api-gateway-proxy-llm-requests)

**Rate Limiting:**
- [API Rate Limiting at Scale: Patterns, Failures, and Control Strategies](https://www.gravitee.io/blog/rate-limiting-apis-scale-patterns-strategies)
- [Mastering API Rate Limiting: Strategies, Challenges, and Best Practices](https://testfully.io/blog/api-rate-limit/)
- [API Rate Limiting Fails: Death by a Thousand (Legitimate) Requests](https://medium.com/@instatunnel/api-rate-limiting-fails-death-by-a-thousand-legitimate-requests-30e24aba8b7f)

**Circuit Breaker Patterns:**
- [Circuit Breaker Pattern - Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker)
- [The Circuit Breaker Pattern - Dos and Don'ts](https://akfpartners.com/growth-blog/the-circuit-breaker-pattern-dos-and-donts)

**Cost Tracking:**
- [Monitor your LiteLLM AI proxy with Datadog](https://www.datadoghq.com/blog/monitor-litellm-with-datadog/)
- [LLM cost attribution: Tracking and optimizing spend for GenAI apps](https://portkey.ai/blog/llm-cost-attribution-for-genai-apps/)
- [Monitoring AI Proxies to optimize performance and costs](https://www.datadoghq.com/blog/optimize-ai-proxies-with-datadog/)

**Provider-Specific (Bedrock, Vertex, etc.):**
- [AWS Bedrock | liteLLM](https://docs.litellm.ai/docs/providers/bedrock)
- [Bedrock, Vertex, and proxies - Claude Code](https://docs.anthropic.com/en/docs/claude-code/bedrock-vertex-proxies)
- [Configuring Claude Code Extension with AWS Bedrock (And How You Can Avoid My Mistakes)](https://aws.plainenglish.io/configuring-claude-code-extension-with-aws-bedrock-and-how-you-can-avoid-my-mistakes-090dbed5215b)

**Credential Management:**
- [API Key Security Best Practices for 2026](https://dev.to/alixd/api-key-security-best-practices-for-2026-1n5d)
- [11 Best API Key Management Tools in 2026](https://www.digitalapi.ai/blogs/top-api-key-management-tools)

---
*Pitfalls research for: Multi-provider LLM proxy for Claude Code*
*Researched: 2026-01-20*
