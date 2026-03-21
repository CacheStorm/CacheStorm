package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Cover Load with invalid YAML content (yaml.Unmarshal error path)
func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(tmpFile, []byte("{{invalid yaml:::"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

// Cover Load with validation error (e.g., invalid port in file)
func TestLoadValidationError(t *testing.T) {
	content := `
server:
  port: 0
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected validation error for port 0")
	}
}

// Cover CACHESTORM_PORT env with non-integer value (strconv.Atoi err != nil branch)
func TestLoadWithEnvPortInvalid(t *testing.T) {
	os.Setenv("CACHESTORM_PORT", "notanumber")
	defer os.Unsetenv("CACHESTORM_PORT")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Port should remain default since parsing failed
	if cfg.Server.Port != 6380 {
		t.Errorf("expected default port 6380 when env port is invalid, got %d", cfg.Server.Port)
	}
}

// Cover Validate: invalid bind address
func TestValidateInvalidBindAddress(t *testing.T) {
	cfg := Default()
	cfg.Server.Bind = "not-an-ip"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for invalid bind address")
	}
}

// Cover Validate: valid non-default bind address (passes the ParseIP check)
func TestValidateValidBindAddress(t *testing.T) {
	cfg := Default()
	cfg.Server.Bind = "192.168.1.1"

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error for valid bind address: %v", err)
	}
}

// Cover Validate: HTTP enabled with invalid port
func TestValidateInvalidHTTPPort(t *testing.T) {
	cfg := Default()
	cfg.HTTP.Enabled = true
	cfg.HTTP.Port = 0

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for invalid HTTP port")
	}
}

// Cover Validate: HTTP enabled with port > 65535
func TestValidateInvalidHTTPPortTooHigh(t *testing.T) {
	cfg := Default()
	cfg.HTTP.Enabled = true
	cfg.HTTP.Port = 70000

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for HTTP port > 65535")
	}
}

// Cover Validate: cluster enabled with invalid bind port
func TestValidateClusterInvalidBindPort(t *testing.T) {
	cfg := Default()
	cfg.Cluster.Enabled = true
	cfg.Cluster.NodeName = "node1"
	cfg.Cluster.BindPort = 70000

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for invalid cluster bind port")
	}
}

// Cover Validate: TLS cert file not found
func TestValidateTLSCertFileNotFound(t *testing.T) {
	cfg := Default()
	cfg.Server.TLSCertFile = "/nonexistent/cert.pem"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for TLS cert file not found")
	}
}

// Cover Validate: TLS key file not found
func TestValidateTLSKeyFileNotFound(t *testing.T) {
	cfg := Default()
	cfg.Server.TLSKeyFile = "/nonexistent/key.pem"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for TLS key file not found")
	}
}

// Cover Validate: TLS cert file exists (no error)
func TestValidateTLSCertFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	if err := os.WriteFile(certFile, []byte("cert"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.Server.TLSCertFile = certFile

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Cover Validate: TLS key file exists (no error)
func TestValidateTLSKeyFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "key.pem")
	if err := os.WriteFile(keyFile, []byte("key"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.Server.TLSKeyFile = keyFile

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Cover Validate: negative critical percentage
func TestValidateNegativeCriticalPct(t *testing.T) {
	cfg := Default()
	cfg.Memory.CriticalPct = -1

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for negative critical percentage")
	}
}

// Cover Validate: negative warning percentage
func TestValidateNegativeWarningPct(t *testing.T) {
	cfg := Default()
	cfg.Memory.WarningPct = -1

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for negative warning percentage")
	}
}

// Cover Validate: warning equals critical
func TestValidateWarningEqualsCritical(t *testing.T) {
	cfg := Default()
	cfg.Memory.WarningPct = 80
	cfg.Memory.CriticalPct = 80

	err := Validate(cfg)
	if err == nil {
		t.Error("expected error when warning equals critical")
	}
}

// Cover ParseMemorySize: various suffixes that haven't been tested
func TestParseMemorySizeTb(t *testing.T) {
	result, err := ParseMemorySize("1tb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(1024 * 1024 * 1024 * 1024)
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestParseMemorySizeMb(t *testing.T) {
	result, err := ParseMemorySize("10mb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(10 * 1024 * 1024)
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestParseMemorySizeKb(t *testing.T) {
	result, err := ParseMemorySize("512kb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(512 * 1024)
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestParseMemorySizeBytes(t *testing.T) {
	result, err := ParseMemorySize("4096b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 4096 {
		t.Errorf("expected 4096, got %d", result)
	}
}

// Cover ParseMemorySize: invalid number with valid suffix
func TestParseMemorySizeInvalidNumberWithSuffix(t *testing.T) {
	_, err := ParseMemorySize("abcgb")
	if err == nil {
		t.Error("expected error for invalid number with suffix")
	}
}

// Cover ParseMemorySize: whitespace handling
func TestParseMemorySizeWithWhitespace(t *testing.T) {
	result, err := ParseMemorySize("  1gb  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(1024 * 1024 * 1024)
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

// Cover Validate: HTTP disabled does not validate port
func TestValidateHTTPDisabledInvalidPort(t *testing.T) {
	cfg := Default()
	cfg.HTTP.Enabled = false
	cfg.HTTP.Port = 0 // Would be invalid if HTTP were enabled

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error when HTTP is disabled: %v", err)
	}
}

// Cover Validate: cluster disabled does not validate bind port
func TestValidateClusterDisabledInvalidBindPort(t *testing.T) {
	cfg := Default()
	cfg.Cluster.Enabled = false
	cfg.Cluster.BindPort = 99999

	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error when cluster is disabled: %v", err)
	}
}

// Cover Validate: valid log levels
func TestValidateAllLogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		cfg := Default()
		cfg.Logging.Level = level
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error for log level %s: %v", level, err)
		}
	}
}
