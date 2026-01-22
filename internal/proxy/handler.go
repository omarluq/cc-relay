// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
)

// contextKey is used for storing values in request context.
type contextKey string

const keyIDContextKey contextKey = "keyID"

// Handler proxies requests to a backend provider.
type Handler struct {
	provider  providers.Provider
	proxy     *httputil.ReverseProxy
	keyPool   *keypool.KeyPool // Key pool for multi-key routing (nil in single-key mode)
	apiKey    string           // Single key (used when keyPool is nil)
	debugOpts config.DebugOptions
}

// NewHandler creates a new proxy handler.
// If pool is provided, it will be used for key selection.
// If pool is nil, apiKey is used directly (single key mode).
func NewHandler(
	provider providers.Provider,
	apiKey string,
	pool *keypool.KeyPool,
	debugOpts config.DebugOptions,
) (*Handler, error) {
	targetURL, err := url.Parse(provider.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid provider base URL: %w", err)
	}

	h := &Handler{
		provider:  provider,
		apiKey:    apiKey,
		keyPool:   pool,
		debugOpts: debugOpts,
	}

	h.proxy = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			// Set backend URL
			r.SetURL(targetURL)
			r.SetXForwarded()

			// Check if client provided auth headers
			clientAuth := r.In.Header.Get("Authorization")
			clientAPIKey := r.In.Header.Get("x-api-key")

			if clientAuth != "" || clientAPIKey != "" {
				// TRANSPARENT MODE: Client has auth - forward it unchanged
				// Do NOT strip Authorization, do NOT add our key
				// Just forward anthropic-* headers alongside client auth

				// Forward anthropic-* headers (version, beta flags)
				for key, values := range r.In.Header {
					canonicalKey := http.CanonicalHeaderKey(key)
					if len(canonicalKey) >= 10 && canonicalKey[:10] == "Anthropic-" {
						r.Out.Header[canonicalKey] = values
					}
				}
				r.Out.Header.Set("Content-Type", "application/json")
			} else {
				// FALLBACK MODE: Client has NO auth - use our configured keys
				// Strip any stale headers and authenticate with our key
				r.Out.Header.Del("Authorization")
				r.Out.Header.Del("x-api-key")

				// Get the selected API key from context (set in ServeHTTP)
				selectedKey := r.In.Header.Get("X-Selected-Key")
				if selectedKey == "" {
					selectedKey = h.apiKey // Fallback to single-key mode
				}

				// Only authenticate if we have a key to use
				if selectedKey != "" {
					//nolint:errcheck // Provider.Authenticate error handling deferred to ErrorHandler
					h.provider.Authenticate(r.Out, selectedKey)
				}
				// If no key available, let backend return 401 (transparent error)

				// Forward anthropic-* headers
				forwardHeaders := h.provider.ForwardHeaders(r.In.Header)
				for key, values := range forwardHeaders {
					r.Out.Header[key] = values
				}
			}
		},

		// CRITICAL: Immediate flush for SSE streaming
		// FlushInterval: -1 means flush after every write
		FlushInterval: -1,

		ModifyResponse: h.modifyResponse,

		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// Use Anthropic-format error response
			WriteError(w, http.StatusBadGateway, "api_error", "upstream connection failed")
		},
	}

	return h, nil
}

// modifyResponse handles response modification including SSE headers and key pool updates.
func (h *Handler) modifyResponse(resp *http.Response) error {
	// Add SSE headers if streaming response
	if resp.Header.Get("Content-Type") == "text/event-stream" {
		SetSSEHeaders(resp.Header)
	}

	// Update key pool from rate limit headers
	if h.keyPool != nil {
		keyID, ok := resp.Request.Context().Value(keyIDContextKey).(string)
		if ok && keyID != "" {
			logger := zerolog.Ctx(resp.Request.Context())

			// Update key state from response headers
			if err := h.keyPool.UpdateKeyFromHeaders(keyID, resp.Header); err != nil {
				logger.Debug().Err(err).Msg("failed to update key from headers")
			}

			// Handle 429 from backend
			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfter := parseRetryAfter(resp.Header)
				h.keyPool.MarkKeyExhausted(keyID, retryAfter)
				logger.Warn().
					Str("key_id", keyID).
					Dur("cooldown", retryAfter).
					Msg("key hit rate limit, marking cooldown")
			}
		}
	}

	return nil
}

// ServeHTTP handles the proxy request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	logger := zerolog.Ctx(r.Context()).With().
		Str("provider", h.provider.Name()).
		Str("backend_url", h.provider.BaseURL()).
		Logger()

	// Update context with provider-aware logger
	r = r.WithContext(logger.WithContext(r.Context()))

	// Check if client provided auth - skip key pool if so
	clientAuth := r.Header.Get("Authorization")
	clientAPIKey := r.Header.Get("x-api-key")
	hasClientAuth := clientAuth != "" || clientAPIKey != ""

	// Select API key (only needed in fallback mode)
	var selectedKey string
	var keyID string

	if hasClientAuth {
		// Transparent mode: client auth will be forwarded, skip key pool
		logger.Debug().
			Bool("has_authorization", clientAuth != "").
			Bool("has_x_api_key", clientAPIKey != "").
			Msg("transparent mode: forwarding client auth")
	} else if h.keyPool != nil {
		// Fallback mode with key pool
		var err error
		keyID, selectedKey, err = h.keyPool.GetKey(r.Context())
		if errors.Is(err, keypool.ErrAllKeysExhausted) {
			retryAfter := h.keyPool.GetEarliestResetTime()
			WriteRateLimitError(w, retryAfter)
			logger.Warn().
				Dur("retry_after", retryAfter).
				Msg("all keys exhausted, returning 429")
			return
		}
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal_error",
				fmt.Sprintf("failed to select API key: %v", err))
			logger.Error().Err(err).Msg("failed to select API key")
			return
		}

		// Add relay headers to response
		w.Header().Set(HeaderRelayKeyID, keyID)
		stats := h.keyPool.GetStats()
		w.Header().Set(HeaderRelayKeysTotal, strconv.Itoa(stats.TotalKeys))
		w.Header().Set(HeaderRelayKeysAvail, strconv.Itoa(stats.AvailableKeys))

		// Store keyID in context for ModifyResponse
		r = r.WithContext(context.WithValue(r.Context(), keyIDContextKey, keyID))

		// Pass selected key to Rewrite via temporary header (removed after)
		r.Header.Set("X-Selected-Key", selectedKey)
		defer r.Header.Del("X-Selected-Key")
	} else if !hasClientAuth {
		// Single key mode - set header directly
		r.Header.Set("X-Selected-Key", h.apiKey)
	}

	// Attach TLS trace if debug metrics enabled
	var getTLSMetrics func() TLSMetrics
	if h.debugOpts.LogTLSMetrics {
		newCtx, metricsFunc := AttachTLSTrace(r.Context(), r)
		r = r.WithContext(newCtx)
		getTLSMetrics = metricsFunc
	}

	// Log proxy start
	logger.Debug().Msg("proxying request to backend")

	// Proxy request
	backendStart := time.Now()
	h.proxy.ServeHTTP(w, r)
	backendTime := time.Since(backendStart)

	// Log TLS metrics if collected
	if getTLSMetrics != nil {
		tlsMetrics := getTLSMetrics()
		LogTLSMetrics(r.Context(), tlsMetrics, h.debugOpts)
	}

	// Log proxy metrics
	if h.debugOpts.IsEnabled() || logger.GetLevel() <= zerolog.DebugLevel {
		proxyMetrics := Metrics{
			BackendTime: backendTime,
			TotalTime:   time.Since(start),
			// BytesSent/BytesReceived would require wrapping http.ResponseWriter
			// StreamingEvents would require parsing SSE stream
			// Defer these to future enhancement
		}
		LogProxyMetrics(r.Context(), proxyMetrics, h.debugOpts)
	}
}

// parseRetryAfter parses the Retry-After header from an HTTP response.
// Returns the duration to wait before retrying. Defaults to 60 seconds if parsing fails.
func parseRetryAfter(headers http.Header) time.Duration {
	val := headers.Get("Retry-After")
	if val == "" {
		return 60 * time.Second // Default 1 minute
	}

	// Try seconds format (integer)
	if seconds, err := strconv.Atoi(val); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try HTTP-date format
	if t, err := http.ParseTime(val); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
	}

	return 60 * time.Second // Default if parsing failed
}
