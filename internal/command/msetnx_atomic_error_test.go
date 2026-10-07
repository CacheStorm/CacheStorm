package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMSetNXAtomicErrorX(t *testing.T) {
	s := store.NewStore()
	s.ConfigureMemory(8000, store.EvictionNoEviction, 100, 100, 5)
	router := NewRouter()
	RegisterStringCommands(router)
	big := strings.Repeat("x", 5000)
	oom := "OOM command not allowed when used memory > 'maxmemory'"
	run := func(t *testing.T, name string, args ...string) *resp.Value {
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
	checkInt := func(t *testing.T, label string, value *resp.Value, want int64) {
		t.Helper()
		if value.Type != resp.TypeInteger || value.Int != want {
			t.Errorf("FAIL %s: expected %d, got %v", label, want, value)
		} else {
			t.Logf("PASS %s: %d", label, value.Int)
		}
	}
	checkErr := func(t *testing.T, label string, value *resp.Value, want string) {
		t.Helper()
		if value.Type != resp.TypeError || value.Err != want {
			t.Errorf("FAIL %s: expected error %q, got %v", label, want, value)
		} else {
			t.Logf("PASS %s: %s", label, value.Err)
		}
	}
	checkGet := func(t *testing.T, label, key, want string) {
		t.Helper()
		value := run(t, "GET", key)
		if want == "" {
			if value.Type != resp.TypeBulkString || !value.IsNull {
				t.Errorf("FAIL %s: expected %s to be unset, got %v", label, key, value)
			} else {
				t.Logf("PASS %s: %s unset", label, key)
			}
			return
		}
		if value.Type != resp.TypeBulkString || string(value.Bulk) != want {
			t.Errorf("FAIL %s: expected %s=%q, got %v", label, key, want, value)
		} else {
			t.Logf("PASS %s: %s=%q", label, key, want)
		}
	}
	if ok := run(t, "SET", "exists", "x"); ok.Type != resp.TypeSimpleString || ok.Str != "OK" {
		t.Fatalf("seed failed: %v", ok)
	}
	checkInt(t, "fresh control", run(t, "MSETNX", "h1", "v", "h2", "v"), 1)
	checkGet(t, "fresh control", "h1", "v")
	checkInt(t, "existing-key control", run(t, "MSETNX", "n1", "v", "exists", "v"), 0)
	checkGet(t, "existing-key control", "n1", "")
	reply := run(t, "MSETNX", "a1", big, "b2", big)
	checkErr(t, "overflow atomic reject", reply, oom)
	checkGet(t, "overflow leaves nothing", "a1", "")
	checkGet(t, "overflow leaves nothing", "b2", "")
	reply = run(t, "MSETNX", "b3", big, "c4", big)
	checkErr(t, "exhausted reject", reply, oom)
	checkGet(t, "exhausted leaves nothing", "b3", "")
	checkGet(t, "exhausted leaves nothing", "c4", "")
	if _, exists := s.Get("a1"); exists {
		t.Errorf("FAIL atomicity: a1 exists after failed MSETNX: %s", func() string { v, _ := s.Get("a1"); return fmt.Sprintf("%v", v.Value) }())
	}
	checkErr(t, "mset control", run(t, "MSET", "m1", big, "m2", big), oom)
}
