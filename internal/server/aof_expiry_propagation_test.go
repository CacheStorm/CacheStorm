package server

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAOFRelativeExpiryKeepsOriginalDeadline(t *testing.T) {
	for _, mode := range []string{"SET-PX", "SET-EX", "SETEX", "PSETEX", "EXPIRE", "PEXPIRE", "GETEX"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, run := aofTxStart(t, cfg)
			run("SET", "k", "v")
			switch mode {
			case "SET-PX":
				run("SET", "k", "v", "PX", "60000")
			case "SET-EX":
				run("SET", "k", "v", "EX", "60")
			case "SETEX":
				run("SETEX", "k", "60", "v")
			case "PSETEX":
				run("PSETEX", "k", "60000", "v")
			case "EXPIRE":
				run("EXPIRE", "k", "60")
			case "PEXPIRE":
				run("PEXPIRE", "k", "60000")
			case "GETEX":
				run("GETEX", "k", "PX", "60000")
			}
			entry, ok := s.store.Get("k")
			if !ok {
				t.Fatal("missing live key")
			}
			deadline := entry.ExpiresAt.Load()
			stopReplServer(s)
			time.Sleep(5 * time.Millisecond)
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			entry, ok = restored.store.Get("k")
			if !ok || entry.ExpiresAt.Load() != deadline {
				t.Fatalf("restart shifted deadline: got %#v want %d", entry, deadline)
			}
		})
	}
}

func TestAOFExpiryHistoryDoesNotDiscardEarlierMutations(t *testing.T) {
	for _, mode := range []string{"PERSIST", "EXTEND", "RECREATE", "EXPIRE-FINAL", "RENAME", "COPY", "MOVE"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, _ := aofTxStart(t, cfg)
			c := &aofSession{t: t, s: s, tx: command.NewTransaction()}
			c.do("HSET", "h", "before", "one")
			deadline := time.Now().Add(200 * time.Millisecond).UnixMilli()
			c.do("PEXPIREAT", "h", strconv.FormatInt(deadline, 10))
			c.do("HSET", "h", "after", "two")
			key, db := "h", 0
			switch mode {
			case "PERSIST":
				c.do("PERSIST", "h")
			case "EXTEND":
				c.do("PEXPIRE", "h", "60000")
			case "RENAME":
				c.do("RENAME", "h", "renamed")
				key = "renamed"
				c.do("PERSIST", key)
			case "COPY":
				c.do("COPY", "h", "copy", "DB", "3")
				key, db = "copy", 3
				c.do("SELECT", "3")
				c.do("PERSIST", key)
			case "MOVE":
				c.do("MOVE", "h", "3")
				db = 3
				c.do("SELECT", "3")
				c.do("PERSIST", key)
			}
			if remaining := time.Until(time.UnixMilli(deadline)); remaining > 0 {
				time.Sleep(remaining + time.Millisecond)
			}
			if mode == "RECREATE" {
				c.do("HSET", "h", "new", "three")
			}
			stopReplServer(s)
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			entry, ok := restored.dbs.DB(db).Get(key)
			if mode == "EXPIRE-FINAL" {
				if ok {
					t.Fatal("expired hash resurrected")
				}
				return
			}
			if !ok {
				t.Fatal("live hash lost during replay")
			}
			hash, ok := entry.Value.(*store.HashValue)
			if !ok {
				t.Fatalf("wrong restored type: %T", entry.Value)
			}
			if mode == "RECREATE" {
				if len(hash.Fields) != 1 || string(hash.Fields["new"]) != "three" {
					t.Fatalf("expired history leaked into recreated hash: %v", hash.Fields)
				}
			} else if len(hash.Fields) != 2 || string(hash.Fields["before"]) != "one" || string(hash.Fields["after"]) != "two" {
				t.Fatalf("earlier hash mutation lost: %v", hash.Fields)
			}
		})
	}
}

func TestLegacyAOFUpgradesToVersionedSnapshot(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	if err := os.WriteFile(path, encodeArgs("SET", "legacy", "kept"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !s.store.Exists("legacy") {
		t.Fatal("legacy value lost")
	}
	r := persistence.NewAOFReader()
	if _, _, err := r.LoadValid(path); err != nil || r.Version != persistence.AOFVersion {
		t.Fatalf("legacy upgrade failed: version %d error %v", r.Version, err)
	}
}

func TestAOFExpiryPropagationThroughExecAndScript(t *testing.T) {
	for _, mode := range []string{"EXEC", "EVAL", "KEEPTTL"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, run := aofTxStart(t, cfg)
			switch mode {
			case "EXEC":
				run("MULTI")
				run("SET", "k", "v", "PX", "60000")
				run("EXEC")
			case "EVAL":
				run("EVAL", "return redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])", "1", "k", "v", "60000")
			case "KEEPTTL":
				run("SET", "k", "v", "PX", "60000")
				run("SET", "k", "updated", "XX", "KEEPTTL", "GET")
			}
			entry, ok := s.store.Get("k")
			if !ok {
				t.Fatal("operation did not store key")
			}
			deadline, value := entry.ExpiresAt.Load(), entry.Value.String()
			stopReplServer(s)
			time.Sleep(5 * time.Millisecond)
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			entry, ok = restored.store.Get("k")
			if !ok || entry.ExpiresAt.Load() != deadline || entry.Value.String() != value {
				t.Fatalf("replay changed %s result", mode)
			}
		})
	}
}

func TestAOFNonMutatingConditionalCommandsAreNotRecorded(t *testing.T) {
	cfg := aofTxConfig(t)
	s, run := aofTxStart(t, cfg)
	run("SET", "k", "original")
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"SET", "k", "ignored", "NX"}, {"SET", "k", "ignored", "NX", "GET"}, {"SET", "missing", "ignored", "XX"},
		{"SETNX", "k", "ignored"}, {"MSETNX", "k", "ignored", "other", "ignored"},
		{"EXPIRE", "k", "60", "XX"}, {"EXPIRE", "k", "60", "GT"}, {"PEXPIRE", "missing", "60000"},
		{"PERSIST", "k"}, {"GETEX", "k"}, {"GETEX", "missing", "PX", "60000"}, {"GETDEL", "missing"},
	} {
		run(args[0], args[1:]...)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("no-op commands changed the durable log: %v", err)
	}
	if s.store.Exists("other") || s.store.Exists("missing") {
		t.Fatal("conditional operation changed data")
	}
}

func TestDelayedReplicaDoesNotRestartRelativeTTL(t *testing.T) {
	master := startReplServer(t, nil)
	master.dbs.StopExpiry()
	mc := dialRepl(t, master)
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
		c.Replication.Role = "replica"
		c.Replication.MasterHost, c.Replication.MasterPort = "127.0.0.1", master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	rc := dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	replica.LockWrites()
	got := mc.do("SET", "delayed", "v", "PX", "100")
	deadline := time.Now().Add(120 * time.Millisecond)
	if got.Str != "OK" {
		replica.UnlockWrites()
		t.Fatalf("SET: %#v", got)
	}
	time.Sleep(time.Until(deadline))
	replica.UnlockWrites()
	waitSynced(t, replica, master)
	if value := rc.do("GET", "delayed"); !value.IsNull {
		t.Fatalf("delayed replication restarted TTL: %#v", value)
	}
}

func TestDelayedReplicaPreservesExpiredHistoryBeforePersist(t *testing.T) {
	master := startReplServer(t, nil)
	master.dbs.StopExpiry()
	mc := dialRepl(t, master)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	replica.LockWrites()
	mc.do("SET", "delayed-persist", "whole", "PX", "100")
	mc.do("APPEND", "delayed-persist", "-value")
	mc.do("PERSIST", "delayed-persist")
	time.Sleep(120 * time.Millisecond)
	replica.UnlockWrites()
	waitSynced(t, replica, master)
	if value := rc.do("GET", "delayed-persist"); string(value.Bulk) != "whole-value" {
		t.Fatalf("late PERSIST lost prior value: %#v", value)
	}
}

func TestReplicaAOFPartialRestartCanPersistExpiredPhysicalValue(t *testing.T) {
	master := startReplServer(t, nil)
	master.dbs.StopExpiry()
	mc := dialRepl(t, master)
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
		c.Replication.Role = "replica"
		c.Replication.MasterHost, c.Replication.MasterPort = "127.0.0.1", master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	rc := dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	mc.do("SET", "pending", "original", "PX", "200")
	waitSynced(t, replica, master)
	stopReplServer(replica)
	mc.do("PERSIST", "pending")
	time.Sleep(220 * time.Millisecond)
	restarted, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.store.Exists("pending") {
		t.Fatal("expired physical value visible before upstream PERSIST")
	}
	if _, exists := restarted.store.ExpiryDeadline("pending"); !exists {
		t.Fatal("replica startup discarded upstream history")
	}
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	rr := dialRepl(t, restarted)
	waitLinkUp(t, rr)
	waitSynced(t, restarted, master)
	if value := rr.do("GET", "pending"); string(value.Bulk) != "original" {
		t.Fatalf("resumed PERSIST lost physical history: %#v", value)
	}
	full, partial, _ := syncStats(master)
	if full != 1 || partial != 1 {
		t.Fatalf("restart did not use retained history: full %d partial %d", full, partial)
	}
}

func TestAOFRejectedNXDoesNotCreateKeyAfterExpiry(t *testing.T) {
	cfg := aofTxConfig(t)
	s, run := aofTxStart(t, cfg)
	deadline := time.Now().Add(100 * time.Millisecond).UnixMilli()
	if got := run("SET", "k", "original", "PXAT", strconv.FormatInt(deadline, 10)); got != "+OK\r\n" {
		t.Fatal(got)
	}
	if got := run("SET", "k", "ghost", "NX"); got != "$-1\r\n" {
		t.Fatal(got)
	}
	stopReplServer(s)
	if remaining := time.Until(time.UnixMilli(deadline)); remaining > 0 {
		time.Sleep(remaining + time.Millisecond)
	}
	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if restored.store.Exists("k") {
		t.Fatal("failed NX write recreated the expired key")
	}
}

func TestRESTExpiryKeepsOriginalDeadline(t *testing.T) {
	cfg := aofTxConfig(t)
	s, _ := aofTxStart(t, cfg)
	h := &HTTPServer{srv: s, store: s.store, dbs: s.dbs}
	w := httptest.NewRecorder()
	h.handleKeys(w, httptest.NewRequest("POST", "/api/keys", strings.NewReader(`{"key":"rest-ttl","value":"v","ttl":60000000000}`)))
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	deadline, ok := s.store.ExpiryDeadline("rest-ttl")
	if !ok || deadline <= 0 {
		t.Fatal("missing expiry")
	}
	stopReplServer(s)
	time.Sleep(5 * time.Millisecond)
	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := restored.store.ExpiryDeadline("rest-ttl")
	if !ok || got != deadline {
		t.Fatalf("expiry got=%d want=%d", got, deadline)
	}
}

func TestRESTReadsWaitForAuthoritativeExpiryApply(t *testing.T) {
	for _, path := range []string{"/api/key/expired", "/api/keys"} {
		t.Run(path, func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, run := aofTxStart(t, cfg)
			s.store.StopExpiry()
			s.dbs.SetPassiveExpiry(true)
			run("SET", "expired", "private", "PXAT", strconv.FormatInt(time.Now().UnixMilli()+20, 10))
			time.Sleep(25 * time.Millisecond)
			h := &HTTPServer{srv: s, store: s.store, dbs: s.dbs}
			s.LockWrites()
			s.dbs.BeginAuthoritativeApply()
			done := make(chan *httptest.ResponseRecorder, 1)
			started := make(chan struct{})
			go func() {
				close(started)
				w := httptest.NewRecorder()
				r := httptest.NewRequest("GET", path, nil)
				if path == "/api/keys" {
					h.handleKeys(w, r)
				} else {
					h.handleKey(w, r)
				}
				done <- w
			}()
			<-started
			early := false
			select {
			case <-done:
				early = true
			case <-time.After(30 * time.Millisecond):
			}
			s.dbs.EndAuthoritativeApply()
			s.UnlockWrites()
			if early {
				t.Fatal("REST read bypassed authoritative apply gate")
			}
			select {
			case w := <-done:
				if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), `"key":"expired"`) {
					t.Fatalf("expired history exposed: %s", w.Body.String())
				}
				if path != "/api/keys" && w.Code != 404 {
					t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
				}
			case <-time.After(time.Second):
				t.Fatal("REST read did not resume")
			}
		})
	}
}
