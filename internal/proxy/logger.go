package proxy

import (
	"context"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/omarluq/cc-relay/internal/config"
)

// RequestIDKey is the context key for request IDs.
type ctxKey string

const RequestIDKey ctxKey = "request_id"

// NewLogger creates a zerolog.Logger from LoggingConfig.
// Returns a configured logger ready for use as global logger.
func NewLogger(cfg config.LoggingConfig) (zerolog.Logger, error) {
	// Determine output writer
	var output io.Writer

	switch cfg.Output {
	case "", "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// File output
		f, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return zerolog.Logger{}, err
		}

		output = f
	}

	// Apply format
	if cfg.Format == "console" || cfg.Pretty {
		// Use ConsoleWriter for human-readable output
		consoleWriter := zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "15:04:05",
			NoColor:    !cfg.Pretty,
		}
		output = consoleWriter
	}

	// Create logger with level
	logger := zerolog.New(output).
		Level(cfg.ParseLevel()).
		With().
		Timestamp().
		Logger()

	return logger, nil
}

// AddRequestID adds or extracts request ID from request headers and adds it to the context.
// If X-Request-ID header exists, use it. Otherwise, generate a new UUID.
func AddRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Add to context for retrieval
	ctx = context.WithValue(ctx, RequestIDKey, requestID)

	// Add to zerolog context
	logger := log.Ctx(ctx).With().Str("request_id", requestID).Logger()

	return logger.WithContext(ctx)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}

	return ""
}
