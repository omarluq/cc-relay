// Package main is the entry point for cc-relay.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	// Setup logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Determine config path
	if *configPath == "" {
		*configPath = findConfigFile()
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err, "path", *configPath)
		os.Exit(1)
	}

	// Find first enabled Anthropic provider
	var provider providers.Provider

	var providerKey string

	for _, p := range cfg.Providers {
		if p.Enabled && p.Type == "anthropic" {
			provider = providers.NewAnthropicProvider(p.Name, p.BaseURL)

			if len(p.Keys) > 0 {
				providerKey = p.Keys[0].Key
			}

			break
		}
	}

	if provider == nil {
		slog.Error("no enabled anthropic provider found in config")
		os.Exit(1)
	}

	// Setup routes
	handler, err := proxy.SetupRoutes(cfg, provider, providerKey)
	if err != nil {
		slog.Error("failed to setup routes", "error", err)
		os.Exit(1)
	}

	// Create server
	server := proxy.NewServer(cfg.Server.Listen, handler)

	// Graceful shutdown on SIGINT/SIGTERM
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		slog.Info("shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "error", err)
		}

		close(done)
	}()

	// Start server
	slog.Info("starting cc-relay", "listen", cfg.Server.Listen)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	<-done
	slog.Info("server stopped")
}

// findConfigFile searches for config.yaml in default locations.
// Priority:
//  1. Current directory (./config.yaml)
//  2. User config directory (~/.config/cc-relay/config.yaml)
func findConfigFile() string {
	// Check current directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	// Check ~/.config/cc-relay/
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		p := filepath.Join(home, ".config", "cc-relay", "config.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return "config.yaml" // Default, will error if not found
}
