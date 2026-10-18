package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newGeoSearchStoreRouter() (*Router, *store.Store) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterGeoCommands(router)
	return router, s
}

func runGeoCmd(t *testing.T, router *Router, s *store.Store, args ...string) string {
	t.Helper()
	raw := make([][]byte, 0, len(args))
	for _, a := range args {
		raw = append(raw, []byte(a))
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], s, resp.NewWriter(&out))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v errored: %v", args, err)
	}
	return out.String()
}

// GEOSEARCHSTORE requires BOTH a centre clause (FROMMEMBER/FROMLONLAT) and an
// extent clause (BYRADIUS/BYBOX). The handler guarded only ArgCount() < 5 and
// parsed the rest in a loop, so a call that merely omitted BYRADIUS left
// radius at zero, matched nothing, and fell into the "nothing matched" path --
// which DELETED the destination key and replied :0. A typo silently destroyed
// data instead of returning a syntax error.
func TestGeoSearchStoreRequiresRadiusClause(t *testing.T) {
	router, s := newGeoSearchStoreRouter()

	runGeoCmd(t, router, s, "GEOADD", "src", "13.361389", "38.115556", "Palermo")
	runGeoCmd(t, router, s, "SET", "dest", "precious-data")

	// No extent clause at all.
	got := runGeoCmd(t, router, s, "GEOSEARCHSTORE", "dest", "src", "FROMLONLAT", "0", "0")
	if len(got) == 0 || got[0] != '-' {
		t.Fatalf("GEOSEARCHSTORE without a radius clause replied %q, want a syntax error", got)
	}

	// The destination must survive the rejected call.
	if got := runGeoCmd(t, router, s, "EXISTS", "dest"); got != ":1\r\n" {
		t.Errorf("destination was destroyed by a malformed call: EXISTS = %q", got)
	}
	if got := runGeoCmd(t, router, s, "GET", "dest"); got != "$13\r\nprecious-data\r\n" {
		t.Errorf("destination value = %q, want \"precious-data\"", got)
	}
}

// A centre clause is equally mandatory: without one, fromLon/fromLat stay at 0.
func TestGeoSearchStoreRequiresCentreClause(t *testing.T) {
	router, s := newGeoSearchStoreRouter()

	runGeoCmd(t, router, s, "GEOADD", "src", "13.361389", "38.115556", "Palermo")
	runGeoCmd(t, router, s, "SET", "dest", "precious-data")

	got := runGeoCmd(t, router, s, "GEOSEARCHSTORE", "dest", "src", "BYRADIUS", "200", "km")
	if len(got) == 0 || got[0] != '-' {
		t.Fatalf("GEOSEARCHSTORE without a centre clause replied %q, want a syntax error", got)
	}
	if got := runGeoCmd(t, router, s, "EXISTS", "dest"); got != ":1\r\n" {
		t.Errorf("destination was destroyed by a malformed call: EXISTS = %q", got)
	}
}

// Control: every well-formed combination still works, including FROMMEMBER and
// BYBOX, and an empty result still clears the destination (that path is
// legitimate -- it only became dangerous because a malformed call reached it).
func TestGeoSearchStoreWellFormedStillWorks(t *testing.T) {
	t.Run("FROMLONLAT + BYRADIUS", func(t *testing.T) {
		router, s := newGeoSearchStoreRouter()
		runGeoCmd(t, router, s, "GEOADD", "src", "13.361389", "38.115556", "Palermo")
		if got := runGeoCmd(t, router, s, "GEOSEARCHSTORE", "dest", "src",
			"FROMLONLAT", "13.361389", "38.115556", "BYRADIUS", "200", "km"); got != ":1\r\n" {
			t.Errorf("well-formed store = %q, want \":1\\r\\n\"", got)
		}
		if got := runGeoCmd(t, router, s, "EXISTS", "dest"); got != ":1\r\n" {
			t.Errorf("destination should exist, EXISTS = %q", got)
		}
	})

	t.Run("FROMMEMBER + BYBOX", func(t *testing.T) {
		router, s := newGeoSearchStoreRouter()
		runGeoCmd(t, router, s, "GEOADD", "src", "13.361389", "38.115556", "Palermo")
		if got := runGeoCmd(t, router, s, "GEOSEARCHSTORE", "dest", "src",
			"FROMMEMBER", "Palermo", "BYBOX", "400", "400", "km"); got != ":1\r\n" {
			t.Errorf("well-formed BYBOX store = %q, want \":1\\r\\n\"", got)
		}
	})

	t.Run("no match clears destination", func(t *testing.T) {
		router, s := newGeoSearchStoreRouter()
		runGeoCmd(t, router, s, "GEOADD", "src", "13.361389", "38.115556", "Palermo")
		runGeoCmd(t, router, s, "SET", "dest", "stale")
		// A far-away centre with a small radius genuinely matches nothing, so
		// clearing the destination is the correct Redis behaviour.
		if got := runGeoCmd(t, router, s, "GEOSEARCHSTORE", "dest", "src",
			"FROMLONLAT", "0", "0", "BYRADIUS", "10", "km"); got != ":0\r\n" {
			t.Errorf("empty search = %q, want \":0\\r\\n\"", got)
		}
		if got := runGeoCmd(t, router, s, "EXISTS", "dest"); got != ":0\r\n" {
			t.Errorf("a genuinely empty search should clear the destination, EXISTS = %q", got)
		}
	})
}
