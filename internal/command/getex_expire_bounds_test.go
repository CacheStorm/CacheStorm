package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGetexExpireBoundsX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
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
	seed := func(key string) {
		t.Helper()
		if v := run("SET", key, "v"); v.Type != resp.TypeSimpleString || v.Str != "OK" {
			t.Fatalf("seed %s failed: %v", key, v)
		}
	}
	checkBounds := func(label string, args ...string) {
		t.Helper()
		seed("k")
		v := run("GETEX", append([]string{"k"}, args...)...)
		if v.Type != resp.TypeError {
			t.Errorf("FAIL %s: expected error reply, got %v", label, v)
		} else {
			t.Logf("PASS %s: error reply", label)
		}
		if exists := run("EXISTS", "k"); exists.Type != resp.TypeInteger || exists.Int != 1 {
			t.Errorf("FAIL %s: key must survive, EXISTS=%v", label, exists)
		}
		if got := run("GET", "k"); got.Type != resp.TypeBulkString || string(got.Bulk) != "v" {
			t.Errorf("FAIL %s: value must survive, got %v", label, got)
		}
	}

	checkBounds("EX max int64", "EX", "9223372036854775807")
	checkBounds("PX max int64", "PX", "9223372036854775807")
	checkBounds("EXAT max int64", "EXAT", "9223372036854775807")
	checkBounds("PXAT max int64", "PXAT", "9223372036854775807")
	checkBounds("EX negative", "EX", "-5")
	checkBounds("PX negative", "PX", "-3")

	seed("k")
	if v := run("GETEX", "k"); v.Type != resp.TypeBulkString || string(v.Bulk) != "v" {
		t.Errorf("FAIL no-option control: got %v", v)
	} else {
		t.Log("PASS no-option control: value returned")
	}
	if v := run("GETEX", "k", "EX", "100"); v.Type != resp.TypeBulkString || string(v.Bulk) != "v" {
		t.Errorf("FAIL EX 100 control: got %v", v)
	} else {
		t.Log("PASS EX 100 control: value returned")
	}
	if got := run("TTL", "k"); got.Type != resp.TypeInteger || got.Int < 99 || got.Int > 100 {
		t.Errorf("FAIL EX 100 TTL: got %v, want ~100", got)
	}
	if v := run("GETEX", "k", "PERSIST"); v.Type != resp.TypeBulkString || string(v.Bulk) != "v" {
		t.Errorf("FAIL PERSIST control: got %v", v)
	} else {
		t.Log("PASS PERSIST control: value returned")
	}
	if got := run("TTL", "k"); got.Type != resp.TypeInteger || got.Int != -1 {
		t.Errorf("FAIL PERSIST TTL: got %v, want -1", got)
	}
	if v := run("GETEX", "missing"); v.Type != resp.TypeBulkString || !v.IsNull {
		t.Errorf("FAIL missing-key control: got %v", v)
	} else {
		t.Log("PASS missing-key control: null")
	}
}
