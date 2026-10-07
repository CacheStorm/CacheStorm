package command

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExpireOptionsX(t *testing.T) {
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
	ttl := func(key string) int64 {
		t.Helper()
		v := run("TTL", key)
		if v.Type != resp.TypeInteger {
			t.Fatalf("TTL %s: expected integer, got %v", key, v)
		}
		return v.Int
	}
	seed := func(key string) {
		t.Helper()
		if v := run("SET", key, "v"); v.Type != resp.TypeSimpleString || v.Str != "OK" {
			t.Fatalf("seed %s failed: %v", key, v)
		}
	}
	seed("k")
	if v := run("EXPIRE", "k", "100", "NX"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL NX fresh: expected 1, got %v", v)
	} else {
		t.Log("PASS NX fresh: 1")
	}
	if got := ttl("k"); got < 99 || got > 100 {
		t.Errorf("FAIL NX fresh TTL: expected ~100, got %d", got)
	}
	if v := run("EXPIRE", "k", "200", "NX"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL NX skip: expected 0, got %v", v)
	} else {
		t.Log("PASS NX skip: 0")
	}
	if got := ttl("k"); got < 99 || got > 100 {
		t.Errorf("FAIL NX skip TTL unchanged: expected ~100, got %d", got)
	}
	if v := run("EXPIRE", "k", "300", "XX"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL XX volatile: expected 1, got %v", v)
	} else {
		t.Log("PASS XX volatile: 1")
	}
	if got := ttl("k"); got < 299 || got > 300 {
		t.Errorf("FAIL XX TTL: expected ~300, got %d", got)
	}
	seed("k2")
	if v := run("EXPIRE", "k2", "100", "XX"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL XX skip: expected 0, got %v", v)
	} else {
		t.Log("PASS XX skip: 0")
	}
	if got := ttl("k2"); got != -1 {
		t.Errorf("FAIL XX skip TTL: expected -1 (persistent), got %d", got)
	}
	seed("k3")
	if v := run("EXPIRE", "k3", "100"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Fatalf("FAIL no-flag control: %v", v)
	}
	t.Log("PASS no-flag control: 1")
	if v := run("EXPIRE", "k3", "50", "GT"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL GT skip: expected 0, got %v", v)
	} else {
		t.Log("PASS GT skip: 0")
	}
	if got := ttl("k3"); got < 99 || got > 100 {
		t.Errorf("FAIL GT skip TTL unchanged: expected ~100, got %d", got)
	}
	if v := run("EXPIRE", "k3", "200", "GT"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL GT apply: expected 1, got %v", v)
	} else {
		t.Log("PASS GT apply: 1")
	}
	seed("k4")
	run("EXPIRE", "k4", "100")
	if v := run("EXPIRE", "k4", "300", "LT"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL LT skip: expected 0, got %v", v)
	} else {
		t.Log("PASS LT skip: 0")
	}
	if v := run("EXPIRE", "k4", "50", "LT"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL LT apply: expected 1, got %v", v)
	} else {
		t.Log("PASS LT apply: 1")
	}
	seed("k5")
	if v := run("EXPIRE", "k5", "100", "GT"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL GT non-volatile: expected 0 (infinite TTL), got %v", v)
	} else {
		t.Log("PASS GT non-volatile: 0")
	}
	if got := ttl("k5"); got != -1 {
		t.Errorf("FAIL GT non-volatile TTL: expected -1, got %d", got)
	}
	seed("k6")
	if v := run("EXPIRE", "k6", "100", "LT"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL LT non-volatile: expected 1 (100 < infinite), got %v", v)
	} else {
		t.Log("PASS LT non-volatile: 1")
	}
	if v := run("EXPIRE", "missing", "100", "NX"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL missing-key: expected 0, got %v", v)
	} else {
		t.Log("PASS missing-key: 0")
	}
	if v := run("EXPIRE", "k6", "100", "BOGUS"); v.Type != resp.TypeError {
		t.Errorf("FAIL unknown-option control: expected error, got %v", v)
	} else {
		t.Log("PASS unknown-option control: error")
	}
	if v := run("PEXPIRE", "k6", "50000", "NX"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL PEXPIRE NX skip: expected 0, got %v", v)
	} else {
		t.Log("PASS PEXPIRE NX skip: 0")
	}
	future := fmt.Sprint(time.Now().Add(200 * time.Second).Unix())
	if v := run("EXPIREAT", "k6", future, "NX"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Errorf("FAIL EXPIREAT NX skip: expected 0, got %v", v)
	} else {
		t.Log("PASS EXPIREAT NX skip: 0")
	}
	if v := run("EXPIREAT", "k5", future, "NX"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL EXPIREAT NX fresh: expected 1, got %v", v)
	} else {
		t.Log("PASS EXPIREAT NX fresh: 1")
	}
	pfuture := fmt.Sprint(time.Now().Add(200 * time.Second).UnixMilli())
	if v := run("PEXPIREAT", "k5", pfuture, "XX"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Errorf("FAIL PEXPIREAT XX apply: expected 1, got %v", v)
	} else {
		t.Log("PASS PEXPIREAT XX apply: 1")
	}
}
