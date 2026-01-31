// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// IsBodyTooLargeError reports whether err is an *http.MaxBytesError, indicating the request body exceeded the configured maximum size.
func IsBodyTooLargeError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

// WriteBodyTooLargeError writes a 413 Request Entity Too Large response in the relay's
// JSON error format with error type "request_too_large" and message
// "Request body exceeds the maximum allowed size".
func WriteBodyTooLargeError(w http.ResponseWriter) {
	WriteError(w, http.StatusRequestEntityTooLarge, "request_too_large",
		"Request body exceeds the maximum allowed size")
}

// Custom header constants for relay metadata.
const (
	HeaderRelayKeyID     = "X-CC-Relay-Key-ID"         // Selected key ID (first 8 chars)
	HeaderRelayCapacity  = "X-CC-Relay-Capacity"       // Remaining capacity %
	HeaderRelayKeysTotal = "X-CC-Relay-Keys-Total"     // Total keys in pool
	HeaderRelayKeysAvail = "X-CC-Relay-Keys-Available" // Available keys
)

// ErrorResponse matches Anthropic's error response format exactly.
type ErrorResponse struct {
	Type  string      `json:"type"`
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the error type and message.
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// WriteError writes an HTTP JSON error response using the Anthropic error envelope.
// The response body is {"type":"error","error":{"type": <errorType>, "message": <message>}} and the HTTP status code is set to statusCode.
func WriteError(w http.ResponseWriter, statusCode int, errorType, message string) {
	response := ErrorResponse{
		Type: "error",
		Error: ErrorDetail{
			Type:    errorType,
			Message: message,
		},
	}

	writeJSON(w, statusCode, response)
}

// WriteRateLimitError writes a 429 Too Many Requests response in Anthropic format.
// WriteRateLimitError writes a 429 Too Many Requests response with rate-limit metadata.
// 
// WriteRateLimitError sets the Retry-After header (in seconds, minimum 1), logs the retry duration,
// and responds with an Anthropic-formatted error of type "rate_limit_error" and a standardized message.
// The retryAfter parameter specifies when capacity will be available.
func WriteRateLimitError(w http.ResponseWriter, retryAfter time.Duration) {
	// Set Retry-After header (RFC 6585)
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1 // Minimum 1 second
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))

	log.Warn().
		Dur("retry_after", retryAfter).
		Int("retry_after_seconds", seconds).
		Msg("Returning 429 rate limit error")

	WriteError(w, http.StatusTooManyRequests, "rate_limit_error",
		"All API keys are currently at rate limit capacity. Please retry after the specified time.")
}

// writeJSON marshals payload to JSON and sends it as the HTTP response with the provided status code.
// If marshaling fails, it sends an Internal Server Error response containing the marshal error message.
// The response Content-Type is set to "application/json". Any error that occurs while writing the response body is logged.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(body); err != nil {
		log.Error().Err(err).Msg("failed to write response")
	}
}