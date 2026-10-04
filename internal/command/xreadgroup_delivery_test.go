package command_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXREADGROUP never advanced the group's last-delivered ID (and
// its start-ID special case never matched the stored "0"), so XREADGROUP ... >
// delivered nothing on first read and could never make progress; the
// inclusive-start boundary entry was re-delivered; NOACK was parsed and
// ignored (entries were pended anyway); and BLOCK was scaled in seconds
// instead of milliseconds. Redis: > delivers each never-delivered entry
// exactly once and advances the group's last-delivered ID; NOACK skips the
// PEL; BLOCK is milliseconds.
func TestXReadGroupDeliveryBookkeeping(t *testing.T) {
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
			t.Errorf("%s execution: %v", name, err)
			return nil
		}
		v, err := resp.NewReader(&buf).ReadValue()
		if err != nil {
			t.Errorf("%s reply: %v", name, err)
			return nil
		}
		return v
	}
	entryIDs := func(v *resp.Value) []string {
		var ids []string
		if v == nil || v.Type != resp.TypeArray {
			return ids
		}
		for _, tuple := range v.Array {
			if tuple == nil || tuple.Type != resp.TypeArray || len(tuple.Array) < 2 {
				continue
			}
			entries := tuple.Array[1]
			if entries == nil || entries.Type != resp.TypeArray {
				continue
			}
			for i := 0; i+1 < len(entries.Array); i += 2 {
				ids = append(ids, string(entries.Array[i].Bulk))
			}
		}
		return ids
	}
	wantIDs := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("delivered %v, want %v", got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("delivered %v, want %v", got, want)
				return
			}
		}
	}
	pendingCount := func() int64 {
		t.Helper()
		got := run("XPENDING", "s", "g")
		if got == nil || got.Type != resp.TypeArray || len(got.Array) == 0 || got.Array[0].Type != resp.TypeInteger {
			t.Errorf("XPENDING replied %v, want a summary with a count", got)
			return -1
		}
		return got.Array[0].Int
	}

	if got := run("XADD", "s", "1-0", "f", "v"); got == nil || got.Type != resp.TypeBulkString || string(got.Bulk) != "1-0" {
		t.Errorf("seed XADD replied %v, want id 1-0", got)
	}
	if got := run("XADD", "s", "2-0", "f", "v"); got == nil || got.Type != resp.TypeBulkString || string(got.Bulk) != "2-0" {
		t.Errorf("seed XADD replied %v, want id 2-0", got)
	}
	if got := run("XGROUP", "CREATE", "s", "g", "0"); got == nil || got.Type != resp.TypeSimpleString || got.Str != "OK" {
		t.Errorf("XGROUP CREATE replied %v, want OK", got)
	}

	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"); got == nil || got.Type != resp.TypeArray {
		t.Errorf("first > delivered %v, want entries 1-0 and 2-0", got)
	} else {
		wantIDs(entryIDs(got), "1-0", "2-0")
	}

	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"); got == nil || got.Type != resp.TypeNull {
		t.Errorf("second > re-delivered %v; entries already delivered must not be re-sent", entryIDs(got))
	}

	if got := run("XADD", "s", "3-0", "f", "v"); got == nil || got.Type != resp.TypeBulkString || string(got.Bulk) != "3-0" {
		t.Errorf("XADD replied %v, want id 3-0", got)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"); got == nil || got.Type != resp.TypeArray {
		t.Errorf("incremental > delivered %v, want entry 3-0", got)
	} else {
		wantIDs(entryIDs(got), "3-0")
	}

	if pc := pendingCount(); pc != 3 {
		t.Errorf("pending count = %d, want 3 (1-0, 2-0, 3-0 delivered without NOACK)", pc)
	}

	if got := run("XADD", "s", "4-0", "f", "v"); got == nil || got.Type != resp.TypeBulkString || string(got.Bulk) != "4-0" {
		t.Errorf("XADD replied %v, want id 4-0", got)
	}
	if got := run("XREADGROUP", "GROUP", "g", "c", "NOACK", "STREAMS", "s", ">"); got == nil || got.Type != resp.TypeArray {
		t.Errorf("NOACK > delivered %v, want entry 4-0", got)
	} else {
		wantIDs(entryIDs(got), "4-0")
	}
	if pc := pendingCount(); pc != 3 {
		t.Errorf("pending count after NOACK delivery = %d, want 3 (NOACK must not pend 4-0)", pc)
	}

	if got := run("XGROUP", "CREATE", "s", "g2", "$"); got == nil || got.Type != resp.TypeSimpleString || got.Str != "OK" {
		t.Errorf("XGROUP CREATE g2 replied %v, want OK", got)
	}
	start := time.Now()
	done := make(chan *resp.Value, 1)
	go func() {
		var buf bytes.Buffer
		argv := [][]byte{
			[]byte("GROUP"), []byte("g2"), []byte("c2"), []byte("BLOCK"), []byte("100"), []byte("STREAMS"), []byte("s"), []byte(">"),
		}
		ctx := command.NewContext("XREADGROUP", argv, s, resp.NewWriter(&buf))
		if err := router.Execute(ctx); err != nil {
			done <- nil
			return
		}
		v, _ := resp.NewReader(&buf).ReadValue()
		done <- v
	}()
	select {
	case v := <-done:
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("BLOCK 100 returned after %v — BLOCK is scaled in seconds, not milliseconds", elapsed)
		}
		if v == nil || v.Type != resp.TypeNull {
			t.Errorf("BLOCK with no new entries replied %v, want null", v)
		}
	case <-time.After(8 * time.Second):
		t.Errorf("BLOCK 100 did not return within 8s — BLOCK is scaled in seconds, not milliseconds")
	}
}
