package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestBitmapGrowthFailurePreservesAOFAndWatch(t *testing.T) {
	for _, mode := range []string{"direct", "exec", "script"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			cfg.Memory.MaxMemory = "4kb"
			cfg.Memory.EvictionPolicy = "noeviction"
			s, run := aofTxStart(t, cfg)
			run("SETBIT", "k", "0", "1")
			deadline := time.Now().Add(time.Minute).UnixMilli()
			run("PEXPIREAT", "k", strconv.FormatInt(deadline, 10))
			run("ADDTAG", "k", "tag")
			path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			usage := s.store.MemoryTracker().Usage()
			version := s.store.WatchKey("k")
			defer s.store.UnwatchKey("k")
			var reply string
			switch mode {
			case "direct":
				reply = run("BITFIELD", "k", "SET", "u8", "0", "7", "SET", "u8", "65528", "1")
			case "exec":
				run("MULTI")
				run("BITFIELD", "k", "SET", "u8", "0", "7", "SET", "u8", "65528", "1")
				reply = run("EXEC")
			case "script":
				reply = run("EVAL", "return redis.call('BITFIELD',KEYS[1],'SET','u8',0,7,'SET','u8',65528,1)", "1", "k")
			}
			if !strings.Contains(reply, "OOM") {
				t.Fatalf("growth succeeded: %q", reply)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("rejected growth recorded AOF effects")
			}
			entry, ok := s.store.Get("k")
			if !ok || entry.Value.String() != string([]byte{128}) || entry.ExpiresAt.Load() != deadline || len(entry.Tags) != 1 || s.store.GetVersion("k") != version || s.store.MemoryTracker().Usage() != usage || s.store.MemUsage() != usage {
				t.Fatal("rejected growth changed data, metadata, watch or accounting")
			}
		})
	}
}

func TestBitmapAcceptedGrowthAndReadOnlyReplay(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Memory.MaxMemory = "4kb"
	cfg.Memory.EvictionPolicy = "noeviction"
	s, run := aofTxStart(t, cfg)
	run("SET", "k", string([]byte{128}), "PX", "60000")
	run("ADDTAG", "k", "tag")
	deadline, _ := s.store.ExpiryDeadline("k")
	if got := run("BITFIELD", "k", "SET", "u8", "#0", "7", "SET", "i16", "#1", "-3"); got != "*2\r\n:128\r\n:0\r\n" {
		t.Fatalf("write=%q", got)
	}
	before, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	run("BITFIELD", "missing", "GET", "u8", "100")
	run("BITFIELD_RO", "missing", "GET", "i64", "0")
	run("MULTI")
	run("BITFIELD", "missing", "GET", "u8", "100")
	run("BITFIELD_RO", "missing", "GET", "i64", "0")
	run("EXEC")
	run("EVAL", "return redis.call('BITFIELD',KEYS[1],'GET','u8',100)", "1", "missing")
	after, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || s.store.Exists("missing") {
		t.Fatal("read-only bitfield persisted or created a key")
	}
	if s.store.MemoryTracker().Usage() != s.store.MemUsage() {
		t.Fatal("accepted growth charged twice")
	}
	stopReplServer(s)
	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := restored.store.Get("k")
	if !ok || entry.Value.String() != string([]byte{7, 0, 255, 253}) || entry.ExpiresAt.Load() != deadline || len(entry.Tags) != 1 || restored.store.Exists("missing") {
		t.Fatal("bitmap history lost bytes, metadata or read-only effects")
	}
	if restored.store.MemoryTracker().Usage() != restored.store.MemUsage() {
		t.Fatal("replay accounting differs from shards")
	}
}

func TestHistoricalBitfieldFramesRemainRecoverable(t *testing.T) {
	for _, versioned := range []bool{false, true} {
		t.Run(strconv.FormatBool(versioned), func(t *testing.T) {
			cfg := aofTxConfig(t)
			data := encodeArgs("BITFIELD", "u64", "SET", "u64", "0", "-1")
			data = append(data, encodeArgs("BITFIELD", "uppercase", "SET", "I8", "0", "-1")...)
			data = append(data, encodeArgs("BITFIELD", "legacy-read", "GET", "u8", "0")...)
			data = append(data, encodeArgs("SETBIT", "numeric", "+7", "01")...)
			data = append(data, encodeArgs("BITFIELD", "old-sat", "OVERFLOW", "SAT", "SET", "u08", "+0", "+300")...)
			if versioned {
				data = append(encodeArgs("CACHESTORM.AOF", "2"), data...)
			}
			path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			for key, length := range map[string]int{"u64": 8, "uppercase": 1, "legacy-read": 0, "numeric": 1, "old-sat": 1} {
				entry, ok := s.store.Get(key)
				if !ok || len(entry.Value.String()) != length {
					t.Fatalf("historical %s not restored", key)
				}
			}
			if entry, _ := s.store.Get("old-sat"); entry.Value.String() != string([]byte{44}) {
				t.Fatal("historical SET overflow behavior changed")
			}
			reader := persistence.NewAOFReader()
			if _, _, err := reader.LoadValid(path); err != nil || reader.Version != persistence.AOFVersion {
				t.Fatalf("migration err=%v version=%d", err, reader.Version)
			}
		})
	}
}

func TestRejectedBitmapGrowthEvictionInvalidatesReplicaCheckpoint(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "target", strings.Repeat("x", 200))
	mc.do("SELECT", "3")
	mc.do("SET", "other", strings.Repeat("y", 400))
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
		c.Memory.MaxMemory = "1kb"
		c.Memory.EvictionPolicy = "allkeys-lru"
		c.Replication.Role = "replica"
		c.Replication.ReadOnly = false
		c.Replication.MasterHost = "127.0.0.1"
		c.Replication.MasterPort = master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	rc := dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	if _, ok := replica.repl.SavedState(); !ok {
		t.Fatal("initial replica was not resumable")
	}
	result := rc.do("SETBIT", "target", "5599", "1")
	if result.Type != resp.TypeError {
		t.Fatalf("growth unexpectedly succeeded: %#v", result)
	}
	if replica.store.Exists("target") || !master.store.Exists("target") {
		t.Fatal("test did not cause a local eviction")
	}
	if _, ok := replica.repl.SavedState(); ok {
		t.Fatal("failed command left locally evicted data eligible for upstream checkpoint")
	}
	fullBefore, _, _ := syncStats(master)
	master.repl.DisconnectReplicas()
	eventually(t, 5*time.Second, "evicted replica recovered with full resync", func() bool {
		full, _, _ := syncStats(master)
		_, eligible := replica.repl.SavedState()
		return full > fullBefore && replica.store.Exists("target") && eligible
	})
	waitSynced(t, replica, master)
}

func TestBitfieldNormalizedEffectsKeepOverflowStateOnReplay(t *testing.T) {
	cfg := aofTxConfig(t)
	s, run := aofTxStart(t, cfg)
	run("BITFIELD", "sat", "OVERFLOW", "SAT", "SET", "u8", "0", "300")
	run("BITFIELD", "failed-growth", "OVERFLOW", "FAIL", "SET", "u8", "0", "300")
	run("BITFIELD", "counter", "INCRBY", "u8", "#2", "300")
	expected := map[string]string{"sat": string([]byte{255}), "failed-growth": string([]byte{0}), "counter": string([]byte{0, 0, 44})}
	for key, want := range expected {
		entry, ok := s.store.Get(key)
		if !ok || entry.Value.String() != want {
			t.Fatalf("initial %s wrong", key)
		}
	}
	stopReplServer(s)
	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range expected {
		entry, ok := restored.store.Get(key)
		if !ok || entry.Value.String() != want {
			t.Fatalf("replay changed %s", key)
		}
	}
}
