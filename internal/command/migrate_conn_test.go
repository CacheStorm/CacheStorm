package command

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// fakeMigrateTarget accepts connections and answers +OK to every command,
// recording the command names it saw and how many connections it accepted.
type fakeMigrateTarget struct {
	ln    net.Listener
	conns atomic.Int32
	mu    sync.Mutex
	cmds  []string
	open  []net.Conn
}

func newFakeMigrateTarget(t *testing.T) *fakeMigrateTarget {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeMigrateTarget{ln: ln}
	t.Cleanup(func() {
		_ = ln.Close()
		f.dropConns()
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			f.conns.Add(1)
			f.mu.Lock()
			f.open = append(f.open, c)
			f.mu.Unlock()
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeMigrateTarget) serve(c net.Conn) {
	rd := resp.NewReader(c)
	w := resp.NewWriter(c)
	for {
		v, err := rd.ReadValue()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.cmds = append(f.cmds, strings.ToUpper(string(v.Array[0].Bulk)))
		f.mu.Unlock()
		_ = w.WriteSimpleString("OK")
		_ = w.Flush()
	}
}

func (f *fakeMigrateTarget) dropConns() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.open {
		_ = c.Close()
	}
	f.open = nil
}

func (f *fakeMigrateTarget) commands() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.cmds, " ")
}

func TestMigrateReusesTheTargetConnection(t *testing.T) {
	f := newFakeMigrateTarget(t)
	port := strconv.Itoa(f.ln.Addr().(*net.TCPAddr).Port)
	s := store.NewStore()
	for _, k := range []string{"a", "b", "c"} {
		migrateRun(t, s, "SET", k, "v")
	}

	for _, k := range []string{"a", "b"} {
		if v := migrateRun(t, s, "MIGRATE", "127.0.0.1", port, k, "0", "1000"); v.Type != resp.TypeSimpleString {
			t.Fatalf("MIGRATE %s: %v %q", k, v.Type, v.Err)
		}
	}
	if n := f.conns.Load(); n != 1 {
		t.Fatalf("two MIGRATEs opened %d connections, want 1", n)
	}
	if got := f.commands(); got != "SELECT RESTORE RESTORE" {
		t.Fatalf("target saw %q, want SELECT once", got)
	}

	// The target drops the cached connection; the next MIGRATE retries on a
	// new one instead of failing.
	f.dropConns()
	if v := migrateRun(t, s, "MIGRATE", "127.0.0.1", port, "c", "1", "1000"); v.Type != resp.TypeSimpleString {
		t.Fatalf("MIGRATE after the target closed the connection: %v %q", v.Type, v.Err)
	}
	if n := f.conns.Load(); n != 2 {
		t.Fatalf("connections %d, want 2 after the reconnect", n)
	}
	if got := f.commands(); got != "SELECT RESTORE RESTORE SELECT RESTORE" {
		t.Fatalf("target saw %q", got)
	}
	if migrateExists(t, s, "c") != 0 {
		t.Fatal("the migrated key is still here")
	}
	if migrateCachedConns() == 0 {
		t.Fatal("the connection was not cached after the transfer")
	}
}
