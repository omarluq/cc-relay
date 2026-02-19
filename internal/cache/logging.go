package cache

import (
	"sync"

	"github.com/rs/zerolog"
)

var (
	// loggerMu protects Logger from concurrent access in tests.
	loggerMu sync.RWMutex

	// Logger is the package-level logger for cache operations.
	// Uses a no-op logger by default to avoid logging until explicitly configured.
	// The logger is tagged with component: cache for easy filtering.
	Logger = zerolog.Nop()
)

// SetLogger sets the package-level logger for cache operations.
// Call this during application initialization to enable cache logging.
// The logger is automatically tagged with component: cache.
//
// Example:
//
//	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
//	cache.SetLogger(&logger)
func SetLogger(l *zerolog.Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	Logger = l.With().Str("component", "cache").Logger()
}

// logger returns the current package logger.
// This is used internally by cache implementations.
func logger() zerolog.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return Logger
}
