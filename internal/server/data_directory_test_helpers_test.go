package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
)

func newOwnedTestServer(t *testing.T, cfg *config.Config) (*Server, error) {
	t.Helper()
	s, err := New(cfg)
	if err == nil && cfg.Persistence.Enabled {
		t.Cleanup(func() { stopReplServer(s) })
	}
	return s, err
}
