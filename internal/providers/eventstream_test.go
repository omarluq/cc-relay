package providers_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEventStreamMessageValid(t *testing.T) {
	t.Parallel()

	t.Run("parses valid message with headers and payload", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{
			":event-type":   "message_start",
			":content-type": "application/json",
			":message-type": "event",
		}
		payload := []byte(`{"type":"message_start","message":{"id":"msg_123"}}`)

		msg := providers.ExportBuildEventStreamMessage(headers, payload)

		parsed, consumed, err := providers.ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, len(msg), consumed)
		assert.Equal(t, "message_start", parsed.Headers[":event-type"])
		assert.Equal(t, "application/json", parsed.Headers[":content-type"])
		assert.Equal(t, payload, parsed.Payload)
	})

	t.Run("parses message with empty payload", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{
			":event-type": "ping",
		}

		msg := providers.ExportBuildEventStreamMessage(headers, []byte{})

		parsed, consumed, err := providers.ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, len(msg), consumed)
		assert.Equal(t, "ping", parsed.Headers[":event-type"])
		assert.Empty(t, parsed.Payload)
	})

	t.Run("parses message with empty headers", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{}
		payload := []byte(`{"data":"test"}`)

		msg := providers.ExportBuildEventStreamMessage(headers, payload)

		parsed, consumed, err := providers.ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, len(msg), consumed)
		assert.Empty(t, parsed.Headers)
		assert.Equal(t, payload, parsed.Payload)
	})
}

func TestParseEventStreamMessageErrors(t *testing.T) {
	t.Parallel()

	t.Run("returns error for message too short", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 10) // Less than minimum

		_, _, err := providers.ParseEventStreamMessage(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
	})

	t.Run("returns error for incomplete message", func(t *testing.T) {
		t.Parallel()
		// Valid prelude but total length exceeds data

		data := make([]byte, 20)
		binary.BigEndian.PutUint32(data[0:4], 100) // Total length = 100
		binary.BigEndian.PutUint32(data[4:8], 0)   // Headers length = 0

		_, _, err := providers.ParseEventStreamMessage(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incomplete message")
	})

	t.Run("returns error for invalid prelude CRC", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{":event-type": "test"}
		msg := providers.ExportBuildEventStreamMessage(headers, []byte("test"))

		// Corrupt prelude CRC
		msg[8] = 0xFF
		msg[9] = 0xFF
		msg[10] = 0xFF
		msg[11] = 0xFF

		_, _, err := providers.ParseEventStreamMessage(msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prelude CRC mismatch")
	})

	t.Run("returns error for invalid message CRC", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{":event-type": "test"}
		msg := providers.ExportBuildEventStreamMessage(headers, []byte("test"))

		// Corrupt message CRC (last 4 bytes)
		msgLen := len(msg)
		msg[msgLen-4] = 0xFF
		msg[msgLen-3] = 0xFF
		msg[msgLen-2] = 0xFF
		msg[msgLen-1] = 0xFF

		_, _, err := providers.ParseEventStreamMessage(msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message CRC mismatch")
	})
}

func TestParseEventStreamMessageConcatenated(t *testing.T) {
	t.Parallel()

	msg1 := providers.ExportBuildEventStreamMessage(
		map[string]string{":event-type": "first"},
		[]byte(`{"seq":1}`),
	)
	msg2 := providers.ExportBuildEventStreamMessage(
		map[string]string{":event-type": "second"},
		[]byte(`{"seq":2}`),
	)

	// Pre-allocate with correct capacity
	combined := make([]byte, 0, len(msg1)+len(msg2))
	combined = append(combined, msg1...)
	combined = append(combined, msg2...)

	// Parse first message
	parsed1, consumed1, err := providers.ParseEventStreamMessage(combined)
	require.NoError(t, err)
	assert.Equal(t, len(msg1), consumed1)
	assert.Equal(t, "first", parsed1.Headers[":event-type"])

	// Parse second message
	parsed2, consumed2, err := providers.ParseEventStreamMessage(combined[consumed1:])
	require.NoError(t, err)
	assert.Equal(t, len(msg2), consumed2)
	assert.Equal(t, "second", parsed2.Headers[":event-type"])
}

func TestParseEventStreamHeaders(t *testing.T) {
	t.Parallel()

	t.Run("parses multiple string headers", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{
			"header1": "value1",
			"header2": "value2",
			"header3": "value3",
		}

		msg := providers.ExportBuildEventStreamMessage(headers, nil)
		parsed, _, err := providers.ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, "value1", parsed.Headers["header1"])
		assert.Equal(t, "value2", parsed.Headers["header2"])
		assert.Equal(t, "value3", parsed.Headers["header3"])
	})

	t.Run("handles empty string values", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{
			"empty": "",
		}

		msg := providers.ExportBuildEventStreamMessage(headers, nil)
		parsed, _, err := providers.ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, "", parsed.Headers["empty"])
	})

	t.Run("handles long header names and values", func(t *testing.T) {
		t.Parallel()

		longName := "x-very-long-header-name-that-exceeds-normal-length"
		longValue := "This is a very long header value that contains lots of text " +
			"and should be handled correctly by the parser"

		headers := map[string]string{
			longName: longValue,
		}

		msg := providers.ExportBuildEventStreamMessage(headers, nil)
		parsed, _, err := providers.ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, longValue, parsed.Headers[longName])
	})
}

func TestFormatSSEEvent(t *testing.T) {
	t.Parallel()

	t.Run("formats simple event", func(t *testing.T) {
		t.Parallel()

		payload := []byte(`{"type":"message_start"}`)
		result := providers.ExportFormatSSEEvent("message_start", payload)

		expected := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n"
		assert.Equal(t, expected, string(result))
	})

	t.Run("handles multi-line payload", func(t *testing.T) {
		t.Parallel()

		payload := []byte("line1\nline2\nline3")
		result := providers.ExportFormatSSEEvent("test", payload)

		expected := "event: test\ndata: line1\ndata: line2\ndata: line3\n\n"
		assert.Equal(t, expected, string(result))
	})

	t.Run("handles empty payload", func(t *testing.T) {
		t.Parallel()

		result := providers.ExportFormatSSEEvent("ping", []byte{})

		expected := "event: ping\ndata: \n\n"
		assert.Equal(t, expected, string(result))
	})
}

// mockResponseWriter implements http.ResponseWriter and http.Flusher for testing.
type mockResponseWriter struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
	flushed    int
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		headers:    make(http.Header),
		body:       bytes.Buffer{},
		statusCode: 0,
		flushed:    0,
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return m.body.Write(b)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func (m *mockResponseWriter) Flush() {
	m.flushed++
}

func TestEventStreamToSSESingleMessage(t *testing.T) {
	t.Parallel()

	t.Run("converts single event stream message to SSE", func(t *testing.T) {
		t.Parallel()
		// Build Event Stream message

		msg := providers.ExportBuildEventStreamMessage(
			map[string]string{
				":event-type":   "message_start",
				":content-type": "application/json",
				":message-type": "event",
			},
			[]byte(`{"type":"message_start","message":{"id":"msg_123"}}`),
		)

		// Create mock response
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}
		resp.Header.Set("Content-Type", "application/vnd.amazon.eventstream")
		resp.Header.Set("x-amzn-request-id", "req-123")

		// Create response writer
		writer := newMockResponseWriter()

		// Convert
		count, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, http.StatusOK, writer.statusCode)

		// Check headers
		assert.Equal(t, "text/event-stream", writer.headers.Get("Content-Type"))
		assert.Equal(t, "no-cache, no-transform", writer.headers.Get("Cache-Control"))
		assert.Equal(t, "no", writer.headers.Get("X-Accel-Buffering"))

		// Check body contains SSE event
		body := writer.body.String()
		assert.Contains(t, body, "event: message_start")
		assert.Contains(t, body, `data: {"type":"message_start"`)

		// Check flushed
		assert.Equal(t, 1, writer.flushed)
	})
}

func TestEventStreamToSSEMultipleMessages(t *testing.T) {
	t.Parallel()

	t.Run("converts multiple event stream messages", func(t *testing.T) {
		t.Parallel()
		// Build multiple Event Stream messages

		msg1 := providers.ExportBuildEventStreamMessage(
			map[string]string{":event-type": "message_start"},
			[]byte(`{"type":"message_start"}`),
		)
		msg2 := providers.ExportBuildEventStreamMessage(
			map[string]string{":event-type": "content_block_start"},
			[]byte(`{"type":"content_block_start","index":0}`),
		)
		msg3 := providers.ExportBuildEventStreamMessage(
			map[string]string{":event-type": "message_stop"},
			[]byte(`{"type":"message_stop"}`),
		)

		// Pre-allocate with correct capacity
		allMsgs := make([]byte, 0, len(msg1)+len(msg2)+len(msg3))
		allMsgs = append(allMsgs, msg1...)
		allMsgs = append(allMsgs, msg2...)
		allMsgs = append(allMsgs, msg3...)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(allMsgs)),
		}

		writer := newMockResponseWriter()

		count, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, 3, count)

		body := writer.body.String()
		assert.Contains(t, body, "event: message_start")
		assert.Contains(t, body, "event: content_block_start")
		assert.Contains(t, body, "event: message_stop")
	})
}

func TestEventStreamToSSEExceptionHandling(t *testing.T) {
	t.Parallel()

	t.Run("handles exception events", func(t *testing.T) {
		t.Parallel()

		msg := providers.ExportBuildEventStreamMessage(
			map[string]string{
				":exception-type": "ValidationException",
				":message-type":   "exception",
			},
			[]byte(`Invalid request parameters`),
		)

		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}

		writer := newMockResponseWriter()

		count, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, 1, count)

		body := writer.body.String()
		assert.Contains(t, body, "event: error")
		assert.Contains(t, body, "ValidationException")
	})
}

func TestEventStreamToSSEEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("skips messages without event type", func(t *testing.T) {
		t.Parallel()
		// Message without :event-type header

		msg := providers.ExportBuildEventStreamMessage(
			map[string]string{":content-type": "application/json"},
			[]byte(`{"type":"unknown"}`),
		)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}

		writer := newMockResponseWriter()

		count, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
		assert.Empty(t, writer.body.String())
	})

	t.Run("handles empty stream", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}

		writer := newMockResponseWriter()

		count, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestEventStreamToSSEHeaderForwarding(t *testing.T) {
	t.Parallel()

	t.Run("preserves non-content headers from response", func(t *testing.T) {
		t.Parallel()

		msg := providers.ExportBuildEventStreamMessage(
			map[string]string{":event-type": "ping"},
			[]byte{},
		)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}
		resp.Header.Set("x-amzn-request-id", "req-456")
		resp.Header.Set("x-custom-header", "custom-value")

		writer := newMockResponseWriter()

		_, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, "req-456", writer.headers.Get("x-amzn-request-id"))
		assert.Equal(t, "custom-value", writer.headers.Get("x-custom-header"))
	})
}

func TestEventStreamToSSEFlushing(t *testing.T) {
	t.Parallel()

	t.Run("returns error when writer does not support flushing", func(t *testing.T) {
		t.Parallel()

		msg := providers.ExportBuildEventStreamMessage(
			map[string]string{":event-type": "message_start"},
			[]byte(`{}`),
		)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}

		// Use a custom non-flushing writer
		nonFlusher := &nonFlushingWriter{
			headers:    make(http.Header),
			body:       bytes.Buffer{},
			statusCode: 0,
		}

		_, err := providers.EventStreamToSSE(resp, nonFlusher)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support flushing")
	})
}

// nonFlushingWriter is a ResponseWriter that doesn't implement Flusher.
type nonFlushingWriter struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
}

func (nfw *nonFlushingWriter) Header() http.Header {
	return nfw.headers
}

func (nfw *nonFlushingWriter) Write(b []byte) (int, error) {
	return nfw.body.Write(b)
}

func (nfw *nonFlushingWriter) WriteHeader(statusCode int) {
	nfw.statusCode = statusCode
}

func TestEventStreamToSSEIntegration(t *testing.T) {
	t.Parallel()

	t.Run("full streaming scenario", func(t *testing.T) {
		t.Parallel()
		// Simulate a complete Bedrock streaming response

		messages := []struct {
			eventType string
			payload   string
		}{
			{
				"message_start",
				`{"type":"message_start","message":{"id":"msg_01","role":"assistant"}}`,
			},
			{
				"content_block_start",
				`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			},
			{
				"content_block_delta",
				`{"type":"content_block_delta","index":0,"delta":{"text":"Hello"}}`,
			},
			{
				"content_block_delta",
				`{"type":"content_block_delta","index":0,"delta":{"text":" world"}}`,
			},
			{
				"content_block_stop",
				`{"type":"content_block_stop","index":0}`,
			},
			{
				"message_delta",
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
			},
			{
				"message_stop",
				`{"type":"message_stop"}`,
			},
		}

		// Pre-allocate with estimated capacity
		allMsgs := make([]byte, 0, len(messages)*100)

		for _, message := range messages {
			msg := providers.ExportBuildEventStreamMessage(
				map[string]string{
					":event-type":   message.eventType,
					":content-type": "application/json",
					":message-type": "event",
				},
				[]byte(message.payload),
			)
			allMsgs = append(allMsgs, msg...)
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(allMsgs)),
		}
		resp.Header.Set("Content-Type", "application/vnd.amazon.eventstream")

		writer := httptest.NewRecorder()

		count, err := providers.EventStreamToSSE(resp, writer)

		require.NoError(t, err)
		assert.Equal(t, len(messages), count)

		body := writer.Body.String()

		// Verify all events are present in order
		for _, message := range messages {
			assert.Contains(t, body, "event: "+message.eventType)
		}

		// Verify SSE format
		assert.Contains(t, body, "data: ")
		assert.Contains(t, body, "\n\n") // Event separators
	})
}

// Test FormatMessageAsSSE

func TestFormatMessageAsSSE_Nil(t *testing.T) {
	t.Parallel()

	result := providers.FormatMessageAsSSE(nil)
	if result != nil {
		t.Errorf("FormatMessageAsSSE(nil) = %v, want nil", result)
	}
}

func TestFormatMessageAsSSE_NormalEvent(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"type":"content_block_delta","index":0}`)
	msg := &providers.EventStreamMessage{
		Headers: map[string]string{
			":event-type": "content_block_delta",
		},
		Payload: payload,
	}

	result := providers.FormatMessageAsSSE(msg)
	if result == nil {
		t.Fatal("FormatMessageAsSSE() returned nil for valid event")
	}

	s := string(result)
	assert.Contains(t, s, "event: content_block_delta\n", "should contain SSE event line with event type")
	assert.Contains(t, s, "data: "+string(payload)+"\n", "should contain SSE data line with payload")
	assert.True(t, strings.HasSuffix(s, "\n\n"), "should terminate with SSE event separator, got %q", s)
}

func TestFormatMessageAsSSE_Exception(t *testing.T) {
	t.Parallel()

	msg := &providers.EventStreamMessage{
		Headers: map[string]string{
			":exception-type": "throttlingException",
		},
		Payload: []byte(`{"message":"Too many requests"}`),
	}

	result := providers.FormatMessageAsSSE(msg)
	if result == nil {
		t.Fatal("FormatMessageAsSSE() returned nil for exception event")
	}

	s := string(result)
	assert.Contains(t, s, "event: error\n", "exception should be formatted as SSE error event")
	assert.Contains(t, s, `"type":"throttlingException"`, "error data should include exception type")
	assert.Contains(t, s, "Too many requests", "error data should include exception payload message")
	assert.True(t, strings.HasSuffix(s, "\n\n"), "should terminate with SSE event separator, got %q", s)
}

func TestFormatMessageAsSSE_NoEventType(t *testing.T) {
	t.Parallel()

	msg := &providers.EventStreamMessage{
		Headers: map[string]string{},
		Payload: []byte(`{"type":"unknown"}`),
	}

	result := providers.FormatMessageAsSSE(msg)
	if result != nil {
		t.Errorf("FormatMessageAsSSE(no event type) = %v, want nil", result)
	}
}

// Test skipNonStringHeader

func TestSkipNonStringHeader_BoolType(t *testing.T) {
	t.Parallel()

	data := []byte{0x00, 0x01, 0x02}
	val, offset, err := providers.ExportSkipNonStringHeader(data, 0, 0, "bool_header")
	if err != nil {
		t.Fatalf("skipNonStringHeader(bool) error: %v", err)
	}
	if val != nil {
		t.Error("skipNonStringHeader(bool) should return nil value")
	}
	if offset != 0 {
		t.Errorf("skipNonStringHeader(bool) offset = %d, want 0", offset)
	}
}

func TestSkipNonStringHeader_TrueBoolType(t *testing.T) {
	t.Parallel()

	data := []byte{0x00, 0x01, 0x02}
	val, offset, err := providers.ExportSkipNonStringHeader(data, 0, 1, "true_header")
	if err != nil {
		t.Fatalf("skipNonStringHeader(true bool) error: %v", err)
	}
	if val != nil {
		t.Error("skipNonStringHeader(true bool) should return nil value")
	}
	if offset != 0 {
		t.Errorf("skipNonStringHeader(true bool) offset = %d, want 0", offset)
	}
}

func TestSkipNonStringHeader_UnknownType(t *testing.T) {
	t.Parallel()

	data := []byte{0x00}
	_, _, err := providers.ExportSkipNonStringHeader(data, 0, 99, "unknown")
	if err == nil {
		t.Error("skipNonStringHeader(unknown type) should return error")
	}
}

func TestSkipNonStringHeader_BytesType(t *testing.T) {
	t.Parallel()

	// Type 6 = variable-length bytes: 2-byte length prefix + data
	data := []byte{0x00, 0x03, 'a', 'b', 'c'} // length=3, data="abc"
	val, offset, err := providers.ExportSkipNonStringHeader(data, 0, 6, "bytes_header")
	if err != nil {
		t.Fatalf("skipNonStringHeader(bytes) error: %v", err)
	}
	if val != nil {
		t.Error("skipNonStringHeader(bytes) should return nil value")
	}
	if offset != 5 {
		t.Errorf("skipNonStringHeader(bytes) offset = %d, want 5", offset)
	}
}

func TestSkipNonStringHeader_BytesTruncated(t *testing.T) {
	t.Parallel()

	// Not enough data for length prefix
	data := []byte{0x00}
	_, _, err := providers.ExportSkipNonStringHeader(data, 0, 6, "truncated")
	if err == nil {
		t.Error("skipNonStringHeader(truncated bytes) should return error")
	}
}

// Test advanceOffset

func TestAdvanceOffset_Valid(t *testing.T) {
	t.Parallel()

	data := make([]byte, 10)
	val, next, err := providers.ExportAdvanceOffset(data, 2, 4, "test")
	if err != nil {
		t.Fatalf("advanceOffset() error: %v", err)
	}
	if val != nil {
		t.Error("advanceOffset() should return nil value")
	}
	if next != 6 {
		t.Errorf("advanceOffset() next = %d, want 6", next)
	}
}

func TestAdvanceOffset_OutOfBounds(t *testing.T) {
	t.Parallel()

	data := make([]byte, 5)
	_, _, err := providers.ExportAdvanceOffset(data, 3, 5, "oob")
	if err == nil {
		t.Error("advanceOffset(out of bounds) should return error")
	}
}

func TestAdvanceOffset_NegativeOffset(t *testing.T) {
	t.Parallel()

	data := make([]byte, 5)
	_, _, err := providers.ExportAdvanceOffset(data, -1, 2, "neg")
	if err == nil {
		t.Error("advanceOffset(negative offset) should return error")
	}
}

// Test writeSSEEvent

func TestWriteSSEEvent_Nil(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	written, err := providers.ExportWriteSSEEvent(rec, rec, nil)
	if err != nil {
		t.Fatalf("writeSSEEvent(nil) error: %v", err)
	}
	if written {
		t.Error("writeSSEEvent(nil) should return false")
	}
}

func TestWriteSSEEvent_NoEventType(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	msg := &providers.EventStreamMessage{
		Headers: map[string]string{},
		Payload: []byte(`{}`),
	}
	written, err := providers.ExportWriteSSEEvent(rec, rec, msg)
	if err != nil {
		t.Fatalf("writeSSEEvent(no event type) error: %v", err)
	}
	if written {
		t.Error("writeSSEEvent(no event type) should return false")
	}
}

func TestWriteSSEEvent_ValidEvent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	msg := &providers.EventStreamMessage{
		Headers: map[string]string{
			":event-type": "message_start",
		},
		Payload: []byte(`{"type":"message_start"}`),
	}
	written, err := providers.ExportWriteSSEEvent(rec, rec, msg)
	if err != nil {
		t.Fatalf("writeSSEEvent() error: %v", err)
	}
	if !written {
		t.Error("writeSSEEvent() should return true for valid event")
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("writeSSEEvent() should write SSE data to writer")
	}
}

func TestWriteSSEEvent_Exception(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	msg := &providers.EventStreamMessage{
		Headers: map[string]string{
			":exception-type": "throttlingException",
		},
		Payload: []byte(`{"message":"rate limited"}`),
	}
	written, err := providers.ExportWriteSSEEvent(rec, rec, msg)
	if err != nil {
		t.Fatalf("writeSSEEvent(exception) error: %v", err)
	}
	if !written {
		t.Error("writeSSEEvent(exception) should return true")
	}
}

// Test writeExceptionEvent

func TestWriteExceptionEvent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	written, err := providers.ExportWriteExceptionEvent(rec, rec, "throttling", []byte(`{"msg":"too fast"}`))
	if err != nil {
		t.Fatalf("writeExceptionEvent() error: %v", err)
	}
	if !written {
		t.Error("writeExceptionEvent() should return true")
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("writeExceptionEvent() Content-Type = %q, want text/event-stream", ct)
	}
}

// Test NewBedrockProvider validation

func TestNewBedrockProvider_MissingRegion(t *testing.T) {
	t.Parallel()

	_, err := providers.NewBedrockProvider(t.Context(), &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test",
		Region:       "",
		Models:       nil,
	})
	require.Error(t, err, "NewBedrockProvider(empty region) should return error")
	assert.ErrorContains(t, err, "region", "error should identify missing region as the cause")
}

// Test NewVertexProvider validation

func TestNewVertexProvider_MissingProjectID(t *testing.T) {
	t.Parallel()

	_, err := providers.NewVertexProvider(t.Context(), &providers.VertexConfig{
		ModelMapping: nil,
		Name:         "test",
		ProjectID:    "",
		Region:       "us-central1",
		Models:       nil,
	})
	require.Error(t, err, "NewVertexProvider(empty project_id) should return error")
	assert.ErrorContains(t, err, "project", "error should identify missing project as the cause")
}

func TestNewVertexProvider_MissingRegion(t *testing.T) {
	t.Parallel()

	_, err := providers.NewVertexProvider(t.Context(), &providers.VertexConfig{
		ModelMapping: nil,
		Name:         "test",
		ProjectID:    "my-project",
		Region:       "",
		Models:       nil,
	})
	require.Error(t, err, "NewVertexProvider(empty region) should return error")
	assert.ErrorContains(t, err, "region", "error should identify missing region as the cause")
}

// Test TransformBodyForCloudProvider edge case

func TestTransformBodyForCloudProvider_EmptyBody(t *testing.T) {
	t.Parallel()

	newBody, model, err := providers.TransformBodyForCloudProvider([]byte("{}"), "test-2024")
	if err != nil {
		t.Fatalf("TransformBodyForCloudProvider({}) error: %v", err)
	}
	if model != "" {
		t.Errorf("TransformBodyForCloudProvider({}) model = %q, want empty", model)
	}
	if newBody == nil {
		t.Fatal("TransformBodyForCloudProvider({}) returned nil body")
	}
}
