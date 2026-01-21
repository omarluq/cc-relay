# cc-relay Quick Reference

## 🚀 Essential Commands

```bash
# Start developing
task dev

# Run all CI checks
task ci

# Before committing
task pre-commit
```

## 📝 Task Commands

```bash
task --list          # Show all tasks
task dev             # Live reload
task build           # Build binary
task test            # Run tests
task test-coverage   # Tests + coverage
task lint            # Run linters
task fmt             # Format code
task ci              # Full CI check
task security        # Security scan
```

## 🔧 Git Workflow

```bash
# 1. Make changes
# 2. Stage files
git add .

# 3. Commit (hooks run automatically)
git commit -m "feat: description"

# 4. Push (pre-push hooks run)
git push
```

## ✍️ Commit Format

```
type(scope): description

Types: feat, fix, docs, test, refactor, perf, chore
```

Examples:

```bash
feat(proxy): add rate limiting
fix: correct SSE bug
docs: update README
test(router): add tests
```

## 🛠️ Manual Tools

```bash
# Format
gofmt -w .
goimports -w .
gofumpt -w .

# Lint
golangci-lint run
golangci-lint run --fix

# Security
govulncheck ./...
gosec ./...

# YAML
yamlfmt -w .
yamllint .

# Markdown
markdownlint --fix '**/*.md'

# Proto
buf lint
buf generate
```

## 🎯 Pre-Commit Hooks

Auto-runs:

- ✓ Format (gofmt, goimports, gofumpt)
- ✓ Lint (golangci-lint)
- ✓ Vet (go vet)
- ✓ YAML format & lint
- ✓ Markdown lint
- ✓ Proto lint
- ✓ Quick tests

## 🚦 Pre-Push Hooks

Auto-runs:

- ✓ Full test suite
- ✓ go mod tidy check
- ✓ Security scan
- ✓ Build check

## 📚 Documentation

- `DEVELOPMENT.md` - Full dev guide
- `SETUP_SUMMARY.md` - Setup overview
- `task --list` - All tasks
- `.claude/CLAUDE.md` - Claude integration

## 🔍 Troubleshooting

```bash
# Verify tools
task doctor

# Check tool versions
mise list

# Reinstall hooks
lefthook install

# Test hooks manually
lefthook run pre-commit
```

## 🎨 File Structure

```
.air.toml              # Live reload
.golangci.yml          # Linting rules
.markdownlint.json     # Markdown rules
.mise.toml             # Tool versions
.yamlfmt/.yamllint     # YAML config
lefthook.yml           # Git hooks
Taskfile.yml           # Tasks
```

## ⚡ Quick Tips

1. Use `task dev` for development
2. Run `task ci` before PRs
3. Let hooks catch issues early
4. Follow Conventional Commits
5. Keep tools updated: `mise upgrade`
