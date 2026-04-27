package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

const (
	formatYAML = "yaml"
	formatTOML = "toml"
)

// UnsupportedFormatError is returned when the config file has an unsupported extension.
type UnsupportedFormatError struct {
	Extension string
	Path      string
}

func (e *UnsupportedFormatError) Error() string {
	return fmt.Sprintf("unsupported config format %q for file %s (supported: .yaml, .yml, .toml)", e.Extension, e.Path)
}

// detectFormat determines the config format from the file extension.
func detectFormat(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return formatYAML, nil
	case ".toml":
		return formatTOML, nil
	default:
		return "", &UnsupportedFormatError{Extension: ext, Path: path}
	}
}

// Load reads and parses a configuration file from the given path.
// The format (YAML or TOML) is detected from the file extension.
// Environment variables in the format ${VAR_NAME} are expanded before parsing.
func Load(path string) (*Config, error) {
	// Clean the path to avoid directory traversal issues
	path = filepath.Clean(path)
	format, err := detectFormat(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}

	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close config file: %w", cerr)
		}
	}()

	return loadFromReaderWithFormat(file, format)
}

// loadFromReaderWithFormat is the internal implementation for reading config with explicit format.
func loadFromReaderWithFormat(r io.Reader, format string) (*Config, error) {
	// Read entire content
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(content))

	// Parse based on format
	var cfg Config
	switch format {
	case formatYAML:
		if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config YAML: %w", err)
		}
	case formatTOML:
		if err := toml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config TOML: %w", err)
		}
	default:
		return nil, fmt.Errorf("internal error: unknown format %s", format)
	}

	// Validate the parsed config so misconfiguration fails fast at load time
	// rather than producing silent fallbacks downstream (e.g., a negative
	// timeout_ms otherwise becomes a negative time.Duration that gets replaced
	// by the default in proxy.NewServer).
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}
