package command

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestBitmapANDZeroPaddingX(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, a := range args {
			raw[i] = []byte(a)
		}
		var b bytes.Buffer
		ctx := NewContext(name, raw, s, resp.NewWriter(&b))
		if err := r.Execute(ctx); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&b).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	check := func(label string, want, got interface{}) {
		t.Helper()
		t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s mismatch", label)
		}
	}
	set := func(key string, data []byte) {
		t.Helper()
		if err := s.Set(key, &store.StringValue{Data: data}, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	data := func(key string) []byte {
		t.Helper()
		entry, ok := s.Get(key)
		if !ok {
			t.Fatalf("missing key %s", key)
		}
		return []byte(entry.Value.String())
	}
	_ = set
	_ = data

	set("equalA", []byte{255, 15})
	set("equalB", []byte{15, 255})
	check("control equal-length reply", int64(2), run("BITOP", "AND", "control", "equalA", "equalB").Int)
	check("control equal-length bytes", []byte{15, 15}, data("control"))
	set("long", []byte{255, 170})
	set("short", []byte{15})
	check("long-first length", int64(2), run("BITOP", "AND", "forward", "long", "short").Int)
	check("long-first zero padding", []byte{15, 0}, data("forward"))
	check("short-first length", int64(2), run("BITOP", "AND", "reverse", "short", "long").Int)
	check("short-first zero padding", []byte{15, 0}, data("reverse"))

	set("empty", []byte{})
	run("BITOP", "AND", "empty-result", "long", "empty")
	check("empty operand zeros", []byte{0, 0}, data("empty-result"))
	set("longer", []byte{255, 255, 255})
	run("BITOP", "AND", "multi", "long", "short", "longer")
	check("later growth retains zeros", []byte{15, 0, 0}, data("multi"))
	run("BITOP", "OR", "or-result", "long", "short")
	check("OR preserves tail", []byte{255, 170}, data("or-result"))
	run("BITOP", "XOR", "xor-result", "long", "short")
	check("XOR preserves tail", []byte{240, 170}, data("xor-result"))
	run("BITOP", "AND", "long", "long", "short")
	check("destination aliases input", []byte{15, 0}, data("long"))

}
