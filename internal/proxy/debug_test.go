package proxy

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/rs/zerolog"
)

func TestLogRequestDetails_DisabledByDefault(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-3"}`))
	opts := config.DebugOptions{LogRequestBody: false}

	LogRequestDetails(ctx, req, opts)

	// Should not log anything if disabled
	if buf.Len() > 0 {
		t.Errorf("Expected no log output when LogRequestBody disabled, got: %s", buf.String())
	}
}

func TestLogRequestDetails_RedactsSensitiveData(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	body := `{"api_key":"sk-secret-123","model":"claude-3","password":"hunter2"}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	opts := config.DebugOptions{LogRequestBody: true, MaxBodyLogSize: 1000}

	LogRequestDetails(ctx, req, opts)

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

func TestLogRequestDetails_ExtractsModel(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	body := `{"model":"claude-3-5-sonnet-20241022","max_tokens":100}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	opts := config.DebugOptions{LogRequestBody: true, MaxBodyLogSize: 1000}

	LogRequestDetails(ctx, req, opts)

	output := buf.String()
	if !strings.Contains(output, "claude-3-5-sonnet-20241022") {
		t.Error("Expected model name in log output")
	}
	if !strings.Contains(output, `"max_tokens":100`) {
		t.Error("Expected max_tokens in log output")
	}
}

func TestLogRequestDetails_TruncatesLargeBody(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	largeBody := strings.Repeat("x", 5000)
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(largeBody))
	opts := config.DebugOptions{LogRequestBody: true, MaxBodyLogSize: 100}

	LogRequestDetails(ctx, req, opts)

	output := buf.String()
	// Should contain truncated portion but not full 5000 chars
	if strings.Count(output, "x") > 150 { // Some slack for JSON encoding
		t.Errorf("Expected truncated body, got %d x's", strings.Count(output, "x"))
	}
}

func TestLogResponseDetails_LogsEventCount(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	headers := http.Header{}
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("X-Anthropic-Stop-Reason", "end_turn")

	opts := config.DebugOptions{LogResponseHeaders: true}
	LogResponseDetails(ctx, headers, 200, 42, opts)

	output := buf.String()
	if !strings.Contains(output, `"streaming_events":42`) {
		t.Error("Expected streaming_events count in output")
	}
	if !strings.Contains(output, "end_turn") {
		t.Error("Expected X-Anthropic-Stop-Reason in output")
	}
}

func TestLogTLSMetrics(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := TLSMetrics{
		Version:     "TLS 1.3",
		Reused:      true,
		DNSTime:     5 * time.Millisecond,
		ConnectTime: 10 * time.Millisecond,
		TLSTime:     15 * time.Millisecond,
		HasMetrics:  true,
	}

	opts := config.DebugOptions{LogTLSMetrics: true}
	LogTLSMetrics(ctx, metrics, opts)

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
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := Metrics{
		BackendTime:     250 * time.Millisecond,
		TotalTime:       300 * time.Millisecond,
		BytesSent:       1024,
		BytesReceived:   2048,
		StreamingEvents: 10,
	}

	opts := config.DebugOptions{}
	LogProxyMetrics(ctx, metrics, opts)

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

func TestDebugOptions_GetMaxBodyLogSize(t *testing.T) {
	tests := []struct {
		name     string
		opts     config.DebugOptions
		expected int
	}{
		{"default", config.DebugOptions{}, 1000},
		{"zero", config.DebugOptions{MaxBodyLogSize: 0}, 1000},
		{"negative", config.DebugOptions{MaxBodyLogSize: -1}, 1000},
		{"custom", config.DebugOptions{MaxBodyLogSize: 5000}, 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opts.GetMaxBodyLogSize()
			if got != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestTLSVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct { //nolint:govet // test table struct alignment
		name     string
		version  uint16
		expected string
	}{
		{"TLS 1.0", 0x0301, "TLS 1.0"},
		{"TLS 1.1", 0x0302, "TLS 1.1"},
		{"TLS 1.2", 0x0303, "TLS 1.2"},
		{"TLS 1.3", 0x0304, "TLS 1.3"},
		{"unknown", 0x0000, "unknown"},
		{"unknown high", 0xFFFF, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tlsVersionString(tt.version)
			if got != tt.expected {
				t.Errorf("tlsVersionString(%#x) = %s, want %s", tt.version, got, tt.expected)
			}
		})
	}
}

func TestAttachTLSTrace_ReturnsMetricsFunction(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	ctx := req.Context()

	newCtx, getMetrics := AttachTLSTrace(ctx, req)

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

func TestLogTLSMetrics_SkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := TLSMetrics{
		Version:    "TLS 1.3",
		HasMetrics: true,
	}

	// Disabled option
	opts := config.DebugOptions{LogTLSMetrics: false}
	LogTLSMetrics(ctx, metrics, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output when LogTLSMetrics disabled, got: %s", buf.String())
	}
}

func TestLogTLSMetrics_SkipsWhenNoMetrics(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := TLSMetrics{
		HasMetrics: false, // No metrics collected
	}

	opts := config.DebugOptions{LogTLSMetrics: true}
	LogTLSMetrics(ctx, metrics, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output when HasMetrics=false, got: %s", buf.String())
	}
}

func TestLogResponseDetails_SkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	opts := config.DebugOptions{LogResponseHeaders: false}
	LogResponseDetails(ctx, headers, 200, 0, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output when LogResponseHeaders disabled, got: %s", buf.String())
	}
}

func TestLogProxyMetrics_SkipsAtHigherLogLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel) // Not debug
	ctx := logger.WithContext(context.Background())

	metrics := Metrics{
		BackendTime: 100 * time.Millisecond,
		TotalTime:   150 * time.Millisecond,
	}

	opts := config.DebugOptions{}
	LogProxyMetrics(ctx, metrics, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output at info level, got: %s", buf.String())
	}
}

func TestLogRequestDetails_SkipsAtHigherLogLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel) // Not debug
	ctx := logger.WithContext(context.Background())

	body := `{"model":"claude-3"}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	opts := config.DebugOptions{LogRequestBody: true, MaxBodyLogSize: 1000}

	LogRequestDetails(ctx, req, opts)

	if buf.Len() > 0 {
		t.Errorf("Expected no log output at info level, got: %s", buf.String())
	}
}

func TestLogRequestDetails_HandlesNilBody(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Body = http.NoBody // Empty body
	opts := config.DebugOptions{LogRequestBody: true, MaxBodyLogSize: 1000}

	// Should not panic
	LogRequestDetails(ctx, req, opts)
}
