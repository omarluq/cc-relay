package proxy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
func NewLogger(cfg config.LoggingConfig) (zerolog.Logger, error) {
	// Select output destination
	output, outputFile, err := selectOutput(cfg.Output)
	if err != nil {
		return zerolog.Logger{}, err
	}

	// Apply pretty console formatting if needed
	if shouldUsePretty(cfg, outputFile) {
		output = buildConsoleWriter(output)
	}

	// Create logger with level
	logger := zerolog.New(output).
		Level(cfg.ParseLevel()).
		With().
		Timestamp().
		Logger()

	return logger, nil
}

// selectOutput returns the output writer and file handle for the given output config.
func selectOutput(outputCfg string) (io.Writer, *os.File, error) {
	switch outputCfg {
	case "", "stdout":
		return os.Stdout, os.Stdout, nil
	case "stderr":
		return os.Stderr, os.Stderr, nil
	default:
		// File output - validate and clean the path
		outputCfg = filepath.Clean(outputCfg)
		f, err := os.OpenFile(outputCfg, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, err
		}
		return f, f, nil
	}
}

// shouldUsePretty determines if pretty console output should be used.
func shouldUsePretty(cfg config.LoggingConfig, outputFile *os.File) bool {
	// Explicit Pretty flag always wins
	if cfg.Pretty {
		return true
	}

	switch cfg.Format {
	case "pretty":
		return true
	case "json":
		return false
	case "console":
		// Auto-detect: use pretty if stdout is a terminal
		return outputFile != nil && isatty.IsTerminal(outputFile.Fd())
	default:
		// Default: auto-detect terminal
		return outputFile != nil && isatty.IsTerminal(outputFile.Fd())
	}
}

// buildConsoleWriter creates a zerolog.ConsoleWriter with custom formatting.
func buildConsoleWriter(output io.Writer) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:             output,
		TimeFormat:      "15:04:05",
		NoColor:         false,
		FormatLevel:     formatLevel,
		FormatMessage:   formatMessage,
		FormatFieldName: formatFieldName,
		FormatFieldValue: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
	}
}

// formatLevel formats log level with ANSI colors.
func formatLevel(i interface{}) string {
	levelStr, ok := i.(string)
	if !ok {
		return ""
	}

	levelColors := map[string]string{
		"debug": "\033[36mDBG\033[0m", // Cyan
		"info":  "\033[32mINF\033[0m", // Green
		"warn":  "\033[33mWRN\033[0m", // Yellow
		"error": "\033[31mERR\033[0m", // Red
		"fatal": "\033[35mFTL\033[0m", // Magenta
		"panic": "\033[35mPNC\033[0m", // Magenta
	}

	if colored, exists := levelColors[levelStr]; exists {
		return colored
	}
	return levelStr
}

// formatMessage formats log message with arrow prefix.
func formatMessage(i interface{}) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("-> %s", i)
}

// formatFieldName formats field names with dim styling.
func formatFieldName(i interface{}) string {
	return fmt.Sprintf("\033[2m%s=\033[0m", i) // Dim
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
