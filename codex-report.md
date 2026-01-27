# Codex Codebase Quality & Security Review Report

Date: 2026-01-27
Repo: /home/omarluq/sandbox/go/cc-relay

## Scope, Methodology, and Limitations

### Scope (requested)
- Full codebase review: Go code under `cmd/` and `internal/`.
- Documentation review: `docs-site/`, `README.md`, `SPEC.md`, `PROJECT_INDEX.*`, and planning docs in `.planning/`.
- Security, correctness vs planning/docs, code smells, missing optimizations, and status accuracy.

### Methodology (what I did)
- Enumerated all files in repo (excluding `.git`) and ran broad searches for auth, routing, hot-reload, security keywords, and TODOs.
- Deep-read critical runtime paths:
  - HTTP proxy handler, provider proxy, middleware, SSE processing.
  - Router, keypool, ratelimit.
  - Config loader/validator/watcher + DI wiring.
  - Server setup.
- Checked docs and planning files for feature status and configuration alignment, with spot-reading in multiple locales.

### Limitations (explicit)
- I **did not read every file end-to-end** (repo is large). I *did* enumerate all files and used aggressive `rg` searches + deep reads of core runtime, config, auth, and docs. For files not explicitly opened, analysis is based on file names, search hits, and cross-references.
- No tests were executed. Observations are static analysis only.

If you want a literal line-by-line read of every file, it’s possible but very time‑consuming; I can continue iteratively and expand this report.

---

## Executive Summary (Key Issues)

**High-impact mismatches and runtime risks were found:**
1) **Routing strategy validation and docs are inconsistent with actual implementations**, leading to valid docs configs failing at startup or being rejected.  
2) **Hot‑reload behavior is over‑promised in docs**: the config pointer updates, but most runtime components don’t consume it, so many documented hot‑reloadable settings likely don’t take effect.
3) **Unbounded request body reads** occur in multiple handler paths (model extraction + thinking signature processing), which is a potential memory/DoS risk.
4) **`server.timeout_ms` and `server.max_concurrent` are documented and validated but unused** in server wiring.
5) **Logging format validation conflicts with logger behavior**, causing config values to be rejected even though the logger supports them.

---

## Findings by Severity (with Evidence)

### CRITICAL / HIGH

#### 1) Routing strategy validation vs implementation mismatch
- Validator accepts `weighted`, `least_loaded`, `weighted_failover`, but router only implements `round_robin`, `weighted_round_robin`, `shuffle`, `failover`, `model_based`.
- This can allow configs to pass validation but fail at runtime, or docs to be inaccurate.

Evidence:
- `internal/config/validator.go:17-27` (valid strategies)
- `internal/router/router.go:27-115` (implemented strategies)
- `docs-site/content/en/docs/routing.md:12-69` (docs list only supported strategies)

**Impact:** invalid or undocumented strategy values can break server startup or routing; docs and config examples are misleading.

#### 2) Key pool strategy mismatch
- Validator allows `weighted` in pooling strategies, but keypool `NewSelector` only supports `least_loaded` and `round_robin`.

Evidence:
- `internal/config/validator.go:17-27`
- `internal/keypool/selector.go:25-43`

**Impact:** pool config can validate but fail at runtime; config UX inconsistency.

#### 3) Hot‑reload is documented as effective for runtime behavior, but only config pointer swaps
- `ConfigService` swaps the config pointer on reload, but runtime services (router, handler, server) are built using `cfgSvc.Config` and never refreshed.
- Docs claim hot‑reload applies to logging, rate limits, routing weights, etc., but those are only read during startup in DI.

Evidence:
- Atomic swap only: `cmd/cc-relay/di/providers.go:27-78`
- DI uses static config pointer: `cmd/cc-relay/di/providers.go:533-622`
- Docs claim runtime changes apply: `docs-site/content/en/docs/configuration.md:1174-1223`

**Impact:** user expects changes to take effect, but they may not; risk of operational confusion.

#### 4) Unbounded request body reads (DoS risk)
- `ExtractModelFromRequest` reads entire body with `io.ReadAll`.
- `processThinkingSignatures` reads entire body with `io.ReadAll`.
- Both run for every request, and no size limit is enforced.

Evidence:
- `internal/proxy/model_extract.go:17-33`
- `internal/proxy/handler.go:262-277`
- `internal/proxy/handler.go:507-524`

**Impact:** large request bodies can cause memory spikes and potential denial of service.

#### 5) `server.timeout_ms` and `max_concurrent` are unused
- Config fields exist and are validated, and docs reference them, but server setup ignores them.

Evidence:
- Config fields: `internal/config/config.go:107-114`
- Validation: `internal/config/validator.go:70-88`
- Server uses fixed timeouts: `internal/proxy/server.go:19-43`
- Docs: `docs-site/content/en/docs/configuration.md:193-200`

**Impact:** users think these values are enforced when they are not.

---

### MEDIUM

#### 6) Logger format mismatch
- Logger supports `pretty`, but validator only allows `json`, `console`, `text`.

Evidence:
- Logger: `internal/proxy/logger.go:63-80`
- Validator: `internal/config/validator.go:48-54`

**Impact:** valid logger config rejected.

#### 7) Request body preview logged even if LogRequestBody=false
- `LoggingMiddleware` always logs a body preview when debug level is enabled, regardless of `LogRequestBody`.

Evidence:
- `internal/proxy/middleware.go:134-168`

**Impact:** unexpected leakage of sensitive prompt content in debug logs.

#### 8) TokenBucketLimiter.GetUsage consumes tokens
- `GetUsage()` calls `Allow()` which consumes a token; it then tries to “cancel” by reserving, but this doesn’t restore the token.

Evidence:
- `internal/ratelimit/token_bucket.go:118-129`

**Impact:** metrics or selection logic that calls GetUsage will silently reduce capacity.

#### 9) Event stream wrapper can return (0, nil)
- `eventStreamToSSEBody.Read` can return `(0, nil)` when no data is available, which may cause tight loops in callers.

Evidence:
- `internal/proxy/provider_proxy.go:271-310`

**Impact:** potential CPU spin in some readers.

#### 10) Security foot‑guns (config behavior)
- `AllowBearer` with empty `BearerSecret` accepts any bearer token.
- HTTP/2 is supported via h2c (cleartext). Safe only in trusted environments.

Evidence:
- `internal/config/config.go:123-134`
- `internal/auth/oauth.go:22-33`
- `internal/proxy/server.go:25-33`

**Impact:** if exposed publicly without additional controls, could allow unintended access or insecure transport.

---

### DOCS / PLANNING MISALIGNMENT (Accuracy Gaps)

#### 11) Docs still claim many features are “planned” but code implements them
- Docs still list Ollama/Bedrock/Azure/Vertex as planned, but providers exist.

Evidence:
- Docs: `docs-site/content/en/about.md:19-36`, `docs-site/content/en/docs/_index.md:12-40`
- Code: `internal/providers/bedrock.go`, `internal/providers/azure.go`, `internal/providers/vertex.go`, `internal/providers/ollama.go`

#### 12) README lists strategies not implemented and uses invalid names
- README lists `simple-shuffle`, `least-busy`, `cost-based`, `latency-based`.

Evidence:
- `README.md:92-102`
- Actual routing constants: `internal/router/router.go:27-115`

#### 13) Planning docs are stale vs current implementation
- `.planning/PROJECT.md` still says hot‑reload, circuit breaker, and cloud providers are out-of-scope.

Evidence:
- `.planning/PROJECT.md:30-40`

#### 14) SPEC still shows core features unchecked
- `SPEC.md` still has MVP tasks unchecked even though code implements them.

Evidence:
- `SPEC.md:21-28`

#### 15) gRPC references without implementation
- Docs mention gRPC management API and gRPC config changes, but no gRPC server or config fields exist in code.

Evidence:
- Docs: `docs-site/content/en/about.md:35-36`, `docs-site/content/en/docs/_index.md:37-40`, `docs-site/content/en/docs/configuration.md:1215-1217`
- Code search: no gRPC server in `internal/` or `cmd/`.

---

## Security Review Summary

- **Primary security risks** are misconfiguration and logging behavior:
  - Unbounded request body reads: potential DoS.
  - Debug logs can leak data even when request logging is disabled.
  - “Allow any bearer token” and h2c need explicit warnings in docs.

- **Authentication design** uses constant‑time compare and hashing; no obvious crypto misuse.
  - `internal/proxy/middleware.go` and `internal/auth/oauth.go` use SHA‑256 + `subtle.ConstantTimeCompare`.

---

## Correctness vs Planning

- Planning and audits assert production‑ready completion for phases 1–7, but `PROJECT.md` and `SPEC.md` remain in “not done” states. This is inconsistent and undermines status reporting.
- Several planned-only items (gRPC) are referenced in docs but are not implemented.

**Recommendation:** update planning and SPEC to reflect reality, or remove docs claims for unimplemented features to avoid misleading users.

---

## Performance / Optimization Notes

- Multiple full body reads per request: `ExtractModelFromRequest`, `processThinkingSignatures`, and debug body preview each read or partially read the body.
- There is no global request size limit or MaxConcurrent enforcement.

**Recommendation:** introduce a request body size limit and shared body buffering (e.g., a single buffered read with size cap) to avoid repeated allocations and prevent memory abuse.

---

## Tests and Coverage

- No tests were executed; this report is static analysis.
- There are extensive tests for router, keypool, config, health, and proxy behavior, but no test appears to validate hot‑reload affecting runtime components or to assert that `server.timeout_ms`/`max_concurrent` are enforced.

---

## Actionable Recommendations (Prioritized)

1) **Fix routing strategy validation/implementation mismatch**
   - Update `validStrategies` to match actual implementations and docs.
   - Decide whether to implement `weighted`, `least_loaded`, `weighted_failover` or remove them from validation/docs.

2) **Fix keypool strategy mismatch**
   - Either implement `weighted` or reject it in validation/docs.

3) **Make hot‑reload effective or document limits clearly**
   - If intended: rebuild router/handler/server or refresh config in runtime components.
   - If not intended: clarify docs and avoid listing config fields as hot‑reloadable.

4) **Enforce request size limits**
   - Add a max request body size; avoid multiple `io.ReadAll` calls.

5) **Use `server.timeout_ms` and `max_concurrent`**
   - Apply to `http.Server` and concurrency gate.

6) **Align logger validation with logger behavior**
   - Allow `pretty` in validator.

7) **Respect LogRequestBody in LoggingMiddleware**
   - Ensure body preview is gated by `LogRequestBody`.

8) **Fix GetUsage token consumption**
   - Make `GetUsage` non-mutating, or document the side effect.

9) **Review event stream reader**
   - Avoid returning `(0, nil)` in Read when no data is ready.

10) **Docs cleanup**
   - Update `README.md`, `docs-site/content/*`, `.planning/PROJECT.md`, and `SPEC.md` for current reality.

---

## Notable Positives

- Solid modular architecture under `internal/` with clear separation of concerns.
- Extensive tests (including property tests) across critical packages.
- Good use of constant‑time comparisons in auth.
- Structured logging and debug tooling are thoughtfully implemented.

---

## Appendix: Files Deep‑Read (non‑exhaustive)

- `internal/proxy/handler.go`
- `internal/proxy/provider_proxy.go`
- `internal/proxy/middleware.go`
- `internal/proxy/sse.go`
- `internal/proxy/server.go`
- `internal/config/config.go`
- `internal/config/validator.go`
- `internal/config/watcher.go`
- `internal/router/router.go`
- `internal/router/failover.go`
- `internal/keypool/pool.go`
- `internal/keypool/selector.go`
- `internal/ratelimit/token_bucket.go`
- `internal/auth/oauth.go`
- `cmd/cc-relay/serve.go`
- `cmd/cc-relay/di/providers.go`
- `docs-site/content/en/docs/configuration.md`
- `docs-site/content/en/docs/routing.md`
- `docs-site/content/en/docs/_index.md`
- `docs-site/content/en/about.md`
- `README.md`
- `.planning/PROJECT.md`
- `SPEC.md`

---

If you want, I can proceed with a file-by-file read and expand this report with a full inventory and per‑file notes. That will take additional time but is doable.

---

# Addendum: Re-Review vs .planning (2026-01-27)

This section re-validates the earlier findings against `.planning/*` and re-checks code paths. It includes **confirmations** and **new mismatches** discovered.

## Confirmations of Prior Findings

### A) Routing strategy mismatch still present
- **Validation allows unimplemented strategies** (`least_loaded`, `weighted_failover`, `weighted`) while router only supports `round_robin`, `weighted_round_robin`, `shuffle`, `failover`, `model_based`.
- **Planning/requirements** still list cost/latency strategies as required in later phases, but they are not implemented.

Evidence:
- `internal/config/validator.go:17-27`
- `internal/router/router.go:27-115`
- `.planning/REQUIREMENTS.md:43-49`
- `.planning/ROADMAP.md:198-207`

### B) Hot‑reload is documented as complete but only swaps config pointer
- Planning and audits claim hot‑reload is “complete” (Phase 7), and docs describe operational behavior.
- Code only updates `ConfigService`’s atomic pointer; DI‑built services do not rebind or rebuild from the new config.

Evidence:
- `cmd/cc-relay/di/providers.go:27-78`
- `cmd/cc-relay/di/providers.go:533-622`
- `.planning/STATE.md:650-663`
- `.planning/audits/v0.0.10-AUDIT.md:9-20`

### C) `server.timeout_ms` / `max_concurrent` still unused
- Planning acknowledges max_concurrent risk; config fields exist and are validated, but not enforced in server wiring.

Evidence:
- `internal/config/config.go:107-114`
- `internal/config/validator.go:70-88`
- `internal/proxy/server.go:19-43`
- `.planning/codebase/CONCERNS.md:215-220`

### D) gRPC and TUI references remain inconsistent
- Planning expects `internal/grpc/` and TUI, but the codebase has neither.

Evidence:
- `.planning/codebase/STRUCTURE.md:172-212` (expects internal/grpc)
- `.planning/REQUIREMENTS.md:105-116` (gRPC requirements)
- `.planning/ROADMAP.md:400-410`
- Code search: no `internal/grpc/` directory in repo file list


## New Findings vs .planning

### 1) Pooling strategy contract is inconsistent across config, validation, and implementation
- `PoolingConfig` comment and tests claim strategies `least_loaded`, `round_robin`, `random`, `weighted`.
- `keypool.NewSelector` only supports `least_loaded` and `round_robin`.
- Validator does not enforce pool strategy correctness (it uses routing `validStrategies` set).

Evidence:
- `internal/config/config.go:201-213` (PoolingConfig comment)
- `internal/config/config_test.go:563-621` (tests expect `weighted` accepted)
- `internal/keypool/selector.go:25-43` (only 2 strategies implemented)
- `internal/config/validator.go:17-27` (uses routing strategy set, not pool strategy set)

**Impact:** users can configure pooling strategies that are not implemented; tests encode misleading expectations.

### 2) Planning requirements are marked incomplete but state/roadmap claims completion
- `.planning/REQUIREMENTS.md` still shows core requirements as unchecked (routing, pooling, auth, config hot‑reload, providers), but `.planning/STATE.md` and `.planning/ROADMAP.md` assert those phases are complete.

Evidence:
- `.planning/REQUIREMENTS.md:35-86` (all unchecked)
- `.planning/STATE.md:482-500` (Phase 3 complete)
- `.planning/ROADMAP.md:470-478` (phases marked complete)

**Impact:** planning artifacts are contradictory and untrustworthy for status tracking.

### 3) CLI roadmap includes commands not implemented
- Roadmap/requirements specify `cc-relay config reload` and `cc-relay tui`.
- Command search shows no config reload or TUI command.

Evidence:
- `.planning/ROADMAP.md:435-444`
- `.planning/REQUIREMENTS.md:126-129`
- `cmd/cc-relay` files list: no `tui.go` or reload subcommand; `rg -n "reload" cmd/cc-relay` shows only hot‑reload watcher references.

**Impact:** planning documents overstate CLI completeness.

### 4) Hot‑reload tests only verify config pointer swap
- Hot‑reload tests confirm `ConfigService.Get()` returns new config after file change but do not verify that server/router/handler/health components reconfigure.

Evidence:
- `cmd/cc-relay/di/providers_test.go:178-236`

**Impact:** tests reinforce a narrow view of hot‑reload, while docs imply functional behavior changes.


## Additional Notes

- Planning includes a risk about `max_concurrent` and OOM but no implementation exists. This is explicitly documented as a concern in `.planning/codebase/CONCERNS.md`, yet not addressed in code.
- The `PoolingConfig`/`keypool` mismatch is a deeper contract issue than previously documented: *comments and tests* also encode unsupported strategies.


## Updated Recommendations (delta)

1) **Split routing vs pooling strategy validation**
   - Add a `validPoolStrategies` set and enforce it separately.
   - Align `PoolingConfig` comments/tests with actual selector implementations.

2) **Clarify hot‑reload scope**
   - Either rebuild runtime services on reload or explicitly document that only config is updated, not live services.

3) **Fix planning/docs inconsistency**
   - Update `.planning/REQUIREMENTS.md` to reflect actual completion, or mark completed checkboxes.  
   - Update `.planning/ROADMAP.md`/`.planning/STATE.md` if these are not actuals.

4) **CLI alignment**
   - Remove roadmap claims or implement `config reload` and `tui` commands.

---

## Appendix: Additional Files Reviewed in Addendum

- `.planning/REQUIREMENTS.md`
- `.planning/ROADMAP.md`
- `.planning/STATE.md`
- `.planning/audits/v0.0.10-AUDIT.md`
- `.planning/codebase/STRUCTURE.md`
- `.planning/codebase/CONCERNS.md`
- `internal/config/config.go` (PoolingConfig)
- `internal/config/config_test.go` (pool strategy expectations)
- `internal/keypool/selector.go`
- `cmd/cc-relay/di/providers_test.go` (hot‑reload tests)

