package config_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
)

const (
	defaultListenAddr = "127.0.0.1:8787"
	testListenAddr    = ":8080"
	testProviderName  = "test"
	testProviderType  = "anthropic"
	testKeyValue      = "key"
	testTypeBedrock   = "bedrock"
	testTypeVertex    = "vertex"
	testTypeAzure     = "azure"
	logLevelInfo      = "info"
	logFormatJSON     = "json"
)

func configWithListen(listen string) *config.Config {
	cfg := config.MakeTestConfig()
	cfg.Server.Listen = listen
	return cfg
}

func configWithProvider(provider *config.ProviderConfig) *config.Config {
	cfg := configWithListen(defaultListenAddr)
	cfg.Providers = []config.ProviderConfig{*provider}
	return cfg
}

func configWithSingleProvider(listen string) *config.Config {
	cfg := configWithListen(listen)

	prov := config.MakeTestProviderConfig()
	prov.Name = testProviderName
	prov.Type = testProviderType
	prov.Enabled = true
	prov.Keys = []config.KeyConfig{config.MakeTestKeyConfig(testKeyValue)}

	cfg.Providers = []config.ProviderConfig{prov}
	return cfg
}

func TestValidateValidMinimalConfig(t *testing.T) {
	t.Parallel()

	cfg := configWithListen(defaultListenAddr)

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidateValidFullConfig(t *testing.T) {
	t.Parallel()

	cfg := configWithListen("0.0.0.0:8787")
	cfg.Server.TimeoutMS = 60000
	cfg.Server.MaxConcurrent = 100

	key := config.MakeTestKeyConfig("sk-ant-test")
	key.RPMLimit = 60
	key.TPMLimit = 100000

	prov := config.MakeTestProviderConfig()
	prov.Name = "anthropic-primary"
	prov.Type = testProviderType
	prov.Enabled = true
	prov.Keys = []config.KeyConfig{key}

	cfg.Providers = []config.ProviderConfig{prov}

	routing := config.MakeTestRoutingConfig()
	routing.Strategy = "failover"
	routing.FailoverTimeout = 5000
	cfg.Routing = routing

	logging := config.MakeTestLoggingConfig()
	logging.Level = "info"
	logging.Format = "json"
	cfg.Logging = logging

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidateMissingServerListen(t *testing.T) {
	t.Parallel()

	cfg := config.MakeTestConfig()
	cfg.Server.Listen = ""
	cfg.Server.TimeoutMS = 60000

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing server.listen")
	}

	if !strings.Contains(err.Error(), "server.listen is required") {
		t.Errorf("Expected 'server.listen is required' error, got: %v", err)
	}
}

func TestValidateInvalidListenFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		listen string
	}{
		{"no_port", "127.0.0.1"},
		{"no_colon", "localhost8787"},
		{"empty_port", "127.0.0.1:"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := configWithListen(testCase.listen)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Expected error for listen=%q", testCase.listen)
			}

			if !strings.Contains(err.Error(), "server.listen") {
				t.Errorf("Expected server.listen error, got: %v", err)
			}
		})
	}
}

func TestValidateValidListenFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		listen string
	}{
		{"localhost", "localhost:8787"},
		{"ipv4", defaultListenAddr},
		{"ipv4_all", "0.0.0.0:8787"},
		{"empty_host", ":8787"},
		{"ipv6", "[::1]:8787"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := configWithListen(testCase.listen)

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid listen=%q, got error: %v", testCase.listen, err)
			}
		})
	}
}

func TestValidateInvalidProviderType(t *testing.T) {
	t.Parallel()

	prov := config.MakeTestProviderConfig()
	prov.Name = testProviderName
	prov.Type = "invalid-type"
	prov.Keys = nil
	prov.Enabled = false

	cfg := configWithProvider(&prov)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid provider type")
	}

	if !strings.Contains(err.Error(), "type") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected provider type error, got: %v", err)
	}
}

func TestValidateValidProviderTypes(t *testing.T) {
	t.Parallel()

	validTypes := []string{testProviderType, "zai", "ollama", testTypeBedrock, testTypeVertex, testTypeAzure}

	for _, provType := range validTypes {
		t.Run(provType, func(t *testing.T) {
			t.Parallel()

			prov := config.MakeTestProviderConfig()
			prov.Name = testProviderName
			prov.Type = provType
			prov.Keys = []config.KeyConfig{config.MakeTestKeyConfig("test-key")}

			cfg := configWithProvider(&prov)

			// Add required cloud provider fields
			switch provType {
			case testTypeBedrock:
				cfg.Providers[0].AWSRegion = "us-east-1"
			case testTypeVertex:
				cfg.Providers[0].GCPProjectID = "test-project"
				cfg.Providers[0].GCPRegion = "us-central1"
			case testTypeAzure:
				cfg.Providers[0].AzureResourceName = "test-resource"
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid type=%q, got error: %v", provType, err)
			}
		})
	}
}

func TestValidateMissingProviderName(t *testing.T) {
	t.Parallel()

	prov := config.MakeTestProviderConfig()
	prov.Name = ""
	prov.Type = testProviderType

	cfg := configWithProvider(&prov)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing provider name")
	}

	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Expected 'name is required' error, got: %v", err)
	}
}

func TestValidateMissingProviderType(t *testing.T) {
	t.Parallel()

	prov := config.MakeTestProviderConfig()
	prov.Name = testProviderName
	prov.Type = ""

	cfg := configWithProvider(&prov)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing provider type")
	}

	if !strings.Contains(err.Error(), "type") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected 'type is required' error, got: %v", err)
	}
}

func TestValidateDuplicateProviderNames(t *testing.T) {
	t.Parallel()

	cfg := configWithListen(defaultListenAddr)

	prov1 := config.MakeTestProviderConfig()
	prov1.Name = testProviderType
	prov1.Type = testProviderType
	prov1.Keys = []config.KeyConfig{config.MakeTestKeyConfig("key1")}

	prov2 := config.MakeTestProviderConfig()
	prov2.Name = testProviderType
	prov2.Type = testProviderType
	prov2.Keys = []config.KeyConfig{config.MakeTestKeyConfig("key2")}

	cfg.Providers = []config.ProviderConfig{prov1, prov2}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for duplicate provider names")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("Expected 'duplicate' error, got: %v", err)
	}
}

func TestValidateInvalidRoutingStrategy(t *testing.T) {
	t.Parallel()

	cfg := configWithListen(defaultListenAddr)

	routing := config.MakeTestRoutingConfig()
	routing.Strategy = "invalid-strategy"
	cfg.Routing = routing

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid routing strategy")
	}

	if !strings.Contains(err.Error(), "routing.strategy") {
		t.Errorf("Expected routing.strategy error, got: %v", err)
	}
}

func TestValidateValidRoutingStrategies(t *testing.T) {
	t.Parallel()

	validStrategies := []string{
		"", "failover", "round_robin", "weighted_round_robin", "shuffle",
		"model_based", "least_loaded", "weighted_failover",
	}

	for _, strategy := range validStrategies {
		t.Run(strategy, func(t *testing.T) {
			t.Parallel()
			cfg := configWithListen(defaultListenAddr)

			routing := config.MakeTestRoutingConfig()
			routing.Strategy = strategy
			cfg.Routing = routing

			// model_based requires model_mapping
			if strategy == "model_based" {
				cfg.Routing.ModelMapping = map[string]string{"claude": testProviderType}
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Expected valid strategy=%q, got error: %v", strategy, err)
			}
		})
	}
}

func TestValidateModelBasedRequiresMapping(t *testing.T) {
	t.Parallel()

	cfg := configWithListen(defaultListenAddr)

	routing := config.MakeTestRoutingConfig()
	routing.Strategy = "model_based"
	routing.ModelMapping = nil
	cfg.Routing = routing

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for model_based without model_mapping")
	}

	if !strings.Contains(err.Error(), "model_mapping") {
		t.Errorf("Expected model_mapping error, got: %v", err)
	}
}

func TestValidateInvalidLoggingLevel(t *testing.T) {
	t.Parallel()

	cfg := configWithListen(defaultListenAddr)

	logging := config.MakeTestLoggingConfig()
	logging.Level = "verbose"
	cfg.Logging = logging

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid logging level")
	}

	if !strings.Contains(err.Error(), "logging.level") {
		t.Errorf("Expected logging.level error, got: %v", err)
	}
}

func TestValidateInvalidLoggingFormat(t *testing.T) {
	t.Parallel()

	cfg := configWithListen(defaultListenAddr)

	logging := config.MakeTestLoggingConfig()
	logging.Format = "xml"
	cfg.Logging = logging

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid logging format")
	}

	if !strings.Contains(err.Error(), "logging.format") {
		t.Errorf("Expected logging.format error, got: %v", err)
	}
}

func TestValidateCloudProviderMissingFields(t *testing.T) {
	t.Parallel()

	bedrockMissingRegion := config.MakeTestProviderConfig()
	bedrockMissingRegion.Name = testTypeBedrock
	bedrockMissingRegion.Type = testTypeBedrock
	bedrockMissingRegion.AWSRegion = ""

	vertexMissingProject := config.MakeTestProviderConfig()
	vertexMissingProject.Name = testTypeVertex
	vertexMissingProject.Type = testTypeVertex
	vertexMissingProject.GCPRegion = "us-central1"
	vertexMissingProject.GCPProjectID = ""

	vertexMissingRegion := config.MakeTestProviderConfig()
	vertexMissingRegion.Name = testTypeVertex
	vertexMissingRegion.Type = testTypeVertex
	vertexMissingRegion.GCPProjectID = "test"
	vertexMissingRegion.GCPRegion = ""

	azureMissingResource := config.MakeTestProviderConfig()
	azureMissingResource.Name = testTypeAzure
	azureMissingResource.Type = testTypeAzure
	azureMissingResource.AzureResourceName = ""

	tests := []struct {
		name     string
		missing  string
		provider config.ProviderConfig
	}{
		{
			name:     "bedrock_missing_region",
			provider: bedrockMissingRegion,
			missing:  "aws_region",
		},
		{
			name:     "vertex_missing_project",
			provider: vertexMissingProject,
			missing:  "gcp_project_id",
		},
		{
			name:     "vertex_missing_region",
			provider: vertexMissingRegion,
			missing:  "gcp_region",
		},
		{
			name:     "azure_missing_resource",
			provider: azureMissingResource,
			missing:  "azure_resource_name",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			provider := testCase.provider
			cfg := configWithProvider(&provider)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Expected error for missing %s", testCase.missing)
			}

			if !strings.Contains(err.Error(), testCase.missing) {
				t.Errorf("Expected %s in error, got: %v", testCase.missing, err)
			}
		})
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	t.Parallel()

	cfg := config.MakeTestConfig()
	cfg.Server.Listen = ""   // Missing listen
	cfg.Server.TimeoutMS = -1 // Invalid

	invalidProv := config.MakeTestProviderConfig()
	invalidProv.Name = "" // Missing name
	invalidProv.Type = "invalid-type"

	cfg.Providers = []config.ProviderConfig{invalidProv}

	logging := config.MakeTestLoggingConfig()
	logging.Level = "verbose"
	cfg.Logging = logging

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected multiple validation errors")
	}

	var validationErr *config.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Expected config.ValidationError, got %T", err)
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

func TestValidateInvalidKeyPriority(t *testing.T) {
	t.Parallel()

	cfg := configWithSingleProvider(defaultListenAddr)

	key := config.MakeTestKeyConfig("test")
	key.Priority = 5
	cfg.Providers[0].Keys = []config.KeyConfig{key}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid priority")
	}

	if !strings.Contains(err.Error(), "priority") {
		t.Errorf("Expected priority error, got: %v", err)
	}
}

func TestValidateMissingKeyValue(t *testing.T) {
	t.Parallel()

	cfg := configWithSingleProvider(defaultListenAddr)

	key := config.MakeTestKeyConfig("")
	key.RPMLimit = 60
	cfg.Providers[0].Keys = []config.KeyConfig{key}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing key value")
	}

	if !strings.Contains(err.Error(), "key") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected key required error, got: %v", err)
	}
}

func TestValidationErrorSingleError(t *testing.T) {
	t.Parallel()

	verr := config.MakeTestValidationError()
	verr.Add("test error")

	expected := "config validation failed: test error"
	if verr.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, verr.Error())
	}
}

func TestValidationErrorMultipleErrors(t *testing.T) {
	t.Parallel()

	verr := config.MakeTestValidationError()
	verr.Add("error 1")
	verr.Add("error 2")
	verr.Add("error 3")

	result := verr.Error()
	if !strings.Contains(result, "3 errors") {
		t.Errorf("Expected '3 errors' in message, got: %s", result)
	}

	for idx := 1; idx <= 3; idx++ {
		if !strings.Contains(result, "error "+strconv.Itoa(idx)) {
			t.Errorf("Expected 'error %d' in message, got: %s", idx, result)
		}
	}
}

func TestValidationErrorEmpty(t *testing.T) {
	t.Parallel()

	verr := config.MakeTestValidationError()

	if verr.HasErrors() {
		t.Error("Expected HasErrors() to be false for empty error")
	}

	if verr.ToError() != nil {
		t.Error("Expected ToError() to be nil for empty error")
	}
}

func TestValidateServerFields(t *testing.T) {
	t.Parallel()

	t.Run("max_concurrent/zero_is_valid", func(t *testing.T) {
		t.Parallel()
		cfg := configWithSingleProvider(testListenAddr)
		cfg.Server.MaxConcurrent = 0
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}
	})

	t.Run("max_concurrent/positive_is_valid", func(t *testing.T) {
		t.Parallel()
		cfg := configWithSingleProvider(testListenAddr)
		cfg.Server.MaxConcurrent = 100
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}
	})

	t.Run("max_concurrent/negative_is_invalid", func(t *testing.T) {
		t.Parallel()
		cfg := configWithSingleProvider(testListenAddr)
		cfg.Server.MaxConcurrent = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for negative max_concurrent")
		} else if !strings.Contains(err.Error(), "max_concurrent") {
			t.Errorf("Expected 'max_concurrent' in error, got: %v", err)
		}
	})

	t.Run("max_body_bytes/zero_is_valid", func(t *testing.T) {
		t.Parallel()
		cfg := configWithSingleProvider(testListenAddr)
		cfg.Server.MaxBodyBytes = 0
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}
	})

	t.Run("max_body_bytes/positive_is_valid", func(t *testing.T) {
		t.Parallel()
		cfg := configWithSingleProvider(testListenAddr)
		cfg.Server.MaxBodyBytes = 10485760 // 10MB
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}
	})

	t.Run("max_body_bytes/negative_is_invalid", func(t *testing.T) {
		t.Parallel()
		cfg := configWithSingleProvider(testListenAddr)
		cfg.Server.MaxBodyBytes = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for negative max_body_bytes")
		} else if !strings.Contains(err.Error(), "max_body_bytes") {
			t.Errorf("Expected 'max_body_bytes' in error, got: %v", err)
		}
	})
}
