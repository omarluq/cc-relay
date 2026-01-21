---
phase: 01-core-proxy
plan: 09
subsystem: logging
tags: [zerolog, debug, tls, metrics, httptrace]

# Dependency graph
requires:
  - phase: 01-06
    provides: zerolog integration, structured logging foundation
provides:
  - DebugOptions config struct for granular debug control
  - Debug logging utilities (LogRequestDetails, LogResponseDetails, LogTLSMetrics, LogProxyMetrics)
  - Sensitive data redaction in request body logs
  - TLS connection metrics via httptrace
  - --debug CLI flag for quick debug mode activation
affects: [troubleshooting, performance-analysis, production-debugging]

# Tech tracking
tech-stack:
  added: [net/http/httptrace]
  patterns: [debug-options-pattern, sensitive-redaction, tls-metrics-collection]

key-files:
  created: []
  modified:
    - cmd/cc-relay/serve.go
    - internal/config/config.go
    - internal/proxy/debug.go
    - internal/proxy/handler.go
    - internal/proxy/middleware.go
    - internal/proxy/routes.go

key-decisions:
  - "Use httptrace for TLS metrics (DNS, connect, handshake timing)"
  - "Redact api_key, password, token, secret, authorization, bearer patterns"
  - "Default MaxBodyLogSize: 1000 bytes to prevent log bloat"
  - "--debug flag enables all debug options + sets level to debug"

patterns-established:
  - "Debug options pattern: separate DebugOptions struct in config"
  - "Sensitive redaction: regex patterns for credential fields"
  - "TLS trace attachment: conditional httptrace for performance"

# Metrics
duration: 8min
completed: 2026-01-21
---

# Phase 01 Plan 09: Enhanced Debug Logging Summary

**Comprehensive debug logging with TLS metrics, request/response details, sensitive data redaction, and --debug CLI flag for quick activation**

## Performance

- **Duration:** 8 min
- **Started:** 2026-01-21T00:00:00Z
- **Completed:** 2026-01-21T00:08:00Z
- **Tasks:** 1 (verification + flag addition)
- **Files modified:** 1 (serve.go - debug flag)

## Accomplishments

- Verified existing DebugOptions implementation in config.go with 4 fields (LogRequestBody, LogResponseHeaders, LogTLSMetrics, MaxBodyLogSize)
- Verified debug.go utilities: LogRequestDetails, LogResponseDetails, LogTLSMetrics, LogProxyMetrics, AttachTLSTrace
- Verified sensitive data redaction for api_key, password, token, secret, authorization, bearer
- Verified handler.go and middleware.go integration with debug options
- Added --debug CLI flag to serve.go for quick debug mode activation
- All 8 debug unit tests passing

## Task Commits

1. **Task: Add --debug CLI flag** - `bd14d68` (feat)

## Files Created/Modified

- `cmd/cc-relay/serve.go` - Added --debug flag and EnableAllDebugOptions() call
- `internal/config/config.go` - Already had DebugOptions struct (verified)
- `internal/proxy/debug.go` - Already had debug utilities (verified)
- `internal/proxy/debug_test.go` - Already had 8 unit tests (verified)
- `internal/proxy/handler.go` - Already integrated debug options (verified)
- `internal/proxy/middleware.go` - Already integrated debug options (verified)
- `internal/proxy/routes.go` - Already passing debug options (verified)

## Implementation Details

### DebugOptions Configuration

```yaml
logging:
  level: debug
  format: json
  debug_options:
    log_request_body: true      # Log request body with redaction
    log_response_headers: true  # Log response headers
    log_tls_metrics: true       # Log TLS connection metrics
    max_body_log_size: 1000     # Max bytes to log (default: 1000)
```

### --debug Flag Usage

```bash
# Enable all debug options via CLI flag
cc-relay serve --debug

# Equivalent to setting in config:
# logging.level: debug
# logging.debug_options.log_request_body: true
# logging.debug_options.log_response_headers: true
# logging.debug_options.log_tls_metrics: true
# logging.debug_options.max_body_log_size: 1000
```

### Debug Log Output Examples

**Request Details:**
```json
{
  "level": "debug",
  "content_type": "application/json",
  "body_length": 156,
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 100,
  "body_preview": "{\"model\":\"claude-3-5-sonnet-20241022\",\"***\":\"REDACTED\",...}",
  "message": "request details"
}
```

**TLS Metrics:**
```json
{
  "level": "debug",
  "tls_version": "TLS 1.3",
  "tls_reused": false,
  "dns_time_ms": 5,
  "connect_time_ms": 10,
  "tls_handshake_ms": 15,
  "message": "tls metrics"
}
```

**Proxy Metrics:**
```json
{
  "level": "debug",
  "backend_time_ms": 250,
  "total_time_ms": 300,
  "streaming_events": 42,
  "message": "proxy metrics"
}
```

### Sensitive Data Redaction

The following patterns are automatically redacted:
- `"api_key": "..."` -> `"***":"REDACTED"`
- `"x-api-key": "..."` -> `"***":"REDACTED"`
- `"password": "..."` -> `"***":"REDACTED"`
- `"token": "..."` -> `"***":"REDACTED"`
- `"secret": "..."` -> `"***":"REDACTED"`
- `"authorization": "..."` -> `"***":"REDACTED"`
- `"bearer": "..."` -> `"***":"REDACTED"`

## Decisions Made

1. **TLS trace overhead:** Only attach httptrace when LogTLSMetrics is enabled to avoid performance impact in production
2. **Body truncation:** Default 1000 bytes max to prevent log bloat from large request bodies
3. **Debug flag behavior:** --debug enables ALL debug options as a convenience for development
4. **Metrics always logged at debug level:** ProxyMetrics logged whenever debug level is enabled, regardless of IsEnabled()

## Deviations from Plan

None - the core implementation was already complete from previous work. This plan only required adding the --debug CLI flag and verifying existing functionality.

## Issues Encountered

None - all existing tests passed, code compiled cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Debug logging fully operational
- Ready to use --debug flag for troubleshooting
- Phase 1 (Core Proxy) is now fully complete with enhanced observability
- Ready to begin Phase 2 (Multi-key pooling & rate limiting)

---
*Phase: 01-core-proxy*
*Completed: 2026-01-21*
