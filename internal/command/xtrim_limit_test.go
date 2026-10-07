package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXTrimLimitX(t *testing.T) {
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
			id := key + "-" + string(rune('0'+i%10))
			id = strconvID(i)
			if v := run("XADD", key, id, "f", "v"); v.Type != resp.TypeBulkString {
				t.Fatalf("seed %s failed: %v", key, v)
			}
		}
	}
	checkInt := func(t *testing.T, label string, value *resp.Value, want int64) {
		t.Helper()
		if value.Type != resp.TypeInteger || value.Int != want {
			t.Errorf("FAIL %s: expected %d, got %v", label, want, value)
		} else {
			t.Logf("PASS %s: %d", label, value.Int)
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
	evicted := run("XTRIM", "k1", "MAXLEN", "~", "2", "LIMIT", "3")
	if evicted.Type != resp.TypeInteger || evicted.Int > 3 {
		t.Errorf("FAIL maxlen limit: expected eviction of at most 3, got %v", evicted)
	} else {
		t.Logf("PASS maxlen limit: evicted %d", evicted.Int)
	}
	checkLenBounds(t, "maxlen limit", "k1", 7, 10)
	seed(t, "k2")
	checkInt(t, "exact control", run("XTRIM", "k2", "MAXLEN", "=", "2"), 8)
	checkLenBounds(t, "exact control", "k2", 2, 2)
	seed(t, "k3")
	if evicted := run("XTRIM", "k3", "MAXLEN", "~", "2"); evicted.Type != resp.TypeInteger {
		t.Errorf("FAIL approx control: %v", evicted)
	}
	checkLenBounds(t, "approx control", "k3", 2, 10)
	seed(t, "k4")
	evicted = run("XTRIM", "k4", "MINID", "~", "8-0", "LIMIT", "2")
	if evicted.Type != resp.TypeInteger || evicted.Int > 2 {
		t.Errorf("FAIL minid limit: expected eviction of at most 2, got %v", evicted)
	} else {
		t.Logf("PASS minid limit: evicted %d", evicted.Int)
	}
	checkLenBounds(t, "minid limit", "k4", 8, 10)
	seed(t, "k5")
	checkInt(t, "limit-zero disables cap", run("XTRIM", "k5", "MAXLEN", "~", "2", "LIMIT", "0"), 8)
	checkLenBounds(t, "limit-zero disables cap", "k5", 2, 2)
	checkInt(t, "missing stream control", run("XTRIM", "missing", "MAXLEN", "~", "2", "LIMIT", "3"), 0)
}

func strconvID(i int64) string {
	digits := ""
	if i == 0 {
		return "0-0"
	}
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits + "-0"
}
