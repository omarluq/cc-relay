package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptrace"
	"regexp"
	"strings"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// Sensitive patterns to redact from request bodies.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`"api_key"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`"x-api-key"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`"bearer"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`"password"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`"token"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`"secret"\s*:\s*"[^"]+"`),
	regexp.MustCompile(`"authorization"\s*:\s*"[^"]+"`),
}

// TLSMetrics holds TLS connection timing and metadata.
type TLSMetrics struct {
	Version     string
	DNSTime     time.Duration
	ConnectTime time.Duration
	TLSTime     time.Duration
	Reused      bool
	HasMetrics  bool
}

// Metrics holds proxy-level performance metrics.
type Metrics struct {
	BackendTime     time.Duration
	TotalTime       time.Duration
	BytesSent       int64
	BytesReceived   int64
	StreamingEvents int
}

// LogRequestDetails logs request body and headers in debug mode.
// Respects DebugOptions.LogRequestBody and MaxBodyLogSize.
func LogRequestDetails(ctx context.Context, r *http.Request, opts config.DebugOptions) {
	if !opts.LogRequestBody {
		return
	}

	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() > zerolog.DebugLevel {
		return
	}

	bodyBytes := readAndRestoreBody(r, logger)
	if bodyBytes == nil {
		return
	}

	// Truncate to max size
	bodyBytes = truncateBody(bodyBytes, opts.GetMaxBodyLogSize())

	// Parse JSON to extract model and tokens if present
	model, maxTokens := extractModelInfo(bodyBytes)

	// Redact sensitive fields
	bodyStr := redactSensitiveFields(string(bodyBytes))

	// Log with context
	logRequestBody(logger, r.Header.Get("Content-Type"), bodyBytes, model, maxTokens, bodyStr)
}

// readAndRestoreBody reads the request body and restores it for downstream handlers.
func readAndRestoreBody(r *http.Request, logger *zerolog.Logger) []byte {
	if r.Body == nil {
		return nil
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Debug().Err(err).Msg("failed to read request body")
		return nil
	}

	// Restore body for downstream handlers
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes
}

// truncateBody truncates body to max size.
func truncateBody(body []byte, maxSize int) []byte {
	if len(body) > maxSize {
		return body[:maxSize]
	}
	return body
}

// extractModelInfo parses JSON body to extract model and max_tokens.
func extractModelInfo(bodyBytes []byte) (model string, maxTokens int) {
	if len(bodyBytes) == 0 {
		return "", 0
	}

	var bodyMap map[string]interface{}
	if json.Unmarshal(bodyBytes, &bodyMap) != nil {
		return "", 0
	}

	if m, ok := bodyMap["model"].(string); ok {
		model = m
	}
	if mt, ok := bodyMap["max_tokens"].(float64); ok {
		maxTokens = int(mt)
	}
	return model, maxTokens
}

// redactSensitiveFields redacts sensitive information from body string.
func redactSensitiveFields(body string) string {
	return lo.Reduce(sensitivePatterns, func(s string, pattern *regexp.Regexp, _ int) string {
		return pattern.ReplaceAllString(s, `"***":"REDACTED"`)
	}, body)
}

// logRequestBody logs the request body with extracted metadata.
func logRequestBody(
	logger *zerolog.Logger, contentType string, body []byte, model string, maxTokens int, bodyStr string,
) {
	logEvent := logger.Debug().
		Str("content_type", contentType).
		Int("body_length", len(body))

	if model != "" {
		logEvent.Str("model", model)
	}
	if maxTokens > 0 {
		logEvent.Int("max_tokens", maxTokens)
	}
	if len(body) > 0 {
		logEvent.Str("body_preview", bodyStr)
	}

	logEvent.Msg("request details")
}

// LogResponseDetails logs response headers and streaming event count in debug mode.
func LogResponseDetails(
	ctx context.Context,
	headers http.Header,
	statusCode, eventCount int,
	opts config.DebugOptions,
) {
	if !opts.LogResponseHeaders {
		return
	}

	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() > zerolog.DebugLevel {
		return
	}

	// Extract usage tokens from headers if present
	usageTokens := headers.Get("X-Anthropic-Usage")
	contentType := headers.Get("Content-Type")

	logEvent := logger.Debug().
		Int("status", statusCode).
		Str("content_type", contentType)

	if usageTokens != "" {
		logEvent.Str("usage_tokens", usageTokens)
	}

	// If SSE streaming, log event count
	if strings.Contains(contentType, providers.ContentTypeSSE) && eventCount > 0 {
		logEvent.Int("streaming_events", eventCount)
	}

	// Log selected headers (not all - too verbose)
	importantHeaders := []string{
		"X-Anthropic-Model",
		"X-Anthropic-Stop-Reason",
		"X-Request-Id",
		"Cache-Control",
	}
	headerData := lo.SliceToMap(
		lo.FilterMap(importantHeaders, func(key string, _ int) (lo.Entry[string, string], bool) {
			val := headers.Get(key)
			return lo.Entry[string, string]{Key: key, Value: val}, val != ""
		}),
		func(entry lo.Entry[string, string]) (string, string) {
			return entry.Key, entry.Value
		},
	)
	if len(headerData) > 0 {
		logEvent.Interface("headers", headerData)
	}

	logEvent.Msg("response details")
}

// LogTLSMetrics logs TLS connection metrics in debug mode.
func LogTLSMetrics(ctx context.Context, metrics TLSMetrics, opts config.DebugOptions) {
	if !opts.LogTLSMetrics || !metrics.HasMetrics {
		return
	}

	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() > zerolog.DebugLevel {
		return
	}

	logEvent := logger.Debug().
		Str("tls_version", metrics.Version).
		Bool("tls_reused", metrics.Reused)
	logEvent = addDurationFields(logEvent, "dns_time", metrics.DNSTime)
	logEvent = addDurationFields(logEvent, "connect_time", metrics.ConnectTime)
	logEvent = addDurationFields(logEvent, "tls_handshake", metrics.TLSTime)
	logEvent.Msg("tls metrics")
}

// LogProxyMetrics logs proxy-level performance metrics in debug mode.
func LogProxyMetrics(ctx context.Context, metrics Metrics, _ config.DebugOptions) {
	// Always log proxy metrics if debug level, regardless of specific flag
	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() > zerolog.DebugLevel {
		return
	}

	logEvent := logger.Debug()
	logEvent = addDurationFields(logEvent, "backend_time", metrics.BackendTime)
	logEvent = addDurationFields(logEvent, "total_time", metrics.TotalTime)

	if metrics.BytesSent > 0 {
		logEvent.Int64("bytes_sent", metrics.BytesSent)
	}
	if metrics.BytesReceived > 0 {
		logEvent.Int64("bytes_received", metrics.BytesReceived)
	}
	if metrics.StreamingEvents > 0 {
		logEvent.Int("streaming_events", metrics.StreamingEvents)
	}

	logEvent.Msg("proxy metrics")
}

// AttachTLSTrace attaches httptrace to request for TLS metric collection.
// Returns updated context with trace and a function to retrieve metrics.
//
//nolint:gocritic // unnamedResult: return values are clear from function signature
func AttachTLSTrace(ctx context.Context, _ *http.Request) (context.Context, func() TLSMetrics) {
	metrics := &TLSMetrics{}
	var dnsStart, connectStart, tlsStart time.Time

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				metrics.DNSTime = time.Since(dnsStart)
			}
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			if !connectStart.IsZero() {
				metrics.ConnectTime = time.Since(connectStart)
			}
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, _ error) {
			if !tlsStart.IsZero() {
				metrics.TLSTime = time.Since(tlsStart)
			}
			metrics.Version = tlsVersionString(state.Version)
			metrics.Reused = state.DidResume
			metrics.HasMetrics = true
		},
	}

	newCtx := httptrace.WithClientTrace(ctx, trace)
	return newCtx, func() TLSMetrics { return *metrics }
}

// tlsVersionString converts TLS version constant to string.
func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}
