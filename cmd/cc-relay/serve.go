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

	"github.com/omarluq/cc-relay/internal/di"
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

func runServe(_ *cobra.Command, _ []string) error {
	// Determine config path
	configPath := cfgFile
	if configPath == "" {
		configPath = findConfigFile()
	}

	// Create DI container with all services
	container, err := di.NewContainer(configPath)
	if err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("failed to initialize services")
		return err
	}

	// Get config service to apply CLI overrides and setup logging
	cfgSvc := di.MustInvoke[*di.ConfigService](container)
	cfg := cfgSvc.Get()

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
		log.Fatal().Err(err).Msg("failed to initialize logger")
	}

	log.Logger = logger
	zerolog.DefaultContextLogger = &logger

	// Get server from DI container (lazy initialization of all dependencies)
	serverSvc, err := di.Invoke[*di.ServerService](container)
	if err != nil {
		log.Error().Err(err).Msg("failed to create server")
		return err
	}

	// Start health checker (after all DI services initialized)
	checkerSvc := di.MustInvoke[*di.CheckerService](container)
	checkerSvc.Checker.Start()

	// Start config file watcher for hot-reload support
	// The watcher context is tied to container shutdown
	watchCtx, watchCancel := context.WithCancel(context.Background())
	cfgSvc.StartWatching(watchCtx)

	// Run server with graceful shutdown (passes watchCancel for cleanup)
	return runWithGracefulShutdown(serverSvc.Server, container, cfg.Server.Listen, watchCancel)
}

// runWithGracefulShutdown handles signal-based graceful shutdown.
// The watchCancel function is called to stop the config watcher before container shutdown.
func runWithGracefulShutdown(
	server *proxy.Server,
	container *di.Container,
	listenAddr string,
	watchCancel context.CancelFunc,
) error {
	// Graceful shutdown on SIGINT/SIGTERM
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Info().Msg("shutting down...")

		// Cancel config watcher first (stops file watching goroutine)
		if watchCancel != nil {
			watchCancel()
		}

		// Shutdown server (drain connections)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("server shutdown error")
		}

		// Then shutdown all DI container services (cache, watcher, etc.)
		if err := container.ShutdownWithContext(ctx); err != nil {
			log.Error().Err(err).Msg("service shutdown error")
		}

		close(done)
	}()

	// Start server
	log.Info().Str("listen", listenAddr).Msg("starting cc-relay")

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
