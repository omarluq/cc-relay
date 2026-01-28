// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// contextKey is used for storing values in request context.
type contextKey string

const (
	keyIDContextKey           contextKey = "keyID"
	providerNameContextKey    contextKey = "providerName"
	modelNameContextKey       contextKey = "modelName"
	thinkingContextContextKey contextKey = "thinkingContext"
	handlerOptionsRequiredMsg            = "handler options are required"
)

// ProviderInfoFunc is a function that returns current provider routing information.
// This enables hot-reload of provider inputs (enabled/disabled, weights, priorities)
// without recreating the handler.
type ProviderInfoFunc func() []router.ProviderInfo

// KeyPoolsFunc returns the current key pools map for hot-reload support.
type KeyPoolsFunc func() map[string]*keypool.KeyPool

// KeysFunc returns the current fallback keys map for hot-reload support.
type KeysFunc func() map[string]string

// HandlerOptions configures handler construction.
type HandlerOptions struct {
	ProviderRouter    router.ProviderRouter
	Provider          providers.Provider
	ProviderPools     map[string]*keypool.KeyPool
	ProviderInfosFunc ProviderInfoFunc
	Pool              *keypool.KeyPool
	ProviderKeys      map[string]string
	GetProviderPools  KeyPoolsFunc
	GetProviderKeys   KeysFunc
	RoutingConfig     *config.RoutingConfig
	HealthTracker     *health.Tracker
	SignatureCache    *SignatureCache
	APIKey            string
	ProviderInfos     []router.ProviderInfo
	DebugOptions      config.DebugOptions
	RoutingDebug      bool
}

// Handler proxies requests to a backend provider.
type Handler struct {
	router           router.ProviderRouter
	runtimeCfg       config.RuntimeConfigGetter
	defaultProvider  providers.Provider
	routingConfig    *config.RoutingConfig
	healthTracker    *health.Tracker
	signatureCache   *SignatureCache
	providerProxies  map[string]*ProviderProxy
	providers        ProviderInfoFunc
	getProviderPools KeyPoolsFunc
	getProviderKeys  KeysFunc
	providerPools    map[string]*keypool.KeyPool
	providerKeys     map[string]string
	debugOpts        config.DebugOptions
	proxyMu          sync.RWMutex
	routingDebug     bool
}

// NewHandler creates a new proxy handler.
// If ProviderRouter is provided, it will be used for provider selection.
// If ProviderRouter is nil, Provider is used directly (single provider mode).
// ProviderPools maps provider names to their key pools (may be nil for providers without pooling).
// ProviderKeys maps provider names to their fallback API keys.
// RoutingConfig contains model-based routing configuration (may be nil).
// If HealthTracker is provided, success/failure will be reported to circuit breakers.
// If SignatureCache is provided, thinking signatures are cached for cross-provider reuse.
//
// For hot-reloadable provider inputs, set ProviderInfosFunc. Otherwise, ProviderInfos is used.
func NewHandler(opts *HandlerOptions) (*Handler, error) {
	return NewHandlerWithLiveProviders(opts)
}

// NewHandlerWithLiveProviders creates a new proxy handler with hot-reloadable provider info.
// ProviderInfosFunc is called per-request to get current provider routing information.
func NewHandlerWithLiveProviders(opts *HandlerOptions) (*Handler, error) {
	return NewHandlerWithLiveKeyPools(opts)
}

// NewHandlerWithLiveKeyPools creates a new proxy handler with hot-reloadable key pools.
// When providers are enabled via config reload, their keys and pools are available immediately.
func NewHandlerWithLiveKeyPools(opts *HandlerOptions) (*Handler, error) {
	if opts == nil {
		return nil, errors.New(handlerOptionsRequiredMsg)
	}
	return newHandlerWithOptions(opts)
}

func newHandlerWithOptions(opts *HandlerOptions) (*Handler, error) {
	providerInfosFunc := opts.ProviderInfosFunc
	if providerInfosFunc == nil {
		providerInfos := opts.ProviderInfos
		providerInfosFunc = func() []router.ProviderInfo { return providerInfos }
	}

	providerPools := opts.ProviderPools
	providerKeys := opts.ProviderKeys
	if opts.GetProviderPools != nil || opts.GetProviderKeys != nil {
		providerPools, providerKeys = resolveInitialMaps(opts.GetProviderPools, opts.GetProviderKeys)
	}

	initialInfos := providerInfosFunc()

	h := &Handler{
		providerProxies:  make(map[string]*ProviderProxy),
		defaultProvider:  opts.Provider,
		providers:        providerInfosFunc,
		router:           opts.ProviderRouter,
		routingConfig:    opts.RoutingConfig,
		debugOpts:        opts.DebugOptions,
		routingDebug:     opts.RoutingDebug,
		healthTracker:    opts.HealthTracker,
		signatureCache:   opts.SignatureCache,
		getProviderPools: opts.GetProviderPools,
		getProviderKeys:  opts.GetProviderKeys,
		providerPools:    providerPools,
		providerKeys:     providerKeys,
	}

	if err := initProviderProxies(h, &providerProxyInit{
		provider:      opts.Provider,
		apiKey:        opts.APIKey,
		pool:          opts.Pool,
		providerPools: providerPools,
		providerKeys:  providerKeys,
		debugOpts:     opts.DebugOptions,
		initialInfos:  initialInfos,
	}); err != nil {
		return nil, err
	}

	return h, nil
}

func resolveInitialMaps(
	getProviderPools KeyPoolsFunc,
	getProviderKeys KeysFunc,
) (providerPools map[string]*keypool.KeyPool, providerKeys map[string]string) {
	if getProviderPools != nil {
		providerPools = getProviderPools()
	}
	if getProviderKeys != nil {
		providerKeys = getProviderKeys()
	}
	return providerPools, providerKeys
}

type providerProxyInit struct {
	provider      providers.Provider
	pool          *keypool.KeyPool
	providerPools map[string]*keypool.KeyPool
	providerKeys  map[string]string
	apiKey        string
	initialInfos  []router.ProviderInfo
	debugOpts     config.DebugOptions
}

func initProviderProxies(h *Handler, init *providerProxyInit) error {
	if len(init.initialInfos) == 0 {
		pp, err := NewProviderProxy(init.provider, init.apiKey, init.pool, init.debugOpts, h.modifyResponse)
		if err != nil {
			return err
		}
		h.providerProxies[init.provider.Name()] = pp
		return nil
	}

	for _, info := range init.initialInfos {
		prov := info.Provider
		key := ""
		if init.providerKeys != nil {
			key = init.providerKeys[prov.Name()]
		}
		var provPool *keypool.KeyPool
		if init.providerPools != nil {
			provPool = init.providerPools[prov.Name()]
		}

		pp, err := NewProviderProxy(prov, key, provPool, init.debugOpts, h.modifyResponse)
		if err != nil {
			return fmt.Errorf("failed to create proxy for %s: %w", prov.Name(), err)
		}
		h.providerProxies[prov.Name()] = pp
	}

	return nil
}

// SetRuntimeConfigGetter sets a live config provider for dynamic routing/debug settings.
// When set, the handler prefers this over static routingConfig/debugOpts/routingDebug.
func (h *Handler) SetRuntimeConfigGetter(cfg config.RuntimeConfigGetter) {
	h.runtimeCfg = cfg
}

func (h *Handler) getRuntimeConfigGetter() *config.Config {
	if h.runtimeCfg == nil {
		return nil
	}
	return h.runtimeCfg.Get()
}

func (h *Handler) getRoutingConfig() *config.RoutingConfig {
	if cfg := h.getRuntimeConfigGetter(); cfg != nil {
		return &cfg.Routing
	}
	return h.routingConfig
}

func (h *Handler) getDebugOptions() config.DebugOptions {
	if cfg := h.getRuntimeConfigGetter(); cfg != nil {
		return cfg.Logging.DebugOptions
	}
	return h.debugOpts
}

func (h *Handler) isRoutingDebugEnabled() bool {
	if cfg := h.getRuntimeConfigGetter(); cfg != nil {
		return cfg.Routing.IsDebugEnabled()
	}
	return h.routingDebug
}

// modifyResponse handles key pool updates and circuit breaker reporting.
// SSE headers are handled by ProviderProxy.modifyResponse before this is called.
func (h *Handler) modifyResponse(resp *http.Response) error {
	// Get provider name from context to find the correct key pool
	//nolint:errcheck // Type assertion failure returns empty string, which is safe
	providerName, _ := resp.Request.Context().Value(providerNameContextKey).(string)
	if providerName != "" {
		// Thread-safe read of provider proxy
		h.proxyMu.RLock()
		pp, ok := h.providerProxies[providerName]
		h.proxyMu.RUnlock()
		if ok && pp.KeyPool != nil {
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

// getOrCreateProxy returns the proxy for the given provider, creating it lazily if needed.
// This allows newly enabled providers (after config reload) to become routable without
// handler recreation. Uses double-checked locking for efficiency.
// Uses live accessor functions (getProviderPools, getProviderKeys) when available for
// hot-reload support, falling back to static maps for backward compatibility.
func (h *Handler) getOrCreateProxy(prov providers.Provider) (*ProviderProxy, error) {
	provName := prov.Name()

	key, pool, keys, pools := h.currentKeyPool(provName)

	// Fast path: read lock to check if proxy exists and matches
	h.proxyMu.RLock()
	pp, exists := h.providerProxies[provName]
	h.proxyMu.RUnlock()

	if exists && h.proxyMatches(pp, prov, keys, pools, key, pool) {
		return pp, nil
	}

	// Slow path: write lock to create proxy
	h.proxyMu.Lock()
	defer h.proxyMu.Unlock()

	// Double-check after acquiring write lock
	pp, exists = h.providerProxies[provName]
	if exists && h.proxyMatches(pp, prov, keys, pools, key, pool) {
		return pp, nil
	}

	// In single-provider mode (maps==nil), preserve existing proxy's values.
	// This ensures we don't lose values set during handler construction.
	key, pool = h.preserveProxyAuthInputs(pp, key, pool, keys, pools)

	// (Re)create proxy if provider instance or auth inputs changed
	pp, err := NewProviderProxy(prov, key, pool, h.getDebugOptions(), h.modifyResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy for %s: %w", provName, err)
	}

	h.providerProxies[provName] = pp
	return pp, nil
}

func (h *Handler) currentKeyPool(
	provName string,
) (
	key string,
	pool *keypool.KeyPool,
	keys map[string]string,
	pools map[string]*keypool.KeyPool,
) {
	keys = h.providerKeys
	pools = h.providerPools
	if h.getProviderKeys != nil {
		keys = h.getProviderKeys()
	}
	if h.getProviderPools != nil {
		pools = h.getProviderPools()
	}

	key = ""
	if keys != nil {
		key = keys[provName]
	}
	pool = nil
	if pools != nil {
		pool = pools[provName]
	}

	return key, pool, keys, pools
}

func (h *Handler) preserveProxyAuthInputs(
	pp *ProviderProxy,
	key string,
	pool *keypool.KeyPool,
	keys map[string]string,
	pools map[string]*keypool.KeyPool,
) (string, *keypool.KeyPool) {
	if keys == nil && pp != nil && pp.APIKey != "" {
		key = pp.APIKey
	}
	if pools == nil && pp != nil && pp.KeyPool != nil {
		pool = pp.KeyPool
	}
	return key, pool
}

// proxyMatches checks if an existing proxy matches the given provider and auth inputs.
// In single-provider mode (maps are nil), keys/pools always match since we preserve
// values set during handler construction. In multi-provider mode, compares values
// to detect hot-reload changes.
func (h *Handler) proxyMatches(
	pp *ProviderProxy,
	prov providers.Provider,
	keys map[string]string,
	pools map[string]*keypool.KeyPool,
	key string,
	pool *keypool.KeyPool,
) bool {
	if pp == nil {
		return false
	}

	// Provider must match name and base URL
	if pp.Provider.Name() != prov.Name() || pp.Provider.BaseURL() != prov.BaseURL() {
		return false
	}

	// In single-provider mode (nil maps), keys/pools always match.
	// This preserves values set during handler construction via NewHandler.
	keysMatch := keys == nil || pp.APIKey == key
	poolsMatch := pools == nil || pp.KeyPool == pool

	return keysMatch && poolsMatch
}

// selectProvider chooses a provider using the router or returns the static provider.
// In single provider mode (router is nil or no providers), returns the static provider.
// If model is provided and model-based routing is enabled, filters providers first.
// If hasThinkingAffinity is true, uses deterministic selection (first healthy provider)
// to ensure thinking signature validation works across conversation turns.
func (h *Handler) selectProvider(
	ctx context.Context, model string, hasThinkingAffinity bool,
) (router.ProviderInfo, error) {
	start := time.Now()
	if timings := getRequestTimings(ctx); timings != nil {
		defer func() {
			timings.Routing = time.Since(start)
		}()
	}

	candidates, ok := h.providerCandidates()
	if !ok {
		return h.defaultProviderInfo(), nil
	}

	// Filter providers if model-based routing is enabled
	candidates, ok = h.applyModelRouting(candidates, model)
	if !ok {
		return h.defaultProviderInfo(), nil
	}

	// If thinking affinity is required, use deterministic selection.
	// This ensures that thinking-enabled conversations always route to the same
	// provider (the first healthy one), preventing signature validation failures.
	candidates = h.applyThinkingAffinity(candidates, hasThinkingAffinity)

	return h.router.Select(ctx, candidates)
}

func (h *Handler) providerCandidates() ([]router.ProviderInfo, bool) {
	if h.router == nil || h.providers == nil {
		return nil, false
	}
	candidates := h.providers()
	if len(candidates) == 0 {
		return nil, false
	}
	return candidates, true
}

func (h *Handler) defaultProviderInfo() router.ProviderInfo {
	return router.ProviderInfo{
		Provider:  h.defaultProvider,
		IsHealthy: func() bool { return true },
	}
}

func (h *Handler) applyModelRouting(
	candidates []router.ProviderInfo, model string,
) ([]router.ProviderInfo, bool) {
	if !h.isModelBasedRouting() || model == "" {
		return candidates, true
	}
	routingConfig := h.getRoutingConfig()
	if routingConfig == nil {
		return nil, false
	}
	return FilterProvidersByModel(
		model,
		candidates,
		routingConfig.ModelMapping,
		routingConfig.DefaultProvider,
	), true
}

func (h *Handler) applyThinkingAffinity(
	candidates []router.ProviderInfo, hasThinkingAffinity bool,
) []router.ProviderInfo {
	if !hasThinkingAffinity || len(candidates) <= 1 {
		return candidates
	}
	healthy := router.FilterHealthy(candidates)
	if len(healthy) > 0 {
		// Force single candidate - first healthy provider (deterministic)
		return healthy[:1]
	}
	return candidates
}

// isModelBasedRouting returns true if model-based routing is configured.
func (h *Handler) isModelBasedRouting() bool {
	routingConfig := h.getRoutingConfig()
	return routingConfig != nil &&
		routingConfig.Strategy == router.StrategyModelBased &&
		len(routingConfig.ModelMapping) > 0
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

	// Store model name in context for response processing (signature caching)
	if model != "" {
		r = r.WithContext(context.WithValue(r.Context(), modelNameContextKey, model))
	}

	// Process thinking signatures if cache is enabled and request has thinking blocks
	r = h.processThinkingSignatures(r, model)

	// Check for thinking signatures in request body for sticky provider routing.
	// When extended thinking is enabled, the provider's signature must be validated
	// by the same provider on subsequent turns.
	// To avoid unnecessary body reads in single-provider mode, only perform this
	// check when multi-provider routing is enabled.
	hasThinking := false
	if h.router != nil && h.providers != nil && len(h.providers()) > 1 {
		hasThinking = HasThinkingSignature(r)
		if hasThinking {
			r = r.WithContext(CacheThinkingAffinityInContext(r.Context(), true))
		}
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

	// Get or lazily create provider's proxy (enables newly enabled providers after reload)
	providerProxy, err := h.getOrCreateProxy(selectedProvider)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("failed to get proxy for provider %s: %v", selectedProvider.Name(), err))
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
	var authOK bool
	r, authOK = h.handleAuthAndKeySelection(w, r, &logger, providerProxy)
	if !authOK {
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

	if !h.isRoutingDebugEnabled() {
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
	debugOpts := h.getDebugOptions()
	if !debugOpts.LogTLSMetrics {
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
	debugOpts := h.getDebugOptions()
	if getTLSMetrics != nil {
		tlsMetrics := getTLSMetrics()
		LogTLSMetrics(r.Context(), tlsMetrics, debugOpts)
	}

	if debugOpts.IsEnabled() || logger.GetLevel() <= zerolog.DebugLevel {
		proxyMetrics := Metrics{
			BackendTime: backendTime,
			TotalTime:   time.Since(start),
		}
		LogProxyMetrics(r.Context(), proxyMetrics, debugOpts)
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

// processThinkingSignatures processes thinking block signatures in the request.
// Looks up cached signatures and replaces/drops blocks as needed.
// Returns the potentially modified request.
func (h *Handler) processThinkingSignatures(r *http.Request, model string) *http.Request {
	// Skip if no signature cache configured
	if h.signatureCache == nil {
		return r
	}

	// Read body for thinking detection
	if r.Body == nil {
		return r
	}
	body, err := io.ReadAll(r.Body)
	//nolint:errcheck // Best effort close
	r.Body.Close()
	// Always restore the body (and ContentLength) using the bytes read,
	// even if io.ReadAll returned an error, so upstream handlers see
	// the same body that was available to us.
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	if err != nil {
		return r
	}

	// Fast path: check if request has thinking blocks
	if !HasThinkingBlocks(body) {
		return r
	}

	// Extract model from body if not already known
	modelName := model
	if modelName == "" {
		modelName = gjson.GetBytes(body, "model").String()
	}

	// Process thinking blocks
	logger := zerolog.Ctx(r.Context())
	modifiedBody, thinkingCtx, err := ProcessRequestThinking(
		r.Context(), body, modelName, h.signatureCache,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to process thinking signatures")
		return r
	}

	// Log processing results
	if thinkingCtx.DroppedBlocks > 0 {
		logger.Debug().
			Int("dropped_blocks", thinkingCtx.DroppedBlocks).
			Msg("dropped unsigned thinking blocks")
	}
	if thinkingCtx.ReorderedBlocks {
		logger.Debug().Msg("reordered content blocks (thinking first)")
	}

	// Update request body
	r.Body = io.NopCloser(bytes.NewReader(modifiedBody))
	r.ContentLength = int64(len(modifiedBody))

	// Store thinking context for response processing
	r = r.WithContext(context.WithValue(r.Context(), thinkingContextContextKey, thinkingCtx))

	return r
}

// GetModelNameFromContext retrieves the model name from context.
func GetModelNameFromContext(ctx context.Context) string {
	//nolint:errcheck // Type assertion failure returns empty string, which is safe
	model, _ := ctx.Value(modelNameContextKey).(string)
	return model
}

// GetThinkingContextFromContext retrieves the thinking context from context.
func GetThinkingContextFromContext(ctx context.Context) *ThinkingContext {
	//nolint:errcheck // Type assertion failure returns nil, which is safe
	thinkingCtx, _ := ctx.Value(thinkingContextContextKey).(*ThinkingContext)
	return thinkingCtx
}
