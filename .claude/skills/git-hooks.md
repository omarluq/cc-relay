# Git Hooks Skill

Use this skill when working with git hooks and commit workflows.

## When to Use

- Understanding what will run on commit
- Testing hooks before committing
- Debugging hook failures
- Bypassing hooks (when necessary)

## Hook Overview

### Pre-commit (Runs on `git commit`)

Automatically runs:
1. **Go Formatting**: gofmt, goimports, gofumpt
2. **Go Linting**: golangci-lint (with auto-fix)
3. **Go Vetting**: go vet
4. **YAML**: yamlfmt (format), yamllint (lint)
5. **Markdown**: markdownlint (with auto-fix)
6. **Proto**: buf lint
7. **Quick Tests**: go test -short

### Pre-push (Runs on `git push`)

Automatically runs:
1. **Full Test Suite**: with coverage
2. **Dependency Check**: go mod tidy verification
3. **Security Scan**: govulncheck
4. **Build Check**: verify project builds

### Commit Message (Runs on commit)

Validates Conventional Commits format:
```
type(scope): description

Valid types: feat, fix, docs, style, refactor, perf, test, chore, ci, build
```

## Commands

```bash
# Test hooks manually
task hooks-run
lefthook run pre-commit

# Install/reinstall hooks
task hooks-install
lefthook install

# Uninstall hooks
task hooks-uninstall
lefthook uninstall

# Run specific hook
lefthook run pre-push
```

## Debugging Hook Failures

When a hook fails:

1. **See the error** - Lefthook shows detailed output
2. **Fix the issue** - Usually auto-fix will handle it
3. **Test manually**:
   ```bash
   # For Go issues
   task lint-fix
   task fmt

   # For tests
   task test-short

   # Full check
   task ci
   ```

## Bypassing Hooks (Not Recommended)

```bash
# Skip hooks (use sparingly!)
git commit --no-verify

# Skip specific hook
LEFTHOOK=0 git commit
```

**Note**: Only bypass hooks if absolutely necessary. CI will catch the same issues.

## Commit Message Examples

✅ **Good**:
```bash
git commit -m "feat(proxy): add rate limiting support"
git commit -m "fix: correct SSE streaming bug"
git commit -m "docs: update installation guide"
git commit -m "test(router): add failover test cases"
git commit -m "refactor(config): simplify YAML parsing"
```

❌ **Bad**:
```bash
git commit -m "update stuff"           # No type
git commit -m "Fix bug"                # Wrong format
git commit -m "WIP"                    # Not descriptive
git commit -m "feat implement feature" # Missing colon
```

## Configuration

Hooks are configured in `lefthook.yml`:
- Can enable/disable specific hooks
- Can adjust which files trigger hooks
- Can skip hooks on merge/rebase

## Tips

- Let hooks auto-fix issues when possible
- Run `task pre-commit` before committing to catch issues early
- If commits are slow, consider running heavy checks on push instead
- Use meaningful commit messages - they help with debugging later
