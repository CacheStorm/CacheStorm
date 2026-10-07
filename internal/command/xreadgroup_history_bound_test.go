package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXReadGroupHistoryBoundX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, arg := range args {
			raw[i] = []byte(arg)
		}
		var output bytes.Buffer
		if err := router.Execute(NewContext(name, raw, s, resp.NewWriter(&output))); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&output).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, id := range []string{"5-0", "6-0", "7-0"} {
		if v := run("XADD", "s", id, "f", "v"+id[:1]); v.Type != resp.TypeBulkString {
			t.Fatalf("seed %s failed: %v", id, v)
		}
	}
	if v := run("XGROUP", "CREATE", "s", "grp", "0"); v.Type != resp.TypeSimpleString || v.Str != "OK" {
		t.Fatalf("group create failed: %v", v)
	}
	delivered := run("XREADGROUP", "GROUP", "grp", "consumer", "STREAMS", "s", ">")
	if delivered.Type != resp.TypeArray || len(delivered.Array) != 1 ||
		len(delivered.Array[0].Array[1].Array) != 6 {
		t.Fatalf("FAIL delivery control: expected all three entries, got %v", delivered)
	}
	t.Log("PASS delivery control: all three entries delivered")
	history := run("XREADGROUP", "GROUP", "grp", "consumer", "STREAMS", "s", "5-0")
	if history.Type != resp.TypeArray || len(history.Array) != 1 {
		t.Fatalf("FAIL history 5-0: expected one stream, got %v", history)
	}
	entries := history.Array[0].Array[1].Array
	if len(entries) != 4 {
		t.Errorf("FAIL history 5-0: expected exactly entries above 5-0 (4 flat elements), got %v", history)
	} else if string(entries[0].Bulk) != "6-0" {
		t.Errorf("FAIL history 5-0: expected first entry 6-0, got %v", entries[0])
	} else {
		t.Log("PASS history 5-0: exclusive bound")
	}
	history = run("XREADGROUP", "GROUP", "grp", "consumer", "STREAMS", "s", "6-0")
	if history.Type != resp.TypeArray || len(history.Array) != 1 {
		t.Fatalf("FAIL history 6-0: expected one stream, got %v", history)
	}
	entries = history.Array[0].Array[1].Array
	if len(entries) != 2 || string(entries[0].Bulk) != "7-0" {
		t.Errorf("FAIL history 6-0: expected exactly 7-0 (2 flat elements), got %v", history)
	} else {
		t.Log("PASS history 6-0: exclusive bound")
	}
	history = run("XREADGROUP", "GROUP", "grp", "consumer", "STREAMS", "s", "0")
	if history.Type != resp.TypeArray || len(history.Array) != 1 ||
		len(history.Array[0].Array[1].Array) != 6 {
		t.Fatalf("FAIL history 0 control: expected all three pending entries, got %v", history)
	}
	t.Log("PASS history 0 control: all three pending entries")
}
