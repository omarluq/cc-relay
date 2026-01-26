---
phase: 07-configuration-management
plan: 03
subsystem: config
tags: [fsnotify, hot-reload, debounce, file-watcher, go]

# Dependency graph
requires:
  - phase: 07-01
    provides: fsnotify v1.9.0 dependency installed
provides:
  - Config file watcher with debouncing for hot-reload
  - ReloadCallback type for config change notifications
  - Context-based cancellation for clean shutdown
  - Parent directory watching for atomic writes
affects: [07-04-hot-reload-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Debounce pattern for file system events
    - Parent directory watching for atomic writes
    - RWMutex callback registration

key-files:
  created:
    - internal/config/watcher.go
    - internal/config/watcher_test.go
  modified: []

key-decisions:
  - "Watch parent directory instead of file directly (handles atomic writes from editors)"
  - "100ms debounce delay balances responsiveness with editor multi-event filtering"
  - "Only process Write/Create events, ignore Chmod (from indexers/antivirus)"
  - "ErrWatcherClosed for double-close detection"

patterns-established:
  - "Debounce pattern: time.AfterFunc with reset on each event"
  - "Directory watching pattern: filepath.Dir(path) + filepath.Base filter"
  - "Callback slice pattern: RWMutex protected append + copy-on-read"

# Metrics
duration: 8min
completed: 2026-01-26
---

# Phase 7 Plan 03: Config File Watcher Summary

**fsnotify-based config watcher with 100ms debounce, parent directory watching for atomic writes, and context-based shutdown**

## Performance

- **Duration:** 8 min
- **Started:** 2026-01-26T04:46:06Z
- **Completed:** 2026-01-26T04:53:47Z
- **Tasks:** 2
- **Files modified:** 2 created

## Accomplishments

- Implemented Watcher type that monitors config file for changes via fsnotify
- Debounce logic handles rapid editor saves (100ms default, configurable)
- Watches parent directory to properly detect atomic writes (temp + rename)
- Only Write/Create events trigger reload (Chmod from indexers ignored)
- Context-based cancellation for clean shutdown
- Comprehensive test coverage including debounce behavior verification

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement config file watcher with debounce** - `441a72b` (feat)
2. **Task 2: Add watcher tests** - `9f623da` (test)

## Files Created/Modified

- `internal/config/watcher.go` - Watcher type with debounce, callback registration, parent directory watching (207 lines)
- `internal/config/watcher_test.go` - Comprehensive tests for watcher behavior (471 lines)

## Decisions Made

1. **Watch parent directory instead of file directly** - Many editors (vim, emacs, vscode) use atomic writes where they write to a temp file then rename. Watching the parent directory catches the Create event from the rename.

2. **100ms debounce delay** - Editors may trigger 2-5 events per save operation. 100ms is short enough to feel responsive but long enough to coalesce rapid events.

3. **Filter to Write/Create only** - Spotlight, antivirus, and backup software trigger Chmod events when scanning files. Ignoring these prevents spurious reloads.

4. **ErrWatcherClosed sentinel error** - Allows callers to detect double-close attempts if needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed loader_test.go Format type reference**
- **Found during:** Task 1 (watcher compilation)
- **Issue:** loader_test.go used `ConfigFormat` but loader.go was renamed to `Format` (by parallel plan 07-02)
- **Fix:** Replaced `ConfigFormat` with `Format` in loader_test.go
- **Files modified:** internal/config/loader_test.go
- **Verification:** All config tests pass
- **Committed in:** 441a72b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Fix was necessary to allow test compilation. No scope creep.

## Issues Encountered

- Pre-commit hook auto-added `ErrWatcherClosed` error and `closed` field for double-close protection, which initially broke compilation (missing errors import). Added the import to fix.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Watcher is ready for integration with hot-reload system
- Plan 07-04 can use Watcher.OnReload() to register config update handlers
- All tests pass, linter clean

---
*Phase: 07-configuration-management*
*Completed: 2026-01-26*
