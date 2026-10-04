package command_test

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXCLAIM parsed min-idle-time, IDLE, TIME, RETRYCOUNT, FORCE
// and JUSTID and discarded them all (_ = minIdleTime; _ = force; _ =
// retryCount). Claim moved every pending entry unconditionally — even when a
// huge min-idle-time should have gated the claim — incremented the delivery
// counter under JUSTID, ignored RETRYCOUNT, and never FORCE-claimed entries
// that were not in the PEL. Redis: only entries idle >= min-idle-time are
// claimed; JUSTID does not increment the counter; RETRYCOUNT sets it; FORCE
// claims stream entries that were never delivered; IDLE/TIME override the
// last-delivery timestamp.
func TestXClaimOptions(t *testing.T) {
	type pendingInfo struct {
		consumer   string
		deliveries int64
	}
	newEnv := func(t *testing.T) (*store.Store, func(name string, args ...string) *resp.Value, func() map[string]pendingInfo) {
		t.Helper()
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterStreamCommands(router)
		run := func(name string, args ...string) *resp.Value {
			t.Helper()
			argv := make([][]byte, len(args))
			for i, arg := range args {
				argv[i] = []byte(arg)
			}
			var buf bytes.Buffer
			ctx := command.NewContext(name, argv, s, resp.NewWriter(&buf))
			if err := router.Execute(ctx); err != nil {
				t.Fatalf("%s execution: %v", name, err)
			}
			v, err := resp.NewReader(&buf).ReadValue()
			if err != nil {
				t.Fatalf("%s reply: %v", name, err)
			}
			return v
		}
		pending := func() map[string]pendingInfo {
			t.Helper()
			got := run("XPENDING", "s", "g", "-", "+", "100")
			if got.Type != resp.TypeArray {
				t.Fatalf("XPENDING replied %v, want an array", got)
			}
			out := map[string]pendingInfo{}
			for _, e := range got.Array {
				if e.Type != resp.TypeArray || len(e.Array) < 4 {
					continue
				}
				out[string(e.Array[0].Bulk)] = pendingInfo{
					consumer:   string(e.Array[1].Bulk),
					deliveries: e.Array[3].Int,
				}
			}
			return out
		}
		return s, run, pending
	}
	seed := func(t *testing.T, run func(name string, args ...string) *resp.Value) {
		t.Helper()
		for _, id := range []string{"1-0", "2-0"} {
			if got := run("XADD", "s", id, "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != id {
				t.Fatalf("seed XADD replied %v %q, want %s", got.Type, got.Bulk, id)
			}
		}
		if got := run("XGROUP", "CREATE", "s", "g", "0"); got.Type != resp.TypeSimpleString || got.Str != "OK" {
			t.Fatalf("XGROUP CREATE replied %v %q, want OK", got.Type, got.Str)
		}
		if got := run("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"); got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("seed > delivered %d streams, want 1", len(got.Array))
		}
	}

	t.Run("min idle time gates the claim", func(t *testing.T) {
		_, run, pending := newEnv(t)
		seed(t, run)
		got := run("XCLAIM", "s", "g", "c2", "999999999999", "1-0")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("XCLAIM with huge min-idle-time claimed %v, want an empty reply", got)
		}
		info := pending()
		if info["1-0"].consumer != "c1" {
			t.Fatalf("1-0 moved to %q; a gated claim must not move ownership", info["1-0"].consumer)
		}
	})

	t.Run("min idle 0 claims", func(t *testing.T) {
		_, run, pending := newEnv(t)
		seed(t, run)
		got := run("XCLAIM", "s", "g", "c2", "0", "1-0")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("XCLAIM replied %v with %d entries, want the claimed entry", got, len(got.Array))
		}
		info := pending()
		if info["1-0"].consumer != "c2" || info["2-0"].consumer != "c1" {
			t.Fatalf("ownership after claim = %v/%v, want c2/c1", info["1-0"].consumer, info["2-0"].consumer)
		}
	})

	t.Run("JUSTID does not increment deliveries", func(t *testing.T) {
		_, run, pending := newEnv(t)
		seed(t, run)
		if got := run("XCLAIM", "s", "g", "c2", "0", "1-0"); got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("setup claim replied %v, want the entry", got)
		}
		got := run("XCLAIM", "s", "g", "c2", "0", "1-0", "JUSTID")
		if got.Type != resp.TypeArray || len(got.Array) != 1 || string(got.Array[0].Bulk) != "1-0" {
			t.Fatalf("JUSTID replied %v, want the bare id 1-0", got)
		}
		if info := pending(); info["1-0"].deliveries != 2 {
			t.Fatalf("deliveries after JUSTID = %d, want 2 (JUSTID must not increment)", info["1-0"].deliveries)
		}
	})

	t.Run("RETRYCOUNT sets deliveries", func(t *testing.T) {
		_, run, pending := newEnv(t)
		seed(t, run)
		got := run("XCLAIM", "s", "g", "c2", "0", "1-0", "RETRYCOUNT", "9", "JUSTID")
		if got.Type != resp.TypeArray || len(got.Array) != 1 || string(got.Array[0].Bulk) != "1-0" {
			t.Fatalf("JUSTID+RETRYCOUNT replied %v, want the bare id 1-0", got)
		}
		if info := pending(); info["1-0"].deliveries != 9 {
			t.Fatalf("deliveries after RETRYCOUNT 9 = %d, want 9", info["1-0"].deliveries)
		}
	})

	t.Run("FORCE claims undelivered stream entries", func(t *testing.T) {
		_, run, pending := newEnv(t)
		seed(t, run)
		if got := run("XADD", "s", "3-0", "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != "3-0" {
			t.Fatalf("XADD replied %v %q, want 3-0", got.Type, got.Bulk)
		}
		got := run("XCLAIM", "s", "g", "c2", "0", "3-0", "FORCE", "JUSTID")
		if got.Type != resp.TypeArray || len(got.Array) != 1 || string(got.Array[0].Bulk) != "3-0" {
			t.Fatalf("FORCE replied %v, want the bare id 3-0", got)
		}
		if info := pending(); info["3-0"].consumer != "c2" {
			t.Fatalf("FORCE must pend 3-0 to c2, pending = %v", info)
		}
	})

	t.Run("IDLE override does not satisfy the gate", func(t *testing.T) {
		_, run, pending := newEnv(t)
		seed(t, run)
		got := run("XCLAIM", "s", "g", "c2", "1000000", "2-0", "IDLE", "9999999", "JUSTID")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("IDLE override must not satisfy the min-idle gate; claimed %v", got)
		}
		if info := pending(); info["2-0"].consumer != "c1" {
			t.Fatalf("2-0 moved to %q; a gated claim must not move ownership", info["2-0"].consumer)
		}
	})

	t.Run("IDLE sets the delivery timestamp", func(t *testing.T) {
		_, run, _ := newEnv(t)
		seed(t, run)
		got := run("XCLAIM", "s", "g", "c2", "0", "2-0", "IDLE", "9999999", "JUSTID")
		if got.Type != resp.TypeArray || len(got.Array) != 1 || string(got.Array[0].Bulk) != "2-0" {
			t.Fatalf("IDLE claim replied %v, want the bare id 2-0", got)
		}
		got = run("XPENDING", "s", "g", "2-0", "2-0", "10", "c2")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("XPENDING replied %v, want the pending entry", got)
		}
		ts := got.Array[0].Array[2].Int
		// XPENDING's third column is idle milliseconds: IDLE 9999000 makes
		// the entry's idle ~9999000 (plus the milliseconds between the
		// claim and this read).
		if ts < 9999000-500 || ts > 9999000+5000 {
			t.Errorf("idle column %d, want ~9999000 (IDLE was not applied)", ts)
		}
	})

	t.Run("TIME sets the delivery timestamp", func(t *testing.T) {
		_, run, _ := newEnv(t)
		seed(t, run)
		past := time.Now().Add(-9999999 * time.Millisecond).UnixMilli()
		got := run("XCLAIM", "s", "g", "c2", "0", "2-0", "TIME",
			strconv.FormatInt(past, 10), "JUSTID")
		if got.Type != resp.TypeArray || len(got.Array) != 1 || string(got.Array[0].Bulk) != "2-0" {
			t.Fatalf("TIME claim replied %v, want the bare id 2-0", got)
		}
		got = run("XPENDING", "s", "g", "2-0", "2-0", "10", "c2")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("XPENDING replied %v, want the pending entry", got)
		}
		if ts := got.Array[0].Array[2].Int; ts < 9999999-500 || ts > 9999999+5000 {
			t.Errorf("idle column = %d, want ~9999999 (the TIME value was not applied)", ts)
		}
	})
}
