package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
)

func TestNewLogger_JSONFormat(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
		Pretty: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	logger = logger.Output(&buf)

	logger.Info().Msg("test message")

	// Verify JSON format
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Log output is not valid JSON: %v", err)
	}

	if logEntry["message"] != "test message" {
		t.Errorf("Expected message 'test message', got %v", logEntry["message"])
	}

	if logEntry["level"] != "info" {
		t.Errorf("Expected level 'info', got %v", logEntry["level"])
	}
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
		Pretty: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	logger = logger.Output(&buf)

	logger.Debug().Msg("debug message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected console output to contain 'debug message', got: %s", output)
	}
}

func TestNewLogger_LevelFiltering(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	var buf bytes.Buffer
	logger = logger.Output(&buf)

	// Debug and Info should be filtered out
	logger.Debug().Msg("should not appear")
	logger.Info().Msg("should not appear")
	logger.Warn().Msg("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Errorf("Debug/Info logs should be filtered at warn level")
	}

	if !strings.Contains(output, "should appear") {
		t.Errorf("Warn logs should appear at warn level")
	}
}

func TestAddRequestID_GeneratesUUID(t *testing.T) {
	ctx := context.Background()
	ctx = AddRequestID(ctx, "")

	requestID := GetRequestID(ctx)
	if requestID == "" {
		t.Error("Expected generated UUID, got empty string")
	}

	// Verify it's a valid UUID format (basic check)
	if len(requestID) != 36 {
		t.Errorf("Expected UUID length 36, got %d", len(requestID))
	}
}

func TestAddRequestID_UsesProvidedID(t *testing.T) {
	ctx := context.Background()
	expectedID := "custom-request-id-123"
	ctx = AddRequestID(ctx, expectedID)

	requestID := GetRequestID(ctx)
	if requestID != expectedID {
		t.Errorf("Expected request ID %s, got %s", expectedID, requestID)
	}
}
