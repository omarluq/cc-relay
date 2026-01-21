package proxy

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/omarluq/cc-relay/internal/config"
)

type ctxKey string

// RequestIDKey is the context key for request IDs.
const RequestIDKey ctxKey = "request_id"

// NewLogger creates a zerolog.Logger from LoggingConfig.
// Returns a configured logger ready for use as global logger.
//
//nolint:gocyclo // logger setup has necessary branching for format/output options
func NewLogger(cfg config.LoggingConfig) (zerolog.Logger, error) {
	// Determine output writer
	var output io.Writer
	var outputFile *os.File

	switch cfg.Output {
	case "", "stdout":
		output = os.Stdout
		outputFile = os.Stdout
	case "stderr":
		output = os.Stderr
		outputFile = os.Stderr
	default:
		// File output
		f, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return zerolog.Logger{}, err
		}

		output = f
		outputFile = f
	}

	// Determine if we should use pretty console output
	usePretty := false

	switch cfg.Format {
	case "pretty":
		usePretty = true
	case "console":
		// Auto-detect: use pretty if stdout is a terminal
		if outputFile != nil {
			usePretty = isatty.IsTerminal(outputFile.Fd())
		}
	case "json":
		usePretty = false
	default:
		// Default: auto-detect terminal
		if outputFile != nil {
			usePretty = isatty.IsTerminal(outputFile.Fd())
		}
	}

	// Override with explicit Pretty flag if set
	if cfg.Pretty {
		usePretty = true
	}

	// Apply format
	if usePretty {
		// Use custom ConsoleWriter for beautiful output
		consoleWriter := zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "15:04:05",
			NoColor:    false,
			// Custom format parts
			FormatLevel: func(i interface{}) string {
				var levelStr string
				if ll, ok := i.(string); ok {
					switch ll {
					case "debug":
						levelStr = "\033[36mDBG\033[0m" // Cyan
					case "info":
						levelStr = "\033[32mINF\033[0m" // Green
					case "warn":
						levelStr = "\033[33mWRN\033[0m" // Yellow
					case "error":
						levelStr = "\033[31mERR\033[0m" // Red
					case "fatal":
						levelStr = "\033[35mFTL\033[0m" // Magenta
					case "panic":
						levelStr = "\033[35mPNC\033[0m" // Magenta
					default:
						levelStr = ll
					}
				}
				return levelStr
			},
			FormatMessage: func(i interface{}) string {
				if i == nil {
					return ""
				}
				return fmt.Sprintf("→ %s", i)
			},
			FormatFieldName: func(i interface{}) string {
				return fmt.Sprintf("\033[2m%s=\033[0m", i) // Dim
			},
			FormatFieldValue: func(i interface{}) string {
				return fmt.Sprintf("%s", i)
			},
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
