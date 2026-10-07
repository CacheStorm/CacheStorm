package command

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func transactionSetSessionX(t *testing.T) (*store.Store, func(string, ...string) *resp.Value) {
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

func TestTransactionSetPreservesOptions(t *testing.T) {
	for _, tc := range []struct {
		name, option string
		exists       bool
	}{
		{"plain_control", "", true},
		{"NX_existing", "NX", true},
		{"NX_missing", "NX", false},
		{"XX_missing", "XX", false},
		{"XX_existing", "XX", true},
		{"GET_existing", "GET", true},
		{"GET_missing", "GET", false},
		{"lowercase_nx", "nx", true},
		{"invalid_option", "INVALID", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, direct := transactionSetSessionX(t)
			_, queued := transactionSetSessionX(t)
			if tc.exists {
				direct("SET", "key", "old")
				queued("SET", "key", "old")
			}
			args := []string{"key", "new"}
			if tc.option != "" {
				args = append(args, tc.option)
			}
			want := direct("SET", args...)
			wantValue := direct("GET", "key")
			queued("MULTI")
			if got := queued("SET", args...); got.Str != "QUEUED" {
				t.Fatal("queue setup failed")
			}
			got := queued("EXEC")
			if got.Type != resp.TypeArray || len(got.Array) != 1 {
				t.Fatal("EXEC setup failed")
			}
			gotValue := queued("GET", "key")
			t.Logf("EXPECTED: reply=%+v value=%+v; ACTUAL: reply=%+v value=%+v", want, wantValue, got.Array[0], gotValue)
			if !reflect.DeepEqual(want, got.Array[0]) || !reflect.DeepEqual(wantValue, gotValue) {
				t.Fatal("PROBLEM CONFIRMED")
			}
			if tc.option == "" {
				t.Log("CONTROL: plain SET behaves identically")
			}
			t.Log("FIX VERIFIED")
		})
	}
}
