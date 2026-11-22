package proxy_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/omarluq/cc-relay/internal/router"
)

const (
	failureTestProvider   = "primary"
	failureTestBackup     = "backup"
	unresolvedUpstreamURL = "http://upstream.invalid"
	upgradeHeader         = "Upgrade"
	connectionHeader      = "Connection"
)

// testUpstream is where a test provider sends requests. A nil transport uses the default.
type testUpstream struct {
	transport http.RoundTripper
	url       string
}

func (u testUpstream) provider(name string) providers.Provider {
	provider := providers.NewAnthropicProvider(name, u.url, nil, nil)
	if u.transport != nil {
		provider.SetOutboundTransport(u.transport)
	}
	return provider
}

func newFailureTracker(threshold uint32, openFor time.Duration) *health.Tracker {
	logger := zerolog.Nop()
	return health.NewTracker(health.CircuitBreakerConfig{
		OpenDurationMS:   int(openFor.Milliseconds()),
		FailureThreshold: threshold,
		HalfOpenProbes:   1,
	}, &logger)
}

// newFailoverHandler routes by failover, preferring providers listed first.
func newFailoverHandler(t *testing.T, tracker *health.Tracker, provs ...providers.Provider) *proxy.Handler {
	t.Helper()

	infos := make([]router.ProviderInfo, 0, len(provs))
	keys := make(map[string]string, len(provs))
	for idx, prov := range provs {
		info := proxy.TestProviderInfoWithHealth(prov, tracker.IsHealthyFunc(prov.Name()))
		info.Priority = len(provs) - idx
		infos = append(infos, info)
		keys[prov.Name()] = testKey
	}

	opts := proxy.MustTestHandlerOptions(provs[0], testKey)
	opts.HealthTracker = tracker
	opts.ProviderRouter = router.NewFailoverRouter(time.Second)
	opts.ProviderInfos = infos
	opts.ProviderKeys = keys

	handler, err := proxy.NewHandler(opts)
	require.NoError(t, err)
	return handler
}

func serveMessage(ctx context.Context, handler http.Handler, edit func(*http.Request)) int {
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, proxy.MessagesPath,
		strings.NewReader(`{"model":"test-model","messages":[]}`))
	request.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
	if edit != nil {
		edit(request)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}

func respondOK(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func abortResponse(http.ResponseWriter, *http.Request) {
	panic(http.ErrAbortHandler) // Drops the connection or stream without a response.
}

func plainUpstream(t *testing.T, handle http.HandlerFunc) testUpstream {
	t.Helper()
	server := httptest.NewServer(handle)
	t.Cleanup(server.Close)
	return testUpstream{transport: nil, url: server.URL}
}

func http2Upstream(t *testing.T, handle http.HandlerFunc) testUpstream {
	t.Helper()
	server := httptest.NewUnstartedServer(handle)
	server.EnableHTTP2 = true
	server.Config.MaxHeaderBytes = 4096
	server.StartTLS()
	t.Cleanup(server.Close)
	return testUpstream{transport: server.Client().Transport, url: server.URL}
}

// refusingTransport refuses every dial, so no local listener can affect the test.
// Each dial must target wantAddr.
func refusingTransport(t *testing.T, wantAddr string) *http.Transport {
	t.Helper()
	base, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)
	transport := base.Clone()
	transport.Proxy = nil
	transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
		assert.Equal(t, wantAddr, addr, "dial target")
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: syscall.ECONNREFUSED}
	}
	return transport
}

func refusedUpstream(t *testing.T) testUpstream {
	t.Helper()
	return testUpstream{transport: refusingTransport(t, "upstream.invalid:80"), url: unresolvedUpstreamURL}
}

func refusedProxyUpstream(t *testing.T) testUpstream {
	t.Helper()
	proxyURL, err := url.Parse("http://proxy.invalid:3128")
	require.NoError(t, err)

	transport := refusingTransport(t, "proxy.invalid:3128")
	transport.Proxy = http.ProxyURL(proxyURL)
	return testUpstream{transport: transport, url: "https://upstream.invalid"}
}

func dnsFailureUpstream(t *testing.T) testUpstream {
	t.Helper()
	base, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)
	transport := base.Clone()
	transport.Proxy = nil
	transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
		return nil, &net.OpError{
			Op: "dial", Net: "tcp", Source: nil, Addr: nil,
			Err: &net.DNSError{Err: "no such host", Name: "upstream.invalid", IsNotFound: true},
		}
	}
	return testUpstream{transport: transport, url: unresolvedUpstreamURL}
}

func tlsFailureUpstream(t *testing.T) testUpstream {
	t.Helper()
	upstream := plainUpstream(t, respondOK)
	upstream.url = strings.Replace(upstream.url, "http:", "https:", 1)
	return upstream
}

// tlsTimeoutUpstream completes the TCP handshake but never answers the TLS one.
func tlsTimeoutUpstream(t *testing.T) testUpstream {
	t.Helper()
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, listener.Close()) })

	base, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)
	transport := base.Clone()
	transport.TLSHandshakeTimeout = 100 * time.Millisecond
	return testUpstream{transport: transport, url: "https://" + listener.Addr().String()}
}

func TestTransportFailuresOpenCircuit(t *testing.T) {
	t.Parallel()

	tests := map[string]func(t *testing.T) testUpstream{
		"connection refused":    refusedUpstream,
		"outbound proxy down":   refusedProxyUpstream,
		"DNS failure":           dnsFailureUpstream,
		"TLS failure":           tlsFailureUpstream,
		"TLS handshake timeout": tlsTimeoutUpstream,
		"closed before response": func(t *testing.T) testUpstream {
			t.Helper()
			return plainUpstream(t, abortResponse)
		},
		"HTTP/2 stream reset": func(t *testing.T) testUpstream {
			t.Helper()
			return http2Upstream(t, abortResponse)
		},
	}

	for name, newUpstream := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tracker := newFailureTracker(1, time.Minute)
			handler := newFailoverHandler(t, tracker, newUpstream(t).provider(failureTestProvider))

			assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
			assert.Equal(t, health.StateOpen, tracker.GetState(failureTestProvider))
		})
	}
}

func TestTransportFailureOnReusedHTTP2ConnectionOpensCircuit(t *testing.T) {
	t.Parallel()

	const failHeader = "X-Test-Fail"
	upstream := http2Upstream(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get(failHeader) != "" {
			abortResponse(writer, request)
		}
		respondOK(writer, request)
	})
	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	require.Equal(t, http.StatusOK, serveMessage(t.Context(), handler, nil))
	status := serveMessage(t.Context(), handler, func(request *http.Request) {
		request.Header.Set(failHeader, "1")
	})
	assert.Equal(t, http.StatusBadGateway, status)
	assert.Equal(t, health.StateOpen, tracker.GetState(failureTestProvider))
}

func TestClientCausedErrorsKeepCircuitClosed(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("a", 16*1024)
	tests := map[string]struct {
		edit    func(request *http.Request)
		useHTTP bool // Plain HTTP/1.1 upstream instead of HTTP/2.
	}{
		"unprintable upgrade": {useHTTP: true, edit: func(request *http.Request) {
			request.Header.Set(connectionHeader, upgradeHeader)
			request.Header.Set(upgradeHeader, "\x01")
		}},
		"invalid header value": {useHTTP: true, edit: func(request *http.Request) {
			request.Header.Set("X-Invalid", "line\nbreak")
		}},
		"invalid trailer": {useHTTP: true, edit: func(request *http.Request) {
			request.Trailer = http.Header{"bad key": {"value"}}
		}},
		"upgrade over HTTP/2": {useHTTP: false, edit: func(request *http.Request) {
			request.Header.Set(connectionHeader, upgradeHeader)
			request.Header.Set(upgradeHeader, "custom")
		}},
		"headers over the HTTP/2 limit": {useHTTP: false, edit: func(request *http.Request) {
			request.Header.Set("X-Large", oversized)
		}},
		"trailers over the HTTP/2 limit": {useHTTP: false, edit: func(request *http.Request) {
			request.Trailer = http.Header{"X-Large": {oversized}}
		}},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			newUpstream := http2Upstream
			if testCase.useHTTP {
				newUpstream = plainUpstream
			}
			upstream := newUpstream(t, func(writer http.ResponseWriter, request *http.Request) {
				if _, err := io.Copy(io.Discard, request.Body); err != nil {
					return
				}
				respondOK(writer, request)
			})
			tracker := newFailureTracker(1, time.Minute)
			handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

			// The first request warms the connection, so reused connections are covered too.
			require.Equal(t, http.StatusOK, serveMessage(t.Context(), handler, nil))
			assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, testCase.edit))
			assert.Equal(t, health.StateClosed, tracker.GetState(failureTestProvider))
		})
	}
}

func TestProxySelectionErrorKeepsCircuitClosed(t *testing.T) {
	t.Parallel()

	base, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)
	transport := base.Clone()
	transport.Proxy = func(*http.Request) (*url.URL, error) {
		return nil, errors.New("invalid proxy setting")
	}
	upstream := testUpstream{transport: transport, url: unresolvedUpstreamURL}
	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, health.StateClosed, tracker.GetState(failureTestProvider))
}

func TestClientCancellationKeepsCircuitClosed(t *testing.T) {
	t.Parallel()

	started, release := make(chan struct{}), make(chan struct{})
	upstream := plainUpstream(t, func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})
	t.Cleanup(func() { close(release) })

	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		select {
		case <-started:
		case <-ctx.Done():
		}
		cancel()
	}()

	assert.Equal(t, http.StatusBadGateway, serveMessage(ctx, handler, nil))
	assert.Equal(t, health.StateClosed, tracker.GetState(failureTestProvider))
}

// A client deadline is reported as DeadlineExceeded, which is not context.Canceled.
func TestClientDeadlineKeepsCircuitClosed(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	upstream := plainUpstream(t, func(http.ResponseWriter, *http.Request) { <-release })
	t.Cleanup(func() { close(release) })

	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	assert.Equal(t, http.StatusBadGateway, serveMessage(ctx, handler, nil))
	assert.Equal(t, health.StateClosed, tracker.GetState(failureTestProvider))
}

func TestErrorAfterResponseIsNotATransportFailure(t *testing.T) {
	t.Parallel()

	upstream := plainUpstream(t, func(writer http.ResponseWriter, _ *http.Request) {
		// An unrequested protocol switch makes ReverseProxy fail after modifyResponse ran.
		writer.Header().Set(connectionHeader, upgradeHeader)
		writer.Header().Set(upgradeHeader, "custom")
		writer.WriteHeader(http.StatusSwitchingProtocols)
	})
	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, health.StateClosed, tracker.GetState(failureTestProvider))
}

func TestTransportFailureFailsOver(t *testing.T) {
	t.Parallel()

	backup := plainUpstream(t, respondOK)
	tracker := newFailureTracker(2, time.Minute)
	handler := newFailoverHandler(t, tracker,
		plainUpstream(t, abortResponse).provider(failureTestProvider), backup.provider(failureTestBackup))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, http.StatusOK, serveMessage(t.Context(), handler, nil), "open circuit must route to the backup")
}

func TestTransportFailureRecovers(t *testing.T) {
	t.Parallel()

	const recoveredStatus = http.StatusAccepted

	recovered := make(chan struct{})
	primary := plainUpstream(t, func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-recovered:
			writer.WriteHeader(recoveredStatus)
		default:
			abortResponse(writer, request)
		}
	})
	tracker := newFailureTracker(1, 100*time.Millisecond)
	handler := newFailoverHandler(t, tracker, primary.provider(failureTestProvider))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	close(recovered)
	require.Eventually(t, func() bool {
		return tracker.GetState(failureTestProvider) == health.StateHalfOpen
	}, 5*time.Second, 20*time.Millisecond)

	assert.Equal(t, recoveredStatus, serveMessage(t.Context(), handler, nil), "recovered primary must get traffic")
	assert.Equal(t, health.StateClosed, tracker.GetState(failureTestProvider))
}

// net/http can report a connection the upstream closed before the request was
// written. Those errors count; other errors in that window are local.
func TestConnectionErrorBeforeWriteOpensCircuit(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err      error
		wantOpen bool
	}{
		"EOF":                      {err: io.EOF, wantOpen: true},
		"unexpected EOF":           {err: io.ErrUnexpectedEOF, wantOpen: true},
		"server closed connection": {err: errors.New("http: server closed idle connection"), wantOpen: true},
		"connection reset": {err: &net.OpError{
			Op: "read", Net: "tcp", Source: nil, Addr: nil, Err: syscall.ECONNRESET,
		}, wantOpen: true},
		"local request error": {err: errors.New("invalid request header"), wantOpen: false},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			upstream := testUpstream{
				url: unresolvedUpstreamURL,
				transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					trace := httptrace.ContextClientTrace(request.Context())
					trace.GetConn(request.URL.Host)
					trace.GotConn(httptrace.GotConnInfo{Conn: nil, Reused: false, WasIdle: false, IdleTime: 0})
					return nil, testCase.err
				}),
			}
			tracker := newFailureTracker(1, time.Minute)
			handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

			assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
			want := health.StateClosed
			if testCase.wantOpen {
				want = health.StateOpen
			}
			assert.Equal(t, want, tracker.GetState(failureTestProvider))
		})
	}
}

func TestTransportFailureSingleProviderReturnsUnavailable(t *testing.T) {
	t.Parallel()

	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, plainUpstream(t, abortResponse).provider(failureTestProvider))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, http.StatusServiceUnavailable, serveMessage(t.Context(), handler, nil))
}

func TestTransportFailureLeavesKeyPoolAlone(t *testing.T) {
	t.Parallel()

	keyConfig := proxy.TestKeyConfig(testKey)
	keyConfig.RPMLimit = 2
	pool, err := keypool.NewKeyPool(failureTestProvider, keypool.PoolConfig{
		Strategy: keypool.StrategyLeastLoaded,
		Keys:     []keypool.KeyConfig{keyConfig},
	})
	require.NoError(t, err)

	opts := proxy.MustTestHandlerOptions(plainUpstream(t, abortResponse).provider(failureTestProvider), "")
	opts.HealthTracker = newFailureTracker(10, time.Minute)
	opts.Pool = pool
	handler, err := proxy.NewHandler(opts)
	require.NoError(t, err)

	// Feeding failures through the response hook would refill the limiter each time.
	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, http.StatusTooManyRequests, serveMessage(t.Context(), handler, nil))
}

func TestTransportFailureReportedByRebuiltProxy(t *testing.T) {
	t.Parallel()

	tracker := newFailureTracker(1, time.Minute)
	current := []router.ProviderInfo{
		proxy.TestProviderInfo(plainUpstream(t, respondOK).provider(failureTestProvider)),
	}

	opts := proxy.MustTestHandlerOptions(current[0].Provider, testKey)
	opts.HealthTracker = tracker
	opts.ProviderRouter = router.NewFailoverRouter(time.Second)
	opts.ProviderInfosFunc = func() []router.ProviderInfo { return current }
	handler, err := proxy.NewHandler(opts)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, serveMessage(t.Context(), handler, nil))

	// A reload replaces the provider, so the handler builds a new proxy for it.
	current[0] = proxy.TestProviderInfo(plainUpstream(t, abortResponse).provider(failureTestProvider))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, health.StateOpen, tracker.GetState(failureTestProvider))
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestTransportFailureGettingRetryConnectionOpensCircuit(t *testing.T) {
	t.Parallel()

	upstream := testUpstream{
		url: unresolvedUpstreamURL,
		transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			// A cached connection turns out to be unusable, then the redial fails.
			trace := httptrace.ContextClientTrace(request.Context())
			trace.GetConn(request.URL.Host)
			trace.GotConn(httptrace.GotConnInfo{Conn: nil, Reused: true, WasIdle: false, IdleTime: 0})
			trace.GetConn(request.URL.Host)
			return nil, errors.New("redial failed")
		}),
	}
	tracker := newFailureTracker(1, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	assert.Equal(t, health.StateOpen, tracker.GetState(failureTestProvider))
}

// The breaker treats context.Canceled as a success, so recording one would reset the failure count.
func TestTransportCancellationDoesNotResetFailureCount(t *testing.T) {
	t.Parallel()

	results := []error{errors.New("first failure"), context.Canceled, errors.New("second failure")}
	var calls int
	upstream := testUpstream{
		url: unresolvedUpstreamURL,
		transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			httptrace.ContextClientTrace(request.Context()).GetConn(request.URL.Host)
			err := results[calls]
			calls++
			return nil, err
		}),
	}
	tracker := newFailureTracker(2, time.Minute)
	handler := newFailoverHandler(t, tracker, upstream.provider(failureTestProvider))

	for range results {
		assert.Equal(t, http.StatusBadGateway, serveMessage(t.Context(), handler, nil))
	}
	assert.Equal(t, health.StateOpen, tracker.GetState(failureTestProvider))
}
