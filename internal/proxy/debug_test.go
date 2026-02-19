package proxy_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/rs/zerolog"
)

func TestLogRequestDetailsDisabledByDefault(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-3"}`))
	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}

	proxy.LogRequestDetails(ctx, req, opts)

	// Should not log anything if disabled
	if buf.Len() > 0 {
		t.Errorf("Expected no log output when LogRequestBody disabled, got: %s", buf.String())
	}
}

func TestLogRequestDetailsRedactsSensitiveData(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	body := `{"api_key":"sk-secret-123","model":"claude-3","password":"hunter2"}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	opts := config.DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     1000,
	}

	proxy.LogRequestDetails(ctx, req, opts)

	output := buf.String()
	if strings.Contains(output, "sk-secret-123") {
		t.Error("Expected api_key to be redacted")
	}
	if strings.Contains(output, "hunter2") {
		t.Error("Expected password to be redacted")
	}
	if !strings.Contains(output, "REDACTED") {
		t.Error("Expected REDACTED placeholder in output")
	}
}

func TestLogRequestDetailsExtractsModel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	body := `{"model":"claude-3-5-sonnet-20241022","max_tokens":100}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	opts := config.DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     1000,
	}

	proxy.LogRequestDetails(ctx, req, opts)

	output := buf.String()
	if !strings.Contains(output, "claude-3-5-sonnet-20241022") {
		t.Error("Expected model name in log output")
	}
	if !strings.Contains(output, `"max_tokens":100`) {
		t.Error("Expected max_tokens in log output")
	}
}

func TestLogRequestDetailsTruncatesLargeBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	largeBody := strings.Repeat("x", 5000)
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(largeBody))
	opts := config.DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     100,
	}

	proxy.LogRequestDetails(ctx, req, opts)

	output := buf.String()
	// Should contain truncated portion but not full 5000 chars
	if strings.Count(output, "x") > 150 { // Some slack for JSON encoding
		t.Errorf("Expected truncated body, got %d x's", strings.Count(output, "x"))
	}
}

func TestLogResponseDetailsLogsEventCount(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	headers := http.Header{}
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("X-Anthropic-Stop-Reason", "end_turn")

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: true,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}
	proxy.LogResponseDetails(ctx, headers, 200, 42, opts)

	output := buf.String()
	if !strings.Contains(output, `"streaming_events":42`) {
		t.Error("Expected streaming_events count in output")
	}
	if !strings.Contains(output, "end_turn") {
		t.Error("Expected X-Anthropic-Stop-Reason in output")
	}
}

func TestLogTLSMetrics(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := proxy.TLSMetrics{
		Version:     "TLS 1.3",
		DNSTime:     5 * time.Millisecond,
		ConnectTime: 10 * time.Millisecond,
		TLSTime:     15 * time.Millisecond,
		Reused:      true,
		HasMetrics:  true,
	}

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      true,
		MaxBodyLogSize:     0,
	}
	proxy.LogTLSMetrics(ctx, metrics, opts)

	output := buf.String()
	if !strings.Contains(output, "TLS 1.3") {
		t.Error("Expected TLS version in output")
	}
	if !strings.Contains(output, `"tls_reused":true`) {
		t.Error("Expected tls_reused in output")
	}
	if !strings.Contains(output, "tls metrics") {
		t.Error("Expected 'tls metrics' message")
	}
}

func TestLogProxyMetrics(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := proxy.Metrics{
		BackendTime:     250 * time.Millisecond,
		TotalTime:       300 * time.Millisecond,
		BytesSent:       1024,
		BytesReceived:   2048,
		StreamingEvents: 10,
	}

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}
	proxy.LogProxyMetrics(ctx, metrics, opts)

	output := buf.String()
	if !strings.Contains(output, `"bytes_sent":1024`) {
		t.Error("Expected bytes_sent in output")
	}
	if !strings.Contains(output, `"bytes_received":2048`) {
		t.Error("Expected bytes_received in output")
	}
	if !strings.Contains(output, `"streaming_events":10`) {
		t.Error("Expected streaming_events in output")
	}
}

func TestDebugOptionsGetMaxBodyLogSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		opts     config.DebugOptions
		expected int
	}{
		{
			"default",
			config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     0,
			},
			1000,
		},
		{
			"zero",
			config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     0,
			},
			1000,
		},
		{
			"negative",
			config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     -1,
			},
			1000,
		},
		{
			"custom",
			config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     5000,
			},
			5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.opts.GetMaxBodyLogSize()
			if got != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestTLSVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		version  uint16
	}{
		{"TLS 1.0", "TLS 1.0", 0x0301},
		{"TLS 1.1", "TLS 1.1", 0x0302},
		{"TLS 1.2", "TLS 1.2", 0x0303},
		{"TLS 1.3", "TLS 1.3", 0x0304},
		{"unknown", "unknown", 0x0000},
		{"unknown high", "unknown", 0xFFFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proxy.TLSVersionString(tt.version)
			if got != tt.expected {
				t.Errorf("tlsVersionString(%#x) = %s, want %s", tt.version, got, tt.expected)
			}
		})
	}
}

func TestAttachTLSTraceReturnsMetricsFunction(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	ctx := req.Context()

	newCtx, getMetrics := proxy.AttachTLSTrace(ctx, req)

	// New context should be different (has trace attached)
	if newCtx == ctx {
		t.Error("Expected new context with trace attached")
	}

	// getMetrics should return TLSMetrics
	metrics := getMetrics()

	// Before any TLS handshake, HasMetrics should be false
	if metrics.HasMetrics {
		t.Error("Expected HasMetrics=false before TLS handshake")
	}
}

func TestLogTLSMetricsSkipConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errMsg  string
		metrics proxy.TLSMetrics
		opts    config.DebugOptions
	}{
		{
			name: "skips when disabled",
			metrics: proxy.TLSMetrics{
				Version:     "TLS 1.3",
				DNSTime:     0,
				ConnectTime: 0,
				TLSTime:     0,
				Reused:      false,
				HasMetrics:  true,
			},
			opts: config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     0,
			},
			errMsg: "Expected no log output when LogTLSMetrics disabled",
		},
		{
			name: "skips when no metrics",
			metrics: proxy.TLSMetrics{
				Version:     "",
				DNSTime:     0,
				ConnectTime: 0,
				TLSTime:     0,
				Reused:      false,
				HasMetrics:  false,
			},
			opts: config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      true,
				MaxBodyLogSize:     0,
			},
			errMsg: "Expected no log output when HasMetrics=false",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
			ctx := logger.WithContext(context.Background())

			proxy.LogTLSMetrics(ctx, testCase.metrics, testCase.opts)

			if buf.Len() > 0 {
				t.Errorf("%s, got: %s", testCase.errMsg, buf.String())
			}
		})
	}
}

func TestLogResponseDetailsSkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}
	proxy.LogResponseDetails(ctx, headers, 200, 0, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output when LogResponseHeaders disabled, got: %s", buf.String())
	}
}

func TestLogProxyMetricsSkipsAtHigherLogLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel) // Not debug
	ctx := logger.WithContext(context.Background())

	metrics := proxy.Metrics{
		BackendTime:     100 * time.Millisecond,
		TotalTime:       150 * time.Millisecond,
		BytesSent:       0,
		BytesReceived:   0,
		StreamingEvents: 0,
	}

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}
	proxy.LogProxyMetrics(ctx, metrics, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output at info level, got: %s", buf.String())
	}
}

func TestLogRequestDetailsSkipsAtHigherLogLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel) // Not debug
	ctx := logger.WithContext(context.Background())

	body := `{"model":"claude-3"}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	opts := config.DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     1000,
	}

	proxy.LogRequestDetails(ctx, req, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output at info level, got: %s", buf.String())
	}
}

func TestLogRequestDetailsHandlesNilBody(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Body = http.NoBody // Empty body
	opts := config.DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     1000,
	}

	// Should not panic
	proxy.LogRequestDetails(ctx, req, opts)
}
