package command_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdGEOSEARCH accepted WITHDIST/WITHCOORD/WITHHASH/COUNT/ASC/DESC
// and discarded every one of them, returning bare members in Go map-iteration
// order (randomized per call). Redis GEOSEARCH returns results nearest-first
// (or farthest-first with DESC), limits with COUNT, and wraps each entry in
// [member, distance, geohash, coordinates] payloads for the WITH* options.
func TestGeoSearchOptions(t *testing.T) {
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

	t.Run("plain control membership", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km")
		if got.Type != resp.TypeArray || len(got.Array) != 2 {
			t.Fatalf("replied %v with %d entries, want 2", got.Type, len(got.Array))
		}
		members := map[string]bool{}
		for _, e := range got.Array {
			members[string(e.Bulk)] = true
		}
		if !members["Catania"] || !members["Palermo"] {
			t.Fatalf("members = %v, want Catania and Palermo", members)
		}
	})

	t.Run("WITHDIST pair shape and membership", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHDIST")
		if got.Type != resp.TypeArray || len(got.Array) != 2 {
			t.Fatalf("replied %v with %d entries, want 2 pair entries", got.Type, len(got.Array))
		}
		for i, e := range got.Array {
			if e.Type != resp.TypeArray || len(e.Array) != 2 {
				t.Fatalf("entry %d type %v len %d, want a [member, distance] pair", i, e.Type, len(e.Array))
			}
		}
		scores := map[string]string{}
		for _, e := range got.Array {
			scores[string(e.Array[0].Bulk)] = string(e.Array[1].Bulk)
		}
		if scores["Catania"] != "0" {
			t.Errorf("Catania distance = %q, want 0", scores["Catania"])
		}
		if !strings.HasPrefix(scores["Palermo"], "166.2") {
			t.Errorf("Palermo distance = %q, want the km distance (~166.2)", scores["Palermo"])
		}
	})

	t.Run("ASC nearest-first", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHDIST", "ASC")
		if got.Type != resp.TypeArray || len(got.Array) != 2 {
			t.Fatalf("replied %v with %d entries, want 2 pair entries", got.Type, len(got.Array))
		}
		for i, e := range got.Array {
			if e.Type != resp.TypeArray || len(e.Array) != 2 {
				t.Fatalf("entry %d type %v len %d, want a [member, distance] pair", i, e.Type, len(e.Array))
			}
		}
		if string(got.Array[0].Array[0].Bulk) != "Catania" || string(got.Array[1].Array[0].Bulk) != "Palermo" {
			t.Errorf("ASC order = [%q %q], want [Catania Palermo]",
				got.Array[0].Array[0].Bulk, got.Array[1].Array[0].Bulk)
		}
	})

	t.Run("COUNT limits results", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "COUNT", "1")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("replied %v with %d entries, want exactly 1", got.Type, len(got.Array))
		}
		if string(got.Array[0].Bulk) != "Catania" {
			t.Errorf("COUNT 1 member = %q, want the nearest Catania", got.Array[0].Bulk)
		}
	})

	t.Run("DESC farthest-first", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHDIST", "DESC")
		if got.Type != resp.TypeArray || len(got.Array) != 2 {
			t.Fatalf("replied %v with %d entries, want 2 pair entries", got.Type, len(got.Array))
		}
		for i, e := range got.Array {
			if e.Type != resp.TypeArray || len(e.Array) != 2 {
				t.Fatalf("entry %d type %v len %d, want a [member, distance] pair", i, e.Type, len(e.Array))
			}
		}
		if string(got.Array[0].Array[0].Bulk) != "Palermo" || string(got.Array[1].Array[0].Bulk) != "Catania" {
			t.Errorf("DESC order = [%q %q], want [Palermo Catania]",
				got.Array[0].Array[0].Bulk, got.Array[1].Array[0].Bulk)
		}
	})

	t.Run("WITHCOORD adds coordinates", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHCOORD", "COUNT", "1")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("replied %v with %d entries, want 1", got.Type, len(got.Array))
		}
		entry := got.Array[0]
		if entry.Type != resp.TypeArray || len(entry.Array) != 2 {
			t.Fatalf("entry type %v len %d, want [member, coords]", entry.Type, len(entry.Array))
		}
		coord := entry.Array[1]
		if coord.Type != resp.TypeArray || len(coord.Array) != 2 {
			t.Fatalf("coords type %v len %d, want [lon lat]", coord.Type, len(coord.Array))
		}
		if string(coord.Array[0].Bulk) != "15.087269" || string(coord.Array[1].Bulk) != "37.502669" {
			t.Errorf("coords = [%q %q], want Catania's", coord.Array[0].Bulk, coord.Array[1].Bulk)
		}
	})

	t.Run("WITHHASH adds geohash integer", func(t *testing.T) {
		want := store.EncodeGeohashInt(15.087269, 37.502669)
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHHASH", "COUNT", "1")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("replied %v with %d entries, want 1", got.Type, len(got.Array))
		}
		entry := got.Array[0]
		if entry.Type != resp.TypeArray || len(entry.Array) != 2 {
			t.Fatalf("entry type %v len %d, want [member, hash]", entry.Type, len(entry.Array))
		}
		if entry.Array[1].Type != resp.TypeInteger || entry.Array[1].Int != int64(want) {
			t.Errorf("hash = %v %v, want %d", entry.Array[1].Type, entry.Array[1].Int, want)
		}
	})

	t.Run("combined WITHDIST COUNT DESC", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHDIST", "COUNT", "1", "DESC")
		if got.Type != resp.TypeArray || len(got.Array) != 1 {
			t.Fatalf("replied %v with %d entries, want 1", got.Type, len(got.Array))
		}
		entry := got.Array[0]
		if entry.Type != resp.TypeArray || len(entry.Array) != 2 {
			t.Fatalf("entry type %v len %d, want a [member, distance] pair", entry.Type, len(entry.Array))
		}
		if string(entry.Array[0].Bulk) != "Palermo" {
			t.Errorf("member = %q, want Palermo (farthest-first with DESC)", entry.Array[0].Bulk)
		}
	})

	t.Run("missing FROMMEMBER control", func(t *testing.T) {
		got := run("GEOSEARCH", "geo", "FROMMEMBER", "NoCity", "BYRADIUS", "200", "km")
		if got.Type != resp.TypeArray || len(got.Array) != 0 {
			t.Fatalf("replied %v with %d entries, want an empty array", got.Type, len(got.Array))
		}
	})
}
