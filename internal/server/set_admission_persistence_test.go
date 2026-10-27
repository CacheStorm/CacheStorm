package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetAdmissionPersistenceX(t *testing.T) {
	for _, kind := range []string{"sadd", "move"} {
		for _, mode := range []string{"direct", "exec", "lua", "rewrite"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				cfg := aofTxConfig(t)
				cfg.Memory.MaxMemory, cfg.Memory.EvictionPolicy = "1024", "noeviction"
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
				member := "kept"
				if kind == "move" {
					member = strings.Repeat("m", 700)
				}
				if got := run("SADD", "s", member); got != ":1\r\n" {
					t.Fatal(got)
				}
				run("ADDTAG", "s", "tag")
				run("PEXPIREAT", "s", "4102444800000")
				before := s.aof.Size()
				args := []string{"SADD", "s", "rejected", strings.Repeat("x", 1000)}
				if kind == "move" {
					args = []string{"SMOVE", "s", strings.Repeat("d", 200), member}
				}
				var reply string
				switch mode {
				case "exec":
					run("MULTI")
					run(args...)
					reply = run("EXEC")
				case "lua":
					script := "return redis.pcall(ARGV[1],KEYS[1],ARGV[2],ARGV[3])"
					reply = run("EVAL", script, "1", "s", args[0], args[2], args[3])
				default:
					reply = run(args...)
				}
				if !strings.Contains(reply, "-OOM") || s.aof.Size() != before {
					t.Fatalf("refusal changed journal: %q", reply)
				}
				entry, _ := s.store.Get("s")
				set := entry.Value.(*store.SetValue)
				if len(set.Members) != 1 || entry.ExpiresAt.Load() != 4102444800000 || len(entry.TagList()) != 1 {
					t.Fatal("refusal changed live set/metadata")
				}
				key := "s"
				if kind == "move" {
					key = "d"
					if got := run("SMOVE", "s", "d", member); got != ":1\r\n" {
						t.Fatal(got)
					}
				} else if got := run("SADD", "s", "new"); got != ":1\r\n" {
					t.Fatal(got)
				}
				before = s.aof.Size()
				if got := run("SADD", key, member, member); got != ":0\r\n" || s.aof.Size() != before {
					t.Fatalf("duplicate=%q", got)
				}
				if got := run("SMOVE", key, "absent", "missing"); got != ":0\r\n" || s.aof.Size() != before {
					t.Fatalf("missing=%q", got)
				}
				if got := run("SMOVE", key, key, member); got != ":1\r\n" || s.aof.Size() != before {
					t.Fatalf("self=%q", got)
				}
				if mode == "rewrite" {
					if err := s.aof.Rewrite(replication.DBSource{DBs: s.dbs}); err != nil {
						t.Fatal(err)
					}
				}
				stopReplServer(s)
				data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
				if err != nil || bytes.Contains(data, []byte("rejected")) {
					t.Fatalf("rejected effect persisted: %v", err)
				}
				restored, err := newOwnedTestServer(t, cfg)
				if err != nil {
					t.Fatal(err)
				}
				entry, ok := restored.store.Get(key)
				if !ok {
					t.Fatal("accepted set lost")
				}
				set = entry.Value.(*store.SetValue)
				if _, ok := set.Members[member]; !ok || restored.store.Exists("absent") || restored.store.MemUsage() != restored.store.MemoryTracker().Usage() {
					t.Fatal("restart diverged")
				}
				if kind == "move" {
					if restored.store.Exists("s") || entry.ExpiresAt.Load() != 0 || len(entry.TagList()) != 0 || len(set.Members) != 1 {
						t.Fatal("move metadata/source mismatch")
					}
				} else if entry.ExpiresAt.Load() != 4102444800000 || len(entry.TagList()) != 1 || len(set.Members) != 2 {
					t.Fatal("SADD metadata mismatch")
				}
			})
		}
	}
}
