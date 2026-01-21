# Live Reload Skill

Use this skill when developing with automatic recompilation.

## When to Use

- Active development
- Rapid iteration
- Testing changes immediately
- Debugging

## Quick Start

```bash
# Start live reload
task dev

# Or directly
air

# Air will:
# 1. Build the project
# 2. Start the server
# 3. Watch for changes
# 4. Rebuild and restart on changes
```

## What Air Watches

Configured in `.air.toml`:

- `*.go` files (all Go source)
- `*.proto` files (protobuf definitions)
- `*.tpl`, `*.tmpl` files (templates)
- `*.html` files (HTML templates)

## Excluded from Watch

- `tmp/` - Build directory
- `vendor/` - Dependencies
- `.git/` - Git files
- `*_test.go` - Test files
- `testdata/` - Test fixtures

## Configuration

`.air.toml` settings:

```toml
[build]
  cmd = "go build -o ./tmp/main ./cmd/cc-relay"
  bin = "./tmp/main"
  delay = 1000  # Wait 1s after change before rebuild
  exclude_regex = ["_test.go"]
  include_ext = ["go", "proto", "html"]
```

## Usage Examples

### Basic Development

```bash
# Terminal 1: Start air
task dev

# Terminal 2: Make changes
vim internal/proxy/proxy.go

# Air automatically:
# - Detects change
# - Rebuilds
# - Restarts server
```

### With Custom Config

```bash
# Start with specific config path
air -c .air.toml

# Specify build command
air -c .air.toml --build.cmd "go build -tags debug ."
```

### Debugging Build Issues

```bash
# Check air logs
cat tmp/build-errors.log

# Disable air temporarily
go build -o tmp/main ./cmd/cc-relay
./tmp/main

# Then restart air
task dev
```

## Build Output

```
tmp/
├── main           # Built binary
└── build-errors.log  # Build errors if any
```

## Air Workflow

```
Change detected
    ↓
Wait delay (1s)
    ↓
Kill old process
    ↓
Build project
    ↓
Start new process
    ↓
Ready for changes
```

## Tips

1. **Use `task dev`**: Easier than remembering air command
2. **Watch the logs**: Air shows build errors immediately
3. **Wait for rebuild**: Takes 1-3 seconds typically
4. **Multiple files**: Air batches changes within delay window
5. **Proto changes**: Regenerate proto first, then air rebuilds

## Common Issues

### Port Already in Use

```bash
# Find and kill process
lsof -ti:8787 | xargs kill -9

# Or use different port in config
```

### Build Errors

```bash
# Check logs
cat tmp/build-errors.log

# Fix errors
# Air will auto-rebuild when you save
```

### Too Many Rebuilds

```bash
# Increase delay in .air.toml
delay = 2000  # 2 seconds

# Or exclude more files
exclude_regex = ["_test.go", ".*_gen.go"]
```

### Changes Not Detected

```bash
# Check file is watched
air -c .air.toml --verbose

# Restart air
pkill air
task dev
```

## Integration with Other Tools

### With Tests

```bash
# Terminal 1: Live reload
task dev

# Terminal 2: Watch tests
watch -n 2 task test-short
```

### With Linting

```bash
# Terminal 1: Live reload
task dev

# Terminal 2: Watch lint
watch -n 5 task lint
```

## Color Output

Air uses colored output:
- **Yellow**: Building
- **Green**: Running
- **Cyan**: Watching
- **Magenta**: Main process

## Customization

Edit `.air.toml` to:
- Change build command
- Modify watched extensions
- Adjust delay
- Configure logging
- Set environment variables

## Stopping Air

```bash
# Graceful stop
Ctrl+C

# Force kill
pkill air

# Kill binary too
pkill -f tmp/main
```

## Production Build

Air is for development only!

For production:
```bash
# Build optimized binary
task build

# Or with optimizations
go build -ldflags="-s -w" -o cc-relay ./cmd/cc-relay
```
