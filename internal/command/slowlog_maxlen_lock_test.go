package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: SLOWLOG.CONFIG MAXLEN writes SlowLog.MaxSize as a bare field,
// bypassing the mutex, while addLocked reads that same field under the lock on
// every recorded command.
//
// WRITER — internal/command/monitoring_commands.go:143, no lock:
//	  case "MAXLEN":
//	    maxLen := int(parseInt64(value))
//	    store.GlobalSlowLog.MaxSize = maxLen      // <-- unsynchronised field write
//
// READER — internal/store/metrics.go:345, inside addLocked, called by AddIfSlow
// with sl.mu held:
//	  if len(sl.Entries) > sl.MaxSize {
//	    sl.Entries = sl.Entries[len(sl.Entries)-sl.MaxSize:]
//
// AddIfSlow is invoked from internal/server/connection.go:162 for EVERY command
// dispatched on EVERY connection, so the guarded read runs concurrently with a
// SLOWLOG.CONFIG MAXLEN from any client.
//
// The sibling THRESHOLD case in the very same switch uses SetThreshold, which
// locks -- so MAXLEN is the odd one out in its own function. store.SlowLog even
// has SetMaxSize (metrics.go:316) precisely for this; CONFIG SET slowlog-max-len
// was routed through it, but this alias was not.
//
// SLOWLOG.CONFIG is registered at monitoring_commands.go:20, so this is reachable
// from a client command.

// slowlogMaxLenSetup enables recording and lowers the threshold so AddIfSlow
// actually stores entries.
func slowlogMaxLenSetup(t *testing.T) {
	t.Helper()
	sl := store.GlobalSlowLog
	sl.Clear()
	sl.SetEnabled(true)
	sl.SetThreshold(0)
	sl.SetMaxSize(128)
	t.Cleanup(func() {
		sl.Clear()
		sl.SetThreshold(10 * time.Millisecond)
		sl.SetMaxSize(128)
	})
}

// TestSlowlogConfigMaxLenRacesAddIfSlow is the primary case: the unsynchronised
// MaxSize write racing the guarded read inside addLocked. Only meaningful
// under -race.
func TestSlowlogConfigMaxLenRacesAddIfSlow(t *testing.T) {
	slowlogMaxLenSetup(t)
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterMonitoringCommands(router)

	run := func(cmd string, args ...string) string {
		buf := &bytes.Buffer{}
		parsed := make([][]byte, len(args))
		for i, a := range args {
			parsed[i] = []byte(a)
		}
		if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		return buf.String()
	}

	done := make(chan struct{})
	// Writer: SLOWLOG.CONFIG MAXLEN — the bare field write at monitoring_commands.go:143.
	go func() {
		defer close(done)
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			for _, n := range []string{"2", "3", "5", "64", "7"} {
				run("SLOWLOG.CONFIG", "MAXLEN", n)
			}
		}
	}()
	// Reader: AddIfSlow takes sl.mu and then reads MaxSize inside addLocked.
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		store.GlobalSlowLog.AddIfSlow(50*time.Millisecond, "GET",
			[][]byte{[]byte("k")}, "10.0.0.1:1", "writer-test")
	}
	<-done
}

// TestSlowlogConfigControls is the control: MAXLEN must still cap the retained
// entries, and THRESHOLD must still apply — these pass before and after the fix
// and prove the race test above actually exercised the write path.
func TestSlowlogConfigMaxLenControls(t *testing.T) {
	slowlogMaxLenSetup(t)
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterMonitoringCommands(router)

	run := func(cmd string, args ...string) string {
		buf := &bytes.Buffer{}
		parsed := make([][]byte, len(args))
		for i, a := range args {
			parsed[i] = []byte(a)
		}
		if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		return buf.String()
	}

	// MAXLEN 3 caps the recorder at three entries.
	if out := run("SLOWLOG.CONFIG", "MAXLEN", "3"); out != "+OK\r\n" {
		t.Fatalf("SLOWLOG.CONFIG MAXLEN 3 = %q, want +OK", out)
	}
	for i := 0; i < 10; i++ {
		store.GlobalSlowLog.AddIfSlow(50*time.Millisecond, "GET",
			[][]byte{[]byte("k")}, "10.0.0.1:1", "ctrl")
	}
	if n := store.GlobalSlowLog.Len(); n != 3 {
		t.Fatalf("CONTROL FAIL: after MAXLEN 3 and 10 records the slowlog holds %d entries, want 3", n)
	}
	if n := store.GlobalSlowLog.MaxSize; n != 3 {
		t.Fatalf("CONTROL FAIL: MaxSize = %d after SLOWLOG.CONFIG MAXLEN 3, want 3", n)
	}

	// THRESHOLD in the same switch uses the locked setter and must still work.
	if out := run("SLOWLOG.CONFIG", "THRESHOLD", "5000"); out != "+OK\r\n" {
		t.Fatalf("SLOWLOG.CONFIG THRESHOLD 5000 = %q, want +OK", out)
	}
	store.GlobalSlowLog.AddIfSlow(time.Millisecond, "GET", nil, "10.0.0.1:1", "ctrl")
	if n := store.GlobalSlowLog.Len(); n != 3 {
		t.Fatalf("CONTROL FAIL: a 1ms command must stay below a 5000us threshold, got %d entries", n)
	}

	t.Log("CONTROL PASS: MAXLEN caps retention and THRESHOLD still gates recording")
}
