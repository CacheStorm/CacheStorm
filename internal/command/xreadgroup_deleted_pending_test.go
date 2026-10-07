package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXReadGroupDeletedPendingX(t *testing.T) {
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
	if v := run("XADD", "s", "1-0", "f", "v"); v.Type != resp.TypeBulkString {
		t.Fatalf("seed 1-0 failed: %v", v)
	}
	if v := run("XADD", "s", "2-0", "g", "w"); v.Type != resp.TypeBulkString {
		t.Fatalf("seed 2-0 failed: %v", v)
	}
	if v := run("XGROUP", "CREATE", "s", "grp", "0"); v.Type != resp.TypeSimpleString || v.Str != "OK" {
		t.Fatalf("group create failed: %v", v)
	}
	delivered := run("XREADGROUP", "GROUP", "grp", "consumer", "STREAMS", "s", ">")
	if delivered.Type != resp.TypeArray || len(delivered.Array) != 1 ||
		len(delivered.Array[0].Array) != 2 ||
		len(delivered.Array[0].Array[1].Array) != 4 {
		t.Fatalf("FAIL delivery control: expected [key, [id, fields, id, fields]], got %v", delivered)
	}
	t.Log("PASS delivery control: both entries delivered")
	if deleted := run("XDEL", "s", "1-0"); deleted.Type != resp.TypeInteger || deleted.Int != 1 {
		t.Fatalf("FAIL xdel control: %v", deleted)
	}
	t.Log("PASS xdel control: 1 entry deleted")
	history := run("XREADGROUP", "GROUP", "grp", "consumer", "STREAMS", "s", "0")
	if history.Type != resp.TypeArray || len(history.Array) != 1 {
		t.Fatalf("FAIL history: expected one stream, got %v", history)
	}
	entries := history.Array[0].Array[1].Array
	if len(entries) != 4 {
		t.Fatalf("FAIL history: expected flat [id, fields, id, fields], got %v", history)
	}
	if entries[0].Type != resp.TypeBulkString || string(entries[0].Bulk) != "1-0" {
		t.Fatalf("FAIL history: expected deleted pending id 1-0, got %v", entries[0])
	}
	if entries[1].Type != resp.TypeArray || !entries[1].IsNull {
		t.Errorf("FAIL deleted-pending payload: expected null, got %v", entries[1])
	} else {
		t.Log("PASS deleted-pending payload: null")
	}
	if entries[2].Type != resp.TypeBulkString || string(entries[2].Bulk) != "2-0" {
		t.Fatalf("FAIL history: expected alive pending id 2-0, got %v", entries[2])
	}
	payload := entries[3]
	if payload.Type != resp.TypeArray || payload.IsNull || len(payload.Array) != 2 ||
		string(payload.Array[0].Bulk) != "g" || string(payload.Array[1].Bulk) != "w" {
		t.Errorf("FAIL alive-pending control: expected [g w], got %v", payload)
	} else {
		t.Log("PASS alive-pending control: [g w]")
	}
}
