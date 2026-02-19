package proxy_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samber/ro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// sseEvent creates an SSEEvent with default zero values for unset fields.
// This reduces duplication from exhaustruct-required field initialization.
func sseEvent(event string, data []byte) proxy.SSEEvent {
	return proxy.SSEEvent{Event: event, ID: "", Data: data, Retry: 0}
}

// sseEventFull creates an SSEEvent with all fields specified.
func sseEventFull(event, id string, data []byte, retry int) proxy.SSEEvent {
	return proxy.SSEEvent{Event: event, ID: id, Data: data, Retry: retry}
}

// testEventData defines a single SSE event test case.
type testEventData struct {
	name     string
	expected string
	event    proxy.SSEEvent
}

// sseEventTestCases returns all test cases for SSEEvent.String() tests.
func sseEventTestCases() []testEventData {
	return []testEventData{
		{
			name:     "empty event",
			event:    proxy.SSEEvent{Event: "", ID: "", Data: nil, Retry: 0},
			expected: "\n",
		},
		{
			name:     "data only",
			event:    sseEvent("", []byte("hello")),
			expected: "data: hello\n\n",
		},
		{
			name:     "event with type",
			event:    sseEvent("message", []byte("hello")),
			expected: "event: message\ndata: hello\n\n",
		},
		{
			name:     "event with id",
			event:    sseEventFull("message", "123", []byte("hello"), 0),
			expected: "event: message\nid: 123\ndata: hello\n\n",
		},
		{
			name:     "event with retry",
			event:    sseEventFull("message", "", []byte("hello"), 3000),
			expected: "event: message\nretry: 3000\ndata: hello\n\n",
		},
		{
			name:     "multiline data",
			event:    sseEvent("message", []byte("line1\nline2\nline3")),
			expected: "event: message\ndata: line1\ndata: line2\ndata: line3\n\n",
		},
		{
			name:     "full event",
			event:    sseEventFull("message", "456", []byte("hello"), 5000),
			expected: "event: message\nid: 456\nretry: 5000\ndata: hello\n\n",
		},
	}
}

func TestSSEEventString(t *testing.T) {
	t.Parallel()
	for _, testCase := range sseEventTestCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, testCase.event.String())
			assert.Equal(t, []byte(testCase.expected), testCase.event.Bytes())
		})
	}
}

// streamSSETestCase defines a single StreamSSE test case.
type streamSSETestCase struct {
	name     string
	input    string
	expected []proxy.SSEEvent
}

// streamSSETestCases returns all test cases for StreamSSE tests.
func streamSSETestCases() []streamSSETestCase {
	return []streamSSETestCase{
		{
			name:     "single event",
			input:    "data: hello\n\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("hello"))},
		},
		{
			name:     "event with type",
			input:    "event: message\ndata: hello\n\n",
			expected: []proxy.SSEEvent{sseEvent("message", []byte("hello"))},
		},
		{
			name:     "event with id",
			input:    "id: 123\ndata: hello\n\n",
			expected: []proxy.SSEEvent{sseEventFull("", "123", []byte("hello"), 0)},
		},
		{
			name:     "event with retry",
			input:    "retry: 3000\ndata: hello\n\n",
			expected: []proxy.SSEEvent{sseEventFull("", "", []byte("hello"), 3000)},
		},
		{
			name:     "multiline data",
			input:    "data: line1\ndata: line2\n\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("line1\nline2"))},
		},
		{
			name:  "multiple events",
			input: "data: first\n\ndata: second\n\n",
			expected: []proxy.SSEEvent{
				sseEvent("", []byte("first")),
				sseEvent("", []byte("second")),
			},
		},
		{
			name:     "comment ignored",
			input:    ": this is a comment\ndata: hello\n\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("hello"))},
		},
		{
			name:     "field without value",
			input:    "event\ndata: hello\n\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("hello"))},
		},
		{
			name:     "value with leading space",
			input:    "data: hello\n\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("hello"))},
		},
		{
			name:     "value without leading space",
			input:    "data:hello\n\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("hello"))},
		},
		{
			name:     "CRLF line endings",
			input:    "data: hello\r\n\r\n",
			expected: []proxy.SSEEvent{sseEvent("", []byte("hello"))},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
	}
}

func TestStreamSSE(t *testing.T) {
	t.Parallel()
	for _, testCase := range streamSSETestCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			reader := strings.NewReader(testCase.input)
			events, err := ro.Collect(proxy.StreamSSE(reader))
			require.NoError(t, err)
			if testCase.expected == nil {
				assert.Empty(t, events)
			} else {
				assert.Equal(t, testCase.expected, events)
			}
		})
	}
}

func TestStreamSSEPendingEventAtEOF(t *testing.T) {
	t.Parallel()

	input := "data: hello"
	reader := strings.NewReader(input)

	events, err := ro.Collect(proxy.StreamSSE(reader))
	require.NoError(t, err)

	expected := []proxy.SSEEvent{sseEvent("", []byte("hello"))}
	assert.Equal(t, expected, events)
}

func TestStreamSSEReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read error")
	reader := &errorReader{err: readErr}

	_, err := ro.Collect(proxy.StreamSSE(reader))
	assert.Error(t, err)
	assert.Equal(t, readErr, err)
}

type errorReader struct {
	err error
}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func TestForwardSSE(t *testing.T) {
	t.Parallel()

	events := []proxy.SSEEvent{
		sseEvent("message", []byte("first")),
		sseEvent("message", []byte("second")),
	}

	source := ro.FromSlice(events)
	rec := httptest.NewRecorder()
	err := proxy.ForwardSSE(source, rec)
	require.NoError(t, err)

	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-transform", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))

	expected := "event: message\ndata: first\n\nevent: message\ndata: second\n\n"
	assert.Equal(t, expected, rec.Body.String())
}

func TestForwardSSENotFlushable(t *testing.T) {
	t.Parallel()

	events := ro.FromSlice([]proxy.SSEEvent{sseEvent("", []byte("hello"))})
	writer := &nonFlushableWriter{
		ResponseWriter: nil,
	}

	err := proxy.ForwardSSE(events, writer)
	assert.Error(t, err)
	assert.Equal(t, proxy.ErrNotFlushable, err)
}

type nonFlushableWriter struct {
	ResponseWriter http.ResponseWriter
}

func (w *nonFlushableWriter) Header() http.Header {
	return http.Header{}
}

func (w *nonFlushableWriter) Write(_ []byte) (int, error) {
	return 0, nil
}

func (w *nonFlushableWriter) WriteHeader(_ int) {}

func TestForwardSSEWriteError(t *testing.T) {
	t.Parallel()

	events := ro.FromSlice([]proxy.SSEEvent{sseEvent("", []byte("hello"))})
	writeErr := errors.New("write error")
	writer := &errorWriter{
		headers: http.Header{},
		err:     writeErr,
	}

	err := proxy.ForwardSSE(events, writer)
	assert.Error(t, err)
	assert.Equal(t, writeErr, err)
}

type errorWriter struct {
	headers http.Header
	err     error
}

func (w *errorWriter) Header() http.Header {
	return w.headers
}

func (w *errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func (w *errorWriter) WriteHeader(_ int) {}

func (w *errorWriter) Flush() {}

func TestWriteSSEEvent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	event := sseEvent("ping", []byte("pong"))

	err := proxy.WriteSSEEvent(rec, event)
	require.NoError(t, err)

	expected := "event: ping\ndata: pong\n\n"
	assert.Equal(t, expected, rec.Body.String())
}

func TestWriteSSEEventNotFlushable(t *testing.T) {
	t.Parallel()

	writer := &nonFlushableWriter{
		ResponseWriter: nil,
	}
	event := sseEvent("", []byte("hello"))

	err := proxy.WriteSSEEvent(writer, event)
	assert.Error(t, err)
	assert.Equal(t, proxy.ErrNotFlushable, err)
}

// TestSetSSEHeaders is defined in sse_test.go
// The SetSSEHeaders function takes http.Header, so use SetSSEHeaders(w.Header())

func TestFilterEvents(t *testing.T) {
	t.Parallel()

	events := []proxy.SSEEvent{
		sseEvent("message", []byte("1")),
		sseEvent("ping", []byte("2")),
		sseEvent("message", []byte("3")),
		sseEvent("error", []byte("4")),
	}

	source := ro.FromSlice(events)
	filtered := ro.Pipe1(source, proxy.FilterEvents("message"))

	results, err := ro.Collect(filtered)
	require.NoError(t, err)

	expected := []proxy.SSEEvent{
		sseEvent("message", []byte("1")),
		sseEvent("message", []byte("3")),
	}
	assert.Equal(t, expected, results)
}

func TestFilterEventsByPrefix(t *testing.T) {
	t.Parallel()

	events := []proxy.SSEEvent{
		sseEvent("content_block_start", []byte("1")),
		sseEvent("message_delta", []byte("2")),
		sseEvent("content_block_delta", []byte("3")),
		sseEvent("content_block_stop", []byte("4")),
	}

	source := ro.FromSlice(events)
	filtered := ro.Pipe1(source, proxy.FilterEventsByPrefix("content_block_"))

	results, err := ro.Collect(filtered)
	require.NoError(t, err)

	expected := []proxy.SSEEvent{
		sseEvent("content_block_start", []byte("1")),
		sseEvent("content_block_delta", []byte("3")),
		sseEvent("content_block_stop", []byte("4")),
	}
	assert.Equal(t, expected, results)
}

func TestMapEventData(t *testing.T) {
	t.Parallel()

	events := []proxy.SSEEvent{
		sseEvent("message", []byte("hello")),
		sseEvent("message", []byte("world")),
	}

	source := ro.FromSlice(events)
	transformed := ro.Pipe1(source, proxy.MapEventData(bytes.ToUpper))

	results, err := ro.Collect(transformed)
	require.NoError(t, err)

	expected := []proxy.SSEEvent{
		sseEvent("message", []byte("HELLO")),
		sseEvent("message", []byte("WORLD")),
	}
	assert.Equal(t, expected, results)
}

func TestCountEvents(t *testing.T) {
	t.Parallel()

	events := []proxy.SSEEvent{
		sseEvent("", []byte("1")),
		sseEvent("", []byte("2")),
		sseEvent("", []byte("3")),
	}

	source := ro.FromSlice(events)
	counted := ro.Pipe1(source, proxy.CountEvents())

	results, err := ro.Collect(counted)
	require.NoError(t, err)

	assert.Equal(t, []int64{3}, results)
}

func TestStreamSSERoundTrip(t *testing.T) {
	t.Parallel()

	original := []proxy.SSEEvent{
		{Event: "message_start", ID: "", Data: []byte(`{"type":"message_start"}`), Retry: 0},
		{Event: "content_block_delta", ID: "", Data: []byte(`{"delta":"Hello"}`), Retry: 0},
		{Event: "content_block_delta", ID: "", Data: []byte(`{"delta":" world"}`), Retry: 0},
		{Event: "message_stop", ID: "", Data: []byte(`{}`), Retry: 0},
	}

	var buf bytes.Buffer
	for _, e := range original {
		buf.WriteString(e.String())
	}

	parsed, err := ro.Collect(proxy.StreamSSE(&buf))
	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func BenchmarkStreamSSE(b *testing.B) {
	var buf bytes.Buffer
	for i := 0; i < 100; i++ {
		buf.WriteString("event: content_block_delta\n")
		buf.WriteString("data: {\"delta\":\"some text content\"}\n")
		buf.WriteString("\n")
	}
	sseData := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(sseData)
		events, err := ro.Collect(proxy.StreamSSE(reader))
		if err != nil {
			b.Fatal(err)
		}
		_ = events
	}
}

func BenchmarkForwardSSE(b *testing.B) {
	events := make([]proxy.SSEEvent, 100)
	for i := range events {
		events[i] = proxy.SSEEvent{
			Event: "content_block_delta",
			ID:    "",
			Data:  []byte(`{"delta":"some text content"}`),
			Retry: 0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source := ro.FromSlice(events)
		rec := httptest.NewRecorder()
		err := proxy.ForwardSSE(source, rec)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSSEEventString(b *testing.B) {
	event := proxy.SSEEvent{
		Event: "content_block_delta",
		ID:    "msg_123",
		Data:  []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello, world!"}}`),
		Retry: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.String()
	}
}

func TestParseSSEFieldInvalidRetry(t *testing.T) {
	t.Parallel()

	input := "retry: invalid\ndata: hello\n\n"
	reader := strings.NewReader(input)

	events, err := ro.Collect(proxy.StreamSSE(reader))
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, 0, events[0].Retry)
	assert.Equal(t, []byte("hello"), events[0].Data)
}

func TestStreamSSELargeEvent(t *testing.T) {
	t.Parallel()

	largeData := bytes.Repeat([]byte("x"), 10000)
	input := "event: large\ndata: " + string(largeData) + "\n\n"
	reader := strings.NewReader(input)

	events, err := ro.Collect(proxy.StreamSSE(reader))
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, "large", events[0].Event)
	assert.Equal(t, largeData, events[0].Data)
}

func TestStreamSSESubscribeCancel(t *testing.T) {
	t.Parallel()

	input := "data: 1\n\ndata: 2\n\ndata: 3\n\n"
	reader := strings.NewReader(input)

	var received []proxy.SSEEvent
	observable := proxy.StreamSSE(reader)

	subscription := observable.Subscribe(ro.NewObserver(
		func(e proxy.SSEEvent) {
			received = append(received, e)
		},
		func(_ error) {},
		func() {},
	))

	_ = subscription
	assert.Len(t, received, 3)
}

// Ensure SSE stream works with io.Pipe (simulating real streaming).
func TestStreamSSEWithPipe(t *testing.T) {
	t.Parallel()

	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer func() {
			if closeErr := pipeWriter.Close(); closeErr != nil {
				t.Logf("pipe close error: %v", closeErr)
			}
		}()
		if _, err := pipeWriter.Write([]byte("event: start\ndata: begin\n\n")); err != nil {
			return
		}
		if _, writeErr := pipeWriter.Write([]byte("event: end\ndata: done\n\n")); writeErr != nil {
			return
		}
	}()

	events, err := ro.Collect(proxy.StreamSSE(pipeReader))
	require.NoError(t, err)

	expected := []proxy.SSEEvent{
		{Event: "start", ID: "", Data: []byte("begin"), Retry: 0},
		{Event: "end", ID: "", Data: []byte("done"), Retry: 0},
	}
	assert.Equal(t, expected, events)
}
