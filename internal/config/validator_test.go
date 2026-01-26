package config

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate_ValidMinimalConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_ValidFullConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen:        "0.0.0.0:8787",
			TimeoutMS:     60000,
			MaxConcurrent: 100,
		},
		Providers: []ProviderConfig{
			{
				Name:    "anthropic-primary",
				Type:    "anthropic",
				Enabled: true,
				Keys: []KeyConfig{
					{Key: "sk-ant-test", RPMLimit: 60, TPMLimit: 100000},
				},
			},
		},
		Routing: RoutingConfig{
			Strategy:        "failover",
			FailoverTimeout: 5000,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_MissingServerListen(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			TimeoutMS: 60000,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing server.listen")
	}

	if !strings.Contains(err.Error(), "server.listen is required") {
		t.Errorf("Expected 'server.listen is required' error, got: %v", err)
	}
}

func TestValidate_InvalidListenFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		listen string
	}{
		{"no_port", "127.0.0.1"},
		{"no_colon", "localhost8787"},
		{"empty_port", "127.0.0.1:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Server: ServerConfig{
					Listen: tt.listen,
				},
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Expected error for listen=%q", tt.listen)
			}

			if !strings.Contains(err.Error(), "server.listen") {
				t.Errorf("Expected server.listen error, got: %v", err)
			}
		})
	}
}

func TestValidate_ValidListenFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		listen string
	}{
		{"localhost", "localhost:8787"},
		{"ipv4", "127.0.0.1:8787"},
		{"ipv4_all", "0.0.0.0:8787"},
		{"empty_host", ":8787"},
		{"ipv6", "[::1]:8787"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Server: ServerConfig{
					Listen: tt.listen,
				},
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid listen=%q, got error: %v", tt.listen, err)
			}
		})
	}
}

func TestValidate_InvalidProviderType(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Providers: []ProviderConfig{
			{
				Name: "test",
				Type: "invalid-type",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid provider type")
	}

	if !strings.Contains(err.Error(), "type") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected provider type error, got: %v", err)
	}
}

func TestValidate_ValidProviderTypes(t *testing.T) {
	t.Parallel()

	validTypes := []string{"anthropic", "zai", "ollama", "bedrock", "vertex", "azure"}

	for _, provType := range validTypes {
		t.Run(provType, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Server: ServerConfig{
					Listen: "127.0.0.1:8787",
				},
				Providers: []ProviderConfig{
					{
						Name: "test",
						Type: provType,
						Keys: []KeyConfig{{Key: "test-key"}},
					},
				},
			}

			// Add required cloud provider fields
			switch provType {
			case "bedrock":
				cfg.Providers[0].AWSRegion = "us-east-1"
			case "vertex":
				cfg.Providers[0].GCPProjectID = "test-project"
				cfg.Providers[0].GCPRegion = "us-central1"
			case "azure":
				cfg.Providers[0].AzureResourceName = "test-resource"
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid type=%q, got error: %v", provType, err)
			}
		})
	}
}

func TestValidate_MissingProviderName(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Providers: []ProviderConfig{
			{
				Type: "anthropic",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing provider name")
	}

	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

func TestValidate_MissingProviderType(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Providers: []ProviderConfig{
			{
				Name: "test",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing provider type")
	}

	if !strings.Contains(err.Error(), "type") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected 'type is required' error, got: %v", err)
	}
}

func TestValidate_DuplicateProviderNames(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Providers: []ProviderConfig{
			{Name: "anthropic", Type: "anthropic", Keys: []KeyConfig{{Key: "key1"}}},
			{Name: "anthropic", Type: "anthropic", Keys: []KeyConfig{{Key: "key2"}}},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for duplicate provider names")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected 'duplicate' error, got: %v", err)
	}
}

func TestValidate_InvalidRoutingStrategy(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Routing: RoutingConfig{
			Strategy: "invalid-strategy",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid routing strategy")
	}

	if !strings.Contains(err.Error(), "routing.strategy") {
		t.Errorf("Expected routing.strategy error, got: %v", err)
	}
}

func TestValidate_ValidRoutingStrategies(t *testing.T) {
	t.Parallel()

	validStrategies := []string{
		"", "failover", "round_robin", "weighted", "shuffle",
		"model_based", "least_loaded", "weighted_failover",
	}

	for _, strategy := range validStrategies {
		t.Run(strategy, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Server: ServerConfig{
					Listen: "127.0.0.1:8787",
				},
				Routing: RoutingConfig{
					Strategy: strategy,
				},
			}

			// model_based requires model_mapping
			if strategy == "model_based" {
				cfg.Routing.ModelMapping = map[string]string{"claude": "anthropic"}
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid strategy=%q, got error: %v", strategy, err)
			}
		})
	}
}

func TestValidate_ModelBasedRequiresMapping(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Routing: RoutingConfig{
			Strategy: "model_based",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for model_based without model_mapping")
	}

	if !strings.Contains(err.Error(), "model_mapping") {
		t.Errorf("Expected model_mapping error, got: %v", err)
	}
}

func TestValidate_InvalidLoggingLevel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Logging: LoggingConfig{
			Level: "verbose",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid logging level")
	}

	if !strings.Contains(err.Error(), "logging.level") {
		t.Errorf("Expected logging.level error, got: %v", err)
	}
}

func TestValidate_InvalidLoggingFormat(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Logging: LoggingConfig{
			Format: "xml",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid logging format")
	}

	if !strings.Contains(err.Error(), "logging.format") {
		t.Errorf("Expected logging.format error, got: %v", err)
	}
}

func TestValidate_CloudProviderMissingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ProviderConfig
		missing  string
	}{
		{
			name:     "bedrock_missing_region",
			provider: ProviderConfig{Name: "bedrock", Type: "bedrock"},
			missing:  "aws_region",
		},
		{
			name:     "vertex_missing_project",
			provider: ProviderConfig{Name: "vertex", Type: "vertex", GCPRegion: "us-central1"},
			missing:  "gcp_project_id",
		},
		{
			name:     "vertex_missing_region",
			provider: ProviderConfig{Name: "vertex", Type: "vertex", GCPProjectID: "test"},
			missing:  "gcp_region",
		},
		{
			name:     "azure_missing_resource",
			provider: ProviderConfig{Name: "azure", Type: "azure"},
			missing:  "azure_resource_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Server: ServerConfig{
					Listen: "127.0.0.1:8787",
				},
				Providers: []ProviderConfig{tt.provider},
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Expected error for missing %s", tt.missing)
			}

			if !strings.Contains(err.Error(), tt.missing) {
				t.Errorf("Expected %s in error, got: %v", tt.missing, err)
			}
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			// Missing listen
			TimeoutMS: -1, // Invalid
		},
		Providers: []ProviderConfig{
			{
				// Missing name
				Type: "invalid-type",
			},
		},
		Logging: LoggingConfig{
			Level: "verbose",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected multiple validation errors")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Expected ValidationError, got %T", err)
	}

	// Should have at least 4 errors:
	// 1. server.listen required
	// 2. invalid provider type
	// 3. provider name required
	// 4. invalid logging level
	if len(validationErr.Errors) < 4 {
		t.Errorf("Expected at least 4 errors, got %d: %v", len(validationErr.Errors), validationErr.Errors)
	}
}

func TestValidate_InvalidKeyPriority(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Providers: []ProviderConfig{
			{
				Name: "test",
				Type: "anthropic",
				Keys: []KeyConfig{
					{Key: "test", Priority: 5},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid priority")
	}

	if !strings.Contains(err.Error(), "priority") {
		t.Errorf("Expected priority error, got: %v", err)
	}
}

func TestValidate_MissingKeyValue(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Server: ServerConfig{
			Listen: "127.0.0.1:8787",
		},
		Providers: []ProviderConfig{
			{
				Name: "test",
				Type: "anthropic",
				Keys: []KeyConfig{
					{RPMLimit: 60},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing key value")
	}

	if !strings.Contains(err.Error(), "key") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected key required error, got: %v", err)
	}
}

func TestValidationError_SingleError(t *testing.T) {
	t.Parallel()

	verr := &ValidationError{}
	verr.Add("test error")

	expected := "config validation failed: test error"
	if verr.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, verr.Error())
	}
}

func TestValidationError_MultipleErrors(t *testing.T) {
	t.Parallel()

	verr := &ValidationError{}
	verr.Add("error 1")
	verr.Add("error 2")
	verr.Add("error 3")

	result := verr.Error()
	if !strings.Contains(result, "3 errors") {
		t.Errorf("Expected '3 errors' in message, got: %s", result)
	}

	for i := 1; i <= 3; i++ {
		if !strings.Contains(result, "error "+string(rune('0'+i))) {
			t.Errorf("Expected 'error %d' in message, got: %s", i, result)
		}
	}
}

func TestValidationError_Empty(t *testing.T) {
	t.Parallel()

	verr := &ValidationError{}

	if verr.HasErrors() {
		t.Error("Expected HasErrors() to be false for empty error")
	}

	if verr.ToError() != nil {
		t.Error("Expected ToError() to be nil for empty error")
	}
}
