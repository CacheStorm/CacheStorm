package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
)

func TestAOFStartupRejectsUnreplayableCommands(t *testing.T) {
	for _, mode := range []string{"unknown", "unknown-torn", "arity", "wrongtype", "integer", "database", "nested-multi", "stray-exec", "marker-arguments", "non-persisted", "transaction-error"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			data := encodeArgs("SET", "before", "kept")
			switch mode {
			case "unknown", "unknown-torn":
				data = append(data, encodeArgs("UNKNOWN.REPLAY")...)
			case "arity":
				data = append(data, encodeArgs("SET", "missing-value")...)
			case "wrongtype":
				data = append(data, encodeArgs("HSET", "before", "f", "v")...)
			case "integer":
				data = append(data, encodeArgs("INCR", "before")...)
			case "database":
				data = append(data, encodeArgs("SELECT", "999")...)
			case "nested-multi":
				data = append(data, encodeArgs("MULTI")...)
				data = append(data, encodeArgs("MULTI")...)
				data = append(data, encodeArgs("EXEC")...)
			case "stray-exec":
				data = append(data, encodeArgs("EXEC")...)
			case "marker-arguments":
				data = append(data, encodeArgs("MULTI", "invalid")...)
				data = append(data, encodeArgs("EXEC")...)
			case "non-persisted":
				data = append(data, encodeArgs("PING")...)
			case "transaction-error":
				data = append(data, encodeArgs("MULTI")...)
				data = append(data, encodeArgs("HSET", "before", "f", "v")...)
				data = append(data, encodeArgs("EXEC")...)
			}
			data = append(data, encodeArgs("SET", "after", "must-not-run")...)
			if mode == "unknown-torn" {
				data = append(data, []byte("*3\r\n$3\r\nSET\r\n$6\r\npartial")...)
			}
			path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if s, err := newOwnedTestServer(t, cfg); err == nil || s != nil {
				t.Fatalf("startup accepted %s AOF: server %p error %v", mode, s, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, data) {
				t.Fatalf("invalid complete AOF was modified: %v", err)
			}
		})
	}
}

func TestAOFIncompleteTransactionDoesNotSwallowLaterWrites(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	prefix := encodeArgs("SET", "before", "kept")
	data := append(append([]byte(nil), prefix...), encodeArgs("MULTI")...)
	data = append(data, encodeArgs("SELECT", "3")...)
	data = append(data, encodeArgs("SET", "uncommitted", "lost")...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, run := aofTxStart(t, cfg)
	if s.dbs.DB(3).Exists("uncommitted") {
		t.Fatal("incomplete transaction applied")
	}
	reader := persistence.NewAOFReader()
	commands, valid, err := reader.LoadValid(path)
	info, statErr := os.Stat(path)
	if err != nil || statErr != nil || valid != info.Size() || reader.Version != persistence.AOFVersion {
		t.Fatalf("transaction recovery did not produce complete versioned AOF: %v %v", err, statErr)
	}
	for _, cmd := range commands {
		if cmd.Name == "MULTI" || cmd.Name == "EXEC" {
			t.Fatal("unfinished transaction remained in recovered snapshot")
		}
	}
	if got := run("SET", "acknowledged", "durable"); got != "+OK\r\n" {
		t.Fatalf("write failed: %q", got)
	}
	if err := s.aof.StopWithError(); err != nil {
		t.Fatal(err)
	}
	stopReplServer(s)
	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.store.Exists("before") || !restored.store.Exists("acknowledged") {
		t.Fatal("an acknowledged write after torn transaction disappeared on restart")
	}
}

func TestAOFDiscardedTransactionPreservesSelectedDatabase(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	var data []byte
	for _, args := range [][]string{{"MULTI"}, {"SELECT", "3"}, {"SET", "discarded", "v"}, {"DISCARD"}, {"SET", "committed", "v"}} {
		data = append(data, encodeArgs(args...)...)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !s.store.Exists("committed") || s.dbs.DB(3).Exists("discarded") || s.aof.CurrentDB() != 0 {
		t.Fatal("discarded transaction affected data or selected DB")
	}
}
