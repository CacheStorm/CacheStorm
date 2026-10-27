package replication

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestFullSyncFailurePreservesExistingDataset(t *testing.T) {
	for _, kind := range []string{"valid", "truncated", "reply-error", "command-error", "persistence-error", "maxmemory", "forbidden"} {
		t.Run(kind, func(t *testing.T) {
			m, dbs := newTestManager(t, Config{})
			old := dbs.DB(0)
			if kind == "maxmemory" {
				old.ConfigureMemory(1024, store.EvictionAllKeysLRU, 80, 90, 5)
			}
			if err := old.Set("old", &store.StringValue{Data: []byte("kept")}, store.SetOptions{Tags: []string{"old-tag"}}); err != nil {
				t.Fatal(err)
			}
			entry, _ := old.Get("old")
			version := old.WatchKey("old")
			defer old.UnwatchKey("old")
			payload := encodeCommand("SET", "new", "replacement")
			size := len(payload)
			switch kind {
			case "maxmemory":
				payload = encodeCommand("SET", "new", strings.Repeat("x", 2048))
				size = len(payload)
			case "forbidden":
				payload = append(payload, encodeCommand("PUBLISH", "channel", "not-a-snapshot-operation")...)
				size = len(payload)
			case "truncated":
				size++
			case "reply-error":
				payload = append(payload, encodeCommand("INCR", "new")...)
				size = len(payload)
			case "command-error":
				payload = append(payload, encodeCommand("NOT_A_COMMAND")...)
				size = len(payload)
			case "persistence-error":
				m.host.(*fakeHost).persistError = errors.New("injected disk failure")
			}
			var input bytes.Buffer
			fmt.Fprintf(&input, "$%d\r\n", size)
			input.Write(payload)
			err := m.loadSnapshot(bufio.NewReader(&input), "0123456789012345678901234567890123456789", 1234)
			if kind == "valid" {
				if err != nil || old.Exists("old") || !old.Exists("new") {
					t.Fatalf("valid snapshot did not replace data: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid snapshot succeeded")
			}
			if got, ok := old.Get("old"); !ok || got != entry {
				t.Fatal("failed snapshot removed or replaced existing data")
			}
			if old.Exists("new") || old.GetVersion("old") != version {
				t.Fatal("failed snapshot published data or invalidated WATCH")
			}
			if keys := old.GetTagIndex().GetKeys("old-tag"); len(keys) != 1 || keys[0] != "old" {
				t.Fatalf("failed snapshot changed tag index: %v", keys)
			}
		})
	}
}

func TestRejectedLiveCommandDoesNotAdvanceOffset(t *testing.T) {
	for _, name := range []string{"INCR", "NOT_A_COMMAND"} {
		t.Run(name, func(t *testing.T) {
			m, dbs := newTestManager(t, Config{})
			if err := dbs.DB(0).Set("k", &store.StringValue{Data: []byte("not-an-integer")}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			m.offset, m.cached = 77, true
			m.mu.Unlock()
			args := [][]byte{[]byte(name), []byte("k")}
			raw := encodeCommand(name, "k")
			_, err := m.applyFromMaster(args, raw)
			if err == nil {
				t.Fatal("rejected command returned no failure")
			}
			m.mu.Lock()
			offset, cached := m.offset, m.cached
			m.mu.Unlock()
			if offset != 77 || cached {
				t.Fatalf("rejected command accepted: offset=%d cached=%v", offset, cached)
			}
		})
	}
}

func TestFailedSnapshotDoesNotPublishFunctions(t *testing.T) {
	for _, failed := range []bool{false, true} {
		m, dbs := newTestManager(t, Config{})
		command.RegisterFunctionCommands(m.router)
		m.router.SetFunctionRegistry(command.NewFunctionRegistry(dbs.DB(0)))
		load := func(router *command.Router, library string) {
			ctx := command.NewContext("FUNCTION", [][]byte{[]byte("LOAD"), []byte("#!lua name=" + library + "\nredis.register_function('" + library + "', function() return 1 end)")}, dbs.DB(0), nil)
			if err := router.ExecuteSilent(ctx); err != nil || ctx.Writer.Errors() > 0 {
				t.Fatalf("load %s: %v", library, err)
			}
		}
		load(m.router, "old_library")
		old := m.router.FunctionRegistry()
		before := old.DumpPayload()
		other := m.router.ForkForReplay(store.NewStore(), []string{"FUNCTION"})
		load(other, "new_library")
		payload := encodeCommand("FUNCTION", "RESTORE", string(other.FunctionRegistry().DumpPayload()), "FLUSH")
		if failed {
			payload = append(payload, encodeCommand("INCR", "bad", "extra")...)
		}
		var input bytes.Buffer
		fmt.Fprintf(&input, "$%d\r\n", len(payload))
		input.Write(payload)
		err := m.loadSnapshot(bufio.NewReader(&input), "0123456789012345678901234567890123456789", 1234)
		if (err != nil) != failed {
			t.Fatalf("failed=%v: load=%v", failed, err)
		}
		if failed && (m.router.FunctionRegistry() != old || !bytes.Equal(before, old.DumpPayload())) {
			t.Fatal("rejected snapshot changed live function libraries")
		}
		if !failed && bytes.Equal(before, m.router.FunctionRegistry().DumpPayload()) {
			t.Fatal("successful snapshot did not replace functions")
		}
	}
}

func TestValidLiveCommandAdvancesOnlyItsBytes(t *testing.T) {
	m, dbs := newTestManager(t, Config{})
	m.offset = 77
	args := [][]byte{[]byte("SET"), []byte("k"), []byte("v")}
	raw := encodeCommand("SET", "k", "v")
	ack, err := m.applyFromMaster(args, raw)
	if err != nil || ack || m.offset != 77+int64(len(raw)) || !m.cached || !dbs.DB(0).Exists("k") {
		t.Fatalf("valid command failed: ack=%v err=%v offset=%d", ack, err, m.offset)
	}
}

func TestUnsynchronizedReplicaDoesNotExportDataset(t *testing.T) {
	m, _ := newTestManager(t, Config{})
	m.role, m.cached = RoleReplica, false
	if err := m.fullSync(&replica{}, false); err == nil {
		t.Fatal("unsynchronized replica exported a full snapshot")
	}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if err := b.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	m.attach(a, nil, syncRequest{replid: "?", offset: -1})
	var data [1]byte
	if _, err := b.Read(data[:]); err != io.EOF {
		t.Fatalf("unsynchronized downstream was not closed: %v", err)
	}
}
