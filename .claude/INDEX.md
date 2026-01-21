# Claude Code Development Guide Index

Complete guide to using development tools in cc-relay.

## Quick Links

- **[Main Guide](CLAUDE.md)** - Project overview and architecture
- **[Development Guide](../DEVELOPMENT.md)** - Complete development workflow
- **[Quick Reference](../QUICK_REFERENCE.md)** - Command cheat sheet
- **[Setup Summary](../SETUP_SUMMARY.md)** - What's installed

## Skills Library

### Core Development Skills

1. **[task-runner](skills/task-runner.md)** - Using the task runner
   - All available tasks
   - Common workflows
   - Build, test, lint commands

2. **[git-hooks](skills/git-hooks.md)** - Working with git hooks
   - What runs on commit/push
   - Debugging hook failures
   - Commit message format

3. **[live-reload](skills/live-reload.md)** - Development with air
   - Starting live reload
   - What files are watched
   - Debugging build issues

### Code Quality Skills

4. **[fix-lint-errors](skills/fix-lint-errors.md)** ⭐ **CRITICAL**
   - **Fix code, not config**
   - Common lint error fixes
   - Workflow for fixing issues

5. **[fix-test-failures](skills/fix-test-failures.md)** ⭐ **CRITICAL**
   - **Fix code/tests, not test config**
   - Debugging test failures
   - Common test issues

6. **[fix-format-issues](skills/fix-format-issues.md)** ⭐ **CRITICAL**
   - **Let formatters fix code**
   - Auto-formatting workflow
   - Don't modify formatter configs

7. **[auto-fix-code](skills/auto-fix-code.md)** ⭐ **ESSENTIAL**
   - Using auto-fix tools
   - What can be auto-fixed
   - Auto-fix workflow

8. **[go-linting](skills/go-linting.md)** - Go linting in depth
   - golangci-lint usage
   - Enabled linters
   - Common issues and fixes

9. **[testing](skills/testing.md)** - Testing guide
   - Running tests
   - Writing tests
   - Debugging failures
   - Coverage

## Commands

**[Slash Commands](commands/README.md)** - Quick command reference

Common commands:
- `/fmt` - Format all code
- `/lint-fix` - Auto-fix lint issues
- `/test` - Run tests
- `/ci` - Run full CI checks
- `/dev` - Start live reload

## Agents

**[Agent Configurations](agents/README.md)** - Specialized agents

Available agents:
- **go-code-fixer** - Fix Go code issues
- **test-debugger** - Debug test failures
- **code-formatter** - Auto-format code
- **security-auditor** - Fix security issues
- **proto-generator** - Proto file generation

## Tool Documentation

### Development Tools

| Tool | Purpose | Config File | Skill Reference |
|------|---------|-------------|-----------------|
| **task** | Task runner | Taskfile.yml | [task-runner](skills/task-runner.md) |
| **lefthook** | Git hooks | lefthook.yml | [git-hooks](skills/git-hooks.md) |
| **air** | Live reload | .air.toml | [live-reload](skills/live-reload.md) |
| **mise** | Tool versions | .mise.toml | - |

### Go Tools

| Tool | Purpose | Config File | Skill Reference |
|------|---------|-------------|-----------------|
| **gofmt** | Basic formatting | - | [fix-format-issues](skills/fix-format-issues.md) |
| **goimports** | Import management | - | [fix-format-issues](skills/fix-format-issues.md) |
| **gofumpt** | Strict formatting | - | [fix-format-issues](skills/fix-format-issues.md) |
| **golangci-lint** | Meta-linter | .golangci.yml | [go-linting](skills/go-linting.md), [fix-lint-errors](skills/fix-lint-errors.md) |
| **govulncheck** | Security scanner | - | Security |
| **gosec** | Security linter | - | Security |

### Other Tools

| Tool | Purpose | Config File |
|------|---------|-------------|
| **buf** | Proto tools | - |
| **yamlfmt** | YAML formatter | .yamlfmt |
| **yamllint** | YAML linter | .yamllint |
| **markdownlint** | MD linter | .markdownlint.json |

## Critical Principles

### ⭐ When Tools Fail

1. **Linters fail** → [Fix the code](skills/fix-lint-errors.md), not .golangci.yml
2. **Tests fail** → [Fix code/tests](skills/fix-test-failures.md), not test config
3. **Format issues** → [Let formatters fix](skills/fix-format-issues.md), don't modify formatter config

### ⭐ Auto-Fix First

1. Run `task fmt` - Auto-format everything
2. Run `task lint-fix` - Auto-fix lint issues
3. Fix remaining issues manually
4. Verify with `task ci`

**See**: [auto-fix-code](skills/auto-fix-code.md)

### ⭐ Development Workflow

```bash
# 1. Start development
task dev

# 2. Make changes
# ... edit code ...

# 3. Auto-fix
task fmt
task lint-fix

# 4. Test
task test-short

# 5. Before commit
task ci

# 6. Commit (hooks run automatically)
git commit -m "feat: description"
```

## Quick Start for Claude

When starting a task:

1. **Check current state**
   ```bash
   task --list  # See available tasks
   git status   # Check git state
   ```

2. **Before making changes**
   - Read relevant skill from `skills/` directory
   - Understand auto-fix capabilities
   - Know what can/cannot be edited

3. **When issues occur**
   - **Lint errors** → Read [fix-lint-errors](skills/fix-lint-errors.md)
   - **Test failures** → Read [fix-test-failures](skills/fix-test-failures.md)
   - **Format issues** → Read [fix-format-issues](skills/fix-format-issues.md)

4. **Use auto-fix**
   - Always run `task fmt` first
   - Then run `task lint-fix`
   - Fix remaining issues manually
   - See [auto-fix-code](skills/auto-fix-code.md)

5. **Verify**
   ```bash
   task ci  # Full CI check
   ```

## File Organization

```
.claude/
├── INDEX.md              # This file
├── CLAUDE.md             # Main project guide
├── skills/               # Development skills
│   ├── task-runner.md
│   ├── git-hooks.md
│   ├── live-reload.md
│   ├── fix-lint-errors.md    ⭐ Critical
│   ├── fix-test-failures.md  ⭐ Critical
│   ├── fix-format-issues.md  ⭐ Critical
│   ├── auto-fix-code.md       ⭐ Essential
│   ├── go-linting.md
│   └── testing.md
├── commands/             # Slash commands
│   └── README.md
└── agents/               # Agent configs
    └── README.md
```

## Learning Path

### For New Claude Sessions

1. Read this INDEX.md
2. Read [CLAUDE.md](CLAUDE.md) for project overview
3. Read critical skills:
   - [fix-lint-errors](skills/fix-lint-errors.md)
   - [fix-test-failures](skills/fix-test-failures.md)
   - [auto-fix-code](skills/auto-fix-code.md)
4. Refer to specific skills as needed

### For Specific Tasks

- **Building**: [task-runner](skills/task-runner.md)
- **Testing**: [testing](skills/testing.md)
- **Linting**: [go-linting](skills/go-linting.md), [fix-lint-errors](skills/fix-lint-errors.md)
- **Development**: [live-reload](skills/live-reload.md)
- **Git**: [git-hooks](skills/git-hooks.md)
- **Auto-fix**: [auto-fix-code](skills/auto-fix-code.md)

## Golden Rules

1. **Fix code, never config** - Configuration is correct, code needs fixing
2. **Auto-fix first** - Use automated tools before manual fixes
3. **Verify always** - Run `task ci` before finishing
4. **Read skills** - Refer to skills for guidance
5. **Follow conventions** - Trust the tools and configurations

## Support

For detailed information:
- **Project**: [CLAUDE.md](CLAUDE.md)
- **Development**: [../DEVELOPMENT.md](../DEVELOPMENT.md)
- **Quick Ref**: [../QUICK_REFERENCE.md](../QUICK_REFERENCE.md)
- **Setup**: [../SETUP_SUMMARY.md](../SETUP_SUMMARY.md)
