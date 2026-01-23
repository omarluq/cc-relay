package proxy

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
)

func TestSSEEvent_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		event    SSEEvent
	}{
		{
			name:     "empty event",
			event:    SSEEvent{},
			expected: "\n",
		},
		{
			name: "data only",
			event: SSEEvent{
				Data: []byte("hello"),
			},
			expected: "data: hello\n\n",
		},
		{
			name: "event with type",
			event: SSEEvent{
				Event: "message",
				Data:  []byte("hello"),
			},
			expected: "event: message\ndata: hello\n\n",
		},
		{
			name: "event with id",
			event: SSEEvent{
				Event: "message",
				Data:  []byte("hello"),
				ID:    "123",
			},
			expected: "event: message\nid: 123\ndata: hello\n\n",
		},
		{
			name: "event with retry",
			event: SSEEvent{
				Event: "message",
				Data:  []byte("hello"),
				Retry: 3000,
			},
			expected: "event: message\nretry: 3000\ndata: hello\n\n",
		},
		{
			name: "multiline data",
			event: SSEEvent{
				Event: "message",
				Data:  []byte("line1\nline2\nline3"),
			},
			expected: "event: message\ndata: line1\ndata: line2\ndata: line3\n\n",
		},
		{
			name: "full event",
			event: SSEEvent{
				Event: "message",
				Data:  []byte("hello"),
				ID:    "456",
				Retry: 5000,
			},
			expected: "event: message\nid: 456\nretry: 5000\ndata: hello\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.event.String())
			assert.Equal(t, []byte(tt.expected), tt.event.Bytes())
		})
	}
}

func TestStreamSSE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []SSEEvent
	}{
		{
			name:  "single event",
			input: "data: hello\n\n",
			expected: []SSEEvent{
				{Data: []byte("hello")},
			},
		},
		{
			name:  "event with type",
			input: "event: message\ndata: hello\n\n",
			expected: []SSEEvent{
				{Event: "message", Data: []byte("hello")},
			},
		},
		{
			name:  "event with id",
			input: "id: 123\ndata: hello\n\n",
			expected: []SSEEvent{
				{ID: "123", Data: []byte("hello")},
			},
		},
		{
			name:  "event with retry",
			input: "retry: 3000\ndata: hello\n\n",
			expected: []SSEEvent{
				{Retry: 3000, Data: []byte("hello")},
			},
		},
		{
			name:  "multiline data",
			input: "data: line1\ndata: line2\n\n",
			expected: []SSEEvent{
				{Data: []byte("line1\nline2")},
			},
		},
		{
			name:  "multiple events",
			input: "data: first\n\ndata: second\n\n",
			expected: []SSEEvent{
				{Data: []byte("first")},
				{Data: []byte("second")},
			},
		},
		{
			name:  "comment ignored",
			input: ": this is a comment\ndata: hello\n\n",
			expected: []SSEEvent{
				{Data: []byte("hello")},
			},
		},
		{
			name:  "field without value",
			input: "event\ndata: hello\n\n",
			expected: []SSEEvent{
				{Event: "", Data: []byte("hello")},
			},
		},
		{
			name:  "value with leading space",
			input: "data: hello\n\n",
			expected: []SSEEvent{
				{Data: []byte("hello")},
			},
		},
		{
			name:  "value without leading space",
			input: "data:hello\n\n",
			expected: []SSEEvent{
				{Data: []byte("hello")},
			},
		},
		{
			name:  "CRLF line endings",
			input: "data: hello\r\n\r\n",
			expected: []SSEEvent{
				{Data: []byte("hello")},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader := strings.NewReader(tt.input)
			events, err := ro.Collect(StreamSSE(reader))
			require.NoError(t, err)
			if tt.expected == nil {
				assert.Empty(t, events)
			} else {
				assert.Equal(t, tt.expected, events)
			}
		})
	}
}

func TestStreamSSE_PendingEventAtEOF(t *testing.T) {
	t.Parallel()

	// Event without trailing empty line
	input := "data: hello"
	reader := strings.NewReader(input)

	events, err := ro.Collect(StreamSSE(reader))
	require.NoError(t, err)
	assert.Equal(t, []SSEEvent{{Data: []byte("hello")}}, events)
}

func TestStreamSSE_ReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read error")
	reader := &errorReader{err: readErr}

	_, err := ro.Collect(StreamSSE(reader))
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

	events := []SSEEvent{
		{Event: "message", Data: []byte("first")},
		{Event: "message", Data: []byte("second")},
	}

	source := ro.FromSlice(events)

	rec := httptest.NewRecorder()
	err := ForwardSSE(source, rec)
	require.NoError(t, err)

	// Check headers
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-transform", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))

	// Check body
	expected := "event: message\ndata: first\n\nevent: message\ndata: second\n\n"
	assert.Equal(t, expected, rec.Body.String())
}

func TestForwardSSE_NotFlushable(t *testing.T) {
	t.Parallel()

	events := ro.FromSlice([]SSEEvent{{Data: []byte("hello")}})
	writer := &nonFlushableWriter{}

	err := ForwardSSE(events, writer)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFlushable, err)
}

type nonFlushableWriter struct {
	http.ResponseWriter
}

func (w *nonFlushableWriter) Header() http.Header {
	return http.Header{}
}

func (w *nonFlushableWriter) Write(_ []byte) (int, error) {
	return 0, nil
}

func (w *nonFlushableWriter) WriteHeader(_ int) {}

func TestForwardSSE_WriteError(t *testing.T) {
	t.Parallel()

	events := ro.FromSlice([]SSEEvent{{Data: []byte("hello")}})
	writeErr := errors.New("write error")
	writer := &errorWriter{err: writeErr}

	err := ForwardSSE(events, writer)
	assert.Error(t, err)
	assert.Equal(t, writeErr, err)
}

type errorWriter struct {
	headers http.Header
	err     error
}

func (w *errorWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = http.Header{}
	}
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
	event := SSEEvent{Event: "ping", Data: []byte("pong")}

	err := WriteSSEEvent(rec, event)
	require.NoError(t, err)

	expected := "event: ping\ndata: pong\n\n"
	assert.Equal(t, expected, rec.Body.String())
}

func TestWriteSSEEvent_NotFlushable(t *testing.T) {
	t.Parallel()

	writer := &nonFlushableWriter{}
	event := SSEEvent{Data: []byte("hello")}

	err := WriteSSEEvent(writer, event)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFlushable, err)
}

// TestSetSSEHeaders is defined in sse_test.go
// The SetSSEHeaders function takes http.Header, so use SetSSEHeaders(w.Header())

func TestFilterEvents(t *testing.T) {
	t.Parallel()

	events := []SSEEvent{
		{Event: "message", Data: []byte("1")},
		{Event: "ping", Data: []byte("2")},
		{Event: "message", Data: []byte("3")},
		{Event: "error", Data: []byte("4")},
	}

	source := ro.FromSlice(events)
	filtered := ro.Pipe1(source, FilterEvents("message"))

	results, err := ro.Collect(filtered)
	require.NoError(t, err)

	expected := []SSEEvent{
		{Event: "message", Data: []byte("1")},
		{Event: "message", Data: []byte("3")},
	}
	assert.Equal(t, expected, results)
}

func TestFilterEventsByPrefix(t *testing.T) {
	t.Parallel()

	events := []SSEEvent{
		{Event: "content_block_start", Data: []byte("1")},
		{Event: "message_delta", Data: []byte("2")},
		{Event: "content_block_delta", Data: []byte("3")},
		{Event: "content_block_stop", Data: []byte("4")},
	}

	source := ro.FromSlice(events)
	filtered := ro.Pipe1(source, FilterEventsByPrefix("content_block_"))

	results, err := ro.Collect(filtered)
	require.NoError(t, err)

	expected := []SSEEvent{
		{Event: "content_block_start", Data: []byte("1")},
		{Event: "content_block_delta", Data: []byte("3")},
		{Event: "content_block_stop", Data: []byte("4")},
	}
	assert.Equal(t, expected, results)
}

func TestMapEventData(t *testing.T) {
	t.Parallel()

	events := []SSEEvent{
		{Event: "message", Data: []byte("hello")},
		{Event: "message", Data: []byte("world")},
	}

	source := ro.FromSlice(events)
	transformed := ro.Pipe1(source, MapEventData(bytes.ToUpper))

	results, err := ro.Collect(transformed)
	require.NoError(t, err)

	expected := []SSEEvent{
		{Event: "message", Data: []byte("HELLO")},
		{Event: "message", Data: []byte("WORLD")},
	}
	assert.Equal(t, expected, results)
}

func TestCountEvents(t *testing.T) {
	t.Parallel()

	events := []SSEEvent{
		{Data: []byte("1")},
		{Data: []byte("2")},
		{Data: []byte("3")},
	}

	source := ro.FromSlice(events)
	counted := ro.Pipe1(source, CountEvents())

	results, err := ro.Collect(counted)
	require.NoError(t, err)

	assert.Equal(t, []int64{3}, results)
}

func TestStreamSSE_RoundTrip(t *testing.T) {
	t.Parallel()

	// Create original events
	original := []SSEEvent{
		{Event: "message_start", Data: []byte(`{"type":"message_start"}`)},
		{Event: "content_block_delta", Data: []byte(`{"delta":"Hello"}`)},
		{Event: "content_block_delta", Data: []byte(`{"delta":" world"}`)},
		{Event: "message_stop", Data: []byte(`{}`)},
	}

	// Convert to wire format
	var buf bytes.Buffer
	for _, e := range original {
		buf.WriteString(e.String())
	}

	// Parse back
	parsed, err := ro.Collect(StreamSSE(&buf))
	require.NoError(t, err)

	// Should match original
	assert.Equal(t, original, parsed)
}

func BenchmarkStreamSSE(b *testing.B) {
	// Create a realistic SSE stream
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
		events, _ := ro.Collect(StreamSSE(reader))
		_ = events
	}
}

func BenchmarkForwardSSE(b *testing.B) {
	events := make([]SSEEvent, 100)
	for i := range events {
		events[i] = SSEEvent{
			Event: "content_block_delta",
			Data:  []byte(`{"delta":"some text content"}`),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source := ro.FromSlice(events)
		rec := httptest.NewRecorder()
		_ = ForwardSSE(source, rec)
	}
}

func BenchmarkSSEEvent_String(b *testing.B) {
	event := SSEEvent{
		Event: "content_block_delta",
		Data:  []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello, world!"}}`),
		ID:    "msg_123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.String()
	}
}

func TestParseSSEField_InvalidRetry(t *testing.T) {
	t.Parallel()

	input := "retry: invalid\ndata: hello\n\n"
	reader := strings.NewReader(input)

	events, err := ro.Collect(StreamSSE(reader))
	require.NoError(t, err)

	// Invalid retry should be ignored, Retry stays 0
	assert.Len(t, events, 1)
	assert.Equal(t, 0, events[0].Retry)
	assert.Equal(t, []byte("hello"), events[0].Data)
}

func TestStreamSSE_LargeEvent(t *testing.T) {
	t.Parallel()

	// Create a large data payload
	largeData := bytes.Repeat([]byte("x"), 10000)
	input := "event: large\ndata: " + string(largeData) + "\n\n"
	reader := strings.NewReader(input)

	events, err := ro.Collect(StreamSSE(reader))
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, "large", events[0].Event)
	assert.Equal(t, largeData, events[0].Data)
}

func TestStreamSSE_SubscribeCancel(t *testing.T) {
	t.Parallel()

	// Create a stream that would produce multiple events
	input := "data: 1\n\ndata: 2\n\ndata: 3\n\n"
	reader := strings.NewReader(input)

	var received []SSEEvent
	observable := StreamSSE(reader)

	// Subscribe and collect only first event
	subscription := observable.Subscribe(ro.NewObserver(
		func(e SSEEvent) {
			received = append(received, e)
		},
		func(_ error) {},
		func() {},
	))

	// Subscription should complete since we read all events
	_ = subscription

	// All events should be received
	assert.Len(t, received, 3)
}

// Ensure SSE stream works with io.Pipe (simulating real streaming).
func TestStreamSSE_WithPipe(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()

	// Write events in a goroutine
	go func() {
		defer pw.Close()
		pw.Write([]byte("event: start\ndata: begin\n\n"))
		pw.Write([]byte("event: end\ndata: done\n\n"))
	}()

	events, err := ro.Collect(StreamSSE(pr))
	require.NoError(t, err)

	expected := []SSEEvent{
		{Event: "start", Data: []byte("begin")},
		{Event: "end", Data: []byte("done")},
	}
	assert.Equal(t, expected, events)
}
