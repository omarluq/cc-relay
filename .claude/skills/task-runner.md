# Task Runner Skill

Use this skill when you need to run development tasks for cc-relay.

## When to Use

- Building the project
- Running tests
- Linting/formatting code
- Running security scans
- Any development task

## Available Tasks

```bash
# Most common tasks
task dev              # Start with live reload
task build            # Build binary
task test             # Run all tests
task test-short       # Quick tests
task test-coverage    # Tests with coverage
task lint             # Run linters
task lint-fix         # Lint with auto-fix
task fmt              # Format all code
task ci               # Full CI checks
task security         # Security scan

# Build tasks
task build-all        # Build for all platforms
task build-linux
task build-darwin
task build-windows

# Proto/gRPC tasks
task proto            # Generate from proto
task proto-lint       # Lint proto files
task proto-format     # Format proto files

# YAML tasks
task yaml-fmt         # Format YAML
task yaml-lint        # Lint YAML

# Markdown tasks
task markdown-lint    # Lint Markdown

# Git hooks
task hooks-install    # Install hooks
task hooks-run        # Test hooks manually
task hooks-uninstall  # Uninstall hooks

# Maintenance
task deps             # Download deps
task deps-update      # Update deps
task deps-tidy        # Tidy go.mod
task clean            # Clean build artifacts
task setup            # Setup dev environment
task info             # Show project info
```

## Usage Examples

### Running Tests

```bash
# Quick test during development
task test-short

# Full test suite
task test

# With coverage report
task test-coverage
```

### Formatting and Linting

```bash
# Format all code
task fmt

# Run linters
task lint

# Run with auto-fix
task lint-fix
```

### Before Committing

```bash
# Run all pre-commit checks
task pre-commit

# Or run full CI locally
task ci
```

### Development Workflow

```bash
# Start live reload server
task dev

# In another terminal, make changes
# They'll auto-reload

# Before committing
task ci
```

## Tips

- Use `task --list` to see all available tasks
- `task dev` is the recommended development mode
- Run `task ci` before creating PRs
- `task pre-commit` is faster than full CI
