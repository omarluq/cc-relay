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
	"github.com/samber/lo"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// contextKey is used for storing values in request context.
type contextKey string

const keyIDContextKey contextKey = "keyID"

// Handler proxies requests to a backend provider.
type Handler struct {
	provider     providers.Provider
	router       router.ProviderRouter
	proxy        *httputil.ReverseProxy
	keyPool      *keypool.KeyPool
	apiKey       string
	providers    []router.ProviderInfo
	debugOpts    config.DebugOptions
	routingDebug bool
}

// NewHandler creates a new proxy handler.
// If providerRouter is provided, it will be used for provider selection.
// If providerRouter is nil, provider is used directly (single provider mode).
// If pool is provided, it will be used for key selection.
// If pool is nil, apiKey is used directly (single key mode).
func NewHandler(
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
	providerRouter router.ProviderRouter,
	apiKey string,
	pool *keypool.KeyPool,
	debugOpts config.DebugOptions,
	routingDebug bool,
) (*Handler, error) {
	targetURL, err := url.Parse(provider.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid provider base URL: %w", err)
	}

	h := &Handler{
		provider:     provider,
		providers:    providerInfos,
		router:       providerRouter,
		apiKey:       apiKey,
		keyPool:      pool,
		debugOpts:    debugOpts,
		routingDebug: routingDebug,
	}

	h.proxy = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			// Set backend URL
			r.SetURL(targetURL)
			r.SetXForwarded()

			// Check if client provided auth headers
			clientAuth := r.In.Header.Get("Authorization")
			clientAPIKey := r.In.Header.Get("x-api-key")
			hasClientAuth := clientAuth != "" || clientAPIKey != ""

			// Transparent mode: forward client auth ONLY if provider supports it.
			// This is true for Anthropic (client's Claude token works directly).
			// For other providers (Z.AI, Ollama, etc.), we must use configured keys.
			if hasClientAuth && h.provider.SupportsTransparentAuth() {
				// TRANSPARENT MODE: Client has auth AND provider accepts it
				// Forward client auth unchanged alongside anthropic-* headers

				// Forward anthropic-* headers (version, beta flags)
				lo.ForEach(lo.Entries(r.In.Header), func(entry lo.Entry[string, []string], _ int) {
					canonicalKey := http.CanonicalHeaderKey(entry.Key)
					if len(canonicalKey) >= 10 && canonicalKey[:10] == "Anthropic-" {
						r.Out.Header[canonicalKey] = entry.Value
					}
				})
				r.Out.Header.Set("Content-Type", "application/json")
			} else {
				// CONFIGURED KEY MODE: Use our configured keys
				// Either client has no auth, or provider doesn't accept client auth
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
				lo.ForEach(lo.Entries(forwardHeaders), func(entry lo.Entry[string, []string], _ int) {
					r.Out.Header[entry.Key] = entry.Value
				})
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

// selectProvider chooses a provider using the router or returns the static provider.
// In single provider mode (router is nil or no providers), returns the static provider.
func (h *Handler) selectProvider(ctx context.Context) (router.ProviderInfo, error) {
	if h.router == nil || len(h.providers) == 0 {
		// Single provider mode - wrap static provider
		return router.ProviderInfo{
			Provider:  h.provider,
			IsHealthy: func() bool { return true },
		}, nil
	}
	return h.router.Select(ctx, h.providers)
}

// selectKeyFromPool handles key selection from the pool.
// Returns keyID, key, updated request, and success.
// If success is false, an error response has been written and caller should return.
func (h *Handler) selectKeyFromPool(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
) (keyID, selectedKey string, updatedReq *http.Request, ok bool) {
	var err error
	keyID, selectedKey, err = h.keyPool.GetKey(r.Context())
	if errors.Is(err, keypool.ErrAllKeysExhausted) {
		retryAfter := h.keyPool.GetEarliestResetTime()
		WriteRateLimitError(w, retryAfter)
		logger.Warn().
			Dur("retry_after", retryAfter).
			Msg("all keys exhausted, returning 429")
		return "", "", r, false
	}
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("failed to select API key: %v", err))
		logger.Error().Err(err).Msg("failed to select API key")
		return "", "", r, false
	}

	// Add relay headers to response
	w.Header().Set(HeaderRelayKeyID, keyID)
	stats := h.keyPool.GetStats()
	w.Header().Set(HeaderRelayKeysTotal, strconv.Itoa(stats.TotalKeys))
	w.Header().Set(HeaderRelayKeysAvail, strconv.Itoa(stats.AvailableKeys))

	// Store keyID in context for ModifyResponse
	updatedReq = r.WithContext(context.WithValue(r.Context(), keyIDContextKey, keyID))

	return keyID, selectedKey, updatedReq, true
}

// ServeHTTP handles the proxy request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Select provider using router (or use static provider)
	selectedProviderInfo, err := h.selectProvider(r.Context())
	if err != nil {
		WriteError(w, http.StatusServiceUnavailable, "api_error",
			fmt.Sprintf("failed to select provider: %v", err))
		return
	}
	selectedProvider := selectedProviderInfo.Provider

	// Create logger with selected provider context
	logger := h.createProviderLoggerWithProvider(r, selectedProvider)
	r = r.WithContext(logger.WithContext(r.Context()))

	// Log routing strategy if router is available
	if h.router != nil {
		logger.Debug().
			Str("strategy", h.router.Name()).
			Str("selected_provider", selectedProvider.Name()).
			Msg("provider selected by router")
	}

	// Add debug headers if routing debug is enabled
	if h.routingDebug && h.router != nil {
		w.Header().Set("X-CC-Relay-Strategy", h.router.Name())
		w.Header().Set("X-CC-Relay-Provider", selectedProvider.Name())
	}

	// Handle auth mode selection and key selection
	r, ok := h.handleAuthAndKeySelection(w, r, &logger)
	if !ok {
		return
	}

	// Attach TLS trace if debug metrics enabled
	r, getTLSMetrics := h.attachTLSTraceIfEnabled(r)

	// Proxy request
	logger.Debug().Msg("proxying request to backend")
	backendStart := time.Now()
	h.proxy.ServeHTTP(w, r)
	backendTime := time.Since(backendStart)

	// Log metrics
	h.logMetricsIfEnabled(r, &logger, start, backendTime, getTLSMetrics)
}

// createProviderLoggerWithProvider creates a logger with the given provider context.
func (h *Handler) createProviderLoggerWithProvider(r *http.Request, p providers.Provider) zerolog.Logger {
	return zerolog.Ctx(r.Context()).With().
		Str("provider", p.Name()).
		Str("backend_url", p.BaseURL()).
		Logger()
}

// handleAuthAndKeySelection handles transparent auth mode detection and key selection.
// Returns the updated request and success status.
func (h *Handler) handleAuthAndKeySelection(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
) (*http.Request, bool) {
	clientAuth := r.Header.Get("Authorization")
	clientAPIKey := r.Header.Get("x-api-key")
	hasClientAuth := clientAuth != "" || clientAPIKey != ""
	useTransparentAuth := hasClientAuth && h.provider.SupportsTransparentAuth()

	if useTransparentAuth {
		logger.Debug().
			Bool("has_authorization", clientAuth != "").
			Bool("has_x_api_key", clientAPIKey != "").
			Msg("transparent mode: forwarding client auth")
		return r, true
	}

	if h.keyPool != nil {
		return h.handleKeyPoolSelection(w, r, logger, hasClientAuth, clientAuth, clientAPIKey)
	}

	// Single key mode - set header directly
	r.Header.Set("X-Selected-Key", h.apiKey)
	return r, true
}

// handleKeyPoolSelection handles key selection from the pool.
func (h *Handler) handleKeyPoolSelection(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
	hasClientAuth bool, clientAuth, clientAPIKey string,
) (*http.Request, bool) {
	if hasClientAuth {
		logger.Debug().
			Bool("has_authorization", clientAuth != "").
			Bool("has_x_api_key", clientAPIKey != "").
			Str("provider", h.provider.Name()).
			Msg("provider does not support transparent auth, using configured keys")
	}

	_, selectedKey, updatedReq, ok := h.selectKeyFromPool(w, r, logger)
	if !ok {
		return r, false
	}

	updatedReq.Header.Set("X-Selected-Key", selectedKey)
	// Note: defer cleanup not needed since header is only used within this request
	return updatedReq, true
}

// attachTLSTraceIfEnabled attaches TLS trace if debug metrics are enabled.
func (h *Handler) attachTLSTraceIfEnabled(r *http.Request) (req *http.Request, getMetrics func() TLSMetrics) {
	if !h.debugOpts.LogTLSMetrics {
		return r, nil
	}
	newCtx, metricsFunc := AttachTLSTrace(r.Context(), r)
	return r.WithContext(newCtx), metricsFunc
}

// logMetricsIfEnabled logs TLS and proxy metrics if debug mode is enabled.
func (h *Handler) logMetricsIfEnabled(
	r *http.Request, logger *zerolog.Logger, start time.Time,
	backendTime time.Duration, getTLSMetrics func() TLSMetrics,
) {
	if getTLSMetrics != nil {
		tlsMetrics := getTLSMetrics()
		LogTLSMetrics(r.Context(), tlsMetrics, h.debugOpts)
	}

	if h.debugOpts.IsEnabled() || logger.GetLevel() <= zerolog.DebugLevel {
		proxyMetrics := Metrics{
			BackendTime: backendTime,
			TotalTime:   time.Since(start),
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
