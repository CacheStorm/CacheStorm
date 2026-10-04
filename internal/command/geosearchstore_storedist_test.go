package command_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdGEOSEARCHSTORE parsed the STOREDIST option and discarded it,
// so the destination always held geo points. Redis GEOSEARCHSTORE STOREDIST
// must store per-member distances (in the requested unit) as ZSET scores.
func TestGeoSearchStoreStoreDist(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCount  int64
		storeDist  bool
		wantScores map[string]float64
	}{
		{
			name:      "without STOREDIST dest stays geo",
			args:      []string{"GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km"},
			wantCount: 2,
		},
		{
			name:       "STOREDIST stores km distances",
			args:       []string{"GEOSEARCHSTORE", "dstore", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "STOREDIST"},
			wantCount:  2,
			storeDist:  true,
			wantScores: map[string]float64{"Catania": 0, "Palermo": 166.2295},
		},
		{
			name:       "STOREDIST converts to the requested unit",
			args:       []string{"GEOSEARCHSTORE", "dmi", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "mi", "STOREDIST"},
			wantCount:  2,
			storeDist:  true,
			wantScores: map[string]float64{"Catania": 0, "Palermo": 103.2935},
		},
		{
			name:      "STOREDIST empty result deletes dest",
			args:      []string{"GEOSEARCHSTORE", "dempty", "geo", "FROMLONLAT", "0", "0", "BYRADIUS", "1", "km", "STOREDIST"},
			wantCount: 0,
			storeDist: true,
		},
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

			dest := tc.args[1]
			got := run(tc.args[0], tc.args[1:]...)
			if got.Type != resp.TypeInteger || got.Int != tc.wantCount {
				t.Fatalf("GEOSEARCHSTORE replied %v %v, want integer %d", got.Type, got.Int, tc.wantCount)
			}

			if tc.wantCount == 0 {
				if _, exists := s.Get(dest); exists {
					t.Fatalf("empty result must delete %s", dest)
				}
				return
			}

			entry, ok := s.Get(dest)
			if !ok {
				t.Fatalf("destination %s missing", dest)
			}
			if !tc.storeDist {
				geo, isGeo := entry.Value.(*store.GeoValue)
				if !isGeo {
					t.Fatalf("without STOREDIST destination type = %T, want GeoValue", entry.Value)
				}
				if len(geo.Points) != 2 {
					t.Fatalf("destination has %d members, want 2", len(geo.Points))
				}
				return
			}

			zset, isZset := entry.Value.(*store.SortedSetValue)
			if !isZset {
				t.Fatalf("STOREDIST destination type = %T, want SortedSetValue with distances as scores", entry.Value)
			}
			for member, want := range tc.wantScores {
				gotScore, exists := zset.Members[member]
				if !exists {
					t.Fatalf("STOREDIST result missing member %q", member)
				}
				if math.Abs(gotScore-want) > 0.01 {
					t.Errorf("STOREDIST score for %q = %v, want ~%v", member, gotScore, want)
				}
			}
		})
	}
}
