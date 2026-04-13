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

// logger returns the current package logger.
// This is used internally by cache implementations.
func logger() zerolog.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return Logger
}
