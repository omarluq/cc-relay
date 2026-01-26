---
phase: 07-configuration-management
plan: 02
subsystem: config
tags: [toml, yaml, validation, loader, multi-format]

dependency_graph:
  requires: [07-01]
  provides: [multi-format-loading, config-validation, clear-errors]
  affects: [07-03, 07-04]

tech_stack:
  added: []
  patterns: [validation-error-collector, format-detection]

key_files:
  created:
    - internal/config/errors.go
    - internal/config/validator.go
    - internal/config/validator_test.go
  modified:
    - internal/config/loader.go
    - internal/config/loader_test.go
    - internal/config/config.go
    - internal/config/watcher.go
    - internal/config/watcher_test.go

decisions:
  - id: validation-error-collector
    choice: "Collect all errors, not just first"
    why: "Better UX - users fix all issues at once"
  - id: format-detection
    choice: "Detect format from file extension"
    why: "Simple, explicit, no magic content sniffing"
  - id: provider-constants
    choice: "Add ProviderBedrock/Vertex/Azure constants"
    why: "DRY - used in both validator.go and config.go"

metrics:
  duration: 14 min
  completed: 2026-01-26
---

# Phase 07 Plan 02: Multi-Format Loading and Validation Summary

**One-liner:** YAML/TOML format detection from extension with comprehensive validation that collects all errors.

## What Was Built

### Multi-Format Config Loading
- Added `Format` type with `FormatYAML` and `FormatTOML` constants
- `Load()` detects format from file extension (.yaml, .yml, .toml)
- `LoadFromReaderWithFormat()` for explicit format parsing
- `UnsupportedFormatError` for clear error messages on unknown extensions
- Environment variable expansion works with both YAML and TOML

### Comprehensive Validation
- `ValidationError` type collects all errors (not just first)
- `Validate()` method on `Config` checks:
  - server.listen format (host:port)
  - provider name, type, cloud fields (AWS region, GCP project, Azure resource)
  - routing strategy validity
  - logging level and format
- Clear error messages with field paths (e.g., `provider[anthropic].aws_region`)

### Bug Fixes (Rule 1)
- Fixed watcher test `writeTestConfigWithContent` to use `fmt.Sprintf`
- Added `ErrWatcherClosed` for double-close detection
- Renamed unused `cfg` params to `_` in test callbacks

## Implementation Notes

### Format Detection
```go
func detectFormat(path string) (Format, error) {
    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".yaml", ".yml": return FormatYAML, nil
    case ".toml": return FormatTOML, nil
    default: return "", &UnsupportedFormatError{Extension: ext, Path: path}
    }
}
```

### Validation Error Collection
```go
func (c *Config) Validate() error {
    errs := &ValidationError{}
    validateServer(c, errs)
    validateProviders(c, errs)
    validateRouting(c, errs)
    validateLogging(c, errs)
    return errs.ToError()  // nil if no errors
}
```

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 9f623da | test | Add comprehensive watcher tests (loader.go changes bundled) |
| c23e5f1 | feat | Add comprehensive configuration validation |

Note: The loader.go changes (format detection) were committed with watcher tests due to commit ordering. Functionally complete.

## Verification

All success criteria met:
- [x] Load() detects format from .yaml/.yml/.toml extensions
- [x] TOML config files parse correctly with environment variable expansion
- [x] Unsupported extensions return clear error message
- [x] Validation catches missing required fields (server.listen, provider name/type)
- [x] Validation catches invalid values (bad provider type, bad routing strategy)
- [x] ValidationError reports all errors, not just the first one
- [x] All tests pass, linters pass

## Deviations from Plan

### Auto-fixed Issues (Rule 1 - Bug)

**1. Fixed watcher test writeTestConfigWithContent**
- **Found during:** Task 1 (pre-commit hook failure)
- **Issue:** YAML content had `%d` but no `fmt.Sprintf`, causing parse error
- **Fix:** Added `fmt.Sprintf` to format timeout_ms
- **Files modified:** internal/config/watcher_test.go

**2. Fixed double-close detection**
- **Found during:** Task 1 (test failure)
- **Issue:** fsnotify.Close() doesn't error on double close, but test expected it
- **Fix:** Added `closed` field to Watcher, return ErrWatcherClosed on double close
- **Files modified:** internal/config/watcher.go

**3. Fixed unused parameter warnings**
- **Found during:** Task 1 (linter failure)
- **Issue:** `cfg *Config` unused in test callbacks
- **Fix:** Renamed to `_ *Config`
- **Files modified:** internal/config/watcher_test.go

## Next Phase Readiness

Ready for 07-03 (Config File Watcher) - already partially implemented and committed.
Ready for 07-04 (Hot Reload Integration) - validation provides foundation for reload safety.
