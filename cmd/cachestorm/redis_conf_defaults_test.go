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
