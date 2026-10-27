package server

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type gatedEntryTransferValue struct {
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func (v *gatedEntryTransferValue) Type() store.DataType { return store.DataTypeString }
func (v *gatedEntryTransferValue) String() string       { return "value" }
func (v *gatedEntryTransferValue) Clone() store.Value {
	return &store.StringValue{Data: []byte("value")}
}
func (v *gatedEntryTransferValue) SizeOf() int64 {
	if v.armed.CompareAndSwap(true, false) {
		close(v.entered)
		<-v.release
	}
	return 29
}

func TestEntryTransfersHoldAtomicCommandGate(t *testing.T) {
	for _, name := range []string{"RENAME", "RENAMENX", "COPY", "MOVE"} {
		t.Run(name, func(t *testing.T) {
			s := startReplServer(t, nil)
			c := dialRepl(t, s)
			value := &gatedEntryTransferValue{entered: make(chan struct{}), release: make(chan struct{})}
			if err := s.store.Set("src", value, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			value.armed.Store(true)
			replies := make(chan *resp.Value, 1)
			go func() {
				dest := "dst"
				if name == "MOVE" {
					dest = "1"
				}
				replies <- c.do(name, "src", dest)
			}()
			select {
			case <-value.entered:
			case got := <-replies:
				t.Fatalf("command returned before guarded transfer: %+v", got)
			}
			if s.gate.global.TryRLock() {
				s.gate.global.RUnlock()
				t.Error("reader could interleave inside transfer")
			}
			close(value.release)
			if got := <-replies; got.Type == resp.TypeError {
				t.Fatalf("transfer=%+v", got)
			}
		})
	}
}

func TestEntryTransfersPersistOnlyAcceptedEffects(t *testing.T) {
	for _, mode := range []string{"direct", "exec", "lua", "rewrite"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			cfg.Memory.MaxMemory, cfg.Memory.EvictionPolicy = "4096", "noeviction"
			cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
			s := startReplServerCfg(t, cfg)
			client := dialRepl(t, s)
			run := func(name string, args ...string) string {
				reply := client.do(append([]string{name}, args...)...)
				var out bytes.Buffer
				if err := resp.NewWriter(&out).WriteValue(reply); err != nil {
					t.Fatal(err)
				}
				return out.String()
			}
			if got := run("SET", "src", strings.Repeat("s", 2200), "PXAT", "4102444800000"); got != "+OK\r\n" {
				t.Fatalf("seed=%q", got)
			}
			run("ADDTAG", "src", "tag")
			before := s.aof.Size()
			var reply string
			switch mode {
			case "exec":
				run("MULTI")
				run("COPY", "src", "rejected")
				reply = run("EXEC")
			case "lua":
				reply = run("EVAL", "return redis.pcall('COPY',KEYS[1],KEYS[2])", "2", "src", "rejected")
			default:
				reply = run("COPY", "src", "rejected")
			}
			if !strings.Contains(reply, "-OOM") || s.aof.Size() != before || s.store.Exists("rejected") {
				t.Fatalf("rejection changed state or journal: %q", reply)
			}
			if got := run("RENAME", "src", "renamed"); got != "+OK\r\n" {
				t.Fatalf("rename=%q", got)
			}
			if got := run("MOVE", "renamed", "1"); got != ":1\r\n" {
				t.Fatalf("move=%q", got)
			}
			entry, ok := s.dbs.DB(1).Get("renamed")
			if !ok || entry.ExpiresAt.Load() != 4102444800000 || !reflect.DeepEqual(entry.Tags, []string{"tag"}) {
				t.Fatal("live transfer lost metadata")
			}
			if mode == "rewrite" {
				if err := s.aof.Rewrite(replication.DBSource{DBs: s.dbs}); err != nil {
					t.Fatal(err)
				}
			}
			stopReplServer(s)
			data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
			if err != nil || bytes.Contains(data, []byte("rejected")) {
				t.Fatalf("rejected copy reached persistence: %v", err)
			}
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			entry, ok = restored.dbs.DB(1).Get("renamed")
			if !ok || entry.Value.String() != strings.Repeat("s", 2200) || entry.ExpiresAt.Load() != 4102444800000 || !reflect.DeepEqual(entry.Tags, []string{"tag"}) || restored.store.Exists("src") || restored.store.Exists("renamed") || restored.store.Exists("rejected") {
				t.Fatal("restart diverged from accepted transfers")
			}
		})
	}
}

func TestHistoricalOversizedCopyRemainsReplayable(t *testing.T) {
	cfg := aofTxConfig(t)
	data := encodeArgs("CACHESTORM.AOF", "2")
	for _, args := range [][]string{{"SET", "src", "kept"}, {"COPY", "src", strings.Repeat("x", store.MaxKeySize+1)}, {"SET", "after", "loaded"}} {
		data = append(data, encodeArgs(args...)...)
	}
	if err := os.WriteFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"), data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if entry, ok := s.store.Get("src"); !ok || entry.Value.String() != "kept" || !s.store.Exists("after") || s.store.KeyCount() != 2 {
		t.Fatal("historical no-op copy changed recovered data")
	}
}
