package command

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func checkAudit50R32X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R32X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	run := func(name string, args ...string) string {
		t.Helper()
		parsed := make([][]byte, len(args))
		for i, arg := range args {
			parsed[i] = []byte(arg)
		}
		var buf bytes.Buffer
		ctx := NewContext(name, parsed, s, resp.NewWriter(&buf))
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("command %s: %v", name, err)
		}
		return buf.String()
	}

	run("SET", "control", "old")
	checkAudit50R32X(t, "control previous value", "$3\r\nold\r\n", run("GETSET", "control", "new"))
	checkAudit50R32X(t, "control replacement", "$3\r\nnew\r\n", run("GET", "control"))
	original := &store.HashValue{Fields: map[string][]byte{"f": []byte("keep")}}
	s.Set("hash", original, store.SetOptions{})
	reply := run("GETSET", "hash", "replacement")
	checkAudit50R32X(t, "wrong type response", true, strings.HasPrefix(reply, "-WRONGTYPE"))
	got, _ := s.Get("hash")
	checkAudit50R32X(t, "wrong type must preserve stored value", store.DataTypeHash, got.Value.Type())

	checkAudit50R32X(t, "original hash fields retained", "keep", string(original.Fields["f"]))
	checkAudit50R32X(t, "new key returns null", "$-1\r\n", run("GETSET", "missing", "created"))
	checkAudit50R32X(t, "new key stored", "$7\r\ncreated\r\n", run("GET", "missing"))
	checkAudit50R32X(t, "empty replacement old value", "$3\r\nnew\r\n", run("GETSET", "control", ""))
	checkAudit50R32X(t, "empty replacement stored", "$0\r\n\r\n", run("GET", "control"))
	checkAudit50R32X(t, "invalid key error propagated", true, strings.HasPrefix(run("GETSET", "", "value"), "-ERR invalid key"))

}
