// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// contextKey is used for storing values in request context.
type contextKey string

const (
	keyIDContextKey        contextKey = "keyID"
	providerNameContextKey contextKey = "providerName"
)

// Handler proxies requests to a backend provider.
type Handler struct {
	providerProxies map[string]*ProviderProxy // Per-provider proxies for correct URL routing
	defaultProvider providers.Provider        // Fallback for single-provider mode
	router          router.ProviderRouter
	healthTracker   *health.Tracker
	routingConfig   *config.RoutingConfig // For model-based routing configuration
	providers       []router.ProviderInfo
	debugOpts       config.DebugOptions
	routingDebug    bool
}

// NewHandler creates a new proxy handler.
// If providerRouter is provided, it will be used for provider selection.
// If providerRouter is nil, provider is used directly (single provider mode).
// providerPools maps provider names to their key pools (may be nil for providers without pooling).
// providerKeys maps provider names to their fallback API keys.
// routingConfig contains model-based routing configuration (may be nil).
// If healthTracker is provided, success/failure will be reported to circuit breakers.
func NewHandler(
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
	providerRouter router.ProviderRouter,
	apiKey string,
	pool *keypool.KeyPool,
	providerPools map[string]*keypool.KeyPool,
	providerKeys map[string]string,
	routingConfig *config.RoutingConfig,
	debugOpts config.DebugOptions,
	routingDebug bool,
	healthTracker *health.Tracker,
) (*Handler, error) {
	h := &Handler{
		providerProxies: make(map[string]*ProviderProxy),
		defaultProvider: provider,
		providers:       providerInfos,
		router:          providerRouter,
		routingConfig:   routingConfig,
		debugOpts:       debugOpts,
		routingDebug:    routingDebug,
		healthTracker:   healthTracker,
	}

	// Create ProviderProxy for each provider in providerInfos (multi-provider mode)
	if len(providerInfos) > 0 {
		for _, info := range providerInfos {
			prov := info.Provider
			key := ""
			if providerKeys != nil {
				key = providerKeys[prov.Name()]
			}
			var provPool *keypool.KeyPool
			if providerPools != nil {
				provPool = providerPools[prov.Name()]
			}

			pp, err := NewProviderProxy(prov, key, provPool, debugOpts, h.modifyResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to create proxy for %s: %w", prov.Name(), err)
			}
			h.providerProxies[prov.Name()] = pp
		}
	} else {
		// Single provider mode - create one proxy for the default provider
		pp, err := NewProviderProxy(provider, apiKey, pool, debugOpts, h.modifyResponse)
		if err != nil {
			return nil, err
		}
		h.providerProxies[provider.Name()] = pp
	}

	return h, nil
}

// modifyResponse handles key pool updates and circuit breaker reporting.
// SSE headers are handled by ProviderProxy.modifyResponse before this is called.
func (h *Handler) modifyResponse(resp *http.Response) error {
	// Get provider name from context to find the correct key pool
	//nolint:errcheck // Type assertion failure returns empty string, which is safe
	providerName, _ := resp.Request.Context().Value(providerNameContextKey).(string)
	if providerName != "" {
		if pp, ok := h.providerProxies[providerName]; ok && pp.KeyPool != nil {
			h.updateKeyPoolFromResponse(resp, pp.KeyPool)
		}
	}

	// Report outcome to circuit breaker
	h.reportOutcome(resp)

	return nil
}

// updateKeyPoolFromResponse updates key pool state from response headers.
func (h *Handler) updateKeyPoolFromResponse(resp *http.Response, pool *keypool.KeyPool) {
	keyID, ok := resp.Request.Context().Value(keyIDContextKey).(string)
	if !ok || keyID == "" {
		return
	}

	logger := zerolog.Ctx(resp.Request.Context())

	// Update key state from response headers
	if err := pool.UpdateKeyFromHeaders(keyID, resp.Header); err != nil {
		logger.Debug().Err(err).Msg("failed to update key from headers")
	}

	// Handle 429 from backend
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header)
		pool.MarkKeyExhausted(keyID, retryAfter)
		logger.Warn().
			Str("key_id", keyID).
			Dur("cooldown", retryAfter).
			Msg("key hit rate limit, marking cooldown")
	}
}

// reportOutcome records success or failure to the circuit breaker.
func (h *Handler) reportOutcome(resp *http.Response) {
	if h.healthTracker == nil {
		return // No health tracking configured
	}

	// Get provider name from context
	providerName, ok := resp.Request.Context().Value(providerNameContextKey).(string)
	if !ok || providerName == "" {
		return // No provider name in context (single provider mode without routing)
	}

	if health.ShouldCountAsFailure(resp.StatusCode, nil) {
		h.healthTracker.RecordFailure(providerName, fmt.Errorf("HTTP %d", resp.StatusCode))
	} else {
		h.healthTracker.RecordSuccess(providerName)
	}
}

// selectProvider chooses a provider using the router or returns the static provider.
// In single provider mode (router is nil or no providers), returns the static provider.
// If model is provided and model-based routing is enabled, filters providers first.
// If hasThinkingAffinity is true, uses deterministic selection (first healthy provider)
// to ensure thinking signature validation works across conversation turns.
func (h *Handler) selectProvider(
	ctx context.Context, model string, hasThinkingAffinity bool,
) (router.ProviderInfo, error) {
	if h.router == nil || len(h.providers) == 0 {
		// Single provider mode - wrap static provider
		return router.ProviderInfo{
			Provider:  h.defaultProvider,
			IsHealthy: func() bool { return true },
		}, nil
	}

	// Filter providers if model-based routing is enabled
	candidates := h.providers
	if h.isModelBasedRouting() && model != "" {
		candidates = FilterProvidersByModel(
			model,
			h.providers,
			h.routingConfig.ModelMapping,
			h.routingConfig.DefaultProvider,
		)
	}

	// If thinking affinity is required, use deterministic selection.
	// This ensures that thinking-enabled conversations always route to the same
	// provider (the first healthy one), preventing signature validation failures.
	if hasThinkingAffinity && len(candidates) > 1 {
		healthy := router.FilterHealthy(candidates)
		if len(healthy) > 0 {
			// Force single candidate - first healthy provider (deterministic)
			candidates = healthy[:1]
		}
	}

	return h.router.Select(ctx, candidates)
}

// isModelBasedRouting returns true if model-based routing is configured.
func (h *Handler) isModelBasedRouting() bool {
	return h.routingConfig != nil &&
		h.routingConfig.Strategy == router.StrategyModelBased &&
		len(h.routingConfig.ModelMapping) > 0
}

// selectKeyFromPool handles key selection from the pool.
// Returns keyID, key, updated request, and success.
// If success is false, an error response has been written and caller should return.
func (h *Handler) selectKeyFromPool(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
	pool *keypool.KeyPool,
) (keyID, selectedKey string, updatedReq *http.Request, ok bool) {
	var err error
	keyID, selectedKey, err = pool.GetKey(r.Context())
	if errors.Is(err, keypool.ErrAllKeysExhausted) {
		retryAfter := pool.GetEarliestResetTime()
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
	stats := pool.GetStats()
	w.Header().Set(HeaderRelayKeysTotal, strconv.Itoa(stats.TotalKeys))
	w.Header().Set(HeaderRelayKeysAvail, strconv.Itoa(stats.AvailableKeys))

	// Store keyID in context for ModifyResponse
	updatedReq = r.WithContext(context.WithValue(r.Context(), keyIDContextKey, keyID))

	return keyID, selectedKey, updatedReq, true
}

// ServeHTTP handles the proxy request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Extract model from request body for model-based routing
	// Cache in context to avoid re-reading body in model rewriter
	modelOpt := ExtractModelFromRequest(r)
	model := modelOpt.OrEmpty()
	if modelOpt.IsPresent() {
		r = r.WithContext(CacheModelInContext(r.Context(), model))
	}

	// Check for thinking signatures in request body for sticky provider routing.
	// When extended thinking is enabled, the provider's signature must be validated
	// by the same provider on subsequent turns.
	hasThinking := HasThinkingSignature(r)
	if hasThinking {
		r = r.WithContext(CacheThinkingAffinityInContext(r.Context(), true))
	}

	// Select provider using router (or use static provider)
	// Model is passed for model-based routing filtering
	// hasThinking forces deterministic (sticky) selection
	selectedProviderInfo, err := h.selectProvider(r.Context(), model, hasThinking)
	if err != nil {
		WriteError(w, http.StatusServiceUnavailable, "api_error",
			fmt.Sprintf("failed to select provider: %v", err))
		return
	}
	selectedProvider := selectedProviderInfo.Provider

	// Get provider's proxy (critical fix: use the correct proxy for the selected provider)
	providerProxy, ok := h.providerProxies[selectedProvider.Name()]
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("no proxy configured for provider %s", selectedProvider.Name()))
		return
	}

	// Create logger with selected provider context
	logger := h.createProviderLoggerWithProvider(r, selectedProvider)
	r = r.WithContext(logger.WithContext(r.Context()))

	// Store provider name in context for modifyResponse to report to circuit breaker
	r = r.WithContext(context.WithValue(r.Context(), providerNameContextKey, selectedProvider.Name()))

	// Log routing strategy and set debug headers
	h.logAndSetDebugHeaders(w, r, &logger, selectedProvider)

	// Handle auth mode selection and key selection (using provider's pool)
	r, ok = h.handleAuthAndKeySelection(w, r, &logger, providerProxy)
	if !ok {
		return
	}

	// Rewrite model name if provider has model mapping configured
	h.rewriteModelIfNeeded(r, &logger, selectedProvider)

	// Attach TLS trace if debug metrics enabled
	r, getTLSMetrics := h.attachTLSTraceIfEnabled(r)

	// Proxy request using the provider-specific proxy
	logger.Debug().Msg("proxying request to backend")
	backendStart := time.Now()
	providerProxy.Proxy.ServeHTTP(w, r)
	backendTime := time.Since(backendStart)

	// Log metrics
	h.logMetricsIfEnabled(r, &logger, start, backendTime, getTLSMetrics)
}

// logAndSetDebugHeaders logs routing strategy and sets debug headers if enabled.
func (h *Handler) logAndSetDebugHeaders(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger, selectedProvider providers.Provider,
) {
	hasThinking := GetThinkingAffinityFromContext(r.Context())

	// Log routing strategy if router is available
	if h.router != nil {
		event := logger.Debug().
			Str("strategy", h.router.Name()).
			Str("selected_provider", selectedProvider.Name())
		if hasThinking {
			event.Bool("thinking_affinity", true)
		}
		event.Msg("provider selected by router")
	}

	if !h.routingDebug {
		return
	}

	// Add debug headers if routing debug is enabled
	if h.router != nil {
		w.Header().Set("X-CC-Relay-Strategy", h.router.Name())
		w.Header().Set("X-CC-Relay-Provider", selectedProvider.Name())
		if hasThinking {
			w.Header().Set("X-CC-Relay-Thinking-Affinity", "true")
		}
	}

	// Add health debug header
	if h.healthTracker != nil {
		state := h.healthTracker.GetState(selectedProvider.Name())
		w.Header().Set("X-CC-Relay-Health", state.String())
	}
}

// rewriteModelIfNeeded rewrites model name if provider has model mapping configured.
func (h *Handler) rewriteModelIfNeeded(
	r *http.Request, logger *zerolog.Logger, selectedProvider providers.Provider,
) {
	if mapping := selectedProvider.GetModelMapping(); len(mapping) > 0 {
		rewriter := NewModelRewriter(mapping)
		if err := rewriter.RewriteRequest(r, logger); err != nil {
			logger.Warn().Err(err).Msg("failed to rewrite model in request body")
			// Continue with original request - don't fail on rewrite errors
		}
	}
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
// Uses the provider from providerProxy for transparent auth checks and key selection.
func (h *Handler) handleAuthAndKeySelection(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
	providerProxy *ProviderProxy,
) (*http.Request, bool) {
	clientAuth := r.Header.Get("Authorization")
	clientAPIKey := r.Header.Get("x-api-key")
	hasClientAuth := clientAuth != "" || clientAPIKey != ""
	useTransparentAuth := hasClientAuth && providerProxy.Provider.SupportsTransparentAuth()

	if useTransparentAuth {
		logger.Debug().
			Bool("has_authorization", clientAuth != "").
			Bool("has_x_api_key", clientAPIKey != "").
			Msg("transparent mode: forwarding client auth")
		return r, true
	}

	if providerProxy.KeyPool != nil {
		return h.handleKeyPoolSelection(w, r, logger, providerProxy, hasClientAuth, clientAuth, clientAPIKey)
	}

	// Single key mode - set header directly
	r.Header.Set("X-Selected-Key", providerProxy.APIKey)
	return r, true
}

// handleKeyPoolSelection handles key selection from the pool.
func (h *Handler) handleKeyPoolSelection(
	w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
	providerProxy *ProviderProxy, hasClientAuth bool, clientAuth, clientAPIKey string,
) (*http.Request, bool) {
	if hasClientAuth {
		logger.Debug().
			Bool("has_authorization", clientAuth != "").
			Bool("has_x_api_key", clientAPIKey != "").
			Str("provider", providerProxy.Provider.Name()).
			Msg("provider does not support transparent auth, using configured keys")
	}

	_, selectedKey, updatedReq, ok := h.selectKeyFromPool(w, r, logger, providerProxy.KeyPool)
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
