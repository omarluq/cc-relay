package proxy_test

import (
	"io"
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/proxy"
)

func TestFormatLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		contains string
	}{
		{"debug level", "debug", "DBG"},
		{"info level", "info", "INF"},
		{"warn level", "warn", "WRN"},
		{"error level", "error", "ERR"},
		{"fatal level", "fatal", "FTL"},
		{"panic level", "panic", "PNC"},
		{"unknown level", "unknown", "unknown"},
		{"non-string input", 123, ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result := proxy.FormatLevelForTest(testCase.input)
			if testCase.contains != "" {
				if !strings.Contains(result, testCase.contains) {
					t.Errorf("formatLevel(%v) = %q, want containing %q", testCase.input, result, testCase.contains)
				}
			} else if result != "" {
				// For empty contains (like non-string input), expect empty result
				t.Errorf("formatLevel(%v) = %q, want empty string", testCase.input, result)
			}
		})
	}
}

func TestFormatMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string message", "test message", "-> test message"},
		{"nil input", nil, ""},
		{"empty string", "", "-> "},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result := proxy.FormatMessageForTest(testCase.input)
			if testCase.want != "" && result != testCase.want {
				t.Errorf("formatMessage(%v) = %q, want %q", testCase.input, result, testCase.want)
			}
		})
	}
}

func TestFormatFieldName(t *testing.T) {
	t.Parallel()

	result := proxy.FormatFieldNameForTest("test_field")
	if result == "" {
		t.Error("formatFieldName() returned empty string")
	}
	// Check for dim ANSI code
	if result[0] != '\033' {
		t.Errorf("formatFieldName() should start with ANSI escape code, got %q", result)
	}
}

func TestBuildConsoleWriter(t *testing.T) {
	t.Parallel()

	writer := proxy.BuildConsoleWriterForTest(io.Discard)
	if writer.Out != io.Discard {
		t.Errorf("buildConsoleWriter() Out should be io.Discard")
	}
	if writer.TimeFormat != "15:04:05" {
		t.Errorf("buildConsoleWriter() TimeFormat = %q, want \"15:04:05\"", writer.TimeFormat)
	}
	if writer.FormatLevel == nil {
		t.Error("buildConsoleWriter() FormatLevel should not be nil")
	}
	if writer.FormatMessage == nil {
		t.Error("buildConsoleWriter() FormatMessage should not be nil")
	}
	if writer.FormatFieldName == nil {
		t.Error("buildConsoleWriter() FormatFieldName should not be nil")
	}
}
