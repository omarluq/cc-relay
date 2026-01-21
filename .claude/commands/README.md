# Slash Commands for cc-relay

Quick commands for common development tasks.

## Format Commands

### `/fmt`
Auto-format all code (Go, YAML, Markdown).

**Usage:**
```
/fmt
```

**What it does:**
```bash
task fmt
```

Runs: gofmt, goimports, gofumpt, yamlfmt, markdownlint

---

### `/lint`
Run all linters.

**Usage:**
```
/lint
```

**What it does:**
```bash
task lint
```

---

### `/lint-fix`
Run linters with auto-fix.

**Usage:**
```
/lint-fix
```

**What it does:**
```bash
task lint-fix
```

Auto-fixes: import issues, error wrapping, simple style violations

---

## Testing Commands

### `/test`
Run all tests.

**Usage:**
```
/test
```

**What it does:**
```bash
task test
```

---

### `/test-short`
Run quick tests (for development).

**Usage:**
```
/test-short
```

**What it does:**
```bash
task test-short
```

---

### `/test-coverage`
Run tests with coverage report.

**Usage:**
```
/test-coverage
```

**What it does:**
```bash
task test-coverage
```

Generates: coverage.out, coverage.html

---

## Build Commands

### `/build`
Build the binary.

**Usage:**
```
/build
```

**What it does:**
```bash
task build
```

Output: bin/cc-relay

---

### `/dev`
Start development server with live reload.

**Usage:**
```
/dev
```

**What it does:**
```bash
task dev
```

Uses: air for automatic rebuild

---

## Quality Commands

### `/ci`
Run full CI checks locally.

**Usage:**
```
/ci
```

**What it does:**
```bash
task ci
```

Runs: format, lint, vet, test, build, security

---

### `/pre-commit`
Quick pre-commit checks.

**Usage:**
```
/pre-commit
```

**What it does:**
```bash
task pre-commit
```

Runs: format, lint-fix, test-short

---

### `/security`
Run security scans.

**Usage:**
```
/security
```

**What it does:**
```bash
task security
```

Runs: govulncheck, gosec

---

## Proto Commands

### `/proto`
Generate code from proto files.

**Usage:**
```
/proto
```

**What it does:**
```bash
task proto
```

---

### `/proto-lint`
Lint proto files.

**Usage:**
```
/proto-lint
```

**What it does:**
```bash
task proto-lint
```

---

## Maintenance Commands

### `/clean`
Clean build artifacts.

**Usage:**
```
/clean
```

**What it does:**
```bash
task clean
```

---

### `/deps`
Download dependencies.

**Usage:**
```
/deps
```

**What it does:**
```bash
task deps
```

---

### `/setup`
Setup development environment.

**Usage:**
```
/setup
```

**What it does:**
```bash
task setup
```

Installs: all tools, git hooks

---

## How to Use

In Claude Code, simply type:
```
/lint-fix
```

Claude will execute the corresponding task command.

## Custom Commands

You can create custom commands by adding them to this directory:

1. Create command file: `.claude/commands/my-command.sh`
2. Make executable: `chmod +x .claude/commands/my-command.sh`
3. Use in Claude: `/my-command`
