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

// Format represents supported configuration file formats.
type Format string

// Supported configuration file formats.
const (
	FormatYAML Format = "yaml"
	FormatTOML Format = "toml"
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
func detectFormat(path string) (Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return FormatYAML, nil
	case ".toml":
		return FormatTOML, nil
	default:
		return "", &UnsupportedFormatError{Extension: ext, Path: path}
	}
}

// Load reads and parses a configuration file from the given path.
// The format (YAML or TOML) is detected from the file extension.
// Environment variables in the format ${VAR_NAME} are expanded before parsing.
func Load(path string) (*Config, error) {
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

// LoadFromReader reads and parses YAML configuration from an io.Reader.
// Environment variables in the format ${VAR_NAME} are expanded before parsing.
//
// Deprecated: Use Load with a file path for format detection, or LoadFromReaderWithFormat.
func LoadFromReader(r io.Reader) (*Config, error) {
	return loadFromReaderWithFormat(r, FormatYAML)
}

// LoadFromReaderWithFormat reads and parses configuration from an io.Reader with explicit format.
// Environment variables in the format ${VAR_NAME} are expanded before parsing.
func LoadFromReaderWithFormat(r io.Reader, format Format) (*Config, error) {
	return loadFromReaderWithFormat(r, format)
}

// loadFromReaderWithFormat is the internal implementation for reading config with explicit format.
func loadFromReaderWithFormat(r io.Reader, format Format) (*Config, error) {
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
	case FormatYAML:
		if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config YAML: %w", err)
		}
	case FormatTOML:
		if err := toml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config TOML: %w", err)
		}
	default:
		return nil, fmt.Errorf("internal error: unknown format %s", format)
	}

	return &cfg, nil
}
