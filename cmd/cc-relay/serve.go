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

var (
	logLevel  string
	logFormat string
	debugMode bool
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

	// Add logging flags
	serveCmd.Flags().StringVar(&logLevel, "log-level", "",
		"log level (debug, info, warn, error) - overrides config")
	serveCmd.Flags().StringVar(&logFormat, "log-format", "",
		"log format (json, pretty) - overrides config")
	serveCmd.Flags().BoolVar(&debugMode, "debug", false,
		"enable debug mode (sets log level to debug and enables all debug options)")
}

//nolint:gocognit,gocyclo // main server startup has necessary complexity
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

	// Apply CLI flag overrides to logging config
	if debugMode {
		cfg.Logging.EnableAllDebugOptions()
		log.Info().Msg("debug mode enabled via --debug flag")
	}

	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}

	if logFormat != "" {
		cfg.Logging.Format = logFormat
	}

	// Setup logging from config
	logger, err := proxy.NewLogger(cfg.Logging)
	if err != nil {
		// Fallback to console logger for error reporting
		log.Fatal().Err(err).Msg("failed to initialize logger")
	}

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger

	// Collect all enabled providers
	var allProviders []providers.Provider
	var primaryProvider providers.Provider
	var providerKey string

	for _, p := range cfg.Providers {
		if !p.Enabled {
			continue
		}

		var prov providers.Provider
		switch p.Type {
		case "anthropic":
			prov = providers.NewAnthropicProviderWithModels(p.Name, p.BaseURL, p.Models)
		case "zai":
			prov = providers.NewZAIProviderWithModels(p.Name, p.BaseURL, p.Models)
		default:
			continue
		}

		allProviders = append(allProviders, prov)

		// First enabled provider becomes the primary (for routing requests)
		if primaryProvider == nil {
			primaryProvider = prov
			if len(p.Keys) > 0 {
				providerKey = p.Keys[0].Key
			}
			log.Info().
				Str("provider", p.Name).
				Str("type", p.Type).
				Msg("using primary provider")
		} else {
			log.Info().
				Str("provider", p.Name).
				Str("type", p.Type).
				Int("models", len(p.Models)).
				Msg("registered provider for /v1/models")
		}
	}

	if primaryProvider == nil {
		log.Error().Msg("no enabled provider found in config (supported types: anthropic, zai)")
		return errors.New("no enabled provider in config")
	}

	// Setup routes with all providers for /v1/models endpoint
	handler, err := proxy.SetupRoutesWithProviders(cfg, primaryProvider, providerKey, allProviders)
	if err != nil {
		log.Error().Err(err).Msg("failed to setup routes")
		return err
	}

	// Create server
	server := proxy.NewServer(cfg.Server.Listen, handler, cfg.Server.EnableHTTP2)

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
func findConfigFile() string {
	// Check current directory
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}
	// Check ~/.config/cc-relay/
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		p := filepath.Join(home, ".config", "cc-relay", defaultConfigFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return defaultConfigFile // Default, will error if not found
}
