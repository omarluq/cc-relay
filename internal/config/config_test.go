package config

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestLoggingConfig_ParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected zerolog.Level
	}{
		{
			name:     "debug level",
			level:    "debug",
			expected: zerolog.DebugLevel,
		},
		{
			name:     "info level",
			level:    "info",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: zerolog.WarnLevel,
		},
		{
			name:     "error level",
			level:    "error",
			expected: zerolog.ErrorLevel,
		},
		{
			name:     "uppercase DEBUG",
			level:    "DEBUG",
			expected: zerolog.DebugLevel,
		},
		{
			name:     "mixed case Info",
			level:    "Info",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "invalid level defaults to info",
			level:    "invalid",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "empty level defaults to info",
			level:    "",
			expected: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoggingConfig{Level: tt.level}

			got := cfg.ParseLevel()
			if got != tt.expected {
				t.Errorf("ParseLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}
