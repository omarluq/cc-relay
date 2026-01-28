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
	// codeql[go/weak-sensitive-data-hashing] SHA-256 is appropriate for high-entropy API keys (not passwords)
	// #nosec G401 -- SHA-256 is appropriate for high-entropy API keys (not passwords)
	expectedHash := sha256.Sum256([]byte(expectedAPIKey))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get("x-api-key")

			if providedKey == "" {
				zerolog.Ctx(r.Context()).Warn().Msg("authentication failed: missing x-api-key header")
				WriteError(w, http.StatusUnauthorized, "authentication_error", "missing x-api-key header")

				return
			}

			// codeql[go/weak-sensitive-data-hashing] SHA-256 is appropriate for high-entropy API keys (not passwords)
			// #nosec G401 -- SHA-256 is appropriate for high-entropy API keys (not passwords)
			providedHash := sha256.Sum256([]byte(providedKey))

			// CRITICAL: Constant-time comparison prevents timing attacks
			if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
				zerolog.Ctx(r.Context()).Warn().Msg("authentication failed: invalid x-api-key")
				WriteError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")

				return
			}

			zerolog.Ctx(r.Context()).Debug().Msg("authentication succeeded")
			next.ServeHTTP(w, r)
		})
	}
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
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result := chainAuth.Validate(r)

			if !result.Valid {
				zerolog.Ctx(r.Context()).Warn().
					Str("auth_type", string(result.Type)).
					Str("error", result.Error).
					Msg("authentication failed")
				WriteError(w, http.StatusUnauthorized, "authentication_error", result.Error)

				return
			}

			zerolog.Ctx(r.Context()).Debug().
				Str("auth_type", string(result.Type)).
				Msg("authentication succeeded")
			next.ServeHTTP(w, r)
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

func logRequestStart(ctx context.Context, r *http.Request, shortID string) {
	logger := withRequestFields(ctx, r, shortID).Logger()
	logEvent := logger.Info()

	if zerolog.GlobalLevel() <= zerolog.DebugLevel {
		bodyPreview := getBodyPreview(r)
		if bodyPreview != "" {
			logEvent = logEvent.Str("body_preview", bodyPreview)
		}
	}

	logEvent.Msgf("%s %s", r.Method, r.URL.Path)
}

func logRequestCompletion(
	ctx context.Context,
	r *http.Request,
	wrapped *responseWriter,
	duration time.Duration,
	shortID string,
) {
	durationStr := formatDuration(duration)
	statusMsg := statusSymbol(wrapped.statusCode)
	completionMsg := formatCompletionMessage(wrapped.statusCode, statusMsg, durationStr)

	logCtx := withRequestFields(ctx, r, shortID).
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
	if wrapped.statusCode >= 500 {
		logger.Error().Msg(completionMsg)
	} else if wrapped.statusCode >= 400 {
		logger.Warn().Msg(completionMsg)
	} else {
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
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			debugOpts := config.DebugOptions{}
			if provider != nil {
				debugOpts = provider()
			}

			start := time.Now()

			// Log request details in debug mode
			LogRequestDetails(r.Context(), r, debugOpts)

			// Attach timings container for downstream middleware/handlers
			ctx, _ := withRequestTimings(r.Context())
			r = r.WithContext(ctx)

			// Wrap ResponseWriter to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Get request ID for logging
			requestID := GetRequestID(r.Context())
			shortID := requestID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}

			logRequestStart(r.Context(), r, shortID)

			// Serve request
			next.ServeHTTP(wrapped, r)

			logRequestCompletion(r.Context(), r, wrapped, time.Since(start), shortID)
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

// authFingerprint computes a small fingerprint of auth-related config fields.
// This avoids relying on config pointer equality for cache invalidation.
// Uses length-prefixed format to avoid delimiter collision vulnerabilities.
func authFingerprint(bearerEnabled bool, bearerSecret, apiKey string) string {
	// Format: "b<0|1>|<len>:<bearerSecret>|<len>:<apiKey>"
	// Length-prefix prevents collision when secrets contain delimiters.
	b := byte('0')
	if bearerEnabled {
		b = '1'
	}

	buf := make([]byte, 0, 8+len(bearerSecret)+len(apiKey))
	buf = append(buf, 'b', b, '|')
	buf = strconv.AppendInt(buf, int64(len(bearerSecret)), 10)
	buf = append(buf, ':')
	buf = append(buf, bearerSecret...)
	buf = append(buf, '|')
	buf = strconv.AppendInt(buf, int64(len(apiKey)), 10)
	buf = append(buf, ':')
	buf = append(buf, apiKey...)
	return string(buf)
}

// LiveAuthMiddleware creates middleware that enforces auth based on live config.
// It rebuilds the authenticator chain when auth-related config values change.
//
//nolint:gocognit,gocyclo,funlen // complexity from double-check locking pattern and nested closure
func LiveAuthMiddleware(cfgProvider config.RuntimeConfig) func(http.Handler) http.Handler {
	var (
		cache atomic.Value // stores *authCache
		mu    sync.Mutex   // serialize rebuilds
	)

	// getOrBuildCache returns the cached authenticator, rebuilding if fingerprint changed.
	getOrBuildCache := func(fp string, authConfig config.AuthConfig, effectiveKey string) *authCache {
		// Fast path: check cache without lock
		if v := cache.Load(); v != nil {
			if c, ok := v.(*authCache); ok && c.fingerprint == fp {
				return c
			}
		}

		// Slow path: acquire lock and rebuild
		mu.Lock()
		defer mu.Unlock()

		// Double-check after acquiring lock
		if v := cache.Load(); v != nil {
			if c, ok := v.(*authCache); ok && c.fingerprint == fp {
				return c
			}
		}

		// Build authenticator chain
		var authenticators []auth.Authenticator
		if authConfig.IsBearerEnabled() {
			authenticators = append(authenticators, auth.NewBearerAuthenticator(authConfig.BearerSecret))
		}
		if effectiveKey != "" {
			authenticators = append(authenticators, auth.NewAPIKeyAuthenticator(effectiveKey))
		}

		var chain auth.Authenticator
		if len(authenticators) == 1 {
			chain = authenticators[0]
		} else if len(authenticators) > 1 {
			chain = auth.NewChainAuthenticator(authenticators...)
		}

		c := &authCache{
			fingerprint: fp,
			chain:       chain,
		}
		cache.Store(c)
		return c
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfgProvider == nil {
				next.ServeHTTP(w, r)
				return
			}

			cfg := cfgProvider.Get()
			if cfg == nil {
				next.ServeHTTP(w, r)
				return
			}

			authConfig := cfg.Server.Auth
			effectiveKey := cfg.Server.GetEffectiveAPIKey()
			fp := authFingerprint(authConfig.IsBearerEnabled(), authConfig.BearerSecret, effectiveKey)

			start := time.Now()
			cached := getOrBuildCache(fp, authConfig, effectiveKey)
			if timings := getRequestTimings(r.Context()); timings != nil {
				timings.Auth = time.Since(start)
			}

			if cached.chain == nil {
				next.ServeHTTP(w, r)
				return
			}

			result := cached.chain.Validate(r)
			if !result.Valid {
				zerolog.Ctx(r.Context()).Warn().
					Str("auth_type", string(result.Type)).
					Str("error", result.Error).
					Msg("authentication failed")
				WriteError(w, http.StatusUnauthorized, "authentication_error", result.Error)
				return
			}

			zerolog.Ctx(r.Context()).Debug().
				Str("auth_type", string(result.Type)).
				Msg("authentication succeeded")
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware adds X-Request-ID header and logger with request ID to context.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract or generate request ID
			requestID := r.Header.Get("X-Request-ID")
			ctx := AddRequestID(r.Context(), requestID)

			// Write request ID to response header
			if requestID == "" {
				requestID = GetRequestID(ctx)
			}

			w.Header().Set("X-Request-ID", requestID)

			// Attach logger to request
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// formatDuration formats duration in human-readable format.
// Shows milliseconds for fast requests (<1s), seconds for slow requests.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		// Fast request: show milliseconds (e.g., "456ms")
		ms := d.Round(time.Millisecond).Milliseconds()
		return fmt.Sprintf("%dms", ms)
	}
	// Slow request: show seconds with 2 decimal places (e.g., "1.23s")
	seconds := float64(d) / float64(time.Second)
	return fmt.Sprintf("%.2fs", seconds)
}

// formatCompletionMessage formats the completion message with status.
func formatCompletionMessage(status int, symbol, duration string) string {
	return symbol + " " + http.StatusText(status) + " (" + duration + ")"
}

// getBodyPreview reads the first 200 characters of the request body.
// Redacts any "api_key" or "key" fields to prevent logging sensitive data.
// Returns empty string if body cannot be read or is empty.
func getBodyPreview(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	// Read body (limited to 500 bytes for safety)
	body, err := io.ReadAll(io.LimitReader(r.Body, 500))
	if err != nil || len(body) == 0 {
		return ""
	}

	// Restore body for downstream handlers
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))

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
func (rw *responseWriter) Write(b []byte) (int, error) {
	// Count SSE events if streaming
	if rw.isStreaming {
		// Count occurrences of "event:" prefix in the data
		data := string(b)
		for i := 0; i < len(data); i++ {
			if i+6 <= len(data) && data[i:i+6] == "event:" {
				rw.sseEvents++
			}
		}
	}
	return rw.ResponseWriter.Write(b)
}
