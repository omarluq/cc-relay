# Quick Task 007: Integrate Fang CLI Starter Kit

## Description

Integrate [charmbracelet/fang](https://github.com/charmbracelet/fang) into cc-relay's existing Cobra-based CLI. Fang provides styled help, errors, automatic versioning, manpages, and shell completions.

## Tasks

### 1. Add Fang dependency

```bash
go get github.com/charmbracelet/fang@latest
```

### 2. Update main.go

Replace `rootCmd.Execute()` with `fang.Execute()`:

```go
package main

import (
    "context"
    "os"

    "github.com/charmbracelet/fang"
    "github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
    Use:   "cc-relay",
    Short: "Multi-provider proxy for Claude Code",
    Long: `cc-relay is a multi-provider proxy that sits between Claude Code and multiple
LLM providers (Anthropic, Z.AI, Ollama), enabling seamless model switching,
rate limit pooling, and intelligent routing.`,
}

func init() {
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
        "config file path (default: ./config.yaml or ~/.config/cc-relay/config.yaml)")
}

func main() {
    if err := fang.Execute(context.Background(), rootCmd); err != nil {
        os.Exit(1)
    }
}
```

### 3. Verify functionality

- Run `cc-relay --help` to see styled output
- Run `cc-relay --version` to see automatic version flag
- Run `cc-relay man` to generate manpages
- Run `cc-relay completion bash/zsh/fish` to see completions

## Success Criteria

- [ ] Fang dependency added to go.mod
- [ ] main.go updated to use `fang.Execute()` instead of `rootCmd.Execute()`
- [ ] Help output is styled
- [ ] `--version` flag works automatically
- [ ] `man` command generates manpages
- [ ] `completion` command generates shell completions
- [ ] All tests pass
- [ ] Atomic commit created

## Notes

- Fang is a drop-in replacement - minimal code changes required
- No breaking changes to existing CLI behavior
- Theme can be customized later if desired
