// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/zerolog/log"
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
type ProviderProxy struct {
	Provider           providers.Provider
	Proxy              *httputil.ReverseProxy
	KeyPool            *keypool.KeyPool
	targetURL          *url.URL
	modifyResponseHook ModifyResponseFunc
	APIKey             string
	debugOpts          config.DebugOptions
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

// modifyResponse handles SSE headers, Event Stream conversion, and calls the optional hook.
func (pp *ProviderProxy) modifyResponse(resp *http.Response) error {
	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err == nil {
			// Standard SSE: set headers
			if mediaType == providers.ContentTypeSSE {
				SetSSEHeaders(resp.Header)
			}

			// Bedrock Event Stream: needs conversion to SSE
			// The provider's StreamingContentType tells us what to expect
			providerStreamType := pp.Provider.StreamingContentType()
			if providerStreamType == providers.ContentTypeEventStream && mediaType == providers.ContentTypeEventStream {
				// Mark response for Event Stream conversion
				// The actual conversion happens via TransformResponse
				// We need to convert the Content-Type for the client
				resp.Header.Set("Content-Type", providers.ContentTypeSSE)
				SetSSEHeaders(resp.Header)

				// Store original response body for conversion
				// The TransformResponse needs http.ResponseWriter which we don't have here
				// Instead, we wrap the body to convert Event Stream to SSE on read
				if resp.Body != nil {
					resp.Body = newEventStreamToSSEBody(resp.Body)
				}
			}
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
	// Handle body transformation for cloud providers (Bedrock, Vertex)
	// This must happen before SetURL because cloud providers return a dynamic target URL
	if pp.Provider.RequiresBodyTransform() {
		pp.rewriteWithTransform(r)
		return
	}

	// Standard providers: use static target URL
	r.SetURL(pp.targetURL)
	r.SetXForwarded()
	pp.setAuth(r)
}

// rewriteWithTransform handles cloud providers that need body transformation.
// Cloud providers like Bedrock and Vertex need to:
// 1. Extract model from request body
// 2. Remove model from body and add anthropic_version
// 3. Construct dynamic URL with model in path.
func (pp *ProviderProxy) rewriteWithTransform(r *httputil.ProxyRequest) {
	// Read the original body
	var originalBody []byte
	if r.In.Body != nil {
		var err error
		originalBody, err = io.ReadAll(r.In.Body)
		if closeErr := r.In.Body.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close request body")
		}
		if err != nil {
			// If we can't read body, fall back to static URL
			r.SetURL(pp.targetURL)
			r.SetXForwarded()
			pp.setAuth(r)
			return
		}
	}

	// Get the endpoint path for transform (e.g., "/v1/messages")
	endpoint := r.In.URL.Path

	// Transform the request body and get the dynamic target URL
	newBody, targetURLStr, err := pp.Provider.TransformRequest(originalBody, endpoint)
	if err != nil {
		// On transform error, fall back to static URL with original body
		r.Out.Body = io.NopCloser(bytes.NewReader(originalBody))
		r.Out.ContentLength = int64(len(originalBody))
		r.SetURL(pp.targetURL)
		r.SetXForwarded()
		pp.setAuth(r)
		return
	}

	// Parse the dynamic target URL returned by the provider
	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		// On URL parse error, fall back to static URL with original body
		r.Out.Body = io.NopCloser(bytes.NewReader(originalBody))
		r.Out.ContentLength = int64(len(originalBody))
		r.SetURL(pp.targetURL)
		r.SetXForwarded()
		pp.setAuth(r)
		return
	}

	// Set the transformed body
	r.Out.Body = io.NopCloser(bytes.NewReader(newBody))
	r.Out.ContentLength = int64(len(newBody))

	// For cloud providers, the targetURL contains the complete path including model.
	// We set r.Out.URL directly instead of using SetURL which would append the original path.
	r.Out.URL = targetURL
	r.Out.Host = targetURL.Host
	r.SetXForwarded()
	pp.setAuth(r)
}

// setAuth handles authentication and header forwarding.
func (pp *ProviderProxy) setAuth(r *httputil.ProxyRequest) {
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
			if err := pp.Provider.Authenticate(r.Out, selectedKey); err != nil {
				log.Error().
					Err(err).
					Str("provider", pp.Provider.Name()).
					Msg("failed to authenticate request")
			}
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

// eventStreamToSSEBody wraps an Event Stream body and converts it to SSE on read.
// This allows the ReverseProxy to transparently convert Bedrock Event Stream
// to SSE format without requiring a custom transport.
type eventStreamToSSEBody struct {
	original  io.ReadCloser
	sseBuffer *bytes.Buffer // Buffered SSE output
	esBuffer  []byte        // Accumulated Event Stream data for parsing
	done      bool
}

// newEventStreamToSSEBody creates a wrapper that converts Event Stream to SSE.
func newEventStreamToSSEBody(original io.ReadCloser) *eventStreamToSSEBody {
	return &eventStreamToSSEBody{
		original:  original,
		sseBuffer: bytes.NewBuffer(nil),
		esBuffer:  make([]byte, 0, 32*1024),
	}
}

// Read implements io.Reader, converting Event Stream messages to SSE events.
func (e *eventStreamToSSEBody) Read(p []byte) (int, error) {
	for {
		if e.sseBuffer.Len() > 0 {
			return e.sseBuffer.Read(p)
		}
		if e.done {
			return 0, io.EOF
		}

		readErr := e.readAndBuffer()
		if errors.Is(readErr, io.EOF) {
			if e.sseBuffer.Len() == 0 {
				return 0, io.EOF
			}
			continue
		}
		if readErr != nil {
			return 0, readErr
		}
	}
}

func (e *eventStreamToSSEBody) readAndBuffer() error {
	chunk := make([]byte, 16*1024) // 16KB chunks
	n, readErr := e.original.Read(chunk)
	if n > 0 {
		e.esBuffer = append(e.esBuffer, chunk[:n]...)
	}

	e.parseEventStreamBuffer()

	if readErr == io.EOF {
		e.done = true
		return io.EOF
	}
	return readErr
}

func (e *eventStreamToSSEBody) parseEventStreamBuffer() {
	for {
		msg, consumed, err := providers.ParseEventStreamMessage(e.esBuffer)
		if err != nil {
			return
		}
		e.esBuffer = e.esBuffer[consumed:]
		e.sseBuffer.Write(providers.FormatMessageAsSSE(msg))
	}
}

// Close implements io.Closer.
func (e *eventStreamToSSEBody) Close() error {
	return e.original.Close()
}
