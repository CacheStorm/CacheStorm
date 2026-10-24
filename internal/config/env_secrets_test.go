package config

import "testing"

func TestEnvOverridesSecretsAndDataDir(t *testing.T) {
	t.Setenv("CACHESTORM_REQUIREPASS", "s3cret")
	t.Setenv("CACHESTORM_HTTP_PASSWORD", "h77p")
	t.Setenv("CACHESTORM_DATA_DIR", "/var/lib/cachestorm")

	cfg := Default()
	cfg.applyEnvOverrides()

	if cfg.Server.RequirePass != "s3cret" {
		t.Fatalf("requirepass %q", cfg.Server.RequirePass)
	}
	if cfg.HTTP.Password != "h77p" {
		t.Fatalf("http password %q", cfg.HTTP.Password)
	}
	if cfg.Persistence.DataDir != "/var/lib/cachestorm" {
		t.Fatalf("data dir %q", cfg.Persistence.DataDir)
	}
}

func TestValidateRejectsPlaceholderPassword(t *testing.T) {
	cfg := Default()
	cfg.Server.RequirePass = PlaceholderPassword
	if err := Validate(cfg); err == nil {
		t.Fatal("the template's placeholder requirepass was accepted")
	}
	cfg = Default()
	cfg.HTTP.Password = PlaceholderPassword
	if err := Validate(cfg); err == nil {
		t.Fatal("the template's placeholder HTTP password was accepted")
	}
}

func TestProtectedModeDefaultsOnAndEnvTurnsItOff(t *testing.T) {
	if !Default().Server.ProtectedMode {
		t.Fatal("protected mode must default to on")
	}
	t.Setenv("CACHESTORM_PROTECTED_MODE", "false")
	cfg := Default()
	cfg.applyEnvOverrides()
	if cfg.Server.ProtectedMode {
		t.Fatal("CACHESTORM_PROTECTED_MODE=false did not turn protected mode off")
	}
}
