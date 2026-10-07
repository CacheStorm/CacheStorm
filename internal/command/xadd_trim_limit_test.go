package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXAddTrimLimitX(t *testing.T) {
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
	seed := func(t *testing.T, key string) {
		t.Helper()
		for i := int64(1); i <= 10; i++ {
			if v := run("XADD", key, strconvID(i), "f", "v"); v.Type != resp.TypeBulkString {
				t.Fatalf("seed %s failed: %v", key, v)
			}
		}
	}
	checkLenBounds := func(t *testing.T, label, key string, min, max int64) {
		t.Helper()
		value := run("XLEN", key)
		if value.Type != resp.TypeInteger || value.Int < min || value.Int > max {
			t.Errorf("FAIL %s: expected XLEN in [%d,%d], got %v", label, min, max, value)
		} else {
			t.Logf("PASS %s: XLEN=%d in [%d,%d]", label, value.Int, min, max)
		}
	}
	seed(t, "k1")
	if id := run("XADD", "k1", "MAXLEN", "~", "2", "LIMIT", "3", "*", "f", "v"); id.Type != resp.TypeBulkString || id.IsNull {
		t.Errorf("FAIL maxlen limit add: got %v", id)
	}
	checkLenBounds(t, "maxlen limit", "k1", 8, 11)
	seed(t, "k2")
	if id := run("XADD", "k2", "MAXLEN", "~", "2", "*", "f", "v"); id.Type != resp.TypeBulkString {
		t.Errorf("FAIL approx no-limit add: got %v", id)
	}
	checkLenBounds(t, "approx no-limit control", "k2", 2, 2)
	seed(t, "k3")
	if id := run("XADD", "k3", "MAXLEN", "~", "2", "LIMIT", "0", "*", "f", "v"); id.Type != resp.TypeBulkString {
		t.Errorf("FAIL limit-zero add: got %v", id)
	}
	checkLenBounds(t, "limit-zero uncapped control", "k3", 2, 2)
	seed(t, "k4")
	if id := run("XADD", "k4", "MAXLEN", "=", "2", "LIMIT", "1", "*", "f", "v"); id.Type != resp.TypeBulkString {
		t.Errorf("FAIL exact with limit add: got %v", id)
	}
	checkLenBounds(t, "exact ignores limit control", "k4", 2, 2)
	seed(t, "k5")
	if id := run("XADD", "k5", "MINID", "~", "8-0", "LIMIT", "2", "*", "f", "v"); id.Type != resp.TypeBulkString || id.IsNull {
		t.Errorf("FAIL minid limit add: got %v", id)
	}
	checkLenBounds(t, "minid limit", "k5", 9, 11)
}
