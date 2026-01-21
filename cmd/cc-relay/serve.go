package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the cc-relay proxy server",
	Long: `Start the proxy server that accepts Claude Code requests and routes them
to configured backend providers.`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	// Determine config path
	configPath := cfgFile
	if configPath == "" {
		configPath = findConfigFile()
	}

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		// Use fallback logger for config load error
		log.Error().Err(err).Str("path", configPath).Msg("failed to load config")
		return err
	}

	// Setup logging from config
	logger, err := proxy.NewLogger(cfg.Logging)
	if err != nil {
		// Fallback to console logger for error reporting
		log.Fatal().Err(err).Msg("failed to initialize logger")
	}

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger

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
		log.Error().Msg("no enabled anthropic provider found in config")
		return errors.New("no enabled anthropic provider in config")
	}

	// Setup routes
	handler, err := proxy.SetupRoutes(cfg, provider, providerKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to setup routes")
		return err
	}

	// Create server
	server := proxy.NewServer(cfg.Server.Listen, handler)

	// Graceful shutdown on SIGINT/SIGTERM
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Info().Msg("shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("shutdown error")
		}

		close(done)
	}()

	// Start server
	log.Info().Str("listen", cfg.Server.Listen).Msg("starting cc-relay")

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("server error")
		return err
	}

	<-done
	log.Info().Msg("server stopped")

	return nil
}

// findConfigFile searches for config.yaml in default locations.
//
//nolint:goconst // config.yaml constant would be shared across subcommands
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
