// Package outbound builds the HTTP transports cc-relay uses for upstream
// (provider-bound) traffic, including optional outbound proxy support.
//
// An outbound proxy setting is one of:
//   - "" (unset): use http.DefaultTransport, which honors the HTTP_PROXY,
//     HTTPS_PROXY and NO_PROXY environment variables.
//   - "direct": connect directly, ignoring every proxy setting including
//     the environment.
//   - a proxy URL with scheme http, https, socks5 or socks5h.
package outbound

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Direct is the outbound proxy value that bypasses all proxies,
// including HTTP_PROXY / HTTPS_PROXY from the environment.
const Direct = "direct"

// redactedUserinfo replaces proxy credentials in logged URLs.
const redactedUserinfo = "xxxxx"

// ErrInvalidProxy is returned when an outbound proxy value cannot be used.
var ErrInvalidProxy = errors.New("invalid outbound proxy")

// supportedSchemes lists the proxy URL schemes net/http supports natively.
var supportedSchemes = map[string]bool{
	"http":    true,
	"https":   true,
	"socks5":  true,
	"socks5h": true,
}

// Resolve returns the effective outbound proxy setting for a provider.
// A provider-level value takes precedence over the global value.
func Resolve(global, provider string) string {
	if p := strings.TrimSpace(provider); p != "" {
		return p
	}
	return strings.TrimSpace(global)
}

// IsDirect reports whether the setting bypasses all proxies.
func IsDirect(setting string) bool {
	return strings.EqualFold(strings.TrimSpace(setting), Direct)
}

// Validate checks an outbound proxy setting. Empty and "direct" are valid.
// Error messages never include the configured value, so credentials embedded
// in the URL cannot leak through validation errors.
func Validate(setting string) error {
	setting = strings.TrimSpace(setting)
	if setting == "" || IsDirect(setting) {
		return nil
	}
	_, err := parseProxyURL(setting)
	return err
}

func parseProxyURL(raw string) (*url.URL, error) {
	proxyURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: not a valid URL", ErrInvalidProxy)
	}
	scheme := strings.ToLower(proxyURL.Scheme)
	if !supportedSchemes[scheme] || proxyURL.Opaque != "" {
		return nil, fmt.Errorf("%w: unsupported scheme %q (valid: http, https, socks5, socks5h, or %q)",
			ErrInvalidProxy, proxyURL.Scheme, Direct)
	}
	if proxyURL.Hostname() == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidProxy)
	}
	proxyURL.Scheme = scheme
	return proxyURL, nil
}

// Redact returns a form of the setting that is safe to log: any userinfo
// (username and password) is masked. Values that do not parse as a proxy
// URL are replaced entirely, since they may still contain credentials.
func Redact(setting string) string {
	setting = strings.TrimSpace(setting)
	if setting == "" || IsDirect(setting) {
		return setting
	}
	proxyURL, err := parseProxyURL(setting)
	if err != nil {
		return "[invalid proxy URL]"
	}
	if proxyURL.User != nil {
		proxyURL.User = url.User(redactedUserinfo)
	}
	return proxyURL.String()
}

// Registry hands out transports for outbound proxy settings.
//
// Transports are cloned from http.DefaultTransport, so connection pooling,
// timeouts and HTTP/2 support match the defaults. Each distinct setting gets
// one shared transport; the full setting (credentials included) is the key,
// so changing a proxy URL or its credentials yields a fresh transport.
// Call Retain after a config reload to release transports that are no
// longer referenced.
type Registry struct {
	transports map[string]*http.Transport
	mu         sync.Mutex
}

// NewRegistry creates an empty transport registry.
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[string]*http.Transport),
		mu:         sync.Mutex{},
	}
}

// Transport returns the round tripper for a resolved outbound proxy setting.
// An empty setting returns http.DefaultTransport itself, preserving the
// environment-based proxy behavior.
func (r *Registry) Transport(setting string) (http.RoundTripper, error) {
	key := normalize(setting)
	if key == "" {
		return http.DefaultTransport, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.transports[key]; ok {
		return existing, nil
	}

	transport, err := newTransport(key)
	if err != nil {
		return nil, err
	}
	r.transports[key] = transport
	return transport, nil
}

// Retain keeps the transports for the given settings and releases the rest.
// Released transports have their idle connections closed; requests already
// in flight on them complete normally.
func (r *Registry) Retain(settings []string) {
	keep := make(map[string]bool, len(settings))
	for _, setting := range settings {
		keep[normalize(setting)] = true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for key, transport := range r.transports {
		if keep[key] {
			continue
		}
		transport.CloseIdleConnections()
		delete(r.transports, key)
	}
}

// Len returns the number of transports currently held.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.transports)
}

func normalize(setting string) string {
	setting = strings.TrimSpace(setting)
	if IsDirect(setting) {
		return Direct
	}
	return setting
}

func newTransport(key string) (*http.Transport, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("outbound: http.DefaultTransport is not an *http.Transport")
	}
	transport := base.Clone()

	if key == Direct {
		transport.Proxy = nil
		return transport, nil
	}

	proxyURL, err := parseProxyURL(key)
	if err != nil {
		return nil, err
	}
	transport.Proxy = http.ProxyURL(proxyURL)
	return transport, nil
}
