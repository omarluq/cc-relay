package outbound_test

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/outbound"
)

const (
	socksVersion       = 0x05
	socksNoAuth        = 0x00
	socksUserPass      = 0x02
	socksNoAcceptable  = 0xff
	socksCmdConnect    = 0x01
	socksAtypIPv4      = 0x01
	socksAtypDomain    = 0x03
	socksReplySuccess  = 0x00
	socksReplyFailure  = 0x01
	userPassVersion    = 0x01
	upstreamBody       = "hello from upstream"
	proxiedHeaderValue = "via-proxy"
	proxiedHeader      = "X-Proxied"
	socksUser          = "alice"
	upstreamTestHost   = "upstream.test"
)

var errSOCKSRejected = errors.New("socks request rejected")

// closeQuietly closes a test resource; close errors do not affect assertions.
func closeQuietly(closer io.Closer) {
	if err := closer.Close(); err != nil {
		return
	}
}

func writeBody(writer http.ResponseWriter, body string) {
	if _, err := io.WriteString(writer, body); err != nil {
		return
	}
}

// socks5Server is a minimal SOCKS5 server (RFC 1928 / RFC 1929) for tests.
// It supports CONNECT to IPv4 or domain targets with optional
// username/password authentication.
type socks5Server struct {
	listener net.Listener
	user     string
	pass     string
	targets  []string
	mu       sync.Mutex
}

func startSOCKS5(t *testing.T, user, pass string) *socks5Server {
	t.Helper()

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &socks5Server{listener: listener, user: user, pass: pass, targets: nil, mu: sync.Mutex{}}
	go server.serve()
	t.Cleanup(func() { closeQuietly(listener) })
	return server
}

func (s *socks5Server) addr() string {
	return s.listener.Addr().String()
}

func (s *socks5Server) connectTargets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.targets...)
}

func (s *socks5Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *socks5Server) handle(conn net.Conn) {
	defer closeQuietly(conn)
	reader := bufio.NewReader(conn)

	if err := s.negotiate(reader, conn); err != nil {
		return
	}

	target, err := readConnectTarget(reader)
	if err != nil {
		writeReply(conn, socksReplyFailure)
		return
	}
	s.mu.Lock()
	s.targets = append(s.targets, target)
	s.mu.Unlock()

	dialer := net.Dialer{}
	upstream, err := dialer.Dial("tcp", resolveTestHost(target))
	if err != nil {
		writeReply(conn, socksReplyFailure)
		return
	}
	defer closeQuietly(upstream)

	if !writeReply(conn, socksReplySuccess) {
		return
	}
	pipe(conn, reader, upstream)
}

// pipe copies bytes both ways until either side closes.
func pipe(client net.Conn, clientReader io.Reader, upstream net.Conn) {
	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(upstream, clientReader)
		done <- err
	}()
	go func() {
		_, err := io.Copy(client, upstream)
		done <- err
	}()
	if err := <-done; err != nil {
		return
	}
}

func writeReply(conn net.Conn, code byte) bool {
	_, err := conn.Write([]byte{socksVersion, code, 0, socksAtypIPv4, 0, 0, 0, 0, 0, 0})
	return err == nil
}

func (s *socks5Server) negotiate(reader *bufio.Reader, conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return err
	}

	want := byte(socksNoAuth)
	if s.user != "" {
		want = socksUserPass
	}
	if !containsByte(methods, want) {
		if _, err := conn.Write([]byte{socksVersion, socksNoAcceptable}); err != nil {
			return err
		}
		return errSOCKSRejected
	}
	if _, err := conn.Write([]byte{socksVersion, want}); err != nil {
		return err
	}
	if want == socksNoAuth {
		return nil
	}
	return s.checkUserPass(reader, conn)
}

func containsByte(values []byte, want byte) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func readLengthPrefixed(reader *bufio.Reader) (string, error) {
	size, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	buf := make([]byte, int(size))
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *socks5Server) checkUserPass(reader *bufio.Reader, conn net.Conn) error {
	if _, err := reader.ReadByte(); err != nil { // sub-negotiation version
		return err
	}
	user, err := readLengthPrefixed(reader)
	if err != nil {
		return err
	}
	pass, err := readLengthPrefixed(reader)
	if err != nil {
		return err
	}

	status := byte(0x00)
	if user != s.user || pass != s.pass {
		status = 0x01
	}
	if _, err := conn.Write([]byte{userPassVersion, status}); err != nil {
		return err
	}
	if status != 0x00 {
		return errSOCKSRejected
	}
	return nil
}

func readConnectTarget(reader *bufio.Reader) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return "", err
	}
	if header[1] != socksCmdConnect {
		return "", fmt.Errorf("%w: command %d", errSOCKSRejected, header[1])
	}

	host, err := readTargetHost(reader, header[3])
	if err != nil {
		return "", err
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBytes); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBytes)
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func readTargetHost(reader *bufio.Reader, addressType byte) (string, error) {
	switch addressType {
	case socksAtypIPv4:
		addr := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(reader, addr); err != nil {
			return "", err
		}
		return net.IP(addr).String(), nil
	case socksAtypDomain:
		return readLengthPrefixed(reader)
	default:
		return "", fmt.Errorf("%w: address type %d", errSOCKSRejected, addressType)
	}
}

func newUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeBody(writer, upstreamBody)
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

// getThrough performs a GET via the transport and returns the headers and body.
func getThrough(t *testing.T, transport http.RoundTripper, target string) (header http.Header, body string) {
	t.Helper()

	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer closeQuietly(resp.Body)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.Header, string(data)
}

// tryGet performs a GET via the transport and returns only the request error.
func tryGet(t *testing.T, transport http.RoundTripper, target string) error {
	t.Helper()

	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer closeQuietly(resp.Body)
	return nil
}

func TestSOCKS5ProxyWithCredentials(t *testing.T) {
	t.Parallel()

	upstream := newUpstream(t)
	socks := startSOCKS5(t, socksUser, secretValue)

	registry := outbound.NewRegistry()
	transport, err := registry.Transport("socks5://" + socksUser + ":" + secretValue + "@" + socks.addr())
	require.NoError(t, err)

	_, body := getThrough(t, transport, viaTestHost(t, upstream))
	assert.Equal(t, upstreamBody, body)
	assert.Equal(t, []string{testHostAddr(t, upstream)}, socks.connectTargets())
}

func TestSOCKS5ProxyRejectsWrongCredentials(t *testing.T) {
	t.Parallel()

	upstream := newUpstream(t)
	socks := startSOCKS5(t, socksUser, secretValue)

	registry := outbound.NewRegistry()
	transport, err := registry.Transport("socks5h://" + socksUser + ":wrong-pass@" + socks.addr())
	require.NoError(t, err)

	err = tryGet(t, transport, viaTestHost(t, upstream))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "wrong-pass", "transport errors must not echo proxy credentials")
	assert.Empty(t, socks.connectTargets())
}

func TestHTTPProxy(t *testing.T) {
	t.Parallel()

	var seenHost, seenAuth string
	var seenMu sync.Mutex
	forwardProxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		seenMu.Lock()
		seenHost = req.URL.Host
		seenAuth = req.Header.Get("Proxy-Authorization")
		seenMu.Unlock()
		writer.Header().Set(proxiedHeader, proxiedHeaderValue)
		writeBody(writer, upstreamBody)
	}))
	t.Cleanup(forwardProxy.Close)

	registry := outbound.NewRegistry()
	proxyURL := "http://bob:" + secretValue + "@" + forwardProxy.Listener.Addr().String()
	transport, err := registry.Transport(proxyURL)
	require.NoError(t, err)

	header, body := getThrough(t, transport, "http://upstream.example:8080/v1/messages")
	assert.Equal(t, proxiedHeaderValue, header.Get(proxiedHeader))
	assert.Equal(t, upstreamBody, body)

	seenMu.Lock()
	defer seenMu.Unlock()
	assert.Equal(t, "upstream.example:8080", seenHost)
	assert.NotEmpty(t, seenAuth, "proxy credentials must be sent as Proxy-Authorization")
}

func TestDirectConnectsWithoutProxy(t *testing.T) {
	t.Parallel()

	upstream := newUpstream(t)

	registry := outbound.NewRegistry()
	transport, err := registry.Transport(outbound.Direct)
	require.NoError(t, err)

	_, body := getThrough(t, transport, upstream.URL)
	assert.Equal(t, upstreamBody, body)
}

// resolveTestHost maps upstreamTestHost to the loopback address the test
// upstream listens on. Proxied requests use the hostname because requests to
// loopback addresses bypass the proxy.
func resolveTestHost(target string) string {
	host, port, err := net.SplitHostPort(target)
	if err != nil || host != upstreamTestHost {
		return target
	}
	return net.JoinHostPort("127.0.0.1", port)
}

// testHostAddr returns upstreamTestHost:port for the test upstream.
func testHostAddr(t *testing.T, upstream *httptest.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
	require.NoError(t, err)
	return net.JoinHostPort(upstreamTestHost, port)
}

// viaTestHost returns the test upstream URL using upstreamTestHost.
func viaTestHost(t *testing.T, upstream *httptest.Server) string {
	t.Helper()
	return "http://" + testHostAddr(t, upstream)
}

func TestProxyBypassedForLoopbackUpstream(t *testing.T) {
	t.Parallel()

	upstream := newUpstream(t)
	socks := startSOCKS5(t, "", "")

	registry := outbound.NewRegistry()
	transport, err := registry.Transport("socks5://" + socks.addr())
	require.NoError(t, err)

	_, body := getThrough(t, transport, upstream.URL)
	assert.Equal(t, upstreamBody, body)
	assert.Empty(t, socks.connectTargets(), "loopback upstreams must not go through the proxy")
}
