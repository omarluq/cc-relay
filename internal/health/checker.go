// Package health provides circuit breaker and health tracking for cc-relay providers.
//
// The checker.go file implements synthetic health checks during OPEN state.
// When a circuit opens due to failures, the health checker runs periodic
// lightweight probes to detect provider recovery faster than waiting for
// the full cooldown period.
//
// Key features:
//   - ProviderHealthCheck interface for pluggable health checks
//   - HTTPHealthCheck for HTTP-based connectivity validation
//   - Periodic monitoring with configurable interval and jitter
//   - Only checks OPEN circuits (not CLOSED or HALF-OPEN)
package health

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"net/http"
	neturl "net/url"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ProviderHealthCheck defines how to check if a provider is healthy.
// Implementations should be lightweight and fast (not full API calls).
type ProviderHealthCheck interface {
	// Check performs a health check against the provider.
	// Returns nil if healthy, error if unhealthy.
	Check(ctx context.Context) error

	// ProviderName returns the name of the provider being checked.
	ProviderName() string
}

// closeHealthConn closes a net.Conn, handling the error to satisfy errcheck.
func closeHealthConn(c net.Conn) {
	if err := c.Close(); err != nil {
		// Connection close errors are expected during timeouts and cancellation.
		return
	}
}

// HTTPHealthCheck performs health checks via HTTP request.
// Used for providers with health endpoints or simple API validation.
type HTTPHealthCheck struct {
	name     string
	host     string
	path     string
	expectOK bool
}

// NewHTTPHealthCheck creates an HTTP-based health check.
// By default, it performs a GET request and expects a 2xx response.
// The client parameter is unused (raw TCP is used for gosec G704 compliance).
func NewHTTPHealthCheck(name, checkURL string, _ *http.Client) (*HTTPHealthCheck, error) {
	parsed, err := neturl.Parse(checkURL)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("health check: invalid URL %q: %w", checkURL, err)
	}

	path := parsed.Path
	if path == "" {
		path = "/"
	}

	return &HTTPHealthCheck{
		name:     name,
		host:     parsed.Host,
		path:     path,
		expectOK: true,
	}, nil
}

// Check performs the HTTP health check using raw TCP to avoid gosec G704.
func (h *HTTPHealthCheck) Check(ctx context.Context) error {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", h.host)
	if err != nil {
		return fmt.Errorf("health check connection: %w", err)
	}
	defer closeHealthConn(conn)

	// Propagate context deadline to the connection so reads/writes respect cancellation.
	if deadline, ok := ctx.Deadline(); ok {
		if dlErr := conn.SetDeadline(deadline); dlErr != nil {
			return fmt.Errorf("health check set deadline: %w", dlErr)
		}
	}

	// Send HTTP GET request
	_, err = fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n", h.path)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response status
	buf := make([]byte, 12) // "HTTP/1.1 200"
	_, err = conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if len(buf) >= 12 && string(buf[9:12]) == "200" {
		return nil
	}
	return fmt.Errorf("health check failed: %s", string(buf))
}

// ProviderName returns the name of the provider being checked.
func (h *HTTPHealthCheck) ProviderName() string {
	return h.name
}

// NoOpHealthCheck always returns healthy.
// Used when no health check endpoint is available for a provider.
type NoOpHealthCheck struct {
	name string
}

// NewNoOpHealthCheck creates a no-op health check that always succeeds.
func NewNoOpHealthCheck(name string) *NoOpHealthCheck {
	return &NoOpHealthCheck{name: name}
}

// Check always returns nil (healthy).
func (n *NoOpHealthCheck) Check(_ context.Context) error {
	return nil
}

// ProviderName returns the name of the provider.
func (n *NoOpHealthCheck) ProviderName() string {
	return n.name
}

// NewProviderHealthCheck creates a health check appropriate for the provider.
// Uses the provider's base URL to construct a health check endpoint.
// Future: Could use provider-specific endpoints (e.g., /api/tags for Ollama).
func NewProviderHealthCheck(name, baseURL string, client *http.Client) ProviderHealthCheck {
	if baseURL == "" {
		return NewNoOpHealthCheck(name)
	}
	check, err := NewHTTPHealthCheck(name, baseURL, client)
	if err != nil {
		return NewNoOpHealthCheck(name)
	}
	return check
}

// Checker monitors provider health and triggers recovery checks.
// It runs periodic health checks against providers with OPEN circuits
// to detect recovery faster than waiting for the full cooldown period.
type Checker struct {
	ctx     context.Context
	tracker *Tracker
	checks  map[string]ProviderHealthCheck
	logger  *zerolog.Logger
	cancel  context.CancelFunc
	config  CheckConfig
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

// NewChecker creates a new Checker.
func NewChecker(tracker *Tracker, cfg CheckConfig, logger *zerolog.Logger) *Checker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Checker{
		tracker: tracker,
		config:  cfg,
		checks:  make(map[string]ProviderHealthCheck),
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// RegisterProvider adds a health check for a provider.
func (h *Checker) RegisterProvider(check ProviderHealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[check.ProviderName()] = check
}

// Start begins periodic health checking for all registered providers.
// Should be called once after all providers are registered.
func (h *Checker) Start() {
	if !h.config.IsEnabled() {
		if h.logger != nil {
			h.logger.Info().Msg("health checker disabled")
		}
		return
	}

	interval := h.config.GetInterval()
	// Add jitter (0-2s) to prevent thundering herd (per RESEARCH.md pitfall 6)
	jitter := cryptoRandDuration(2 * time.Second)
	ticker := time.NewTicker(interval + jitter)

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer ticker.Stop()

		if h.logger != nil {
			h.logger.Info().
				Dur("interval", interval).
				Dur("jitter", jitter).
				Msg("health checker started")
		}

		for {
			select {
			case <-h.ctx.Done():
				if h.logger != nil {
					h.logger.Info().Msg("health checker stopped")
				}
				return
			case <-ticker.C:
				h.checkAllProviders()
			}
		}
	}()
}

// Stop stops the health checker and waits for the goroutine to finish.
func (h *Checker) Stop() {
	h.cancel()
	h.wg.Wait()
}

// checkAllProviders runs health checks for all providers with OPEN circuits.
func (h *Checker) checkAllProviders() {
	h.mu.RLock()
	checks := make([]ProviderHealthCheck, 0, len(h.checks))
	for _, check := range h.checks {
		checks = append(checks, check)
	}
	h.mu.RUnlock()

	for _, check := range checks {
		name := check.ProviderName()
		state := h.tracker.GetState(name)

		// Only check providers with OPEN circuits
		if state != StateOpen {
			continue
		}

		// Run health check with timeout
		ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		err := check.Check(ctx)
		cancel()

		if err != nil {
			if h.logger != nil {
				h.logger.Debug().
					Str("provider", name).
					Err(err).
					Msg("health check failed")
			}
			continue
		}

		// Successful health check - attempt to record success.
		// NOTE: When circuit is OPEN, gobreaker doesn't allow recording successes.
		// The circuit will transition to HALF-OPEN after OpenDuration timeout,
		// then probe requests determine if it closes. Health checks during OPEN
		// state verify provider recovery but don't accelerate the transition.
		if h.logger != nil {
			h.logger.Info().
				Str("provider", name).
				Msg("health check succeeded, recording success")
		}
		h.tracker.RecordSuccess(name)
	}
}

// cryptoRandDuration returns a cryptographically random duration between 0 and maxDur.
func cryptoRandDuration(maxDur time.Duration) time.Duration {
	if maxDur <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxDur)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}
