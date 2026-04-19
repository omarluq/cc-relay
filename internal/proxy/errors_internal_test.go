package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteBodyTooLargeError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteBodyTooLargeError(rec)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("WriteBodyTooLargeError() status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("WriteBodyTooLargeError() Content-Type = %q, want \"application/json\"", contentType)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("WriteBodyTooLargeError() invalid JSON: %v", err)
	}

	if resp.Type != "error" {
		t.Errorf("WriteBodyTooLargeError() type = %q, want \"error\"", resp.Type)
	}
	if resp.Error.Type != "request_too_large" {
		t.Errorf("WriteBodyTooLargeError() error.type = %q, want \"request_too_large\"", resp.Error.Type)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	payload := map[string]string{"key": "value"}
	writeJSON(rec, http.StatusOK, payload)

	if rec.Code != http.StatusOK {
		t.Errorf("writeJSON() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("writeJSON() invalid JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("writeJSON() key = %q, want \"value\"", result["key"])
	}
}
