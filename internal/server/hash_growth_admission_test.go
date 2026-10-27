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

func TestHashGrowthAdmissionPersistenceX(t *testing.T) {
	for _, name := range []string{"HSET", "HMSET", "HSETNX", "HINCRBY", "HINCRBYFLOAT"} {
		for _, mode := range []string{"direct", "exec", "lua", "rewrite"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				cfg := aofTxConfig(t)
				cfg.Memory.MaxMemory, cfg.Memory.EvictionPolicy = "1000", "noeviction"
				cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
				s := startReplServerCfg(t, cfg)
				c := dialRepl(t, s)
				run := func(args ...string) string {
					var output bytes.Buffer
					if err := resp.NewWriter(&output).WriteValue(c.do(args...)); err != nil {
						t.Fatal(err)
					}
					return output.String()
				}
				numeric := name == "HINCRBY" || name == "HINCRBYFLOAT"
				initial := "kept"
				if numeric {
					initial = "99"
				}
				if got := run("HSET", "h", "f", initial); got != ":1\r\n" {
					t.Fatal(got)
				}
				run("ADDTAG", "h", "tag")
				run("PEXPIREAT", "h", "4102444800000")
				if numeric {
					size := 949 - s.store.MemUsage() - (store.EntryOverhead + 24 + int64(len("p")) + 16)
					if got := run("SET", "p", strings.Repeat("p", int(size))); got != "+OK\r\n" {
						t.Fatal(got)
					}
				}
				before := s.aof.Size()
				field, value := "rejected", strings.Repeat("x", 1000)
				if numeric {
					field, value = "f", "1"
				}
				var reply string
				switch mode {
				case "exec":
					run("MULTI")
					run(name, "h", field, value)
					reply = run("EXEC")
				case "lua":
					reply = run("EVAL", "return redis.pcall(ARGV[1],KEYS[1],ARGV[2],ARGV[3])", "1", "h", name, field, value)
				default:
					reply = run(name, "h", field, value)
				}
				if !strings.Contains(reply, "-OOM") || s.aof.Size() != before {
					t.Fatalf("rejection changed journal: %q", reply)
				}
				entry, _ := s.store.Get("h")
				hash := entry.Value.(*store.HashValue)
				if string(hash.Fields["f"]) != initial || len(hash.Fields) != 1 || entry.ExpiresAt.Load() != 4102444800000 || len(entry.TagList()) != 1 {
					t.Fatal("rejected growth changed hash/metadata")
				}
				if got := run("HSETNX", "h", "f", "wrong"); got != ":0\r\n" || s.aof.Size() != before {
					t.Fatalf("NX no-op=%q", got)
				}
				want := initial
				if numeric {
					want = "98"
					if got := run(name, "h", "f", "-1"); strings.HasPrefix(got, "-") {
						t.Fatal(got)
					}
				} else if got := run("HSET", "h", "f", initial); got != ":0\r\n" {
					t.Fatal(got)
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
				entry, ok := restored.store.Get("h")
				if !ok {
					t.Fatal("hash lost on restart")
				}
				hash = entry.Value.(*store.HashValue)
				if string(hash.Fields["f"]) != want || len(hash.Fields) != 1 || entry.ExpiresAt.Load() != 4102444800000 || len(entry.TagList()) != 1 || restored.store.MemUsage() != restored.store.MemoryTracker().Usage() {
					t.Fatal("restart diverged from accepted effects")
				}
			})
		}
	}
}
