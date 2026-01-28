package providers

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildEventStreamMessage constructs a valid AWS Event Stream message for testing.
func buildEventStreamMessage(headers map[string]string, payload []byte) []byte {
	// Build headers section
	var headersBuf bytes.Buffer
	for name, value := range headers {
		// Name length (1 byte) + name
		headersBuf.WriteByte(byte(len(name)))
		headersBuf.WriteString(name)
		// Type (string = 7)
		headersBuf.WriteByte(headerTypeString)
		// Value length (2 bytes) + value
		valLenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(valLenBuf, uint16(len(value)))
		headersBuf.Write(valLenBuf)
		headersBuf.WriteString(value)
	}
	headersData := headersBuf.Bytes()

	// Calculate total length
	totalLen := eventStreamPreludeLen + uint32(len(headersData)) +
		uint32(len(payload)) + eventStreamTrailerLen

	// Build message
	msg := make([]byte, totalLen)

	// Prelude
	binary.BigEndian.PutUint32(msg[0:4], totalLen)
	binary.BigEndian.PutUint32(msg[4:8], uint32(len(headersData)))

	// Prelude CRC
	preludeCRC := crc32.Checksum(msg[0:8], eventStreamCRCTable)
	binary.BigEndian.PutUint32(msg[8:12], preludeCRC)

	// Headers
	copy(msg[eventStreamPreludeLen:], headersData)

	// Payload
	payloadStart := eventStreamPreludeLen + len(headersData)
	copy(msg[payloadStart:], payload)

	// Message CRC (covers everything except the trailing CRC)
	msgCRC := crc32.Checksum(msg[0:totalLen-eventStreamTrailerLen], eventStreamCRCTable)
	binary.BigEndian.PutUint32(msg[totalLen-eventStreamTrailerLen:], msgCRC)

	return msg
}

func TestParseEventStreamMessage(t *testing.T) {
	t.Run("parses valid message with headers and payload", func(t *testing.T) {
		headers := map[string]string{
			":event-type":   "message_start",
			":content-type": "application/json",
			":message-type": "event",
		}
		payload := []byte(`{"type":"message_start","message":{"id":"msg_123"}}`)

		msg := buildEventStreamMessage(headers, payload)

		parsed, consumed, err := ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, len(msg), consumed)
		assert.Equal(t, "message_start", parsed.Headers[":event-type"])
		assert.Equal(t, "application/json", parsed.Headers[":content-type"])
		assert.Equal(t, payload, parsed.Payload)
	})

	t.Run("parses message with empty payload", func(t *testing.T) {
		headers := map[string]string{
			":event-type": "ping",
		}

		msg := buildEventStreamMessage(headers, []byte{})

		parsed, consumed, err := ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, len(msg), consumed)
		assert.Equal(t, "ping", parsed.Headers[":event-type"])
		assert.Empty(t, parsed.Payload)
	})

	t.Run("parses message with empty headers", func(t *testing.T) {
		headers := map[string]string{}
		payload := []byte(`{"data":"test"}`)

		msg := buildEventStreamMessage(headers, payload)

		parsed, consumed, err := ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, len(msg), consumed)
		assert.Empty(t, parsed.Headers)
		assert.Equal(t, payload, parsed.Payload)
	})

	t.Run("returns error for message too short", func(t *testing.T) {
		data := make([]byte, 10) // Less than minimum

		_, _, err := ParseEventStreamMessage(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
	})

	t.Run("returns error for incomplete message", func(t *testing.T) {
		// Valid prelude but total length exceeds data
		data := make([]byte, 20)
		binary.BigEndian.PutUint32(data[0:4], 100) // Total length = 100
		binary.BigEndian.PutUint32(data[4:8], 0)   // Headers length = 0

		_, _, err := ParseEventStreamMessage(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incomplete message")
	})

	t.Run("returns error for invalid prelude CRC", func(t *testing.T) {
		headers := map[string]string{":event-type": "test"}
		msg := buildEventStreamMessage(headers, []byte("test"))

		// Corrupt prelude CRC
		msg[8] = 0xFF
		msg[9] = 0xFF
		msg[10] = 0xFF
		msg[11] = 0xFF

		_, _, err := ParseEventStreamMessage(msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prelude CRC mismatch")
	})

	t.Run("returns error for invalid message CRC", func(t *testing.T) {
		headers := map[string]string{":event-type": "test"}
		msg := buildEventStreamMessage(headers, []byte("test"))

		// Corrupt message CRC (last 4 bytes)
		msgLen := len(msg)
		msg[msgLen-4] = 0xFF
		msg[msgLen-3] = 0xFF
		msg[msgLen-2] = 0xFF
		msg[msgLen-1] = 0xFF

		_, _, err := ParseEventStreamMessage(msg)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message CRC mismatch")
	})

	t.Run("parses multiple concatenated messages", func(t *testing.T) {
		msg1 := buildEventStreamMessage(
			map[string]string{":event-type": "first"},
			[]byte(`{"seq":1}`),
		)
		msg2 := buildEventStreamMessage(
			map[string]string{":event-type": "second"},
			[]byte(`{"seq":2}`),
		)

		// Pre-allocate with correct capacity
		combined := make([]byte, 0, len(msg1)+len(msg2))
		combined = append(combined, msg1...)
		combined = append(combined, msg2...)

		// Parse first message
		parsed1, consumed1, err := ParseEventStreamMessage(combined)
		require.NoError(t, err)
		assert.Equal(t, len(msg1), consumed1)
		assert.Equal(t, "first", parsed1.Headers[":event-type"])

		// Parse second message
		parsed2, consumed2, err := ParseEventStreamMessage(combined[consumed1:])
		require.NoError(t, err)
		assert.Equal(t, len(msg2), consumed2)
		assert.Equal(t, "second", parsed2.Headers[":event-type"])
	})
}

func TestParseEventStreamHeaders(t *testing.T) {
	t.Run("parses multiple string headers", func(t *testing.T) {
		headers := map[string]string{
			"header1": "value1",
			"header2": "value2",
			"header3": "value3",
		}

		msg := buildEventStreamMessage(headers, nil)
		parsed, _, err := ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, "value1", parsed.Headers["header1"])
		assert.Equal(t, "value2", parsed.Headers["header2"])
		assert.Equal(t, "value3", parsed.Headers["header3"])
	})

	t.Run("handles empty string values", func(t *testing.T) {
		headers := map[string]string{
			"empty": "",
		}

		msg := buildEventStreamMessage(headers, nil)
		parsed, _, err := ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, "", parsed.Headers["empty"])
	})

	t.Run("handles long header names and values", func(t *testing.T) {
		longName := "x-very-long-header-name-that-exceeds-normal-length"
		longValue := "This is a very long header value that contains lots of text " +
			"and should be handled correctly by the parser"

		headers := map[string]string{
			longName: longValue,
		}

		msg := buildEventStreamMessage(headers, nil)
		parsed, _, err := ParseEventStreamMessage(msg)

		require.NoError(t, err)
		assert.Equal(t, longValue, parsed.Headers[longName])
	})
}

func TestFormatSSEEvent(t *testing.T) {
	t.Run("formats simple event", func(t *testing.T) {
		payload := []byte(`{"type":"message_start"}`)
		result := formatSSEEvent("message_start", payload)

		expected := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n"
		assert.Equal(t, expected, string(result))
	})

	t.Run("handles multi-line payload", func(t *testing.T) {
		payload := []byte("line1\nline2\nline3")
		result := formatSSEEvent("test", payload)

		expected := "event: test\ndata: line1\ndata: line2\ndata: line3\n\n"
		assert.Equal(t, expected, string(result))
	})

	t.Run("handles empty payload", func(t *testing.T) {
		result := formatSSEEvent("ping", []byte{})

		expected := "event: ping\ndata: \n\n"
		assert.Equal(t, expected, string(result))
	})
}

func TestMapBedrockEventType(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"message_start", "message_start"},
		{"content_block_start", "content_block_start"},
		{"content_block_delta", "content_block_delta"},
		{"content_block_stop", "content_block_stop"},
		{"message_delta", "message_delta"},
		{"message_stop", "message_stop"},
		{"ping", "ping"},
		{"unknown_type", "unknown_type"}, // Pass through unknown
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := mapBedrockEventType(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
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
		statusCode: 0,
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

func TestEventStreamToSSE(t *testing.T) {
	t.Run("converts single event stream message to SSE", func(t *testing.T) {
		// Build Event Stream message
		msg := buildEventStreamMessage(
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
		w := newMockResponseWriter()

		// Convert
		count, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, http.StatusOK, w.statusCode)

		// Check headers
		assert.Equal(t, "text/event-stream", w.headers.Get("Content-Type"))
		assert.Equal(t, "no-cache, no-transform", w.headers.Get("Cache-Control"))
		assert.Equal(t, "no", w.headers.Get("X-Accel-Buffering"))

		// Check body contains SSE event
		body := w.body.String()
		assert.Contains(t, body, "event: message_start")
		assert.Contains(t, body, `data: {"type":"message_start"`)

		// Check flushed
		assert.Equal(t, 1, w.flushed)
	})

	t.Run("converts multiple event stream messages", func(t *testing.T) {
		// Build multiple Event Stream messages
		msg1 := buildEventStreamMessage(
			map[string]string{":event-type": "message_start"},
			[]byte(`{"type":"message_start"}`),
		)
		msg2 := buildEventStreamMessage(
			map[string]string{":event-type": "content_block_start"},
			[]byte(`{"type":"content_block_start","index":0}`),
		)
		msg3 := buildEventStreamMessage(
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

		w := newMockResponseWriter()

		count, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, 3, count)

		body := w.body.String()
		assert.Contains(t, body, "event: message_start")
		assert.Contains(t, body, "event: content_block_start")
		assert.Contains(t, body, "event: message_stop")
	})

	t.Run("handles exception events", func(t *testing.T) {
		msg := buildEventStreamMessage(
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

		w := newMockResponseWriter()

		count, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, 1, count)

		body := w.body.String()
		assert.Contains(t, body, "event: error")
		assert.Contains(t, body, "ValidationException")
	})

	t.Run("skips messages without event type", func(t *testing.T) {
		// Message without :event-type header
		msg := buildEventStreamMessage(
			map[string]string{":content-type": "application/json"},
			[]byte(`{"type":"unknown"}`),
		)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}

		w := newMockResponseWriter()

		count, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
		assert.Empty(t, w.body.String())
	})

	t.Run("handles empty stream", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}

		w := newMockResponseWriter()

		count, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("preserves non-content headers from response", func(t *testing.T) {
		msg := buildEventStreamMessage(
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

		w := newMockResponseWriter()

		_, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, "req-456", w.headers.Get("x-amzn-request-id"))
		assert.Equal(t, "custom-value", w.headers.Get("x-custom-header"))
	})

	t.Run("returns error when writer does not support flushing", func(t *testing.T) {
		msg := buildEventStreamMessage(
			map[string]string{":event-type": "message_start"},
			[]byte(`{}`),
		)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}

		// Use httptest.ResponseRecorder which doesn't implement Flusher in older go versions
		// Actually it does, so let's create a custom non-flushing writer
		nonFlusher := &nonFlushingWriter{headers: make(http.Header)}

		_, err := EventStreamToSSE(resp, nonFlusher)

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

func (n *nonFlushingWriter) Header() http.Header {
	return n.headers
}

func (n *nonFlushingWriter) Write(b []byte) (int, error) {
	return n.body.Write(b)
}

func (n *nonFlushingWriter) WriteHeader(statusCode int) {
	n.statusCode = statusCode
}

func TestEventStreamToSSEIntegration(t *testing.T) {
	t.Run("full streaming scenario", func(t *testing.T) {
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
		for _, m := range messages {
			msg := buildEventStreamMessage(
				map[string]string{
					":event-type":   m.eventType,
					":content-type": "application/json",
					":message-type": "event",
				},
				[]byte(m.payload),
			)
			allMsgs = append(allMsgs, msg...)
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(allMsgs)),
		}
		resp.Header.Set("Content-Type", "application/vnd.amazon.eventstream")

		w := httptest.NewRecorder()

		count, err := EventStreamToSSE(resp, w)

		require.NoError(t, err)
		assert.Equal(t, len(messages), count)

		body := w.Body.String()

		// Verify all events are present in order
		for _, m := range messages {
			assert.Contains(t, body, "event: "+m.eventType)
		}

		// Verify SSE format
		assert.Contains(t, body, "data: ")
		assert.Contains(t, body, "\n\n") // Event separators
	})
}
