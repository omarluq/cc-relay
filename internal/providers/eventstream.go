// Package providers provides AWS Event Stream to SSE conversion utilities.
package providers

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	// eventStreamPreludeLen is the length of the Event Stream prelude.
	// (total_length + headers_length + prelude_crc).
	eventStreamPreludeLen = 12

	// eventStreamTrailerLen is the length of the trailing CRC.
	eventStreamTrailerLen = 4

	// eventStreamMinMsgLen is the minimum valid message length.
	eventStreamMinMsgLen = eventStreamPreludeLen + eventStreamTrailerLen

	// headerTypeString is the type byte for string headers.
	headerTypeString = 7
)

// ErrMessageSkipped indicates a message was skipped due to a parse error.
// This is not a fatal error - the stream can continue processing.
var ErrMessageSkipped = errors.New("eventstream: message skipped due to parse error")

// eventStreamCRCTable is the precomputed CRC32-C table used by AWS Event Stream.
// AWS uses CRC32-C (Castagnoli polynomial).
var eventStreamCRCTable = crc32.MakeTable(crc32.Castagnoli)

// EventStreamMessage represents a decoded AWS Event Stream message.
type EventStreamMessage struct {
	Headers map[string]string
	Payload []byte
}

// ParseEventStreamMessage parses a single AWS Event Stream message from binary data.
// Returns the parsed message and the number of bytes consumed.
//
// AWS Event Stream format:
// - Total byte length (4 bytes, big-endian)
// - Headers byte length (4 bytes, big-endian)
// - Prelude CRC (4 bytes, CRC32-C of first 8 bytes)
// - Headers (variable length)
// - Payload (variable length)
// - Message CRC (4 bytes, CRC32-C of entire message excluding this field).
func ParseEventStreamMessage(data []byte) (*EventStreamMessage, int, error) {
	if len(data) < eventStreamMinMsgLen {
		return nil, 0, fmt.Errorf("eventstream: message too short: %d bytes", len(data))
	}

	// Parse and validate prelude
	totalLen, headersLen, err := parseAndValidatePrelude(data)
	if err != nil {
		return nil, 0, err
	}

	// Calculate payload length
	payloadLen := totalLen - eventStreamPreludeLen - headersLen - eventStreamTrailerLen

	// Verify message CRC
	if err := verifyMessageCRC(data, totalLen); err != nil {
		return nil, 0, err
	}

	// Parse headers
	headers, err := parseEventStreamHeaders(data[eventStreamPreludeLen : eventStreamPreludeLen+headersLen])
	if err != nil {
		return nil, 0, fmt.Errorf("eventstream: %w", err)
	}

	// Extract payload
	payloadStart := eventStreamPreludeLen + headersLen
	payload := data[payloadStart : payloadStart+payloadLen]

	return &EventStreamMessage{
		Headers: headers,
		Payload: payload,
	}, int(totalLen), nil
}

// parseAndValidatePrelude parses the message prelude and validates its CRC.
func parseAndValidatePrelude(data []byte) (totalLen, headersLen uint32, err error) {
	totalLen = binary.BigEndian.Uint32(data[0:4])
	headersLen = binary.BigEndian.Uint32(data[4:8])
	preludeCRC := binary.BigEndian.Uint32(data[8:12])

	if totalLen < eventStreamMinMsgLen {
		return 0, 0, fmt.Errorf("eventstream: invalid total length: %d", totalLen)
	}
	if int(totalLen) > len(data) {
		return 0, 0, fmt.Errorf("eventstream: incomplete message: have %d, need %d", len(data), totalLen)
	}

	computedPreludeCRC := crc32.Checksum(data[0:8], eventStreamCRCTable)
	if computedPreludeCRC != preludeCRC {
		return 0, 0, fmt.Errorf(
			"eventstream: prelude CRC mismatch: got %08x, want %08x",
			computedPreludeCRC, preludeCRC,
		)
	}

	return totalLen, headersLen, nil
}

// verifyMessageCRC verifies the message CRC.
func verifyMessageCRC(data []byte, totalLen uint32) error {
	msgCRCOffset := totalLen - eventStreamTrailerLen
	expectedMsgCRC := binary.BigEndian.Uint32(data[msgCRCOffset:totalLen])
	computedMsgCRC := crc32.Checksum(data[0:msgCRCOffset], eventStreamCRCTable)
	if computedMsgCRC != expectedMsgCRC {
		return fmt.Errorf(
			"eventstream: message CRC mismatch: got %08x, want %08x",
			computedMsgCRC, expectedMsgCRC,
		)
	}
	return nil
}

// parseEventStreamHeaders parses the headers section of an Event Stream message.
// Header format: name_len (1 byte) + name + type (1 byte) + value_len (2 bytes for strings) + value.
//

func parseEventStreamHeaders(data []byte) (map[string]string, error) {
	headers := make(map[string]string)
	offset := 0

	for offset < len(data) {
		name, newOffset, err := parseHeaderName(data, offset)
		if err != nil {
			return nil, err
		}
		offset = newOffset

		if offset >= len(data) {
			return nil, fmt.Errorf("header type missing for %q", name)
		}
		headerType := data[offset]
		offset++

		value, newOffset, err := parseHeaderValue(data, offset, headerType, name)
		if err != nil {
			return nil, err
		}
		offset = newOffset

		if value != nil {
			headers[name] = *value
		}
	}

	return headers, nil
}

// parseHeaderName parses a header name from the data at the given offset.
func parseHeaderName(data []byte, offset int) (name string, newOffset int, err error) {
	b, next, err := readByte(data, offset)
	if err != nil {
		return "", offset, err
	}
	nameLen := int(b)
	nameBytes, end, err := readSlice(data, next, nameLen)
	if err != nil {
		return "", 0, fmt.Errorf("header name truncated")
	}
	return string(nameBytes), end, nil
}

// parseHeaderValue parses a header value based on its type.
// Returns nil value pointer for non-string types (we skip them).
func parseHeaderValue(
	data []byte,
	offset int,
	headerType byte,
	headerName string,
) (value *string, newOffset int, err error) {
	switch headerType {
	case headerTypeString:
		return parseStringHeaderValue(data, offset, headerName)
	default:
		return skipNonStringHeader(data, offset, headerType, headerName)
	}
}

// parseStringHeaderValue parses a string header value.
func parseStringHeaderValue(
	data []byte,
	offset int,
	name string,
) (val *string, newOffset int, err error) {
	if offset+2 > len(data) {
		return nil, 0, fmt.Errorf("header value length truncated for %q", name)
	}
	lenBytes, next, err := readSlice(data, offset, 2)
	if err != nil {
		return nil, 0, fmt.Errorf("header value length truncated for %q", name)
	}
	valueLen := int(binary.BigEndian.Uint16(lenBytes))
	offset = next

	if offset+valueLen > len(data) {
		return nil, 0, fmt.Errorf("header value truncated for %q", name)
	}
	valueBytes, end, err := readSlice(data, offset, valueLen)
	if err != nil {
		return nil, 0, fmt.Errorf("header value truncated for %q", name)
	}
	strVal := string(valueBytes)
	return &strVal, end, nil
}

// skipNonStringHeader skips non-string header types, returning the new offset.
func skipNonStringHeader(
	data []byte,
	offset int,
	headerType byte,
	name string,
) (value *string, newOffset int, err error) {
	switch headerType {
	case 0, 1: // bool true/false - no value bytes
		return nil, offset, nil
	case 2: // byte
		return advanceOffset(data, offset, 1, name)
	case 3: // short
		return advanceOffset(data, offset, 2, name)
	case 4: // int
		return advanceOffset(data, offset, 4, name)
	case 5: // long
		return advanceOffset(data, offset, 8, name)
	case 6: // bytes
		lenBytes, next, err := readSlice(data, offset, 2)
		if err != nil {
			return nil, 0, fmt.Errorf("bytes header length truncated for %q", name)
		}
		bLen := int(binary.BigEndian.Uint16(lenBytes))
		return advanceOffset(data, next, bLen, name)
	case 8: // timestamp
		return advanceOffset(data, offset, 8, name)
	case 9: // uuid
		return advanceOffset(data, offset, 16, name)
	default:
		return nil, 0, fmt.Errorf("unknown header type %d for %q", headerType, name)
	}
}

func readByte(data []byte, offset int) (b byte, next int, err error) {
	if offset < 0 || offset >= len(data) {
		return 0, offset, fmt.Errorf("offset out of bounds")
	}
	return data[offset], offset + 1, nil
}

func readSlice(data []byte, offset, length int) (out []byte, next int, err error) {
	if length < 0 || offset < 0 || offset+length > len(data) {
		return nil, 0, fmt.Errorf("slice out of bounds")
	}
	return data[offset : offset+length], offset + length, nil
}

func advanceOffset(data []byte, offset, length int, name string) (val *string, next int, err error) {
	if offset < 0 || offset+length > len(data) {
		return nil, 0, fmt.Errorf("header value truncated for %q", name)
	}
	return nil, offset + length, nil
}

// EventStreamToSSE converts an AWS Event Stream response to SSE format.
// It reads Event Stream messages from the response body and writes SSE events to the writer.
// Returns the number of events converted.
func EventStreamToSSE(resp *http.Response, w http.ResponseWriter) (int, error) {
	setSSEHeaders(w, resp)

	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return 0, fmt.Errorf("eventstream: response writer does not support flushing")
	}

	return processEventStream(resp.Body, w, flusher)
}

// setSSEHeaders sets the SSE response headers.
func setSSEHeaders(w http.ResponseWriter, resp *http.Response) {
	w.Header().Set("Content-Type", ContentTypeSSE)
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	// Copy non-content headers from response
	for key, values := range resp.Header {
		lowerKey := strings.ToLower(key)
		if isContentHeader(lowerKey) {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

// isContentHeader returns true if the header is content-related.
func isContentHeader(lowerKey string) bool {
	return lowerKey == "content-type" ||
		lowerKey == "content-length" ||
		lowerKey == "transfer-encoding"
}

// processEventStream reads and processes Event Stream messages.
func processEventStream(
	body io.Reader,
	w http.ResponseWriter,
	flusher http.Flusher,
) (int, error) {
	reader := bufio.NewReaderSize(body, 64*1024)
	eventCount := 0

	for {
		msg, err := readNextMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, ErrMessageSkipped) {
				// Continue processing remaining messages
				continue
			}
			return eventCount, err
		}

		written, err := writeSSEEvent(w, flusher, msg)
		if err != nil {
			return eventCount, err
		}
		if written {
			eventCount++
		}
	}

	return eventCount, nil
}

// readNextMessage reads the next Event Stream message from the reader.
func readNextMessage(reader *bufio.Reader) (*EventStreamMessage, error) {
	prelude := make([]byte, eventStreamPreludeLen)
	if _, err := io.ReadFull(reader, prelude); err != nil {
		return nil, err
	}

	totalLen := binary.BigEndian.Uint32(prelude[0:4])
	if totalLen < eventStreamMinMsgLen {
		return nil, fmt.Errorf("eventstream: invalid message length: %d", totalLen)
	}

	msgData := make([]byte, totalLen)
	copy(msgData, prelude)

	if _, err := io.ReadFull(reader, msgData[eventStreamPreludeLen:]); err != nil {
		return nil, fmt.Errorf("eventstream: failed to read message body: %w", err)
	}

	msg, _, err := ParseEventStreamMessage(msgData)
	if err != nil {
		log.Warn().Err(err).Msg("eventstream: failed to parse message, skipping")
		return nil, ErrMessageSkipped
	}

	return msg, nil
}

// writeSSEEvent writes an Event Stream message as an SSE event.
// Returns true if an event was written.
func writeSSEEvent(
	w http.ResponseWriter,
	flusher http.Flusher,
	msg *EventStreamMessage,
) (bool, error) {
	if msg == nil {
		return false, nil
	}

	// Check for exception
	if exceptionType := msg.Headers[":exception-type"]; exceptionType != "" {
		return writeExceptionEvent(w, flusher, exceptionType, msg.Payload)
	}

	// Get event type from headers
	eventType := msg.Headers[":event-type"]
	if eventType == "" {
		return false, nil
	}

	// Convert to SSE format
	sseEvent := formatSSEEvent(eventType, msg.Payload)

	if _, err := w.Write(sseEvent); err != nil {
		return false, fmt.Errorf("eventstream: failed to write SSE event: %w", err)
	}
	flusher.Flush()

	return true, nil
}

// writeExceptionEvent writes an exception as an SSE error event.
func writeExceptionEvent(
	w http.ResponseWriter,
	flusher http.Flusher,
	exceptionType string,
	payload []byte,
) (bool, error) {
	errEvent := fmt.Sprintf(
		"event: error\ndata: {\"error\":{\"type\":%q,\"message\":%q}}\n\n",
		exceptionType, string(payload),
	)
	if _, err := w.Write([]byte(errEvent)); err != nil {
		return false, fmt.Errorf("eventstream: failed to write error event: %w", err)
	}
	flusher.Flush()
	return true, nil
}

// formatSSEEvent formats an Event Stream event as an SSE event.
// Maps Bedrock event types to Anthropic SSE event types.
func formatSSEEvent(eventType string, payload []byte) []byte {
	var buf bytes.Buffer

	sseEventType := mapBedrockEventType(eventType)

	buf.WriteString("event: ")
	buf.WriteString(sseEventType)
	buf.WriteByte('\n')

	lines := bytes.Split(payload, []byte("\n"))
	for i, line := range lines {
		buf.WriteString("data: ")
		buf.Write(line)
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}
	buf.WriteString("\n\n")

	return buf.Bytes()
}

// mapBedrockEventType maps Bedrock event types to Anthropic SSE event types.
func mapBedrockEventType(bedrockType string) string {
	switch bedrockType {
	case "message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
		"ping":
		return bedrockType
	default:
		return bedrockType
	}
}

// FormatMessageAsSSE converts an EventStreamMessage to SSE format bytes.
// This is used by the proxy to convert Bedrock Event Stream responses
// to SSE format for Claude Code compatibility.
func FormatMessageAsSSE(msg *EventStreamMessage) []byte {
	if msg == nil {
		return nil
	}

	// Check for exception
	if exceptionType := msg.Headers[":exception-type"]; exceptionType != "" {
		return formatExceptionAsSSE(exceptionType, msg.Payload)
	}

	// Get event type from headers
	eventType := msg.Headers[":event-type"]
	if eventType == "" {
		return nil
	}

	return formatSSEEvent(eventType, msg.Payload)
}

// formatExceptionAsSSE formats an exception as an SSE error event.
func formatExceptionAsSSE(exceptionType string, payload []byte) []byte {
	return []byte(fmt.Sprintf(
		"event: error\ndata: {\"error\":{\"type\":%q,\"message\":%q}}\n\n",
		exceptionType, string(payload),
	))
}
