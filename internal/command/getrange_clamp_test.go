package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdGETRANGE adjusted a negative end offset by the length but
// never clamped the result to zero. GETRANGE key 0 -100 on a 5-byte value
// computed end = -95, hit the start > end check, and returned an empty
// string — Redis clamps the adjusted offsets to the valid range
// (redis/redis#13207 documents GETRANGE mykey -200 -100 returning a
// non-empty string: both offsets clamp to 0, yielding the first byte).
// SUBSTR shares the handler.
func TestGetRangeNegativeEndClamp(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
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
	if got := run("SET", "k", "hello"); got.Type != resp.TypeSimpleString {
		t.Fatalf("SET replied %v, want OK", got.Type)
	}

	cases := []struct {
		name string
		cmd  string
		args []string
		want string
	}{
		{"wildly negative end clamps to 0", "GETRANGE", []string{"k", "0", "-100"}, "h"},
		{"both negative clamp to 0", "GETRANGE", []string{"k", "-200", "-100"}, "h"},
		{"SUBSTR alias shares the fix", "SUBSTR", []string{"k", "0", "-100"}, "h"},
		{"start past clamped end stays empty", "GETRANGE", []string{"k", "2", "-100"}, ""},
		{"whole string control", "GETRANGE", []string{"k", "0", "-1"}, "hello"},
		{"negative pair control", "GETRANGE", []string{"k", "-5", "-2"}, "hell"},
		{"overflowing end control", "GETRANGE", []string{"k", "2", "100"}, "llo"},
		{"out of range control", "GETRANGE", []string{"k", "10", "20"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := run(tc.cmd, tc.args...)
			argv := append([]string{tc.cmd}, tc.args...)
			if got.Type != resp.TypeBulkString {
				t.Fatalf("%v replied %v, want a bulk string", argv, got.Type)
			}
			if string(got.Bulk) != tc.want {
				t.Fatalf("%v = %q, want %q", argv, got.Bulk, tc.want)
			}
		})
	}

	t.Run("missing key control", func(t *testing.T) {
		got := run("GETRANGE", "nosuch", "0", "-100")
		if got.Type != resp.TypeBulkString || len(got.Bulk) != 0 {
			t.Fatalf("GETRANGE on a missing key replied %v %q, want empty", got.Type, got.Bulk)
		}
	})
}
