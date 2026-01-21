# Auto-Fix Code Skill

**Use auto-fix tools to automatically correct code issues before manual intervention.**

## Core Workflow

```bash
# 1. Auto-format code
task fmt

# 2. Auto-fix lint issues
task lint-fix

# 3. Check remaining issues
task lint

# 4. Manually fix what's left
# ... edit code ...

# 5. Verify everything
task ci
```

## Auto-Fix Capabilities

### What Can Be Auto-Fixed

✅ **Formatting** (100% auto-fixable)
- Indentation
- Line spacing
- Import organization
- Code simplifications

✅ **Simple Lint Issues** (80% auto-fixable)
- Unused imports
- Missing error wrapping
- Unnecessary conversions
- Simple style issues

❌ **Complex Issues** (requires manual fix)
- Logic errors
- Complex refactoring
- High cyclomatic complexity
- Security vulnerabilities
- Race conditions

## Step-by-Step Auto-Fix Guide

### Step 1: Format Everything

```bash
# Run all formatters
task fmt

# This automatically fixes:
# - Go formatting (gofmt)
# - Import organization (goimports)
# - Strict formatting (gofumpt)
# - YAML formatting (yamlfmt)
# - Markdown formatting (markdownlint)
```

**What it fixes:**
```go
// Before
import "net/http"
import "fmt"
func process( ){
if ready{
do( )
}
}

// After (automatically fixed)
import (
    "fmt"
    "net/http"
)

func process() {
    if ready {
        do()
    }
}
```

### Step 2: Auto-Fix Lint Issues

```bash
# Run linters with auto-fix
task lint-fix

# Or directly
golangci-lint run --fix
```

**What it auto-fixes:**

#### a) Unused Imports
```go
// Before
import (
    "fmt"      // unused
    "net/http"
)

// After (auto-removed)
import (
    "net/http"
)
```

#### b) Error Wrapping
```go
// Before
return fmt.Errorf("failed: %s", err)

// After (auto-fixed to use %w)
return fmt.Errorf("failed: %w", err)
```

#### c) Unnecessary Conversions
```go
// Before
var x int = int(5)

// After (auto-simplified)
var x int = 5
```

#### d) Style Issues
```go
// Before
if x == true {
    return true
}

// After (auto-simplified)
if x {
    return true
}
```

### Step 3: Check What Remains

```bash
# Check for remaining issues
task lint

# Example output:
# internal/proxy/proxy.go:42:2: Error return value is not checked (errcheck)
# internal/router/router.go:15:1: Function too complex (gocyclo)
```

These require manual fixes.

### Step 4: Manual Fixes

For issues that can't be auto-fixed:

```go
// Issue: Error return value is not checked
// Before
file, _ := os.Open("config.yaml")

// After (manual fix required)
file, err := os.Open("config.yaml")
if err != nil {
    return fmt.Errorf("failed to open config: %w", err)
}
defer file.Close()
```

## Auto-Fix by File Type

### Go Files

```bash
# Format and fix single file
gofmt -w internal/proxy/proxy.go
goimports -w internal/proxy/proxy.go
gofumpt -w internal/proxy/proxy.go
golangci-lint run --fix internal/proxy/proxy.go

# Or all at once
task fmt
task lint-fix
```

### YAML Files

```bash
# Auto-format YAML
yamlfmt -w config/example.yaml

# Fix all YAML files
task yaml-fmt
```

### Markdown Files

```bash
# Auto-fix markdown
markdownlint --fix README.md

# Fix all markdown
task markdown-lint
```

## Pre-Commit Auto-Fix

Git hooks automatically fix on commit:

```bash
git add internal/proxy/proxy.go
git commit -m "feat: add feature"

# Hooks automatically:
# 1. Run gofmt, goimports, gofumpt ✓
# 2. Run golangci-lint --fix ✓
# 3. Run yamlfmt ✓
# 4. Run markdownlint --fix ✓
# 5. Stage fixed files ✓
# 6. Complete commit ✓
```

## Development Workflow with Auto-Fix

### Rapid Development

```bash
# Terminal 1: Live reload
task dev

# Terminal 2: Make changes
vim internal/proxy/proxy.go

# Terminal 3: Auto-fix on save (optional)
# Use editor plugin or:
while inotifywait -e modify internal/; do
    task fmt
    task lint-fix
done
```

### Before Committing

```bash
# 1. Auto-fix everything
task fmt
task lint-fix

# 2. Check remaining
task lint

# 3. Fix manually if needed
vim internal/proxy/proxy.go

# 4. Verify all pass
task ci

# 5. Commit
git add .
git commit -m "feat: your feature"
```

## IDE Integration

### VS Code

```json
// .vscode/settings.json
{
  "go.formatTool": "goimports",
  "editor.formatOnSave": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace"
}
```

### Vim/Neovim

```vim
" Auto-format on save
autocmd BufWritePre *.go :silent !goimports -w %
autocmd BufWritePre *.go :silent !gofumpt -w %

" Run lint-fix on save
autocmd BufWritePost *.go :silent !golangci-lint run --fix %
```

## Understanding Auto-Fix Limitations

### Can Auto-Fix

✅ Formatting issues
✅ Import management
✅ Simple style violations
✅ Error wrapping format
✅ Unnecessary conversions
✅ Basic code simplifications

### Cannot Auto-Fix

❌ Unchecked errors (need error handling logic)
❌ Race conditions (need synchronization)
❌ High complexity (need refactoring)
❌ Security issues (need security logic)
❌ Logic errors (need correct algorithm)
❌ Missing tests (need test writing)

## Examples of Auto-Fix in Action

### Example 1: Import Cleanup

```bash
$ goimports -w internal/proxy/proxy.go
```

```go
// Before
import (
    "fmt"      // Used
    "os"       // Unused - removed
    "net/http" // Used
    "strings"  // Unused - removed
)

// After (auto-fixed)
import (
    "fmt"
    "net/http"
)
```

### Example 2: Error Wrapping

```bash
$ golangci-lint run --fix internal/proxy/proxy.go
```

```go
// Before
if err != nil {
    return fmt.Errorf("failed: %s", err)
    return fmt.Errorf("error: %v", err)
}

// After (auto-fixed)
if err != nil {
    return fmt.Errorf("failed: %w", err)
    return fmt.Errorf("error: %w", err)
}
```

### Example 3: Style Simplification

```bash
$ gofumpt -w internal/proxy/proxy.go
```

```go
// Before
if x == true {
    return true
} else {
    return false
}

// After (auto-fixed)
return x
```

## Batch Auto-Fix

### Fix Entire Project

```bash
# Format everything
task fmt

# Fix all lint issues
task lint-fix

# Check results
task lint
```

### Fix Specific Package

```bash
# Format package
gofmt -w internal/proxy/
goimports -w internal/proxy/
gofumpt -w internal/proxy/

# Fix lint issues in package
golangci-lint run --fix internal/proxy/...
```

### Fix Single File

```bash
# Format file
gofmt -w internal/proxy/proxy.go
goimports -w internal/proxy/proxy.go
gofumpt -w internal/proxy/proxy.go

# Fix lint issues
golangci-lint run --fix internal/proxy/proxy.go
```

## Verification After Auto-Fix

```bash
# 1. Auto-fix
task fmt
task lint-fix

# 2. Verify no issues
task lint
task test

# 3. Check diff
git diff

# 4. Commit if good
git add .
git commit -m "style: auto-fix formatting and lint issues"
```

## Tips

1. **Always format first**: Run `task fmt` before `task lint-fix`
2. **Review changes**: Check `git diff` after auto-fix
3. **Commit separately**: Auto-fixes can be separate commits
4. **Run frequently**: Don't let issues accumulate
5. **Use pre-commit**: Let hooks do it automatically
6. **Trust the tools**: Auto-fixes are almost always correct
7. **Manual after auto**: Fix remaining issues manually

## Common Patterns

### Clean Up Workflow

```bash
# Full cleanup
task fmt && task lint-fix && task test
```

### Quick Fix

```bash
# Just format
task fmt
```

### Deep Clean

```bash
# Format, fix, test, verify
task fmt && task lint-fix && task test && task ci
```

## What to Do When Auto-Fix Fails

```bash
# 1. Check error message
task lint-fix

# 2. If auto-fix fails on file
# Fix syntax errors first
go build ./internal/proxy

# 3. Then retry auto-fix
task fmt
task lint-fix

# 4. Fix remaining manually
task lint  # See what's left
```
