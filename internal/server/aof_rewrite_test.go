package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func aofRewriteConfig(t *testing.T, dir string, edit func(*config.Config)) *config.Config {
	return replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = true
		c.Persistence.AOFSync = "always"
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "0"
		if edit != nil {
			edit(c)
		}
	})
}

func waitAOFRewrites(t *testing.T, s *Server, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.snapshotMu.Lock()
		done, running, err := s.aofRewrites, s.aofRewriteRunning, s.aofRewriteLastError
		s.snapshotMu.Unlock()
		if err != nil {
			t.Fatalf("AOF rewrite failed: %v", err)
		}
		if done >= want && !running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("AOF rewrite count did not reach %d", want)
}

func TestBGREWRITEAOFCompactsAndKeepsData(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	for i := 0; i < 200; i++ {
		c.do("INCR", "counter")
		c.do("SET", "overwritten", strings.Repeat("x", i))
	}
	c.do("SET", "ttl", "v", "PXAT", "16725225600000")
	c.do("SELECT", "3")
	c.do("RPUSH", "list", "a", "b")
	c.do("SELECT", "0")
	path := filepath.Join(dir, "appendonly.aof")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.do("BGREWRITEAOF"); got.Type != resp.TypeSimpleString || !strings.Contains(got.Str, "rewriting started") {
		t.Fatalf("BGREWRITEAOF = %+v", got)
	}
	waitAOFRewrites(t, s, 1)
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() >= before.Size()/4 {
		t.Fatalf("rewrite did not compact the AOF: %d -> %d bytes", before.Size(), after.Size())
	}
	c.do("INCR", "counter")
	info := c.do("INFO", "persistence")
	if !strings.Contains(string(info.Bulk), "aof_rewrites:1") || !strings.Contains(string(info.Bulk), "aof_last_bgrewrite_status:ok") {
		t.Fatalf("INFO persistence = %q", info.Bulk)
	}
	stopReplServer(s)

	restored := startReplServerCfg(t, cfg)
	r := dialRepl(t, restored)
	if got := r.do("GET", "counter"); string(got.Bulk) != "201" {
		t.Fatalf("counter after rewrite and restart = %+v, want 201", got)
	}
	if got := r.do("STRLEN", "overwritten"); got.Int != 199 {
		t.Fatalf("overwritten length = %+v, want 199", got)
	}
	if got := r.do("PEXPIRETIME", "ttl"); got.Int != 16725225600000 {
		t.Fatalf("ttl = %+v", got)
	}
	r.do("SELECT", "3")
	if got := r.do("LRANGE", "list", "0", "-1"); len(got.Array) != 2 {
		t.Fatalf("db3 list = %+v", got)
	}
}

func TestBGREWRITEAOFWithoutAOFIsRefused(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	if got := c.do("BGREWRITEAOF"); got.Type != resp.TypeError {
		t.Fatalf("BGREWRITEAOF without an AOF = %+v, want an error", got)
	}
}

func TestAOFRewritesAutomaticallyAfterGrowth(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) {
		c.Persistence.AOFRewritePercentage = 100
		c.Persistence.AOFRewriteMinSize = "16kb"
	})
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	value := strings.Repeat("v", 512)
	for i := 0; i < 100; i++ {
		c.do("SET", "same", value)
	}
	waitAOFRewrites(t, s, 1)
	if size := s.aof.Size(); size > 4<<10 {
		t.Fatalf("AOF after automatic rewrite is %d bytes", size)
	}
	stopReplServer(s)
	restored := startReplServerCfg(t, cfg)
	if got := dialRepl(t, restored).do("GET", "same"); string(got.Bulk) != value {
		t.Fatalf("value after automatic rewrite and restart = %d bytes", len(got.Bulk))
	}
}

func TestAOFRewriteDueThresholds(t *testing.T) {
	s := &Server{aofRewriteMinSize: 100, aofRewritePercentage: 100, aofRewriteBase: 100}
	cases := []struct {
		size    int64
		failed  bool
		running bool
		want    bool
	}{
		{size: 50, want: false},
		{size: 199, want: false},
		{size: 200, want: true},
		{size: 200, running: true, want: false},
		{size: 199, failed: true, want: false},
		{size: 200, failed: true, want: true},
	}
	for _, c := range cases {
		s.aofRewriteRunning = c.running
		s.aofRewriteLastError = nil
		if c.failed {
			s.aofRewriteLastError = os.ErrClosed
		}
		if got := s.aofRewriteDueForSize(c.size); got != c.want {
			t.Errorf("size=%d failed=%v running=%v: due=%v, want %v", c.size, c.failed, c.running, got, c.want)
		}
	}
	s.aofRewritePercentage = 0
	s.aofRewriteLastError = nil
	s.aofRewriteRunning = false
	if s.aofRewriteDueForSize(1 << 30) {
		t.Error("a zero percentage must disable automatic rewrites")
	}
}

func TestAOFRewriteBatchesLargeCollections(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	for i := 0; i < 200; i++ {
		n := strconv.Itoa(i)
		c.do("HSET", "h", "f"+n, "v"+n)
		c.do("SADD", "s", "m"+n)
		c.do("ZADD", "z", n+".5", "m"+n)
		c.do("RPUSH", "l", n)
	}
	c.do("BGREWRITEAOF")
	waitAOFRewrites(t, s, 1)
	data, err := os.ReadFile(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "HSET"); n != 4 {
		t.Fatalf("200 hash fields were written as %d HSET commands, want 4 batches of at most 64", n)
	}
	stopReplServer(s)

	r := dialRepl(t, startReplServerCfg(t, cfg))
	if got := r.do("HLEN", "h"); got.Int != 200 {
		t.Fatalf("HLEN = %+v", got)
	}
	if got := r.do("HGET", "h", "f199"); string(got.Bulk) != "v199" {
		t.Fatalf("HGET = %+v", got)
	}
	if got := r.do("SCARD", "s"); got.Int != 200 {
		t.Fatalf("SCARD = %+v", got)
	}
	if got := r.do("ZSCORE", "z", "m150"); string(got.Bulk) != "150.5" {
		t.Fatalf("ZSCORE = %+v", got)
	}
	if got := r.do("LINDEX", "l", "199"); string(got.Bulk) != "199" {
		t.Fatalf("LINDEX = %+v", got)
	}
}
