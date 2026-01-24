// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"fmt"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/samber/lo"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
)

// ModifyResponseFunc is a callback for additional response processing.
type ModifyResponseFunc func(resp *http.Response) error

// ProviderProxy bundles a provider with its dedicated reverse proxy.
// Each proxy has the provider's URL and auth baked in at creation time,
// ensuring requests are routed to the correct backend with correct authentication.
//
//nolint:govet // fieldalignment: Prefer logical grouping over memory optimization
type ProviderProxy struct {
	Provider           providers.Provider
	Proxy              *httputil.ReverseProxy
	KeyPool            *keypool.KeyPool // May be nil for single-key mode
	APIKey             string           // Fallback key when no pool
	debugOpts          config.DebugOptions
	targetURL          *url.URL
	modifyResponseHook ModifyResponseFunc // Optional hook for additional response processing
}

// NewProviderProxy creates a provider-specific proxy with correct URL and auth.
// The proxy is configured to use this provider's BaseURL for all requests.
// The modifyResponseHook is called after SSE header handling for additional processing.
func NewProviderProxy(
	provider providers.Provider,
	apiKey string,
	pool *keypool.KeyPool,
	debugOpts config.DebugOptions,
	modifyResponseHook ModifyResponseFunc,
) (*ProviderProxy, error) {
	targetURL, err := url.Parse(provider.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid provider base URL %q: %w", provider.BaseURL(), err)
	}

	pp := &ProviderProxy{
		Provider:           provider,
		KeyPool:            pool,
		APIKey:             apiKey,
		debugOpts:          debugOpts,
		targetURL:          targetURL,
		modifyResponseHook: modifyResponseHook,
	}

	pp.Proxy = &httputil.ReverseProxy{
		Rewrite:        pp.rewrite,
		FlushInterval:  -1, // Immediate flush for SSE
		ModifyResponse: pp.modifyResponse,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			WriteError(w, http.StatusBadGateway, "api_error", "upstream connection failed")
		},
	}

	return pp, nil
}

// modifyResponse handles SSE headers and calls the optional hook for additional processing.
func (pp *ProviderProxy) modifyResponse(resp *http.Response) error {
	// Add SSE headers if streaming response (handles "text/event-stream; charset=utf-8" etc.)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if mediaType, _, err := mime.ParseMediaType(ct); err == nil && mediaType == "text/event-stream" {
			SetSSEHeaders(resp.Header)
		}
	}

	// Call the hook for additional processing (key pool updates, circuit breaker)
	if pp.modifyResponseHook != nil {
		return pp.modifyResponseHook(resp)
	}

	return nil
}

// rewrite creates the Rewrite function for this provider's proxy.
func (pp *ProviderProxy) rewrite(r *httputil.ProxyRequest) {
	r.SetURL(pp.targetURL)
	r.SetXForwarded()

	// Remove internal header before proxying to avoid key leakage
	r.Out.Header.Del("X-Selected-Key")

	clientAuth := r.In.Header.Get("Authorization")
	clientAPIKey := r.In.Header.Get("x-api-key")
	hasClientAuth := clientAuth != "" || clientAPIKey != ""

	if hasClientAuth && pp.Provider.SupportsTransparentAuth() {
		// TRANSPARENT MODE: Client has auth AND provider accepts it
		// Forward client auth unchanged alongside anthropic-* headers
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

		// Get the selected API key from context (set in ServeHTTP via header)
		selectedKey := r.In.Header.Get("X-Selected-Key")
		if selectedKey == "" {
			selectedKey = pp.APIKey // Fallback to single-key mode
		}

		// Only authenticate if we have a key to use
		if selectedKey != "" {
			//nolint:errcheck // Provider.Authenticate error handling deferred to ErrorHandler
			pp.Provider.Authenticate(r.Out, selectedKey)
		}
		// If no key available, let backend return 401 (transparent error)

		// Forward anthropic-* headers
		forwardHeaders := pp.Provider.ForwardHeaders(r.In.Header)
		lo.ForEach(lo.Entries(forwardHeaders), func(entry lo.Entry[string, []string], _ int) {
			r.Out.Header[entry.Key] = entry.Value
		})
	}
}

// GetTargetURL returns the target URL for this provider's proxy.
// Useful for testing and debugging.
func (pp *ProviderProxy) GetTargetURL() *url.URL {
	return pp.targetURL
}
