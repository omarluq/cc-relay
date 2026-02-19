package providers

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

// Exported test helpers for eventstream_test.go (when using package providers_test).

// ExportFormatSSEEvent exports the formatSSEEvent function.
func ExportFormatSSEEvent(eventType string, payload []byte) []byte {
	return formatSSEEvent(eventType, payload)
}

// ExportMapBedrockEventType exports the mapBedrockEventType function.
func ExportMapBedrockEventType(bedrockType string) string {
	return mapBedrockEventType(bedrockType)
}

// safeUint8 converts int to uint8, panicking if the value overflows.
func safeUint8(v int) uint8 {
	if v < 0 || v > 255 {
		panic("value out of range for uint8")
	}
	return uint8(v)
}

// safeUint16 converts int to uint16, panicking if the value overflows.
func safeUint16(v int) uint16 {
	if v < 0 || v > 65535 {
		panic("value out of range for uint16")
	}
	return uint16(v)
}

// safeUint32 converts int to uint32, panicking if the value overflows.
func safeUint32(v int) uint32 {
	if v < 0 || v > 4294967295 {
		panic("value out of range for uint32")
	}
	return uint32(v)
}

// ExportBuildEventStreamMessage constructs a valid AWS Event Stream message for testing.
// This helper is used by tests in providers_test package.
func ExportBuildEventStreamMessage(headers map[string]string, payload []byte) []byte {
	// Build headers section
	var headersBuf bytes.Buffer

	for name, value := range headers {
		headersBuf.WriteByte(safeUint8(len(name)))
		headersBuf.WriteString(name)
		// Type (string = 7)
		headersBuf.WriteByte(headerTypeString)

		// Value length (2 bytes) + value
		valLenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(valLenBuf, safeUint16(len(value)))
		headersBuf.Write(valLenBuf)
		headersBuf.WriteString(value)
	}

	headersData := headersBuf.Bytes()
	headersDataLen := safeUint32(len(headersData))
	payloadLen := safeUint32(len(payload))

	// Calculate total length
	totalLen := eventStreamPreludeLen + headersDataLen + payloadLen + eventStreamTrailerLen

	// Build message
	msg := make([]byte, totalLen)

	// Prelude
	binary.BigEndian.PutUint32(msg[0:4], totalLen)
	binary.BigEndian.PutUint32(msg[4:8], headersDataLen)

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
