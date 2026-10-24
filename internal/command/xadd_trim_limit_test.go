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
	// Redis trims "~" by whole listpack nodes (100 entries each by
	// default), so an 11-entry stream sits in one node and is never trimmed.
	seed(t, "k1")
	if id := run("XADD", "k1", "MAXLEN", "~", "2", "LIMIT", "3", "*", "f", "v"); id.Type != resp.TypeBulkString || id.IsNull {
		t.Errorf("FAIL maxlen limit add: got %v", id)
	}
	checkLenBounds(t, "maxlen limit", "k1", 11, 11)
	seed(t, "k2")
	if id := run("XADD", "k2", "MAXLEN", "~", "2", "*", "f", "v"); id.Type != resp.TypeBulkString {
		t.Errorf("FAIL approx no-limit add: got %v", id)
	}
	checkLenBounds(t, "approx single node kept", "k2", 11, 11)
	seed(t, "k3")
	if id := run("XADD", "k3", "MAXLEN", "~", "0", "LIMIT", "0", "*", "f", "v"); id.Type != resp.TypeBulkString {
		t.Errorf("FAIL limit-zero add: got %v", id)
	}
	checkLenBounds(t, "whole node removable", "k3", 0, 0)
	seed(t, "k4")
	if v := run("XADD", "k4", "MAXLEN", "=", "2", "LIMIT", "1", "*", "f", "v"); v.Type != resp.TypeError {
		t.Errorf("FAIL exact with limit must be a syntax error, got %v", v)
	}
	checkLenBounds(t, "rejected add leaves stream", "k4", 10, 10)
	seed(t, "k5")
	if id := run("XADD", "k5", "MINID", "~", "8-0", "LIMIT", "2", "*", "f", "v"); id.Type != resp.TypeBulkString || id.IsNull {
		t.Errorf("FAIL minid limit add: got %v", id)
	}
	checkLenBounds(t, "minid limit", "k5", 11, 11)

	for i := int64(1); i <= 250; i++ {
		run("XADD", "big", strconvID(i), "f", "v")
	}
	if v := run("XADD", "big", "MAXLEN", "~", "10", "LIMIT", "100", "*", "f", "v"); v.Type != resp.TypeBulkString {
		t.Fatalf("XADD big: %v", v)
	}
	checkLenBounds(t, "limit stops after one node", "big", 151, 151)
	if v := run("XADD", "big", "MAXLEN", "~", "10", "*", "f", "v"); v.Type != resp.TypeBulkString {
		t.Fatalf("XADD big: %v", v)
	}
	checkLenBounds(t, "default limit drops full nodes", "big", 52, 52)
}
