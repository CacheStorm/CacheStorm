package command_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGeoSearchStoreReplacesDestination(t *testing.T) {
	for _, mode := range []string{"fresh", "repeat", "string", "ttl", "same-key", "empty", "missing-source"} {
		t.Run(mode, func(t *testing.T) {
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
			checkCount := func(v *resp.Value, want int64) {
				t.Helper()
				if v.Type != resp.TypeInteger || v.Int != want {
					t.Fatalf("want integer %d, got %+v", want, v)
				}
			}
			checkCount(run("GEOADD", "src", "13.361389", "38.115556", "Palermo", "15.087269", "37.502669", "Catania"), 2)
			dest, src := "dest", "src"
			switch mode {
			case "repeat", "ttl", "empty", "missing-source":
				checkCount(run("GEOSEARCHSTORE", dest, src, "FROMMEMBER", "Catania", "BYRADIUS", "200", "km"), 2)
				entry, ok := s.Get(dest)
				if !ok || len(entry.Value.(*store.GeoValue).Points) != 2 {
					t.Fatal("fresh destination control must contain both cities")
				}
				if mode == "ttl" {
					if err := s.Set(dest, entry.Value, store.SetOptions{TTL: time.Hour}); err != nil {
						t.Fatal(err)
					}
				}
			case "string":
				if err := s.Set(dest, &store.StringValue{Data: []byte("old")}, store.SetOptions{}); err != nil {
					t.Fatal(err)
				}
			case "same-key":
				dest = src
			}
			if mode == "empty" || mode == "missing-source" {
				if mode == "missing-source" {
					src = "missing"
				}
				checkCount(run("GEOSEARCHSTORE", dest, src, "FROMLONLAT", "0", "0", "BYRADIUS", "1", "km"), 0)
				if _, ok := s.Get(dest); ok {
					t.Fatal("empty result must delete destination")
				}
				return
			}
			checkCount(run("GEOSEARCHSTORE", dest, src, "FROMMEMBER", "Catania", "BYRADIUS", "1", "km"), 1)
			entry, ok := s.Get(dest)
			if !ok {
				t.Fatal("destination missing")
			}
			geo, ok := entry.Value.(*store.GeoValue)
			if !ok {
				t.Fatalf("destination type = %T, want GeoValue", entry.Value)
			}
			if len(geo.Points) != 1 {
				t.Errorf("destination has %d members, want only Catania; stale members survived overwrite", len(geo.Points))
			}
			if point, ok := geo.Get("Catania"); !ok || point.Lon != 15.087269 || point.Lat != 37.502669 {
				t.Errorf("Catania missing or changed: %+v", point)
			}
			if entry.ExpiresAt != 0 {
				t.Error("overwrite retained destination TTL")
			}
			if dest != src {
				entry, ok := s.Get(src)
				if !ok || len(entry.Value.(*store.GeoValue).Points) != 2 {
					t.Error("search modified the source")
				}
			}
		})
	}
}
