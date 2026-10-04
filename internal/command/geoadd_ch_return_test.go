package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdGEOADD parsed the CH option and discarded it, while counting
// every geo.Add — including updates of existing members — toward the reply.
// The command therefore behaved as if CH were always on. Redis GEOADD returns
// the number of elements ADDED (updates count 0) unless CH is given, in which
// case it returns the number CHANGED (added or updated; a rewrite with the
// same coordinates is not a change).
func TestGeoAddReturnCounts(t *testing.T) {
	cases := []struct {
		name            string
		seed            bool
		args            []string
		want            int64
		assertKeyAbsent bool
	}{
		{"new member plain", false, []string{"geo", "13.361389", "38.115556", "palermo"}, 1, false},
		{"update plain counts nothing", true, []string{"geo", "13.5", "38.2", "palermo"}, 0, false},
		{"update CH counts change", true, []string{"geo", "CH", "13.5", "38.2", "palermo"}, 1, false},
		{"same coords CH counts nothing", true, []string{"geo", "CH", "13.361389", "38.115556", "palermo"}, 0, false},
		{"new member CH", false, []string{"geo", "CH", "15.087269", "37.502669", "catania"}, 1, false},
		{"nx existing plain", true, []string{"geo", "NX", "13.4", "38.1", "palermo"}, 0, false},
		{"nx missing plain", false, []string{"geo", "NX", "15.087269", "37.502669", "catania"}, 1, false},
		{"xx existing plain counts nothing", true, []string{"geo", "XX", "13.4", "38.1", "palermo"}, 0, false},
		{"xx existing CH counts change", true, []string{"geo", "XX", "CH", "13.4", "38.1", "palermo"}, 1, false},
		{"xx missing plain leaves key absent", false, []string{"geo", "XX", "13.4", "38.1", "palermo"}, 0, true},
		{"mixed batch plain counts additions only", true, []string{"geo", "15.087269", "37.502669", "catania", "13.4", "38.1", "palermo"}, 1, false},
		{"mixed batch CH counts both", true, []string{"geo", "CH", "15.087269", "37.502669", "catania", "13.4", "38.1", "palermo"}, 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			router := command.NewRouter()
			command.RegisterGeoCommands(router)
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
			if tc.seed {
				if got := run("GEOADD", "geo", "13.361389", "38.115556", "palermo"); got.Type != resp.TypeInteger || got.Int != 1 {
					t.Fatalf("seed GEOADD replied %v %v, want 1", got.Type, got.Int)
				}
			}
			got := run("GEOADD", tc.args...)
			if got.Type != resp.TypeInteger {
				t.Fatalf("GEOADD %v replied %v %q, want an integer", tc.args, got.Type, got.Str)
			}
			if got.Int != tc.want {
				t.Errorf("GEOADD %v returned %d, want %d", tc.args, got.Int, tc.want)
			}
			if tc.assertKeyAbsent {
				if _, exists := s.Get("geo"); exists {
					t.Errorf("GEOADD XX on a missing key created the key; Redis creates nothing")
				}
			}
		})
	}
}
