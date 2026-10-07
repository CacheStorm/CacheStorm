package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func transactionIntegerSessionX(t *testing.T) (*store.Store, func(string, ...string) *resp.Value) {
	t.Helper()
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterTransactionCommands(router)
	ctx := NewContext("", nil, s, nil)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		var buf bytes.Buffer
		ctx.Command, ctx.Args, ctx.Writer = name, make([][]byte, len(args)), resp.NewWriter(&buf)
		for i, arg := range args {
			ctx.Args[i] = []byte(arg)
		}
		if err := router.Execute(ctx); err != nil {
			t.Fatal(err)
		}
		v, err := resp.NewReader(&buf).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	return s, run
}

func TestTransactionIntegerParsingRejectsInvalidInput(t *testing.T) {
	_, control := transactionIntegerSessionX(t)
	control("SET", "key", "7")
	control("MULTI")
	control("INCRBY", "key", "5")
	if got := control("EXEC"); got.Type != resp.TypeArray || len(got.Array) != 1 || got.Array[0].Int != 12 {
		t.Fatal("unaffected control failed")
	}
	t.Log("CONTROL: queued INCRBY 5 produces 12")
	for _, tc := range []struct {
		amount string
		want   int64
	}{
		{"9223372036854775807", 9223372036854775807},
		{"-9223372036854775808", -9223372036854775808},
		{"0", 0},
		{"+1", 1},
	} {
		_, run := transactionIntegerSessionX(t)
		run("SET", "key", "0")
		run("MULTI")
		run("INCRBY", "key", tc.amount)
		got := run("EXEC")
		if len(got.Array) != 1 || got.Array[0].Type != resp.TypeInteger || got.Array[0].Int != tc.want {
			t.Fatalf("boundary %s: %+v", tc.amount, got)
		}
	}
	for _, bad := range []string{"", "-", "9223372036854775808", "18446744073709551616", "-9223372036854775809", "abc"} {
		t.Run(bad, func(t *testing.T) {
			s, run := transactionIntegerSessionX(t)
			run("SET", "key", "7")
			if got := run("INCRBY", "key", bad); got.Type != resp.TypeError {
				t.Fatal("direct rejection control failed")
			}
			version := s.GetVersion("key")
			run("MULTI")
			if got := run("INCRBY", "key", bad); got.Str != "QUEUED" {
				t.Fatal("queue setup failed")
			}
			got := run("EXEC")
			if got.Type != resp.TypeArray || len(got.Array) != 1 {
				t.Fatalf("EXEC: %v", got)
			}
			value := run("GET", "key")
			t.Logf("EXPECTED: error, value=7, version=%d; ACTUAL: reply=%+v value=%s version=%d", version, got.Array[0], value.Bulk, s.GetVersion("key"))
			if got.Array[0].Type != resp.TypeError || string(value.Bulk) != "7" || s.GetVersion("key") != version {
				t.Fatal("PROBLEM CONFIRMED")
			}
			t.Log("FIX VERIFIED")
		})
	}
}
