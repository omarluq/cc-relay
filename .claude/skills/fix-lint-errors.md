# Fix Lint Errors Skill

**CRITICAL**: When linters fail, FIX THE CODE, not the configuration.

## Core Principle

❌ **NEVER** edit `.golangci.yml` to disable linters
✅ **ALWAYS** fix the actual code issues

## When Linters Fail

### Step 1: Understand the Error

```bash
# Run linters to see errors
task lint

# Example output:
# internal/proxy/proxy.go:42:2: Error return value is not checked (errcheck)
```

### Step 2: Fix the Code

**Example: Unchecked Error**

❌ Wrong (editing config):
```yaml
# .golangci.yml
linters:
  disable:
    - errcheck  # NEVER DO THIS
```

✅ Right (fixing code):
```go
// Before (bad)
file, _ := os.Open("config.yaml")

// After (good)
file, err := os.Open("config.yaml")
if err != nil {
    return fmt.Errorf("failed to open config: %w", err)
}
```

### Step 3: Verify Fix

```bash
# Run linters again
task lint

# Should pass now
```

## Common Fixes

### 1. Unchecked Errors (errcheck)

```go
// ❌ Bad
res, _ := http.Get(url)

// ✅ Good
res, err := http.Get(url)
if err != nil {
    return fmt.Errorf("request failed: %w", err)
}
defer res.Body.Close()
```

### 2. Error Wrapping (errorlint)

```go
// ❌ Bad
return fmt.Errorf("failed: %s", err)

// ✅ Good
return fmt.Errorf("failed: %w", err)
```

### 3. Exported Loop Var (exportloopref)

```go
// ❌ Bad
for _, item := range items {
    go func() {
        process(item)
    }()
}

// ✅ Good
for _, item := range items {
    item := item
    go func() {
        process(item)
    }()
}
```

### 4. Unused Variables (unused)

```go
// ❌ Bad
result := compute()
// never used

// ✅ Good - use it
result := compute()
fmt.Println(result)

// ✅ Good - or explicitly ignore
_ = compute()
```

### 5. High Complexity (gocyclo)

```go
// ❌ Bad - too complex
func process(data Data) error {
    if data.Type == "A" {
        if data.Valid {
            if data.Ready {
                // 20+ nested ifs...
            }
        }
    }
}

// ✅ Good - extract functions
func process(data Data) error {
    if err := validateData(data); err != nil {
        return err
    }
    return processValidData(data)
}

func validateData(data Data) error { /* ... */ }
func processValidData(data Data) error { /* ... */ }
```

### 6. Security Issues (gosec)

```go
// ❌ Bad
password := "hardcoded123"

// ✅ Good
password := os.Getenv("PASSWORD")
if password == "" {
    return errors.New("PASSWORD not set")
}
```

### 7. Repeated Strings (goconst)

```go
// ❌ Bad
if status == "success" {
    log.Println("success")
}

// ✅ Good
const StatusSuccess = "success"

if status == StatusSuccess {
    log.Println(StatusSuccess)
}
```

## Workflow

1. **Run linters**: `task lint`
2. **Read the error** carefully
3. **Understand the problem**
4. **Fix the code** (not config!)
5. **Verify**: `task lint`
6. **Repeat** until all pass

## When Code is Correct

In rare cases where linter is wrong:

```go
// Only use //nolint with justification
//nolint:gosec // G101: False positive, not a credential
comment := "password field description"
```

But **always prefer fixing the code**.

## Integration

- **Pre-commit**: Linters run with `--fix` automatically
- **Manual**: `task lint-fix` for auto-fix
- **CI**: All lint errors block PR merge

## Never Do This

❌ Disable linters in config
❌ Add files to `.golangci.yml` exclude list
❌ Increase complexity thresholds
❌ Remove linters from enabled list

## Always Do This

✅ Fix the actual code issue
✅ Refactor complex functions
✅ Handle errors properly
✅ Use constants for repeated strings
✅ Wrap errors with %w
✅ Check all error return values

## Tips

1. **Auto-fix first**: `task lint-fix` fixes many issues
2. **One at a time**: Fix errors incrementally
3. **Understand why**: Learn from each fix
4. **Ask for help**: If unsure, ask user for guidance
5. **Test after fixing**: Run `task test` to ensure still works
