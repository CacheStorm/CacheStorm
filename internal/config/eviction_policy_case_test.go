package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/server"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestValidatedEvictionPolicyCaseReachesRuntime(t *testing.T) {
	for _, policy := range []string{"noeviction", "NOEVICTION", "NoEviction"} {
		cfg := config.Default()
		cfg.Memory.MaxMemory = "1mb"
		cfg.Memory.EvictionPolicy = policy
		cfg.HTTP.Enabled = false
		cfg.Plugins.Metrics.Enabled = false
		if err := config.Validate(cfg); err != nil {
			t.Fatal(err)
		}
		if err := config.Validate(cfg); err != nil {
			t.Fatal(err)
		}
		s, err := server.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Store().Set("retain", &store.StringValue{Data: []byte("value")}, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
		got := s.Store().Evictor().ForceEvict(1)
		t.Logf("policy=%s EXPECTED: evicted=0 ACTUAL: %d", policy, got)
		if got != 0 || !s.Store().Exists("retain") {
			t.Fatal("noeviction policy lost")
		}
	}
	for _, policy := range []string{"noeviction", "allkeys-lru", "allkeys-lfu", "volatile-lru", "allkeys-random"} {
		cfg := config.Default()
		cfg.Memory.EvictionPolicy = strings.ToUpper(policy)
		if err := config.Validate(cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Memory.EvictionPolicy != policy {
			t.Fatalf("policy normalization: %q want %q", cfg.Memory.EvictionPolicy, policy)
		}
	}
	for _, policy := range []string{"", "unknown", "NOEVICTION-extra"} {
		cfg := config.Default()
		cfg.Memory.EvictionPolicy = policy
		if err := config.Validate(cfg); err == nil {
			t.Fatalf("invalid policy accepted: %q", policy)
		}
	}
	if err := config.Validate(config.Default()); err != nil {
		t.Fatal(err)
	}
	fmt.Println("FIX VERIFIED")
}
