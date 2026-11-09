package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A server started from a redis.conf behaves like redis-server: one RESP
// port, no HTTP API or metrics listener, and noeviction unless the file says
// otherwise. CacheStorm's own directives turn the listeners back on.
func TestRedisConfUsesRedisDefaults(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cfg, _, err := loadConfig(write("plain.conf", "port 6399\ndir "+dir+"\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Enabled || cfg.Plugins.Metrics.Enabled {
		t.Fatalf("listeners on: http %v metrics %v", cfg.HTTP.Enabled, cfg.Plugins.Metrics.Enabled)
	}
	if cfg.Memory.EvictionPolicy != "noeviction" {
		t.Fatalf("policy %q, want noeviction", cfg.Memory.EvictionPolicy)
	}
	// Without port and dir lines Redis listens on 6379 and keeps its files
	// in the working directory.
	cfg, _, err = loadConfig(write("bare.conf", "appendonly yes\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 6379 || cfg.Persistence.DataDir != "." {
		t.Fatalf("port %d dir %q, want 6379 and the working directory", cfg.Server.Port, cfg.Persistence.DataDir)
	}
	// So do Redis-style options without a file, as redis-server --appendonly yes.
	cfg, _, err = loadConfig("", []string{"--appendonly", "yes"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 6379 || cfg.Persistence.DataDir != "." || cfg.HTTP.Enabled || !cfg.Persistence.AOF {
		t.Fatalf("options only: port %d dir %q http %v aof %v", cfg.Server.Port, cfg.Persistence.DataDir, cfg.HTTP.Enabled, cfg.Persistence.AOF)
	}
	// No file and no options keep CacheStorm's own defaults.
	cfg, _, err = loadConfig("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 6380 || !cfg.HTTP.Enabled {
		t.Fatalf("bare start: port %d http %v", cfg.Server.Port, cfg.HTTP.Enabled)
	}

	cfg, _, err = loadConfig(write("on.conf", "dir "+dir+"\nmaxmemory-policy allkeys-lfu\nhttp-port 8181\nmetrics-port 9191\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HTTP.Enabled || cfg.HTTP.Port != 8181 || !cfg.Plugins.Metrics.Enabled || cfg.Plugins.Metrics.Port != 9191 {
		t.Fatalf("http %v:%d metrics %v:%d", cfg.HTTP.Enabled, cfg.HTTP.Port, cfg.Plugins.Metrics.Enabled, cfg.Plugins.Metrics.Port)
	}
	if cfg.Memory.EvictionPolicy != "allkeys-lfu" {
		t.Fatalf("policy %q", cfg.Memory.EvictionPolicy)
	}

	if _, _, err := loadConfig(write("bad.conf", "dir "+dir+"\nhttp-port 70000\n"), nil); err == nil {
		t.Fatal("http-port 70000 accepted")
	}
}
