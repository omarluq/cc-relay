// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"net/http"
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
