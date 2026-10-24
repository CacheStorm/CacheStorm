package server

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// Sentinel is a separate role: a data server opens the sentinel port only
// when sentinel.enabled is set. Every server used to listen on 26379.
func TestSentinelListenerIsOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := config.Default()
		cfg.Server.Bind = "127.0.0.1"
		cfg.Server.Port = 0
		cfg.HTTP.Enabled = false
		cfg.Plugins.Metrics.Enabled = false
		cfg.Sentinel.Enabled = enabled
		cfg.Sentinel.Port = freeTCPPort(t)
		s, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Sentinel.Port))
		listening := false
		for i := 0; i < 20 && !listening; i++ {
			if c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
				c.Close()
				listening = true
			} else if !enabled {
				break
			} else {
				time.Sleep(25 * time.Millisecond)
			}
		}
		s.Stop(context.Background())
		if listening != enabled {
			t.Fatalf("sentinel.enabled=%v but sentinel port listening=%v", enabled, listening)
		}
	}
}
