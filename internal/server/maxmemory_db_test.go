package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// maxmemory is global in Redis: a database other than 0 is bounded by it too.
func TestMaxMemoryBoundsEveryDatabase(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Bind = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.HTTP.Enabled = false
	cfg.Plugins.Metrics.Enabled = false
	cfg.Memory.MaxMemory = "1mb"
	cfg.Memory.EvictionPolicy = "noeviction"
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer s.Stop(context.Background())

	c, err := net.Dial("tcp", s.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	r := bufio.NewReader(c)
	send := func(args ...string) string {
		var b strings.Builder
		fmt.Fprintf(&b, "*%d\r\n", len(args))
		for _, a := range args {
			fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
		}
		_ = c.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := c.Write([]byte(b.String())); err != nil {
			t.Fatal(err)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return line
	}
	if got := send("SELECT", "3"); got != "+OK\r\n" {
		t.Fatalf("SELECT: %q", got)
	}
	val := strings.Repeat("x", 1024)
	for i := 0; i < 10000; i++ {
		if got := send("RPUSH", "big", val); strings.HasPrefix(got, "-OOM") {
			return
		}
	}
	t.Fatal("10 MB pushed into database 3 never hit the 1 MB limit")
}
