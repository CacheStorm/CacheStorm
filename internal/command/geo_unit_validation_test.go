package command_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the GEO handlers never validated the unit argument. GEODIST's
// default case treated any unknown unit as meters (dist *= 1000), GEORADIUS,
// GEORADIUSBYMEMBER, GEOSEARCH and GEOSEARCHSTORE silently treated unknown
// units as kilometers — so a typo'd unit returned silent nonsense instead of
// Redis's "ERR unsupported unit provided. please use m, km, ft, mi".
func TestGeoUnitValidation(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantError  bool
		wantPrefix string
		wantCount  int
	}{
		{"GEODIST unknown unit errors", []string{"GEODIST", "geo", "Palermo", "Catania", "kilometers"}, true, "", 0},
		{"GEODIST m control", []string{"GEODIST", "geo", "Palermo", "Catania", "m"}, false, "166", 0},
		{"GEODIST km control", []string{"GEODIST", "geo", "Palermo", "Catania", "km"}, false, "166", 0},
		{"GEODIST mi control", []string{"GEODIST", "geo", "Palermo", "Catania", "mi"}, false, "103", 0},
		{"GEODIST ft control", []string{"GEODIST", "geo", "Palermo", "Catania", "ft"}, false, "5453", 0},
		{"GEODIST no unit control", []string{"GEODIST", "geo", "Palermo", "Catania"}, false, "166", 0},
		{"GEORADIUS unknown unit errors", []string{"GEORADIUS", "geo", "13.361389", "38.115556", "200", "kilometers"}, true, "", 0},
		{"GEORADIUS mi control", []string{"GEORADIUS", "geo", "13.361389", "38.115556", "200", "mi"}, false, "", 2},
		{"GEORADIUSBYMEMBER unknown unit errors", []string{"GEORADIUSBYMEMBER", "geo", "Palermo", "200", "kilometers"}, true, "", 0},
		{"GEOSEARCH BYRADIUS unknown unit errors", []string{"GEOSEARCH", "geo", "FROMMEMBER", "Palermo", "BYRADIUS", "200", "kilometers"}, true, "", 0},
		{"GEOSEARCH BYBOX unknown unit errors", []string{"GEOSEARCH", "geo", "FROMMEMBER", "Palermo", "BYBOX", "200", "200", "kilometers"}, true, "", 0},
		{"GEOSEARCH BYRADIUS km control", []string{"GEOSEARCH", "geo", "FROMMEMBER", "Palermo", "BYRADIUS", "200", "km"}, false, "", 2},
		{"GEOSEARCHSTORE BYRADIUS unknown unit errors", []string{"GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Palermo", "BYRADIUS", "200", "kilometers"}, true, "", 0},
		{"GEOSEARCHSTORE BYRADIUS km control", []string{"GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Palermo", "BYRADIUS", "200", "km"}, false, "", 2},
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
			if got := run("GEOADD", "geo", "13.361389", "38.115556", "Palermo", "15.087269", "37.502669", "Catania"); got.Type != resp.TypeInteger || got.Int != 2 {
				t.Fatalf("seed GEOADD replied %v %v, want 2", got.Type, got.Int)
			}
			got := run(tc.args[0], tc.args[1:]...)
			if tc.wantError {
				if got.Type != resp.TypeError {
					t.Fatalf("%s replied %v %q, want an unsupported-unit error", tc.args, got.Type, got.Str)
				}
				return
			}
			if got.Type == resp.TypeError {
				t.Fatalf("%s replied an unexpected error %q", tc.args, got.Err)
			}
			if tc.wantPrefix != "" {
				if got.Type != resp.TypeBulkString || !strings.HasPrefix(string(got.Bulk), tc.wantPrefix) {
					t.Fatalf("%s replied %v %q, want a bulk starting with %q", tc.args, got.Type, got.Bulk, tc.wantPrefix)
				}
			}
			if tc.wantCount != 0 {
				if got.Type != resp.TypeArray && got.Type != resp.TypeInteger {
					t.Fatalf("%s replied %v, want an array or integer result", tc.args, got.Type)
				}
				count := int64(len(got.Array))
				if got.Type == resp.TypeInteger {
					count = got.Int
				}
				if count != int64(tc.wantCount) {
					t.Fatalf("%s returned %d results, want %d", tc.args, count, tc.wantCount)
				}
			}
		})
	}
}
