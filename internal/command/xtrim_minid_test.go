package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXTRIM rejected every non-MAXLEN strategy outright, so the
// documented Redis form XTRIM key MINID [=|~] <id> failed with a syntax error
// even though the store's numeric TrimByMinID exists (and XADD already wires
// the same option). XTRIM must support MINID (with optional ~ and =), return
// the number of removed entries, and keep MAXLEN (including the negative
// count error) working.
func TestXTrimMinID(t *testing.T) {
	seed := func(t *testing.T, run func(name string, args ...string) *resp.Value) {
		t.Helper()
		for _, id := range []string{"1-0", "2-0", "3-0"} {
			if got := run("XADD", "s", id, "f", "v"); got.Type != resp.TypeBulkString || string(got.Bulk) != id {
				t.Fatalf("seed XADD replied %v %q, want %s", got.Type, got.Bulk, id)
			}
		}
	}
	xlen := func(t *testing.T, run func(name string, args ...string) *resp.Value) int64 {
		t.Helper()
		got := run("XLEN", "s")
		if got.Type != resp.TypeInteger {
			t.Fatalf("XLEN replied %v, want an integer", got.Type)
		}
		return got.Int
	}
	remaining := func(t *testing.T, run func(name string, args ...string) *resp.Value, want ...string) {
		t.Helper()
		got := run("XRANGE", "s", "-", "+")
		if got.Type != resp.TypeArray || len(got.Array) != len(want) {
			t.Fatalf("XRANGE returned %d entries, want %d", len(got.Array), len(want))
		}
		for i, id := range want {
			if string(got.Array[i].Array[0].Bulk) != id {
				t.Errorf("entry %d = %q, want %q", i, got.Array[i].Array[0].Bulk, id)
			}
		}
	}

	cases := []struct {
		name        string
		trimArgs    []string
		wantRemoved int64
		wantLeft    []string
		wantError   bool
	}{
		{"MINID trims older entries", []string{"XTRIM", "s", "MINID", "2-0"}, 1, []string{"2-0", "3-0"}, false},
		{"MINID with = form", []string{"XTRIM", "s", "MINID", "=", "2-0"}, 1, []string{"2-0", "3-0"}, false},
		{"MINID with ~ form trims exactly", []string{"XTRIM", "s", "MINID", "~", "2-0"}, 1, []string{"2-0", "3-0"}, false},
		{"MINID with partial id", []string{"XTRIM", "s", "MINID", "2"}, 1, []string{"2-0", "3-0"}, false},
		{"MINID newer than everything", []string{"XTRIM", "s", "MINID", "9-0"}, 3, nil, false},
		{"MAXLEN control", []string{"XTRIM", "s", "MAXLEN", "2"}, 1, []string{"2-0", "3-0"}, false},
		{"MAXLEN with = form", []string{"XTRIM", "s", "MAXLEN", "=", "2"}, 1, []string{"2-0", "3-0"}, false},
		{"MAXLEN negative stays an error", []string{"XTRIM", "s", "MAXLEN", "-1"}, 0, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			router := command.NewRouter()
			command.RegisterStreamCommands(router)
			run := func(name string, args ...string) *resp.Value {
				t.Helper()
				argv := make([][]byte, len(args))
				for i, arg := range args {
					argv[i] = []byte(arg)
				}
				var buf bytes.Buffer
				ctx := command.NewContext(name, argv, s, resp.NewWriter(&buf))
				if err := router.Execute(ctx); err != nil {
					t.Fatalf("%s execution: %v", name, err)
				}
				v, err := resp.NewReader(&buf).ReadValue()
				if err != nil {
					t.Fatalf("%s reply: %v", name, err)
				}
				return v
			}
			seed(t, run)

			got := run(tc.trimArgs[0], tc.trimArgs[1:]...)
			if tc.wantError {
				if got.Type != resp.TypeError {
					t.Fatalf("%v replied %v, want an error", tc.trimArgs, got.Type)
				}
				if xlen(t, run) != 3 {
					t.Errorf("a failed XTRIM must not trim; XLEN changed")
				}
				return
			}
			if got.Type != resp.TypeInteger || got.Int != tc.wantRemoved {
				t.Fatalf("%v replied %v %v, want removed=%d", tc.trimArgs, got.Type, got.Int, tc.wantRemoved)
			}
			remaining(t, run, tc.wantLeft...)
		})
	}

	t.Run("missing key trims nothing", func(t *testing.T) {
		s := store.NewStore()
		router := command.NewRouter()
		command.RegisterStreamCommands(router)
		var buf bytes.Buffer
		ctx := command.NewContext("XTRIM", [][]byte{[]byte("nosuch"), []byte("MINID"), []byte("1-0")}, s, resp.NewWriter(&buf))
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("execution: %v", err)
		}
		got, err := resp.NewReader(&buf).ReadValue()
		if err != nil {
			t.Fatalf("reply: %v", err)
		}
		if got.Type != resp.TypeInteger || got.Int != 0 {
			t.Fatalf("XTRIM on a missing key replied %v %v, want 0", got.Type, got.Int)
		}
	})
}
