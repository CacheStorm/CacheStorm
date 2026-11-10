package server

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

func startToggleServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

// CONFIG SET appendonly yes switches the AOF on while the server runs, as
// Redis does: the dataset is written to a new AOF, and every write from then
// on, the ones made while that file is written included, follows it.
func TestConfigSetAppendOnlySwitchesTheAOFOn(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Persistence.AOF = false
	cfg.Persistence.AOFSync = "everysec"
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	for i := range 200 {
		c.do("SET", "before:"+strconv.Itoa(i), "v")
	}
	c.do("SELECT", "3")
	c.do("HSET", "db3:hash", "f", "v")
	c.do("SELECT", "0")
	if got := c.do("CONFIG", "GET", "appendonly"); string(got.Array[1].Bulk) != "no" {
		t.Fatalf("appendonly before: %v", got)
	}

	// A client keeps incrementing while the AOF is switched on.
	var acked atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := dialRepl(t, s)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if v := w.do("INCR", "counter"); v.Type == ':' {
				acked.Store(v.Int)
			}
		}
	}()
	time.Sleep(20 * time.Millisecond)
	if got := c.do("CONFIG", "SET", "appendonly", "yes"); got.Str != "OK" {
		t.Fatalf("CONFIG SET appendonly yes: %v", got)
	}
	if got := c.do("CONFIG", "GET", "appendonly"); string(got.Array[1].Bulk) != "yes" {
		t.Fatalf("appendonly after: %v", got)
	}
	waitFor(t, "the AOF to be on", func() bool { return s.aof() != nil })
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
	c.do("SET", "after", "v")
	waitFor(t, "the switch to finish", func() bool {
		s.aofToggleMu.Lock()
		defer s.aofToggleMu.Unlock()
		return true
	})
	if _, err := os.Stat(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")); err != nil {
		t.Fatal(err)
	}
	want := acked.Load()
	stopReplServer(s)

	cfg2 := aofTxConfig(t)
	cfg2.Persistence.DataDir = cfg.Persistence.DataDir
	cfg2.Persistence.AOFSync = "everysec"
	s2 := startToggleServer(t, cfg2)
	defer stopReplServer(s2)
	c2 := dialRepl(t, s2)
	if got := c2.do("GET", "counter"); string(got.Bulk) != strconv.FormatInt(want, 10) {
		t.Fatalf("counter after restart: %q, %d INCRs acknowledged", got.Bulk, want)
	}
	for _, key := range []string{"before:0", "before:199", "after"} {
		if got := c2.do("EXISTS", key); got.Int != 1 {
			t.Errorf("%s lost", key)
		}
	}
	c2.do("SELECT", "3")
	if got := c2.do("HGET", "db3:hash", "f"); string(got.Bulk) != "v" {
		t.Errorf("db 3 hash: %v", got)
	}
}

// CONFIG SET appendonly no writes out and closes the AOF; later writes no
// longer reach it.
func TestConfigSetAppendOnlyNoClosesTheAOF(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Persistence.AOFSync = "everysec"
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	c.do("SET", "kept", "v")
	if got := c.do("CONFIG", "SET", "appendonly", "no"); got.Str != "OK" {
		t.Fatalf("CONFIG SET appendonly no: %v", got)
	}
	waitFor(t, "the AOF to be off", func() bool { return s.aof() == nil })
	c.do("SET", "not-in-aof", "v")
	if got := c.do("CONFIG", "GET", "appendonly"); string(got.Array[1].Bulk) != "no" {
		t.Fatalf("appendonly: %v", got)
	}
	cmds, err := persistence.NewAOFReader().Load(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, cmd := range cmds {
		if cmd.Name == "SET" {
			keys = append(keys, string(cmd.Args[0]))
		}
	}
	if len(keys) != 1 || keys[0] != "kept" {
		t.Fatalf("SETs in the AOF: %v", keys)
	}
}

// CONFIG SET appendfsync changes the policy while the AOF runs.
func TestConfigSetAppendFsync(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Persistence.AOFSync = "everysec"
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	for _, policy := range []string{"always", "no", "everysec"} {
		if got := c.do("CONFIG", "SET", "appendfsync", policy); got.Str != "OK" {
			t.Fatalf("CONFIG SET appendfsync %s: %v", policy, got)
		}
		if got := c.do("CONFIG", "GET", "appendfsync"); string(got.Array[1].Bulk) != policy {
			t.Fatalf("appendfsync: %v", got)
		}
		if got := c.do("SET", "k:"+policy, "v"); got.Str != "OK" {
			t.Fatalf("SET under %s: %v", policy, got)
		}
	}
	if got := c.do("CONFIG", "SET", "appendfsync", "sometimes"); got.Type != '-' {
		t.Fatalf("an invalid policy was accepted: %v", got)
	}
}

// Without persistence there is no data directory to put an AOF in.
func TestConfigSetAppendOnlyNeedsPersistence(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.Enabled = false
	cfg.Plugins.Metrics.Enabled = false
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	if got := c.do("CONFIG", "SET", "appendonly", "yes"); got.Type != '-' {
		t.Fatalf("CONFIG SET appendonly yes without persistence: %v", got)
	}
	if got := c.do("CONFIG", "GET", "appendonly"); string(got.Array[1].Bulk) != "no" {
		t.Fatalf("appendonly: %v", got)
	}
}
