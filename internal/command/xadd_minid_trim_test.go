package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: StreamValue.TrimByMinID compared entry IDs as plain strings, so
// XADD ... MINID trimmed by lexicographic order. Variable-width ID parts broke
// both directions: "9-0" survives MINID 10-0 ("9-0" >= "10-0" lexically), and
// MINID 9-0 evicts NEWER entries 10-0/11-0 ("10-0" >= "9-0" is false). MINID
// must compare numerically as (ms, seq).
func TestXAddMinIDTrimsNumerically(t *testing.T) {
	cases := []struct {
		name    string
		seedIDs []string
		minID   string
		wantIDs []string
	}{
		{"older narrower id evicted", []string{"9-0", "10-0"}, "10-0", []string{"10-0"}},
		{"newer ids survive smaller minid", []string{"10-0", "11-0"}, "9-0", []string{"10-0", "11-0"}},
		{"sequence width varies under same ms", []string{"1000-9", "1000-10"}, "1000-10", []string{"1000-10"}},
		{"equal-width boundary still works", []string{"1-0", "2-0", "3-0"}, "2-0", []string{"2-0", "3-0"}},
		{"minid below everything keeps all", []string{"1-0", "2-0"}, "0-0", []string{"1-0", "2-0"}},
		{"minid equal to entry keeps it", []string{"5-0"}, "5-0", []string{"5-0"}},
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

			for _, id := range tc.seedIDs {
				got := run("XADD", "s", id, "f", "v")
				if got.Type != resp.TypeBulkString || string(got.Bulk) != id {
					t.Fatalf("seed XADD %s replied %v %q, want id %q", id, got.Type, got.Bulk, id)
				}
			}
			added := run("XADD", "s", "MINID", tc.minID, "*", "g", "h")
			if added.Type != resp.TypeBulkString || len(added.Bulk) == 0 {
				t.Fatalf("XADD MINID %s replied %v %q, want a new entry id", tc.minID, added.Type, added.Bulk)
			}

			rng := run("XRANGE", "s", "-", "+")
			if rng.Type != resp.TypeArray || len(rng.Array) != len(tc.wantIDs)+1 {
				t.Fatalf("XRANGE after MINID %s returned %d entries, want %d (trimmed result plus the new entry)",
					tc.minID, len(rng.Array), len(tc.wantIDs)+1)
			}
			for i, want := range tc.wantIDs {
				if got := rng.Array[i].Array[0].Bulk; string(got) != want {
					t.Errorf("entry %d id = %q, want %q: MINID %s trimmed the wrong entries", i, got, want, tc.minID)
				}
			}
			if last := rng.Array[len(rng.Array)-1].Array[0].Bulk; string(last) != string(added.Bulk) {
				t.Errorf("new entry id = %q, want the XADD reply %q", last, added.Bulk)
			}
		})
	}
}
