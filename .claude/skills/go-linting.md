# Go Linting Skill

Use this skill when working with Go code quality and linting.

## When to Use

- Before committing Go code
- Debugging linting errors
- Understanding lint rules
- Fixing code quality issues

## Primary Tool: golangci-lint

We use `golangci-lint` which runs 40+ linters simultaneously.

### Quick Commands

```bash
# Run all linters
task lint
golangci-lint run

# Run with auto-fix
task lint-fix
golangci-lint run --fix

# Run on specific files
golangci-lint run ./internal/proxy/...

# See which linters are enabled
golangci-lint linters
```

## Key Linters Enabled

### Error Handling
- **errcheck**: Checks for unchecked errors
- **errorlint**: Go 1.13+ error wrapping
- **nilerr**: Returns nil error incorrectly

### Security
- **gosec**: Security vulnerabilities
- **exportloopref**: Loop variable pointer issues

### Code Quality
- **staticcheck**: Advanced static analysis
- **revive**: Drop-in golint replacement
- **unused**: Unused code detection
- **govet**: Official Go analyzer

### Complexity
- **gocyclo**: Cyclomatic complexity
- **gocognit**: Cognitive complexity
- **funlen**: Function length

### Style
- **gofmt**: Code formatting
- **goimports**: Import ordering
- **stylecheck**: Style consistency

### Best Practices
- **goconst**: Repeated strings → constants
- **dupl**: Code duplication
- **unconvert**: Unnecessary conversions
- **unparam**: Unused parameters

## Common Lint Errors

### 1. Unchecked Errors

```go
// ❌ Bad
file, _ := os.Open("file.txt")

// ✅ Good
file, err := os.Open("file.txt")
if err != nil {
    return err
}
```

### 2. Error Wrapping

```go
// ❌ Bad
if err != nil {
    return fmt.Errorf("failed: %s", err)
}

// ✅ Good
if err != nil {
    return fmt.Errorf("failed: %w", err)
}
```

### 3. Exported Loop Variables

```go
// ❌ Bad
for _, item := range items {
    go func() {
        process(item) // item reference changes
    }()
}

// ✅ Good
for _, item := range items {
    item := item // capture
    go func() {
        process(item)
    }()
}
```

### 4. Unused Variables

```go
// ❌ Bad
func process() {
    result := compute()
    // result never used
}

// ✅ Good
func process() {
    _ = compute() // explicitly ignored
}
```

## Configuration

Linting rules are in `.golangci.yml`:
- 40+ linters enabled
- Custom complexity thresholds
- Per-file exclusions for tests
- Security settings

## Disabling Linters (Use Sparingly)

```go
// Disable for next line
//nolint:errcheck
file, _ := os.Open("file.txt")

// Disable specific linter
//nolint:gosec
password := "hardcoded" // G101: Potential hardcoded credentials

// Disable for entire file (top of file)
//nolint:all
package main
```

## Integration

- **Pre-commit Hook**: Runs automatically with `--fix`
- **GitHub Actions**: Runs on every PR
- **Task**: `task lint` and `task lint-fix`

## Tips

1. **Run before committing**: `task lint-fix`
2. **Understand the error**: Read the linter message
3. **Fix the root cause**: Don't just disable
4. **Use auto-fix**: Many issues fix automatically
5. **Check configuration**: `.golangci.yml` has all settings

## Debugging Lint Issues

```bash
# Show detailed lint output
golangci-lint run --verbose

# Run specific linter
golangci-lint run --disable-all --enable=errcheck

# Show configuration
golangci-lint config path

# Update linters
golangci-lint cache clean
```

## Performance

```bash
# Use cache for speed
golangci-lint cache status

# Clear cache if issues
golangci-lint cache clean

# Lint only changed files
golangci-lint run --new-from-rev=HEAD~1
```
