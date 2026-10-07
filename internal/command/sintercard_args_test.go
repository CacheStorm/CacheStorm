package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSintercardArgsX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSetCommands(router)
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
	seed := func(key string, members ...string) {
		t.Helper()
		args := append([]string{key}, members...)
		if v := run("SADD", args...); v.Type != resp.TypeInteger {
			t.Fatalf("seed %s failed: %v", key, v)
		}
	}
	seed("a", "1", "2")
	seed("b", "2", "3")

	if v := run("SINTERCARD", "2", "a", "b"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Fatalf("FAIL plain control: got %v, want 1", v)
	}
	t.Log("PASS plain control: 1")
	if v := run("SINTERCARD", "2", "a", "b", "LIMIT", "1"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL LIMIT control: got %v, want 1", v)
	} else {
		t.Log("PASS LIMIT control: 1")
	}
	if v := run("SINTERCARD", "2", "a", "b", "LIMIT", "0"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL LIMIT 0 control: got %v, want 1 (0 means unlimited)", v)
	} else {
		t.Log("PASS LIMIT 0 control: 1")
	}
	if v := run("SINTERCARD", "2", "a", "b", "LIMIT", "-1"); v.Type != resp.TypeError {
		t.Errorf("FAIL negative LIMIT control: expected error, got %v", v)
	} else {
		t.Log("PASS negative LIMIT control: error")
	}
	if v := run("SINTERCARD", "1", "a"); v.Type != resp.TypeInteger || v.Int != 2 {
		t.Errorf("FAIL single-set control: got %v, want 2", v)
	} else {
		t.Log("PASS single-set control: 2")
	}
	if v := run("SINTERCARD", "0"); v.Type != resp.TypeError {
		t.Errorf("FAIL numkeys-0 control: expected error, got %v", v)
	} else {
		t.Log("PASS numkeys-0 control: error")
	}
	if v := run("SINTERCARD", "2", "a", "missing"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL missing-set control: got %v, want 0", v)
	} else {
		t.Log("PASS missing-set control: 0")
	}

	if v := run("SINTERCARD", "2", "a", "b", "LIMIT"); v.Type != resp.TypeError {
		t.Errorf("FAIL dangling LIMIT: expected error, got %v", v)
	} else {
		t.Log("PASS dangling LIMIT: error")
	}
	if v := run("SINTERCARD", "2", "a", "b", "FOO"); v.Type != resp.TypeError {
		t.Errorf("FAIL unknown trailing arg: expected error, got %v", v)
	} else {
		t.Log("PASS unknown trailing arg: error")
	}
}
