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
