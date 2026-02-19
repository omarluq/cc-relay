// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/omarluq/cc-relay/internal/auth"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/rs/zerolog"
)

const authSucceededMsg = "authentication succeeded"

// AuthMiddleware creates middleware that validates x-api-key header.
// Uses constant-time comparison to prevent timing attacks.
//
// Security note: SHA-256 is appropriate for API key hashing because:
// - API keys are high-entropy secrets (32+ random characters), not passwords
// - SHA-256 provides sufficient pre-image resistance for high-entropy inputs
// - Pre-hashing at middleware creation prevents per-request hash computation
// - Constant-time comparison (subtle.ConstantTimeCompare) prevents timing attacks.
func AuthMiddleware(expectedAPIKey string) func(http.Handler) http.Handler {
	// Pre-hash expected key at creation time (not per-request)
	expectedHash := sha256.Sum256([]byte(expectedAPIKey))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			providedKey := request.Header.Get("x-api-key")

			if providedKey == "" {
				failAuth(writer, request, "missing x-api-key header")
				return
			}

			providedHash := sha256.Sum256([]byte(providedKey))

			// CRITICAL: Constant-time comparison prevents timing attacks
			if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
				failAuth(writer, request, "invalid x-api-key")
				return
			}

			zerolog.Ctx(request.Context()).Debug().Msg(authSucceededMsg)
			next.ServeHTTP(writer, request)
		})
	}
}

func failAuth(writer http.ResponseWriter, request *http.Request, reason string) {
	zerolog.Ctx(request.Context()).Warn().Msg("authentication failed: " + reason)
	WriteError(writer, http.StatusUnauthorized, "authentication_error", reason)
}

func handleAuthResult(ctx context.Context, writer http.ResponseWriter, result auth.Result) bool {
	if !result.Valid {
		zerolog.Ctx(ctx).Warn().
			Str("auth_type", string(result.Type)).
			Str("error", result.Error).
			Msg("authentication failed")
		WriteError(writer, http.StatusUnauthorized, "authentication_error", result.Error)
		return false
	}

	zerolog.Ctx(ctx).Debug().
		Str("auth_type", string(result.Type)).
		Msg(authSucceededMsg)
	return true
}

// MultiAuthMiddleware creates middleware supporting multiple authentication methods.
// Supports both x-api-key and Authorization: Bearer token authentication.
// If authConfig has no methods enabled, all requests pass through.
func MultiAuthMiddleware(authConfig *config.AuthConfig) func(http.Handler) http.Handler {
	// Build the authenticator chain based on config
	var authenticators []auth.Authenticator

	// Bearer token auth (checked first as it's more specific)
	// IsBearerEnabled() returns true for both AllowBearer and AllowSubscription
	if authConfig.IsBearerEnabled() {
		authenticators = append(authenticators, auth.NewBearerAuthenticator(authConfig.BearerSecret))
	}

	// API key auth
	if authConfig.APIKey != "" {
		authenticators = append(authenticators, auth.NewAPIKeyAuthenticator(authConfig.APIKey))
	}

	// If no auth configured, return pass-through middleware
	if len(authenticators) == 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	chainAuth := auth.NewChainAuthenticator(authenticators...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			result := chainAuth.Validate(request)

			if !handleAuthResult(request.Context(), writer, result) {
				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}

// DebugOptionsProvider returns current debug options for live-config logging.
type DebugOptionsProvider func() config.DebugOptions

func withRequestFields(ctx context.Context, r *http.Request, shortID string) zerolog.Context {
	return zerolog.Ctx(ctx).With().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("req_id", shortID)
}

func logRequestStart(ctx context.Context, request *http.Request, shortID string, debugOpts config.DebugOptions) {
	logger := withRequestFields(ctx, request, shortID).Logger()
	logEvent := logger.Info()

	if zerolog.GlobalLevel() <= zerolog.DebugLevel && debugOpts.LogRequestBody {
		bodyPreview := getBodyPreview(request)
		if bodyPreview != "" {
			logEvent = logEvent.Str("body_preview", bodyPreview)
		}
	}

	logEvent.Msgf("%s %s", request.Method, request.URL.Path)
}

func logRequestCompletion(
	ctx context.Context,
	request *http.Request,
	wrapped *responseWriter,
	duration time.Duration,
	shortID string,
) {
	durationStr := formatDuration(duration)
	statusMsg := statusSymbol(wrapped.statusCode)
	completionMsg := formatCompletionMessage(wrapped.statusCode, statusMsg, durationStr)

	logCtx := withRequestFields(ctx, request, shortID).
		Int("status", wrapped.statusCode).
		Str("duration", durationStr)

	if timings := getRequestTimings(ctx); timings != nil {
		addDurationFieldsCtx(&logCtx, "auth_time", timings.Auth)
		addDurationFieldsCtx(&logCtx, "route_time", timings.Routing)
	}

	if wrapped.isStreaming && wrapped.sseEvents > 0 {
		logCtx = logCtx.Int("sse_events", wrapped.sseEvents)
	}

	logger := logCtx.Logger()
	switch {
	case wrapped.statusCode >= 500:
		logger.Error().Msg(completionMsg)
	case wrapped.statusCode >= 400:
		logger.Warn().Msg(completionMsg)
	default:
		logger.Info().Msg(completionMsg)
	}
}

func statusSymbol(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "✗"
	case statusCode >= 400:
		return "⚠"
	default:
		return "✓"
	}
}

// LoggingMiddlewareWithProvider logs each request using live debug options.
func LoggingMiddlewareWithProvider(provider DebugOptionsProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			debugOpts := config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     0,
			}
			if provider != nil {
				debugOpts = provider()
			}

			start := time.Now()

			// Log request details in debug mode
			LogRequestDetails(request.Context(), request, debugOpts)

			// Attach timings container for downstream middleware/handlers
			ctx, _ := withRequestTimings(request.Context())
			request = request.WithContext(ctx)

			// Wrap ResponseWriter to capture status code
			wrapped := &responseWriter{
				ResponseWriter: writer,
				statusCode:     http.StatusOK,
				sseEvents:      0,
				isStreaming:     false,
			}

			// Get request ID for logging
			requestID := GetRequestID(request.Context())
			shortID := requestID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}

			logRequestStart(request.Context(), request, shortID, debugOpts)

			// Serve request
			next.ServeHTTP(wrapped, request)

			logRequestCompletion(request.Context(), request, wrapped, time.Since(start), shortID)
		})
	}
}

// LoggingMiddleware logs each request with method, path, and duration.
// If debugOpts has debug logging enabled, logs additional request/response details.
func LoggingMiddleware(debugOpts config.DebugOptions) func(http.Handler) http.Handler {
	return LoggingMiddlewareWithProvider(func() config.DebugOptions { return debugOpts })
}

type authCache struct {
	chain       auth.Authenticator // 16 bytes (interface)
	fingerprint string             // 16 bytes (string header)
}

type authCacheStore struct {
	cache atomic.Value
	mu    sync.Mutex
}

func (s *authCacheStore) cached(fingerprint string) *authCache {
	if v := s.cache.Load(); v != nil {
		if c, ok := v.(*authCache); ok && c.fingerprint == fingerprint {
			return c
		}
	}
	return nil
}

func buildAuthCache(fingerprint string, authConfig config.AuthConfig, effectiveKey string) *authCache {
	var authenticators []auth.Authenticator
	if authConfig.IsBearerEnabled() {
		authenticators = append(authenticators, auth.NewBearerAuthenticator(authConfig.BearerSecret))
	}
	if effectiveKey != "" {
		authenticators = append(authenticators, auth.NewAPIKeyAuthenticator(effectiveKey))
	}

	var chain auth.Authenticator
	switch len(authenticators) {
	case 1:
		chain = authenticators[0]
	case 0:
		chain = nil
	default:
		chain = auth.NewChainAuthenticator(authenticators...)
	}

	return &authCache{
		fingerprint: fingerprint,
		chain:       chain,
	}
}

func (s *authCacheStore) getOrBuild(
	fingerprint string,
	authConfig config.AuthConfig,
	effectiveKey string,
) *authCache {
	// Fast path: check cache without lock.
	if c := s.cached(fingerprint); c != nil {
		return c
	}

	// Slow path: acquire lock and rebuild.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring lock.
	if c := s.cached(fingerprint); c != nil {
		return c
	}

	cache := buildAuthCache(fingerprint, authConfig, effectiveKey)
	s.cache.Store(cache)
	return cache
}

// authFingerprint computes a small fingerprint of auth-related config fields.
// This avoids relying on config pointer equality for cache invalidation.
// Uses length-prefixed format to avoid delimiter collision vulnerabilities.
func authFingerprint(bearerEnabled bool, bearerSecret, apiKey string) string {
	// Format: "b<0|1>|<len>:<bearerSecret>|<len>:<apiKey>"
	// Length-prefix prevents collision when secrets contain delimiters.
	bearerByte := byte('0')
	if bearerEnabled {
		bearerByte = '1'
	}

	buffer := make([]byte, 0, 8+len(bearerSecret)+len(apiKey))
	buffer = append(buffer, 'b', bearerByte, '|')
	buffer = strconv.AppendInt(buffer, int64(len(bearerSecret)), 10)
	buffer = append(buffer, ':')
	buffer = append(buffer, bearerSecret...)
	buffer = append(buffer, '|')
	buffer = strconv.AppendInt(buffer, int64(len(apiKey)), 10)
	buffer = append(buffer, ':')
	buffer = append(buffer, apiKey...)
	return string(buffer)
}

func getRuntimeConfig(cfgProvider config.RuntimeConfigGetter) *config.Config {
	if cfgProvider == nil {
		return nil
	}
	return cfgProvider.Get()
}

func recordAuthTiming(ctx context.Context, start time.Time) {
	if timings := getRequestTimings(ctx); timings != nil {
		timings.Auth = time.Since(start)
	}
}

// LiveAuthMiddleware creates middleware that enforces auth based on live config.
// It rebuilds the authenticator chain when auth-related config values change.
func LiveAuthMiddleware(cfgProvider config.RuntimeConfigGetter) func(http.Handler) http.Handler {
	store := &authCacheStore{
		cache: atomic.Value{},
		mu:    sync.Mutex{},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			cfg := getRuntimeConfig(cfgProvider)
			if cfg == nil {
				next.ServeHTTP(writer, request)
				return
			}

			authConfig := cfg.Server.Auth
			effectiveKey := cfg.Server.GetEffectiveAPIKey()
			fpValue := authFingerprint(authConfig.IsBearerEnabled(), authConfig.BearerSecret, effectiveKey)

			start := time.Now()
			cached := store.getOrBuild(fpValue, authConfig, effectiveKey)
			recordAuthTiming(request.Context(), start)

			if cached.chain == nil {
				next.ServeHTTP(writer, request)
				return
			}

			result := cached.chain.Validate(request)
			if !handleAuthResult(request.Context(), writer, result) {
				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}

// RequestIDMiddleware adds X-Request-ID header and logger with request ID to context.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			// Extract or generate request ID
			requestID := request.Header.Get("X-Request-ID")
			ctx := AddRequestID(request.Context(), requestID)

			// Write request ID to response header
			if requestID == "" {
				requestID = GetRequestID(ctx)
			}

			writer.Header().Set("X-Request-ID", requestID)

			// Attach logger to request
			request = request.WithContext(ctx)

			next.ServeHTTP(writer, request)
		})
	}
}

// formatDuration formats duration in a human-readable form with microsecond precision.
// Uses dynamic units so very fast requests show in µs while longer ones show in ms/s.
func formatDuration(duration time.Duration) string {
	if duration <= 0 {
		return "0s"
	}
	duration = duration.Round(time.Microsecond)
	switch {
	case duration < time.Millisecond:
		return fmt.Sprintf("%dµs", duration.Microseconds())
	case duration < time.Second:
		return fmt.Sprintf("%.2fms", float64(duration)/float64(time.Millisecond))
	case duration < time.Minute:
		return fmt.Sprintf("%.2fs", duration.Seconds())
	default:
		return duration.Truncate(time.Second).String()
	}
}

// formatCompletionMessage formats the completion message with status.
func formatCompletionMessage(status int, symbol, duration string) string {
	return symbol + " " + http.StatusText(status) + " (" + duration + ")"
}

// getBodyPreview reads the first 200 characters of the request body.
// Redacts any "api_key" or "key" fields to prevent logging sensitive data.
// Returns empty string if body cannot be read or is empty.
func getBodyPreview(request *http.Request) string {
	if request.Body == nil {
		return ""
	}

	// Read body (limited to 500 bytes for safety)
	body, err := io.ReadAll(io.LimitReader(request.Body, 500))
	if err != nil || len(body) == 0 {
		return ""
	}

	// Restore body for downstream handlers
	request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), request.Body))

	// Convert to string and truncate to 200 chars
	preview := string(body)
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	// Redact sensitive fields using regex
	preview = redactSensitiveFields(preview)

	return preview
}

// Note: redactSensitiveFields is defined in debug.go and used here via getBodyPreview

// responseWriter wraps http.ResponseWriter to capture status code and SSE events.
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	sseEvents   int
	isStreaming bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	// Check if this is a streaming response
	if rw.Header().Get("Content-Type") == providers.ContentTypeSSE {
		rw.isStreaming = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write intercepts writes to count SSE events.
func (rw *responseWriter) Write(data []byte) (int, error) {
	// Count SSE events if streaming
	if rw.isStreaming {
		// Count occurrences of "event:" prefix in the data
		dataStr := string(data)
		for i := 0; i < len(dataStr); i++ {
			if i+6 <= len(dataStr) && dataStr[i:i+6] == "event:" {
				rw.sseEvents++
			}
		}
	}
	return rw.ResponseWriter.Write(data)
}

// ConcurrencyLimiter enforces a global maximum number of concurrent requests.
// It uses an atomic counter with a configurable limit that supports hot-reload.
// When the limit is reached, new requests receive 503 Service Unavailable.
type ConcurrencyLimiter struct {
	limit   atomic.Int64
	current atomic.Int64
}

// NewConcurrencyLimiter creates a new concurrency limiter with the given max limit.
// A limit of 0 or negative means unlimited.
func NewConcurrencyLimiter(maxLimit int64) *ConcurrencyLimiter {
	limiter := &ConcurrencyLimiter{
		limit:   atomic.Int64{},
		current: atomic.Int64{},
	}
	limiter.limit.Store(maxLimit)
	return limiter
}

// SetLimit updates the concurrency limit for hot-reload support.
// A limit of 0 or negative means unlimited.
func (l *ConcurrencyLimiter) SetLimit(maxLimit int64) {
	l.limit.Store(maxLimit)
}

// GetLimit returns the current configured limit.
func (l *ConcurrencyLimiter) GetLimit() int64 {
	return l.limit.Load()
}

// CurrentInFlight returns the current number of in-flight requests.
func (l *ConcurrencyLimiter) CurrentInFlight() int64 {
	return l.current.Load()
}

// TryAcquire attempts to acquire a slot for a request.
// Returns true if the request can proceed, false if the limit is reached.
// If limit is 0 or negative, always returns true (unlimited).
func (l *ConcurrencyLimiter) TryAcquire() bool {
	limit := l.limit.Load()
	if limit <= 0 {
		// Unlimited - always succeed but still track count
		l.current.Add(1)
		return true
	}

	// Try to increment if below limit using compare-and-swap loop
	for {
		current := l.current.Load()
		if current >= limit {
			return false
		}
		if l.current.CompareAndSwap(current, current+1) {
			return true
		}
		// CAS failed, retry
	}
}

// Release releases a slot after request completion.
// Must be called after a successful TryAcquire.
func (l *ConcurrencyLimiter) Release() {
	l.current.Add(-1)
}

// ConcurrencyMiddleware creates middleware that enforces a global concurrency limit.
// Uses the provided ConcurrencyLimiter which supports hot-reload via SetLimit.
func ConcurrencyMiddleware(limiter *ConcurrencyLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if !limiter.TryAcquire() {
				zerolog.Ctx(request.Context()).Warn().
					Int64("limit", limiter.GetLimit()).
					Int64("current", limiter.CurrentInFlight()).
					Msg("request rejected: concurrency limit reached")
				WriteError(writer, http.StatusServiceUnavailable, "server_busy",
					"server is at maximum capacity, please retry later")
				return
			}
			defer limiter.Release()
			next.ServeHTTP(writer, request)
		})
	}
}

// MaxBodyBytesMiddleware creates middleware that limits request body size.
// Uses http.MaxBytesReader to enforce the limit efficiently.
// The limitProvider is called per-request to support hot-reload.
func MaxBodyBytesMiddleware(limitProvider func() int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			limit := limitProvider()
			if limit > 0 && request.Body != nil {
				request.Body = http.MaxBytesReader(writer, request.Body, limit)
			}
			next.ServeHTTP(writer, request)
		})
	}
}
