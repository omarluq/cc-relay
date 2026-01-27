# Repository Guidelines

## Project Structure & Module Organization
- `cmd/cc-relay/`: main entrypoint for the relay binary.
- `internal/`: core packages (auth, cache, config, providers, proxy, router, etc.).
- `relay.proto`: protobuf API definitions; generated code is produced via `task proto`.
- `scripts/`: dev tooling helpers (notably `scripts/setup-tools.sh`).
- `docs-site/`, `docs/`, `README.md`: user-facing documentation.
- `testdata/`: fixtures and test assets.
- Build output defaults to `bin/cc-relay`.

## Build, Test, and Development Commands
- `./scripts/setup-tools.sh`: install required dev tools (mise-managed).
- `task dev`: live-reload development with Air.
- `task build`: build current platform binary into `bin/`.
- `task test`: run all Go tests (`go test ./...`).
- `task test-coverage`: run tests with coverage and generate `coverage.out`.
- `task lint`: run `golangci-lint` and related checks.
- `task ci`: run the full local CI suite (format, lint, tests, security, build).
- `task proto`: generate protobuf code (via `buf generate`).

## Coding Style & Naming Conventions
- Go formatting is enforced with `gofmt`, `goimports`, and `gofumpt` (`task fmt`).
- Linting uses `golangci-lint` (`.golangci.yml`), so keep code lint-clean.
- Use standard Go naming: mixedCaps for exported identifiers, lowerCamel for locals.
- YAML/Markdown formatting is enforced via `task yaml-fmt` and `task markdown-lint`.

## Testing Guidelines
- Tests live alongside code as `*_test.go`.
- Quick checks: `task test-short` (uses `go test -short ./...`).
- Integration tests: `task test-integration` (uses `-tags=integration`).
- Coverage: `task test-coverage` outputs `coverage.out` and HTML via `go tool cover`.

## Commit & Pull Request Guidelines
- Commit messages must follow Conventional Commits:
  - `feat(scope): add rate limiting`
  - `fix: correct SSE streaming bug`
- Valid types include `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`.
- Open an issue before submitting a PR and run `task ci` before pushing.

## Security & Configuration Notes
- Local tool versions are managed by `mise` (`.mise.toml`); use `mise install`.
- Hooks are managed by `lefthook` (`lefthook.yml`) and run on commit/push.
