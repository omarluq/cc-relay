package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file from the given path.
// Environment variables in the format ${VAR_NAME} are expanded before parsing.
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}

	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close config file: %w", cerr)
		}
	}()

	return LoadFromReader(file)
}

// LoadFromReader reads and parses YAML configuration from an io.Reader.
// Environment variables in the format ${VAR_NAME} are expanded before parsing.
func LoadFromReader(r io.Reader) (*Config, error) {
	// Read entire content
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(content))

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return &cfg, nil
}
