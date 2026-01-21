# Skills and Tools Summary

Complete Claude Code learning system for cc-relay development.

## 📁 Directory Structure

```
.claude/
├── INDEX.md                      # Master index - start here
├── CLAUDE.md                     # Project overview
├── SKILLS_SUMMARY.md             # This file
│
├── skills/                       # Development skills library
│   ├── task-runner.md           # Using task for development
│   ├── git-hooks.md             # Git hooks and commit workflow
│   ├── live-reload.md           # Development with air
│   ├── fix-lint-errors.md       # ⭐ Fix code when linters fail
│   ├── fix-test-failures.md     # ⭐ Fix code when tests fail
│   ├── fix-format-issues.md     # ⭐ Let formatters fix code
│   ├── auto-fix-code.md         # ⭐ Using auto-fix tools
│   ├── go-linting.md            # Deep dive on Go linting
│   └── testing.md               # Testing guide
│
├── commands/                     # Slash commands
│   └── README.md                # Quick command reference
│
└── agents/                       # Agent configurations
    └── README.md                # Specialized agents
```

## 🎯 Core Principles

### Golden Rule: Fix Code, Not Config

When tools fail:

❌ **NEVER**:
- Edit `.golangci.yml` to disable linters
- Edit `lefthook.yml` to skip hooks
- Edit `.yamlfmt` or `.yamllint` to allow bad formatting
- Skip or disable tests
- Modify test infrastructure

✅ **ALWAYS**:
- Fix the actual source code
- Use auto-fix tools first (`task fmt`, `task lint-fix`)
- Understand the error before fixing
- Verify fixes work (`task ci`)

### Auto-Fix First Workflow

```bash
# 1. Auto-format
task fmt

# 2. Auto-fix lint issues
task lint-fix

# 3. Check what remains
task lint
task test

# 4. Fix remaining issues manually

# 5. Verify everything
task ci
```

## 📚 Skills Library

### Critical Skills (Read First)

| Skill | Purpose | When to Read |
|-------|---------|-------------|
| [fix-lint-errors](skills/fix-lint-errors.md) | Fix code when golangci-lint fails | Lint errors occur |
| [fix-test-failures](skills/fix-test-failures.md) | Fix code/tests when tests fail | Tests don't pass |
| [fix-format-issues](skills/fix-format-issues.md) | Use formatters to fix code | Format checks fail |
| [auto-fix-code](skills/auto-fix-code.md) | Use auto-fix tools efficiently | Before fixing anything manually |

### Development Skills

| Skill | Purpose | When to Read |
|-------|---------|-------------|
| [task-runner](skills/task-runner.md) | Use task for development | Need to run build/test/lint |
| [git-hooks](skills/git-hooks.md) | Understand git hooks | Commit/push hooks fail |
| [live-reload](skills/live-reload.md) | Use air for development | Active development |
| [go-linting](skills/go-linting.md) | Deep dive on linting | Understanding golangci-lint |
| [testing](skills/testing.md) | Testing guide | Writing/debugging tests |

## 🚀 Quick Commands

All commands are defined in [commands/README.md](commands/README.md).

### Most Used

```bash
task dev          # Start live reload
task fmt          # Format all code
task lint-fix     # Auto-fix lint issues
task test         # Run tests
task ci           # Full CI check
```

### Quality Checks

```bash
task lint         # Run linters
task test-short   # Quick tests
task test-coverage  # Tests with coverage
task security     # Security scan
```

### Build & Deploy

```bash
task build        # Build binary
task build-all    # Build for all platforms
task clean        # Clean artifacts
```

## 🤖 Specialized Agents

Agents are configured in [agents/README.md](agents/README.md).

| Agent | Purpose | Skills Used |
|-------|---------|-------------|
| **go-code-fixer** | Fix Go code issues | fix-lint-errors, fix-test-failures, auto-fix-code |
| **test-debugger** | Debug test failures | testing, fix-test-failures |
| **code-formatter** | Auto-format code | fix-format-issues, auto-fix-code |
| **security-auditor** | Fix security issues | fix-lint-errors (gosec) |
| **proto-generator** | Proto generation | Proto tools |

## 🛠️ Tool Reference

### Development Tools

| Tool | Purpose | Config | Usage |
|------|---------|--------|-------|
| task | Task runner | Taskfile.yml | `task <command>` |
| lefthook | Git hooks | lefthook.yml | Automatic on commit/push |
| air | Live reload | .air.toml | `task dev` or `air` |
| mise | Tool versions | .mise.toml | `mise install` |

### Go Quality Tools

| Tool | Purpose | Config | Auto-Fix |
|------|---------|--------|----------|
| gofmt | Basic format | - | Yes |
| goimports | Import mgmt | - | Yes |
| gofumpt | Strict format | - | Yes |
| golangci-lint | Meta-linter | .golangci.yml | Partial |
| govulncheck | Vuln scanner | - | No |
| gosec | Security lint | - | No |

### Other Tools

| Tool | Purpose | Config | Auto-Fix |
|------|---------|--------|----------|
| buf | Proto tools | - | Partial |
| yamlfmt | YAML format | .yamlfmt | Yes |
| yamllint | YAML lint | .yamllint | No |
| markdownlint | MD lint | .markdownlint.json | Yes |

## 📖 Learning Paths

### For New Sessions

1. **Read [INDEX.md](INDEX.md)** - Overview of everything
2. **Read critical skills**:
   - [fix-lint-errors](skills/fix-lint-errors.md)
   - [fix-test-failures](skills/fix-test-failures.md)
   - [auto-fix-code](skills/auto-fix-code.md)
3. **Bookmark [commands/README.md](commands/README.md)** - Quick reference

### For Specific Tasks

- **Starting development** → [live-reload](skills/live-reload.md)
- **Running tests** → [testing](skills/testing.md)
- **Fixing lint errors** → [fix-lint-errors](skills/fix-lint-errors.md)
- **Test failures** → [fix-test-failures](skills/fix-test-failures.md)
- **Before committing** → [git-hooks](skills/git-hooks.md)
- **Understanding linters** → [go-linting](skills/go-linting.md)
- **Auto-fixing issues** → [auto-fix-code](skills/auto-fix-code.md)

## 🎓 Quick Start Guide

### 1. First Time Setup

```bash
# Install all tools
./scripts/setup-tools.sh

# Verify installation
task --list
```

### 2. Development Workflow

```bash
# Start live reload
task dev

# Make changes...

# Before committing
task fmt
task lint-fix
task test-short

# Full check
task ci

# Commit (hooks run automatically)
git commit -m "feat: description"
```

### 3. When Things Fail

**Linters fail:**
1. Read error message
2. Run `task lint-fix` for auto-fix
3. Fix remaining issues manually
4. See [fix-lint-errors](skills/fix-lint-errors.md)

**Tests fail:**
1. Run with verbose: `go test -v`
2. Understand the failure
3. Fix the code or test
4. See [fix-test-failures](skills/fix-test-failures.md)

**Format issues:**
1. Run `task fmt`
2. Let formatters fix it
3. See [fix-format-issues](skills/fix-format-issues.md)

## 📝 Configuration Files

**DO NOT EDIT** these files when tools fail:

- `.golangci.yml` - Linting rules
- `lefthook.yml` - Git hooks
- `.yamlfmt` - YAML formatting
- `.yamllint` - YAML linting
- `.markdownlint.json` - Markdown rules
- `.air.toml` - Live reload config
- `Taskfile.yml` - Task definitions

**Instead**: Fix the actual code issues.

## ✅ Verification Checklist

Before considering a task complete:

```bash
# All must pass
task fmt          # ✓ Code formatted
task lint         # ✓ No lint errors
task test         # ✓ All tests pass
task security     # ✓ No vulnerabilities
task build        # ✓ Builds successfully
```

Or use:
```bash
task ci  # Runs all checks
```

## 🆘 Getting Help

1. **Check INDEX**: [INDEX.md](INDEX.md)
2. **Read relevant skill**: See skills/ directory
3. **Check commands**: [commands/README.md](commands/README.md)
4. **Verify tools work**: `task doctor`
5. **Ask user**: If truly stuck

## 🎯 Key Takeaways

1. **Fix code, not config** - Configuration is correct
2. **Auto-fix first** - Use `task fmt` and `task lint-fix`
3. **Read skills** - Guidance for every situation
4. **Verify always** - Run `task ci` before finishing
5. **Follow tools** - Trust the formatters and linters

---

**For complete details, see [INDEX.md](INDEX.md)**
