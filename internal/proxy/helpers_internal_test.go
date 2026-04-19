package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// Test selectOutput, shouldUsePretty, GetRequestID, AddRequestID

func TestSelectOutput_Stdout(t *testing.T) {
	t.Parallel()

	out, file, err := selectOutput("")
	if err != nil {
		t.Fatalf("selectOutput('') error: %v", err)
	}
	if out != os.Stdout {
		t.Error("selectOutput('') should return os.Stdout")
	}
	if file != os.Stdout {
		t.Error("selectOutput('') file should be os.Stdout")
	}
}

func TestSelectOutput_ExplicitStdout(t *testing.T) {
	t.Parallel()

	out, file, err := selectOutput("stdout")
	if err != nil {
		t.Fatalf("selectOutput('stdout') error: %v", err)
	}
	if out != os.Stdout {
		t.Error("selectOutput('stdout') should return os.Stdout")
	}
	if file != os.Stdout {
		t.Error("selectOutput('stdout') file should be os.Stdout")
	}
}

func TestSelectOutput_Stderr(t *testing.T) {
	t.Parallel()

	out, file, err := selectOutput("stderr")
	if err != nil {
		t.Fatalf("selectOutput('stderr') error: %v", err)
	}
	if out != os.Stderr {
		t.Error("selectOutput('stderr') should return os.Stderr")
	}
	if file != os.Stderr {
		t.Error("selectOutput('stderr') file should be os.Stderr")
	}
}

func TestSelectOutput_File(t *testing.T) {
	t.Parallel()

	tmpFile := t.TempDir() + "/test.log"
	out, file, err := selectOutput(tmpFile)
	if err != nil {
		t.Fatalf("selectOutput(file) error: %v", err)
	}
	if out == nil {
		t.Error("selectOutput(file) returned nil writer")
	}
	if file == nil {
		t.Error("selectOutput(file) returned nil file")
	}
	if file != nil {
		closeErr := file.Close()
		if closeErr != nil {
			t.Errorf("file.Close() error: %v", closeErr)
		}
	}
}

func TestSelectOutput_InvalidPath(t *testing.T) {
	t.Parallel()

	_, _, err := selectOutput("/nonexistent/dir/test.log")
	if err == nil {
		t.Error("selectOutput(invalid path) should return error")
	}
}

func newLoggingConfig(pretty bool, format string) config.LoggingConfig {
	return config.LoggingConfig{
		Pretty: pretty,
		Level:  "",
		Format: format,
		Output: "",
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
	}
}

func TestShouldUsePretty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file *os.File
		name string
		cfg  config.LoggingConfig
		want bool
	}{
		{
			name: "Pretty flag true",
			cfg:  newLoggingConfig(true, ""),
			file: nil,
			want: true,
		},
		{
			name: "format pretty",
			cfg:  newLoggingConfig(false, "pretty"),
			file: nil,
			want: true,
		},
		{
			name: "format json",
			cfg:  newLoggingConfig(false, "json"),
			file: nil,
			want: false,
		},
		{
			name: "format console with nil file",
			cfg:  newLoggingConfig(false, "console"),
			file: nil,
			want: false,
		},
		{
			name: "default format with nil file",
			cfg:  newLoggingConfig(false, ""),
			file: nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shouldUsePretty(tc.cfg, tc.file)
			if got != tc.want {
				t.Errorf("shouldUsePretty() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	t.Parallel()

	got := GetRequestID(t.Context())
	if got != "" {
		t.Errorf("GetRequestID() = %q, want empty", got)
	}
}

func TestGetRequestID_WithValue(t *testing.T) {
	t.Parallel()

	ctx := AddRequestID(t.Context(), "test-id-123")
	got := GetRequestID(ctx)
	if got != "test-id-123" {
		t.Errorf("GetRequestID() = %q, want %q", got, "test-id-123")
	}
}

func TestAddRequestID_GeneratesUUID(t *testing.T) {
	t.Parallel()

	ctx := AddRequestID(t.Context(), "")
	got := GetRequestID(ctx)
	if got == "" {
		t.Error("AddRequestID('') should generate a UUID, got empty")
	}
}

// Test closeBody

type mockReadCloser struct {
	closeErr error
}

func (m *mockReadCloser) Read(_ []byte) (n int, err error) {
	return 0, io.EOF
}

func (m *mockReadCloser) Close() error {
	return m.closeErr
}

func TestCloseBody_Success(t *testing.T) {
	t.Parallel()

	body := io.NopCloser(strings.NewReader("test"))
	closeBody(body)
	// Should not panic
}

func TestCloseBody_Error(t *testing.T) {
	t.Parallel()

	body := &mockReadCloser{closeErr: fmt.Errorf("close error")}
	closeBody(body)
	// Should not panic, error is discarded
}

// Test addDurationFields, addDurationFieldsCtx

func TestAddDurationFields_Positive(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	log := zerolog.New(&buf)
	event := log.Info()

	result := addDurationFields(event, "test_duration", 150*time.Millisecond)
	if result == nil {
		t.Fatal("addDurationFields() returned nil")
	}

	result.Msg("test")
	output := buf.String()

	if !strings.Contains(output, "150") {
		t.Errorf("Expected duration in output, got: %s", output)
	}
}

func TestAddDurationFields_Zero(t *testing.T) {
	t.Parallel()

	logger := zerolog.New(io.Discard)
	event := logger.Info()
	result := addDurationFields(event, "test_duration", 0)
	if result == nil {
		t.Fatal("addDurationFields() returned nil for zero duration")
	}
}

func TestAddDurationFieldsCtx_Positive(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := zerolog.New(&buf)
	ctx := logger.With()

	addDurationFieldsCtx(&ctx, "test_duration", 250*time.Millisecond)

	finalLogger := ctx.Logger()
	finalLogger.Log().Msg("test")
	output := buf.String()

	if !strings.Contains(output, "250") {
		t.Errorf("Expected duration in output, got: %s", output)
	}
}

func TestAddDurationFieldsCtx_Zero(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	ctx := logger.With()

	addDurationFieldsCtx(&ctx, "test_duration", 0)
	// Should not panic
}

func TestAddDurationFieldsCtx_Negative(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	ctx := logger.With()

	addDurationFieldsCtx(&ctx, "test_duration", -100*time.Millisecond)
	// Should not panic
}

// Test getRequestTimings, withRequestTimings

func TestGetRequestTimings_NilContext(t *testing.T) {
	t.Parallel()

	got := getRequestTimings(nil) //nolint:staticcheck // Test nil context behavior explicitly
	if got != nil {
		t.Errorf("getRequestTimings(nil) = %v, want nil", got)
	}
}

func TestGetRequestTimings_NoTimingsInContext(t *testing.T) {
	t.Parallel()

	got := getRequestTimings(context.Background())
	if got != nil {
		t.Errorf("getRequestTimings(bg context) = %v, want nil", got)
	}
}

func TestGetRequestTimings_WithTimings(t *testing.T) {
	t.Parallel()

	ctx, timings := withRequestTimings(context.Background())
	timings.Auth = 10 * time.Millisecond
	timings.Routing = 20 * time.Millisecond

	got := getRequestTimings(ctx)
	if got == nil {
		t.Fatal("getRequestTimings(ctx) returned nil")
	}
	if got.Auth != 10*time.Millisecond {
		t.Errorf("Auth = %v, want 10ms", got.Auth)
	}
	if got.Routing != 20*time.Millisecond {
		t.Errorf("Routing = %v, want 20ms", got.Routing)
	}
}

// Test statusSymbol

func TestStatusSymbol_AllRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want string
		code int
	}{
		{want: "✓", code: 200},
		{want: "✓", code: 299},
		{want: "✓", code: 300},
		{want: "✓", code: 399},
		{want: "⚠", code: 400},
		{want: "⚠", code: 499},
		{want: "✗", code: 500},
		{want: "✗", code: 503},
		{want: "✗", code: 599},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.code), func(t *testing.T) {
			t.Parallel()
			got := statusSymbol(tc.code)
			if got != tc.want {
				t.Errorf("statusSymbol(%d) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

// Test debug.go functions

func TestReadAndRestoreBody_NilBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
	req.Body = nil // force nil body (httptest sets http.NoBody)
	logger := zerolog.Nop()

	got := readAndRestoreBody(req, &logger)
	if got != nil {
		t.Errorf("readAndRestoreBody(nil) = %v, want nil", got)
	}
}

func TestReadAndRestoreBody_ValidBody(t *testing.T) {
	t.Parallel()

	body := `{"test":"data"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", strings.NewReader(body))
	logger := zerolog.Nop()

	got := readAndRestoreBody(req, &logger)
	if got == nil {
		t.Fatal("readAndRestoreBody() returned nil")
	}

	if string(got) != body {
		t.Errorf("readAndRestoreBody() = %q, want %q", string(got), body)
	}

	newBody, readErr := io.ReadAll(req.Body)
	if readErr != nil {
		t.Fatalf("io.ReadAll() error: %v", readErr)
	}
	if string(newBody) != body {
		t.Error("Body was not properly restored")
	}
}

func TestReadAndRestoreBody_ReadError(t *testing.T) {
	t.Parallel()

	// Create a request with a body that will fail to read
	errorBody := &errorReadCloser{readErr: fmt.Errorf("read error")}
	req := &http.Request{
		Method: "POST",
		Body:   errorBody,
	}
	logger := zerolog.New(io.Discard)

	got := readAndRestoreBody(req, &logger)
	if got != nil {
		t.Errorf("readAndRestoreBody(error) = %v, want nil", got)
	}
}

type errorReadCloser struct {
	readErr error
}

func (e *errorReadCloser) Read(_ []byte) (n int, err error) {
	return 0, e.readErr
}

func (e *errorReadCloser) Close() error {
	return nil
}

func TestExtractModelInfo_Empty(t *testing.T) {
	t.Parallel()

	model, maxTokens := extractModelInfo([]byte{})
	if model != "" {
		t.Errorf("extractModelInfo(empty) model = %q, want empty", model)
	}
	if maxTokens != 0 {
		t.Errorf("extractModelInfo(empty) maxTokens = %d, want 0", maxTokens)
	}
}

func TestExtractModelInfo_Valid(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-3","max_tokens":4096}`)
	model, maxTokens := extractModelInfo(body)

	if model != "claude-3" {
		t.Errorf("extractModelInfo() model = %q, want claude-3", model)
	}
	if maxTokens != 4096 {
		t.Errorf("extractModelInfo() maxTokens = %d, want 4096", maxTokens)
	}
}

func TestExtractModelInfo_InvalidJSON(t *testing.T) {
	t.Parallel()

	body := []byte(`not json`)
	model, maxTokens := extractModelInfo(body)

	if model != "" {
		t.Errorf("extractModelInfo(invalid) model = %q, want empty", model)
	}
	if maxTokens != 0 {
		t.Errorf("extractModelInfo(invalid) maxTokens = %d, want 0", maxTokens)
	}
}

func TestTruncateBody_UnderLimit(t *testing.T) {
	t.Parallel()

	body := []byte("short")
	got := truncateBody(body, 100)

	if string(got) != "short" {
		t.Errorf("truncateBody(short) = %q, want short", string(got))
	}
}

func TestTruncateBody_OverLimit(t *testing.T) {
	t.Parallel()

	body := []byte("this is a long body")
	got := truncateBody(body, 10)

	if len(got) != 10 {
		t.Errorf("truncateBody() length = %d, want 10", len(got))
	}
	if string(got) != "this is a " {
		t.Errorf("truncateBody() = %q, want 'this is a '", string(got))
	}
}

func TestTLSVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want    string
		version uint16
	}{
		{want: "TLS 1.0", version: 0x0301},
		{want: "TLS 1.1", version: 0x0302},
		{want: "TLS 1.2", version: 0x0303},
		{want: "TLS 1.3", version: 0x0304},
		{want: "unknown", version: 0x1234},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("0x%04x", tc.version), func(t *testing.T) {
			t.Parallel()
			got := tlsVersionString(tc.version)
			if got != tc.want {
				t.Errorf("tlsVersionString(0x%04x) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

// Test LogProxyMetrics

func TestLogProxyMetrics_NoData(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := Metrics{
		BackendTime:     0,
		TotalTime:       0,
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

	LogProxyMetrics(ctx, metrics, opts)

	output := buf.String()
	if !strings.Contains(output, "proxy metrics") {
		t.Error("Expected 'proxy metrics' in output")
	}
}

func TestLogProxyMetrics_WithData(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := Metrics{
		BackendTime:     100 * time.Millisecond,
		TotalTime:       200 * time.Millisecond,
		BytesSent:       1024,
		BytesReceived:   2048,
		StreamingEvents: 15,
	}

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}

	LogProxyMetrics(ctx, metrics, opts)

	output := buf.String()
	if !strings.Contains(output, "100") {
		t.Errorf("Expected backend time in output, got: %s", output)
	}
	if !strings.Contains(output, "bytes_sent") {
		t.Error("Expected bytes_sent in output")
	}
	if !strings.Contains(output, "streaming_events") {
		t.Error("Expected streaming_events in output")
	}
}

// Test LogResponseDetails

func TestLogResponseDetails_NotEnabled(t *testing.T) {
	t.Parallel()

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	LogResponseDetails(ctx, nil, 200, 5, opts)

	if buf.String() != "" {
		t.Error("LogResponseDetails should not log when disabled")
	}
}

func TestLogResponseDetails_WithHeaders(t *testing.T) {
	t.Parallel()

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: true,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	headers := http.Header{}
	headers.Set("X-Anthropic-Model", "claude-3")
	headers.Set("Content-Type", providers.ContentTypeSSE)

	LogResponseDetails(ctx, headers, 200, 10, opts)

	output := buf.String()
	if !strings.Contains(output, "response details") {
		t.Error("Expected 'response details' in output")
	}
	if !strings.Contains(output, "streaming_events") {
		t.Error("Expected streaming_events in output")
	}
}

// Test AttachTLSTrace

func TestAttachTLSTrace_MetricsStruct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, getMetrics := AttachTLSTrace(ctx, nil)

	metrics := getMetrics()

	// Check zero values before callbacks
	if metrics.Version != "" {
		t.Errorf("Version = %q, want empty", metrics.Version)
	}
	if metrics.DNSTime != 0 {
		t.Errorf("DNSTime = %v, want 0", metrics.DNSTime)
	}
	if metrics.HasMetrics {
		t.Error("HasMetrics should be false before callbacks")
	}
}

// Test NewModelsHandler_NilProviders

func TestNewModelsHandler_NilProviders(t *testing.T) {
	t.Parallel()

	h := NewModelsHandler(nil)
	if h == nil {
		t.Fatal("NewModelsHandler(nil) returned nil")
	}

	providersList := h.providerList()
	if providersList != nil {
		t.Errorf("providerList() = %v, want nil", providersList)
	}
}

// Test NewProvidersHandler_NilProviders

func TestNewProvidersHandler_NilProviders(t *testing.T) {
	t.Parallel()

	h := NewProvidersHandler(nil)
	if h == nil {
		t.Fatal("NewProvidersHandler(nil) returned nil")
	}

	providersList := h.providerList()
	if providersList != nil {
		t.Errorf("providerList() = %v, want nil", providersList)
	}
}

// Test handler methods with proper mock setup

// newTestHandler creates a minimal Handler for testing.
// Only debugOpts is set from the parameter; all other fields are zero/nil.
// Use when testing handler methods that don't require full setup.
func newTestHandler(t *testing.T, debugOpts config.DebugOptions) *Handler {
	t.Helper()
	return &Handler{
		router:           nil,
		runtimeCfg:       nil,
		defaultProvider:  nil,
		routingConfig:    nil,
		healthTracker:    nil,
		signatureCache:   nil,
		providerProxies:  nil,
		providers:        nil,
		getProviderPools: nil,
		getProviderKeys:  nil,
		providerPools:    nil,
		providerKeys:     nil,
		debugOpts:        debugOpts,
		proxyMu:          sync.RWMutex{},
		routingDebug:     false,
	}
}

func TestRewriteModelIfNeeded_NoMapping(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	provider := newMockProvider("test")

	body := `{"model":"claude-3"}`
	req := httptest.NewRequestWithContext(
		context.Background(), "POST", "/v1/messages",
		strings.NewReader(body))
	handler := newTestHandler(t, config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	})

	handler.rewriteModelIfNeeded(req, &logger, provider)
	// Should not panic
}

func TestAttachTLSTraceIfEnabled_Disabled(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	newReq, getMetrics := handler.attachTLSTraceIfEnabled(req)

	if newReq != req {
		t.Error("attachTLSTraceIfEnabled() should return original request when disabled")
	}
	if getMetrics != nil {
		t.Error("attachTLSTraceIfEnabled() should return nil getMetrics when disabled")
	}
}

func TestAttachTLSTraceIfEnabled_Enabled(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      true,
		MaxBodyLogSize:     0,
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	newReq, getMetrics := handler.attachTLSTraceIfEnabled(req)

	if newReq == req {
		t.Error("attachTLSTraceIfEnabled() should return new context when enabled")
	}
	if getMetrics == nil {
		t.Error("attachTLSTraceIfEnabled() should return getMetrics when enabled")
	}
}

func TestLogMetricsIfEnabled_Disabled(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	handler := newTestHandler(t, config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	start := time.Now()

	handler.logMetricsIfEnabled(req, &logger, start, 100*time.Millisecond, nil)
	// Should not panic
}

func TestLogMetricsIfEnabled_WithTLSMetrics(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	handler := newTestHandler(t, config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      true,
		MaxBodyLogSize:     0,
	})

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	reqCtx := logger.WithContext(req.Context())
	req = req.WithContext(reqCtx)
	start := time.Now()

	getMetrics := func() TLSMetrics {
		return TLSMetrics{
			Version:     "TLS 1.3",
			DNSTime:     50 * time.Millisecond,
			ConnectTime: 100 * time.Millisecond,
			TLSTime:     75 * time.Millisecond,
			Reused:      false,
			HasMetrics:  true,
		}
	}

	handler.logMetricsIfEnabled(req, &logger, start, 200*time.Millisecond, getMetrics)

	output := buf.String()
	if !strings.Contains(output, "tls metrics") {
		t.Error("Expected TLS metrics in output")
	}
	if !strings.Contains(output, "proxy metrics") {
		t.Error("Expected proxy metrics in output")
	}
}

// Test selectProviderWithTracking

// selectProviderWithTracking is tested indirectly through handler integration tests.

// Test surgicalUpdate (thinking.go)

func TestSurgicalUpdate(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"messages": [{
			"role": "user",
			"content": [
				{"type": "thinking", "id": "th1", "signature": "old_sig"},
				{"type": "text", "text": "hello"}
			]
		}]
	}`)

	results := []thinkingBlockResult{
		{signature: "new_sig", blockIndex: 0, keep: true},
	}

	modified, dropped, err := surgicalUpdate(body, 0, results, nil)
	if err != nil {
		t.Fatalf("surgicalUpdate() error: %v", err)
	}
	if dropped {
		t.Error("surgicalUpdate() should not drop message")
	}

	if !gjson.GetBytes(modified, "messages.0.content.0.signature").Exists() {
		t.Error("surgicalUpdate() should preserve signature field")
	}

	newSig := gjson.GetBytes(modified, "messages.0.content.0.signature").String()
	if newSig != "new_sig" {
		t.Errorf("surgicalUpdate() signature = %q, want new_sig", newSig)
	}
}

func TestSurgicalUpdate_RemoveSignature(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"messages": [{
			"role": "assistant",
			"content": [
				{"type": "tool_use", "id": "t1", "name": "test", "signature": "must_remove"}
			]
		}]
	}`)

	modified, dropped, err := surgicalUpdate(body, 0, nil, []int64{0})
	if err != nil {
		t.Fatalf("surgicalUpdate() error: %v", err)
	}
	if dropped {
		t.Error("surgicalUpdate() should not drop message when removing signature")
	}

	// After removing signature, the field should not exist
	if gjson.GetBytes(modified, "messages.0.content.0.signature").Exists() {
		t.Error("surgicalUpdate() should remove signature from tool_use")
	}
}

// Test sjson helpers

func TestSjsonSetBytes(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-3","max_tokens":1000}`)

	updated, err := sjson.SetBytes(body, "model", "claude-3.5")
	if err != nil {
		t.Fatalf("sjson.SetBytes() error: %v", err)
	}

	if gjson.GetBytes(updated, "model").String() != "claude-3.5" {
		t.Errorf("sjson.SetBytes() model = %q, want claude-3.5", gjson.GetBytes(updated, "model").String())
	}

	// Max tokens should be preserved
	if gjson.GetBytes(updated, "max_tokens").Int() != 1000 {
		t.Error("sjson.SetBytes() should preserve other fields")
	}
}

func TestSjsonSetBytes_NestedPath(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-3","kwargs":{"max_tokens":1000}}`)

	updated, err := sjson.SetBytes(body, "kwargs.max_tokens", 4096)
	if err != nil {
		t.Fatalf("sjson.SetBytes() error: %v", err)
	}

	if gjson.GetBytes(updated, "kwargs.max_tokens").Int() != 4096 {
		t.Errorf("sjson.SetBytes() max_tokens = %d, want 4096", gjson.GetBytes(updated, "kwargs.max_tokens").Int())
	}
}

// Test logRequestStart (indirectly through middleware helpers)

func TestLogRequestStart_NoDebug(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	debugOpts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}

	// Use the internal logRequestStart via a test that calls it
	var buf strings.Builder
	logger := zerolog.New(&buf)
	ctx = logger.WithContext(ctx)

	logRequestStart(ctx, req, "test-id", debugOpts)

	if !strings.Contains(buf.String(), "POST /v1/messages") {
		t.Error("logRequestStart() should log method and path")
	}
}

func TestLogRequestStart_WithDebug(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	body := `{"test":"data"}`
	req := httptest.NewRequestWithContext(
		context.Background(), "POST", "/v1/messages", strings.NewReader(body))
	debugOpts := config.DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     200,
	}

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx = logger.WithContext(ctx)

	logRequestStart(ctx, req, "test-id", debugOpts)

	output := buf.String()
	if !strings.Contains(output, "POST /v1/messages") {
		t.Error("logRequestStart() should log method and path")
	}
	if !strings.Contains(output, "body_preview") {
		t.Error("logRequestStart() with debug should include body preview")
	}
}

// Test redactSensitiveFields

func TestRedactSensitiveFields(t *testing.T) {
	t.Parallel()

	body := `{"api_key":"secret123","x-api-key":"key456","bearer":"token789",` +
		`"password":"pass","token":"t","secret":"s","authorization":"auth_value","other":"value"}`
	got := redactSensitiveFields(body)

	// Check that "REDACTED" appears (signifies redaction happened)
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("redactSensitiveFields() should contain REDACTED, got: %s", got)
	}

	// Check non-sensitive field is preserved
	if !strings.Contains(got, `"other":"value"`) {
		t.Error("redactSensitiveFields() should preserve non-sensitive fields")
	}
}

// Test formatCompletionMessage indirectly via logRequestCompletion

func TestLogRequestCompletion(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	ctx := logger.WithContext(context.Background())
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)

	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     200,
		sseEvents:      0,
		isStreaming:    false,
	}

	logRequestCompletion(ctx, req, wrapped, 100*time.Millisecond, "test-id")
	// Should not panic
}

func TestLogRequestCompletion_500(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := zerolog.New(&buf)
	ctx := logger.WithContext(context.Background())

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     500,
		sseEvents:      0,
		isStreaming:    false,
	}

	logRequestCompletion(ctx, req, wrapped, 100*time.Millisecond, "test-id")

	output := buf.String()
	if !strings.Contains(output, "error") || !strings.Contains(output, "500") {
		t.Errorf("logRequestCompletion() should log error for 500 status, got: %s", output)
	}
}

func TestLogRequestCompletion_Streaming(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := zerolog.New(&buf)
	ctx := logger.WithContext(context.Background())

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages", http.NoBody)
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     200,
		sseEvents:      15,
		isStreaming:    true,
	}

	logRequestCompletion(ctx, req, wrapped, 100*time.Millisecond, "test-id")

	output := buf.String()
	if !strings.Contains(output, "sse_events") {
		t.Error("logRequestCompletion() should log SSE event count for streaming")
	}
	if !strings.Contains(output, "15") {
		t.Error("logRequestCompletion() should include event count")
	}
}

// Test withRequestFields helper

func TestWithRequestFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	ctx = logger.WithContext(ctx)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/messages?test=1", http.NoBody)
	req.RequestURI = "/v1/messages?test=1"

	resultCtx := withRequestFields(ctx, req, "test-id")

	// Just verify it doesn't panic and returns a valid context
	finalLogger := resultCtx.Logger()
	_ = finalLogger // ensure context produced a logger
}

// Test LogTLSMetrics

func TestLogTLSMetrics_Disabled(t *testing.T) {
	t.Parallel()

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}

	var buf strings.Builder
	logger := zerolog.New(&buf)
	ctx := logger.WithContext(context.Background())

	metrics := TLSMetrics{
		Version:     "TLS 1.3",
		DNSTime:     50 * time.Millisecond,
		ConnectTime: 100 * time.Millisecond,
		TLSTime:     75 * time.Millisecond,
		Reused:      false,
		HasMetrics:  true,
	}

	LogTLSMetrics(ctx, metrics, opts)

	if buf.String() != "" {
		t.Error("LogTLSMetrics() should not log when disabled")
	}
}

func TestLogTLSMetrics_Enabled(t *testing.T) {
	t.Parallel()

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      true,
		MaxBodyLogSize:     0,
	}

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := TLSMetrics{
		Version:     "TLS 1.3",
		DNSTime:     50 * time.Millisecond,
		ConnectTime: 100 * time.Millisecond,
		TLSTime:     75 * time.Millisecond,
		Reused:      true,
		HasMetrics:  true,
	}

	LogTLSMetrics(ctx, metrics, opts)

	output := buf.String()
	if !strings.Contains(output, "tls metrics") {
		t.Error("LogTLSMetrics() should log when enabled")
	}
	if !strings.Contains(output, "TLS 1.3") {
		t.Error("LogTLSMetrics() should include TLS version")
	}
}

func TestLogTLSMetrics_NoMetrics(t *testing.T) {
	t.Parallel()

	opts := config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      true,
		MaxBodyLogSize:     0,
	}

	var buf strings.Builder
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	metrics := TLSMetrics{
		Version:     "",
		DNSTime:     0,
		ConnectTime: 0,
		TLSTime:     0,
		Reused:      false,
		HasMetrics:  false,
	}

	LogTLSMetrics(ctx, metrics, opts)

	if buf.String() != "" {
		t.Error("LogTLSMetrics() should not log when HasMetrics is false")
	}
}

// Test AttachTLSTrace callback execution

func TestAttachTLSTrace_Callbacks(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), "GET", "https://example.com", http.NoBody)
	newCtx, getMetrics := AttachTLSTrace(req.Context(), req)

	// Extract the trace and fire callbacks
	trace := httptrace.ContextClientTrace(newCtx)
	if trace == nil {
		t.Fatal("AttachTLSTrace() did not attach trace to context")
	}

	// Fire DNS callbacks
	trace.DNSStart(httptrace.DNSStartInfo{Host: "example.com"})
	trace.DNSDone(httptrace.DNSDoneInfo{})

	// Fire Connect callbacks
	trace.ConnectStart("tcp", "127.0.0.1:443")
	trace.ConnectDone("tcp", "127.0.0.1:443", nil)

	// Fire TLS callbacks
	trace.TLSHandshakeStart()
	trace.TLSHandshakeDone(tls.ConnectionState{
		Version:   tls.VersionTLS13,
		DidResume: false,
	}, nil)

	tlsMetrics := getMetrics()
	if !tlsMetrics.HasMetrics {
		t.Error("AttachTLSTrace() HasMetrics should be true after TLS handshake")
	}
	if tlsMetrics.Version != "TLS 1.3" {
		t.Errorf("AttachTLSTrace() Version = %q, want TLS 1.3", tlsMetrics.Version)
	}
	if tlsMetrics.DNSTime == 0 {
		t.Error("AttachTLSTrace() DNSTime should be non-zero")
	}
	if tlsMetrics.ConnectTime == 0 {
		t.Error("AttachTLSTrace() ConnectTime should be non-zero")
	}
	if tlsMetrics.TLSTime == 0 {
		t.Error("AttachTLSTrace() TLSTime should be non-zero")
	}
}

func TestAttachTLSTrace_ReusedConnection(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), "GET", "https://example.com", http.NoBody)
	newCtx, getMetrics := AttachTLSTrace(req.Context(), req)

	trace := httptrace.ContextClientTrace(newCtx)

	// Fire TLS with DidResume = true
	trace.TLSHandshakeStart()
	trace.TLSHandshakeDone(tls.ConnectionState{
		Version:   tls.VersionTLS12,
		DidResume: true,
	}, nil)

	m := getMetrics()
	if !m.Reused {
		t.Error("AttachTLSTrace() Reused should be true when DidResume is true")
	}
	if m.Version != "TLS 1.2" {
		t.Errorf("AttachTLSTrace() Version = %q, want TLS 1.2", m.Version)
	}
}

// Test parseRetryAfter

func TestParseRetryAfter_Empty(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	d := parseRetryAfter(h)
	if d != 60*time.Second {
		t.Errorf("parseRetryAfter(empty) = %v, want 60s", d)
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("Retry-After", "30")
	d := parseRetryAfter(h)
	if d != 30*time.Second {
		t.Errorf("parseRetryAfter(30) = %v, want 30s", d)
	}
}

func TestParseRetryAfter_InvalidValue(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Set("Retry-After", "not-a-number-or-date")
	d := parseRetryAfter(h)
	if d != 60*time.Second {
		t.Errorf("parseRetryAfter(invalid) = %v, want 60s default", d)
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(120 * time.Second)
	h := http.Header{}
	h.Set("Retry-After", future.UTC().Format(http.TimeFormat))
	d := parseRetryAfter(h)
	// Should be roughly 120 seconds (allow 5 second tolerance)
	if d < 115*time.Second || d > 125*time.Second {
		t.Errorf("parseRetryAfter(http-date) = %v, want ~120s", d)
	}
}

func TestParseRetryAfter_PastHTTPDate(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-60 * time.Second)
	h := http.Header{}
	h.Set("Retry-After", past.UTC().Format(http.TimeFormat))
	d := parseRetryAfter(h)
	if d != 60*time.Second {
		t.Errorf("parseRetryAfter(past date) = %v, want 60s default", d)
	}
}

// rewriteModelIfNeeded with mapping is tested in handler_test.go.
// formatLevel, formatMessage, formatFieldName, buildConsoleWriter are tested in logger_format_test.go
