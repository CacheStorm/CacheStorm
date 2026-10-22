package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSlowlogGetAllX(t *testing.T) {
	// SLOWLOG reads store.GlobalSlowLog — the recorder the request path
	// actually appends to (internal/server/connection.go). It used to read a
	// command-package SlowLog that nothing in production ever wrote to, so
	// this test was seeding an orphan and passing for the wrong reason.
	store.GlobalSlowLog.Clear()
	store.GlobalSlowLog.SetEnabled(true)
	store.GlobalSlowLog.SetThreshold(0)
	t.Cleanup(func() { store.GlobalSlowLog.Clear() })

	router := NewRouter()
	RegisterServerCommands(router)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, arg := range args {
			raw[i] = []byte(arg)
		}
		var output bytes.Buffer
		if err := router.Execute(NewContext(name, raw, nil, resp.NewWriter(&output))); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&output).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	count := func(name string, args ...string) int {
		t.Helper()
		v := run(name, args...)
		if v.Type != resp.TypeArray {
			t.Fatalf("%s %v: expected array, got %v", name, args, v)
		}
		return len(v.Array)
	}

	for i := 0; i < 12; i++ {
		store.GlobalSlowLog.Add(time.Microsecond, "GET", [][]byte{[]byte("k")}, "127.0.0.1:0")
	}

	if v := run("SLOWLOG", "LEN"); v.Type != resp.TypeInteger || v.Int != 12 {
		t.Fatalf("FAIL LEN control: got %v, want 12", v)
	}
	t.Log("PASS LEN control: 12")

	if got := count("SLOWLOG", "GET", "-1"); got != 12 {
		t.Errorf("FAIL GET -1 returns all: got %d entries, want 12", got)
	} else {
		t.Log("PASS GET -1 returns all: 12")
	}

	if got := count("SLOWLOG", "GET"); got != 10 {
		t.Errorf("FAIL omitted-count default: got %d entries, want 10", got)
	} else {
		t.Log("PASS omitted-count default: 10")
	}

	if got := count("SLOWLOG", "GET", "5"); got != 5 {
		t.Errorf("FAIL GET 5: got %d entries, want 5", got)
	} else {
		t.Log("PASS GET 5: 5")
	}

	if got := count("SLOWLOG", "GET", "100"); got != 12 {
		t.Errorf("FAIL GET above total: got %d entries, want 12", got)
	} else {
		t.Log("PASS GET above total: 12")
	}

	if got := count("SLOWLOG", "GET", "0"); got != 0 {
		t.Errorf("FAIL GET 0: got %d entries, want 0 (at most count entries)", got)
	} else {
		t.Log("PASS GET 0: empty")
	}

	// Newest-first ordering. Do NOT assert a fixed id: store.GlobalSlowLog numbers
	// entries from a monotonic sequence that Clear() does not reset, so the base
	// depends on how many entries earlier tests in this binary recorded. What
	// matters is that SLOWLOG GET 1 returns the most recently recorded entry,
	// so take that entry's id from the recorder instead of hardcoding it.
	newestID := store.GlobalSlowLog.Entries[len(store.GlobalSlowLog.Entries)-1].ID
	newest := run("SLOWLOG", "GET", "1")
	if newest.Type != resp.TypeArray || len(newest.Array) != 1 ||
		newest.Array[0].Array[0].Type != resp.TypeInteger || newest.Array[0].Array[0].Int != newestID {
		t.Errorf("FAIL newest-first control: got %v, want latest id %d first", newest, newestID)
	} else {
		t.Logf("PASS newest-first control: id %d first", newestID)
	}

	if v := run("SLOWLOG", "RESET"); v.Type != resp.TypeSimpleString || v.Str != "OK" {
		t.Fatalf("FAIL RESET control: got %v", v)
	}
	if got := count("SLOWLOG", "GET"); got != 0 {
		t.Errorf("FAIL post-RESET control: got %d entries, want 0", got)
	} else {
		t.Log("PASS post-RESET control: empty")
	}

	_ = store.NewStore
}
