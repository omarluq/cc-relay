// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

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

// WriteError writes a JSON error response in Anthropic API format.
func WriteError(w http.ResponseWriter, statusCode int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Type: "error",
		Error: ErrorDetail{
			Type:    errorType,
			Message: message,
		},
	}

	//nolint:errcheck // Response is already committed with status code
	json.NewEncoder(w).Encode(response)
}

// WriteRateLimitError writes a 429 Too Many Requests response in Anthropic format.
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
