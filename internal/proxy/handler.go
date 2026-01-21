// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rs/zerolog"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// Handler proxies requests to a backend provider.
type Handler struct {
	provider  providers.Provider
	proxy     *httputil.ReverseProxy
	apiKey    string
	debugOpts config.DebugOptions
}

// NewHandler creates a new proxy handler.
// The provider parameter defines the backend LLM provider to proxy to.
// The apiKey parameter is the API key to use for authenticating with the backend.
// The debugOpts parameter controls debug logging behavior.
func NewHandler(provider providers.Provider, apiKey string, debugOpts config.DebugOptions) (*Handler, error) {
	targetURL, err := url.Parse(provider.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid provider base URL: %w", err)
	}

	h := &Handler{
		provider:  provider,
		apiKey:    apiKey,
		debugOpts: debugOpts,
	}

	h.proxy = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			// Set backend URL
			r.SetURL(targetURL)
			r.SetXForwarded()

			// Authenticate with provider
			//nolint:errcheck // Provider.Authenticate error handling deferred to ErrorHandler
			h.provider.Authenticate(r.Out, h.apiKey)

			// Forward anthropic-* headers
			forwardHeaders := h.provider.ForwardHeaders(r.In.Header)
			for key, values := range forwardHeaders {
				r.Out.Header[key] = values
			}
		},

		// CRITICAL: Immediate flush for SSE streaming
		// FlushInterval: -1 means flush after every write
		FlushInterval: -1,

		ModifyResponse: func(resp *http.Response) error {
			// Add SSE headers if streaming response
			if resp.Header.Get("Content-Type") == "text/event-stream" {
				SetSSEHeaders(resp.Header)
			}
			return nil
		},

		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// Use Anthropic-format error response
			WriteError(w, http.StatusBadGateway, "api_error", "upstream connection failed")
		},
	}

	return h, nil
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
