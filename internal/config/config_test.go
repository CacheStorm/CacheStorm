package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 6380 {
		t.Errorf("expected port 6380, got %d", cfg.Server.Port)
	}

	if cfg.Server.Bind != "0.0.0.0" {
		t.Errorf("expected bind 0.0.0.0, got %s", cfg.Server.Bind)
	}

	if cfg.Memory.EvictionPolicy != "allkeys-lru" {
		t.Errorf("expected eviction policy allkeys-lru, got %s", cfg.Memory.EvictionPolicy)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level info, got %s", cfg.Logging.Level)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 6380 {
		t.Errorf("expected default port")
	}
}

func TestLoadWithEnvPort(t *testing.T) {
	os.Setenv("CACHESTORM_PORT", "7000")
	defer os.Unsetenv("CACHESTORM_PORT")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 7000 {
		t.Errorf("expected port 7000, got %d", cfg.Server.Port)
	}
}

func TestLoadWithEnvBind(t *testing.T) {
	os.Setenv("CACHESTORM_BIND", "127.0.0.1")
	defer os.Unsetenv("CACHESTORM_BIND")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Bind != "127.0.0.1" {
		t.Errorf("expected bind 127.0.0.1, got %s", cfg.Server.Bind)
	}
}

func TestLoadWithEnvMaxMemory(t *testing.T) {
	os.Setenv("CACHESTORM_MAX_MEMORY", "2gb")
	defer os.Unsetenv("CACHESTORM_MAX_MEMORY")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Memory.MaxMemory != "2gb" {
		t.Errorf("expected max memory 2gb, got %s", cfg.Memory.MaxMemory)
	}
}

func TestLoadWithEnvLogLevel(t *testing.T) {
	os.Setenv("CACHESTORM_LOG_LEVEL", "debug")
	defer os.Unsetenv("CACHESTORM_LOG_LEVEL")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := `
server:
  port: 7000
  bind: "127.0.0.1"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 7000 {
		t.Errorf("expected port 7000, got %d", cfg.Server.Port)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := Default()
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 0

	if err := Validate(cfg); err == nil {
		t.Error("expected error for invalid port")
	}

	cfg.Server.Port = 70000
	if err := Validate(cfg); err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestValidateNegativeMaxConnections(t *testing.T) {
	cfg := Default()
	cfg.Server.MaxConnections = -1

	if err := Validate(cfg); err == nil {
		t.Error("expected error for negative max connections")
	}
}

func TestValidateInvalidWarningPct(t *testing.T) {
	cfg := Default()
	cfg.Memory.WarningPct = 150

	if err := Validate(cfg); err == nil {
		t.Error("expected error for invalid warning percentage")
	}
}

func TestValidateInvalidCriticalPct(t *testing.T) {
	cfg := Default()
	cfg.Memory.CriticalPct = 150

	if err := Validate(cfg); err == nil {
		t.Error("expected error for invalid critical percentage")
	}
}

func TestValidateWarningGteCritical(t *testing.T) {
	cfg := Default()
	cfg.Memory.WarningPct = 90
	cfg.Memory.CriticalPct = 80

	if err := Validate(cfg); err == nil {
		t.Error("expected error for warning >= critical")
	}
}

func TestValidateInvalidEvictionPolicy(t *testing.T) {
	cfg := Default()
	cfg.Memory.EvictionPolicy = "invalid"

	if err := Validate(cfg); err == nil {
		t.Error("expected error for invalid eviction policy")
	}
}

func TestValidateValidEvictionPolicies(t *testing.T) {
	policies := []string{"noeviction", "allkeys-lru", "allkeys-lfu", "volatile-lru", "allkeys-random"}

	for _, policy := range policies {
		cfg := Default()
		cfg.Memory.EvictionPolicy = policy

		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error for policy %s: %v", policy, err)
		}
	}
}

func TestValidateInvalidLogLevel(t *testing.T) {
	cfg := Default()
	cfg.Logging.Level = "invalid"

	if err := Validate(cfg); err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestValidateClusterEnabledWithoutNodeName(t *testing.T) {
	cfg := Default()
	cfg.Cluster.Enabled = true
	cfg.Cluster.NodeName = ""

	if err := Validate(cfg); err == nil {
		t.Error("expected error for cluster without node name")
	}
}

func TestValidateClusterEnabledWithNodeName(t *testing.T) {
	cfg := Default()
	cfg.Cluster.Enabled = true
	cfg.Cluster.NodeName = "node1"

	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0", 0},
		{"", 0},
		{"100", 100},
		{"2gb", 2 * 1024 * 1024 * 1024},
		{"1000", 1000},
	}

	for _, tt := range tests {
		result, err := ParseMemorySize(tt.input)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", tt.input, err)
		}
		if result != tt.expected {
			t.Errorf("expected %d for %s, got %d", tt.expected, tt.input, result)
		}
	}
}

func TestParseMemorySizeGb(t *testing.T) {
	result, err := ParseMemorySize("1GB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1024*1024*1024 {
		t.Errorf("expected %d, got %d", 1024*1024*1024, result)
	}
}

func TestParseMemorySizeInvalid(t *testing.T) {
	_, err := ParseMemorySize("invalid")
	if err == nil {
		t.Error("expected error for invalid memory size")
	}
}

func TestServerConfigReadTimeoutDuration(t *testing.T) {
	cfg := &ServerConfig{ReadTimeout: "5s"}
	d := cfg.ReadTimeoutDuration()

	if d != 5000000000 {
		t.Errorf("expected 5s, got %v", d)
	}
}

func TestServerConfigReadTimeoutDurationInvalid(t *testing.T) {
	cfg := &ServerConfig{ReadTimeout: "invalid"}
	d := cfg.ReadTimeoutDuration()

	if d != 0 {
		t.Errorf("expected 0 for invalid duration, got %v", d)
	}
}

func TestServerConfigWriteTimeoutDuration(t *testing.T) {
	cfg := &ServerConfig{WriteTimeout: "10s"}
	d := cfg.WriteTimeoutDuration()

	if d != 10000000000 {
		t.Errorf("expected 10s, got %v", d)
	}
}

func TestServerConfigWriteTimeoutDurationInvalid(t *testing.T) {
	cfg := &ServerConfig{WriteTimeout: "invalid"}
	d := cfg.WriteTimeoutDuration()

	if d != 0 {
		t.Errorf("expected 0 for invalid duration, got %v", d)
	}
}

func TestConfigFromEnvPath(t *testing.T) {
	content := `
server:
  port: 8000
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	os.Setenv("CACHESTORM_CONFIG", tmpFile)
	defer os.Unsetenv("CACHESTORM_CONFIG")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8000 {
		t.Errorf("expected port 8000, got %d", cfg.Server.Port)
	}
}

func TestParseMemorySizeKib(t *testing.T) {
	result, err := ParseMemorySize("1KiB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1024 {
		t.Errorf("expected 1024, got %d", result)
	}
}

func TestParseMemorySizeMib(t *testing.T) {
	result, err := ParseMemorySize("1MiB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1024*1024 {
		t.Errorf("expected %d, got %d", 1024*1024, result)
	}
}

func TestParseMemorySizeGib(t *testing.T) {
	result, err := ParseMemorySize("1GiB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1024*1024*1024 {
		t.Errorf("expected %d, got %d", 1024*1024*1024, result)
	}
}

func TestParseMemorySizeTib(t *testing.T) {
	result, err := ParseMemorySize("1TiB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1024*1024*1024*1024 {
		t.Errorf("expected %d, got %d", 1024*1024*1024*1024, result)
	}
}
