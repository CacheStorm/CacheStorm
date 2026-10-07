package command

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestBitmapRedisBitOrderX(t *testing.T) {
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

	set("full", []byte{255})
	check("control all-set bit", int64(1), run("GETBIT", "full", "0").Int)
	check("control all-set position", int64(0), run("BITPOS", "full", "1").Int)
	check("initial old bit", int64(0), run("SETBIT", "fresh", "0", "1").Int)
	check("offset zero stored byte", []byte{128}, data("fresh"))
	set("high", []byte{128})
	check("read most significant bit", int64(1), run("GETBIT", "high", "0").Int)
	check("first set bit position", int64(0), run("BITPOS", "high", "1").Int)

	check("first clear bit", int64(1), run("BITPOS", "high", "0").Int)
	for _, c := range []struct {
		offset string
		want   []byte
	}{
		{"7", []byte{1}}, {"8", []byte{0, 128}}, {"15", []byte{0, 1}},
	} {
		key := "edge" + c.offset
		check("edge old bit "+c.offset, int64(0), run("SETBIT", key, c.offset, "1").Int)
		check("edge bytes "+c.offset, c.want, data(key))
		check("edge read "+c.offset, int64(1), run("GETBIT", key, c.offset).Int)
		check("edge clear old bit "+c.offset, int64(1), run("SETBIT", key, c.offset, "0").Int)
		check("edge cleared "+c.offset, int64(0), run("GETBIT", key, c.offset).Int)
	}
	set("second", []byte{0, 128})
	check("position in second byte", int64(8), run("BITPOS", "second", "1", "1", "1").Int)
	check("BITFIELD agrees", int64(1), run("BITFIELD", "fresh", "GET", "u1", "0").Array[0].Int)
	check("missing key control", int64(0), run("GETBIT", "missing", "0").Int)

}
