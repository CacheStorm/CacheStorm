package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMSetAtomicReaderGateX(t *testing.T) {
	for _, name := range []string{"EXEC", "MSET", "MSETNX"} {
		t.Run(name, func(t *testing.T) {
			s := &Server{store: store.NewStore()}
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				ctx := command.NewContext(name, [][]byte{[]byte("a"), []byte("1"), []byte("b"), []byte("2")}, s.store, nil)
				unlock := s.holdGate(ctx)
				defer unlock()
				if err := s.store.Set("a", &store.StringValue{Data: []byte("1")}, store.SetOptions{}); err != nil {
					panic(err)
				}
				close(entered)
				<-release
				if err := s.store.Set("b", &store.StringValue{Data: []byte("2")}, store.SetOptions{}); err != nil {
					panic(err)
				}
			}()
			<-entered
			admitted := s.gate.global.TryRLock()
			partial := false
			if admitted {
				partial = s.store.Exists("a") && !s.store.Exists("b")
				s.gate.global.RUnlock()
			}
			close(release)
			<-done
			t.Logf("EXPECTED: reader excluded during batch | ACTUAL: admitted=%v partial=%v", admitted, partial)
			if admitted {
				t.Error("reader can observe partially applied batch")
			}
		})
	}
}

func TestMSetAtomicGateHeldThroughCommitX(t *testing.T) {
	for _, name := range []string{"MSET", "MSETNX"} {
		t.Run(name, func(t *testing.T) {
			s := startReplServer(t, nil)
			entered, release := make(chan struct{}), make(chan struct{})
			s.router.SetPostExecute(func(cmd string, _ [][]byte) {
				if cmd == "MSET" {
					close(entered)
					<-release
				}
			})
			c := dialRepl(t, s)
			done := make(chan *resp.Value, 1)
			go func() { done <- c.do(name, "a", "1", "b", "2") }()
			select {
			case <-entered:
			case got := <-done:
				t.Fatalf("returned before commit: %+v", got)
			}
			if s.gate.global.TryRLock() {
				s.gate.global.RUnlock()
				t.Error("reader admitted before commit completed")
			}
			close(release)
			if got := <-done; got.Type == resp.TypeError {
				t.Fatalf("batch=%+v", got)
			}
		})
	}
}

func TestMSetAtomicPersistenceX(t *testing.T) {
	for _, mode := range []string{"direct", "exec", "lua", "rewrite"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			cfg.Memory.MaxMemory, cfg.Memory.EvictionPolicy = "2048", "noeviction"
			cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
			s := startReplServerCfg(t, cfg)
			c := dialRepl(t, s)
			run := func(args ...string) string {
				var out bytes.Buffer
				if err := resp.NewWriter(&out).WriteValue(c.do(args...)); err != nil {
					t.Fatal(err)
				}
				return out.String()
			}
			for _, key := range []string{"a", "b"} {
				if got := run("SET", key, strings.Repeat("x", 600), "PXAT", "4102444800000"); got != "+OK\r\n" {
					t.Fatal(got)
				}
				run("ADDTAG", key, "old")
			}
			before := s.aof.Size()
			args := []string{"MSET", "rejected", strings.Repeat("x", 3000), "a", "wrong"}
			var reply string
			switch mode {
			case "exec":
				run("MULTI")
				run(args...)
				reply = run("EXEC")
			case "lua":
				reply = run("EVAL", "return redis.pcall('MSET',KEYS[1],ARGV[1],KEYS[2],ARGV[2])", "2", "rejected", "a", strings.Repeat("x", 3000), "wrong")
			default:
				reply = run(args...)
			}
			if !strings.Contains(reply, "-OOM") || s.aof.Size() != before || s.store.Exists("rejected") {
				t.Fatalf("rejection changed journal: %q", reply)
			}
			if got := run("MSET", "a", strings.Repeat("z", 1200), "b", ""); got != "+OK\r\n" {
				t.Fatal(got)
			}
			before = s.aof.Size()
			if got := run("MSETNX", "c", "new", "a", "wrong"); got != ":0\r\n" || s.aof.Size() != before {
				t.Fatalf("NX no-op=%q", got)
			}
			if mode == "rewrite" {
				if err := s.aof.Rewrite(replication.DBSource{DBs: s.dbs}); err != nil {
					t.Fatal(err)
				}
			}
			stopReplServer(s)
			data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
			if err != nil || bytes.Contains(data, []byte("rejected")) {
				t.Fatalf("rejected effect in AOF: %v", err)
			}
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"a", "b"} {
				entry, ok := restored.store.Get(key)
				want := ""
				if key == "a" {
					want = strings.Repeat("z", 1200)
				}
				if !ok || entry.Value.String() != want || entry.ExpiresAt.Load() != 0 || len(entry.TagList()) != 0 {
					t.Fatalf("restart mismatch: %s %+v", key, entry)
				}
			}
			if restored.store.Exists("rejected") || restored.store.Exists("c") || restored.store.MemUsage() != restored.store.MemoryTracker().Usage() {
				t.Fatal("restart has rejected state/accounting divergence")
			}
		})
	}
}
