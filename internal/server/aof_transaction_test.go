package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func aofTxConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.HTTP.Enabled = false
	cfg.Plugins.Metrics.Enabled = false
	cfg.Persistence.Enabled = true
	cfg.Persistence.AOF = true
	cfg.Persistence.DataDir = t.TempDir()
	cfg.Persistence.AOFSync = "always"
	return cfg
}

func aofTxStart(t *testing.T, cfg *config.Config) (*Server, func(name string, args ...string) string) {
	t.Helper()
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.aof.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	tx := command.NewTransaction()
	run := func(name string, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		argv := make([][]byte, len(args))
		for i, arg := range args {
			argv[i] = []byte(arg)
		}
		ctx := command.NewContext(name, argv, s.store, resp.NewWriter(&out))
		ctx.Transaction = tx
		if err := s.router.Execute(ctx); err != nil {
			ctx.Writer.WriteError(err.Error())
		}
		return out.String()
	}
	return s, run
}

func TestAOFRestoresTransactionWrites(t *testing.T) {
	cfg := aofTxConfig(t)
	s, run := aofTxStart(t, cfg)

	run("SET", "direct", "1")
	run("MULTI")
	run("SET", "tx:a", "1")
	run("HSET", "tx:h", "f", "v")
	run("INCR", "tx:a")
	run("GET", "tx:a")
	if got := run("EXEC"); got != "*4\r\n+OK\r\n:1\r\n:2\r\n$1\r\n2\r\n" {
		t.Fatalf("EXEC reply: %q", got)
	}
	stopReplServer(s)

	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"direct", "tx:a", "tx:h"} {
		if !restored.store.Exists(key) {
			t.Fatalf("%s was not restored", key)
		}
	}
	entry, _ := restored.store.Get("tx:a")
	if sv, ok := entry.Value.(*store.StringValue); !ok || string(sv.Data) != "2" {
		t.Fatalf("tx:a restored as %#v, want 2", entry.Value)
	}
}

func TestAOFDiscardsTransactionWithoutExec(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	data := "*3\r\n$3\r\nSET\r\n$6\r\nbefore\r\n$1\r\n1\r\n" +
		"*1\r\n$5\r\nMULTI\r\n" +
		"*3\r\n$3\r\nSET\r\n$4\r\nhalf\r\n$1\r\n1\r\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !s.store.Exists("before") {
		t.Fatal("the command before the transaction was not restored")
	}
	if s.store.Exists("half") {
		t.Fatal("a transaction with no EXEC marker was applied")
	}
}

func TestAOFTruncatesTornTail(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	complete := "*3\r\n$3\r\nSET\r\n$4\r\nkept\r\n$1\r\n1\r\n"
	torn := "*3\r\n$3\r\nSET\r\n$4\r\nto"
	if err := os.WriteFile(path, []byte(complete+torn), 0o644); err != nil {
		t.Fatal(err)
	}

	s, run := aofTxStart(t, cfg)
	if !s.store.Exists("kept") {
		t.Fatal("the complete command was not restored")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	reader := persistence.NewAOFReader()
	_, valid, err := reader.LoadValid(path)
	if err != nil || valid != info.Size() || reader.Version != persistence.AOFVersion {
		t.Fatalf("recovered AOF is not a complete upgraded snapshot: valid %d size %d version %d error %v", valid, info.Size(), reader.Version, err)
	}

	run("SET", "after", "1")
	stopReplServer(s)

	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("reload after truncation: %v", err)
	}
	if !restored.store.Exists("kept") || !restored.store.Exists("after") {
		t.Fatal("writes before and after the torn tail must both survive a restart")
	}
}

func TestAOFRefusesCorruptFile(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	data := "*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\n1\r\n" +
		"garbage\r\n" +
		"*3\r\n$3\r\nSET\r\n$1\r\nb\r\n$1\r\n1\r\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newOwnedTestServer(t, cfg); err == nil {
		t.Fatal("a corrupt AOF must stop startup instead of being silently ignored")
	}
}
