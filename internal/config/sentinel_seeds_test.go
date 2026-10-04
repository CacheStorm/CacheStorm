package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSentinelSeedsLoadFromYAML covers the config half of the gossip bootstrap:
// the `sentinel.seeds` list must actually be populated by Load, otherwise an
// operator could write it in a config file and have it silently dropped before
// it ever reached the sentinel.
func TestSentinelSeedsLoadFromYAML(t *testing.T) {
	content := `
sentinel:
  id: "sentinel-from-yaml"
  addr: "10.0.0.5"
  port: 26380
  quorum: 3
  down_after: "12s"
  failover_time: "4m"
  seeds:
    - "10.0.0.1:26379"
    - "10.0.0.2:26379"
    - "10.0.0.3:26379"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"10.0.0.1:26379", "10.0.0.2:26379", "10.0.0.3:26379"}
	if len(cfg.Sentinel.Seeds) != len(want) {
		t.Fatalf("seeds = %v, want %v", cfg.Sentinel.Seeds, want)
	}
	for i, w := range want {
		if cfg.Sentinel.Seeds[i] != w {
			t.Errorf("seeds[%d] = %q, want %q", i, cfg.Sentinel.Seeds[i], w)
		}
	}

	// The rest of the section must still load alongside it, so adding seeds did
	// not disturb the existing fields.
	if cfg.Sentinel.ID != "sentinel-from-yaml" {
		t.Errorf("id = %q, want sentinel-from-yaml", cfg.Sentinel.ID)
	}
	if cfg.Sentinel.Quorum != 3 {
		t.Errorf("quorum = %d, want 3", cfg.Sentinel.Quorum)
	}
	if d := cfg.Sentinel.DownAfterDuration(); d.String() != "12s" {
		t.Errorf("down_after = %v, want 12s", d)
	}
}

// TestSentinelSeedsDefaultIsEmpty pins the default: a single-sentinel deployment
// configures no peers, and must not acquire an invented one from Default().
func TestSentinelSeedsDefaultIsEmpty(t *testing.T) {
	cfg := Default()
	if len(cfg.Sentinel.Seeds) != 0 {
		t.Errorf("Default().Sentinel.Seeds = %v, want empty — a lone sentinel has no "+
			"peers to announce to", cfg.Sentinel.Seeds)
	}
}
