package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newGeoSearchRouter() (*Router, *store.Store) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterGeoCommands(router)
	return router, s
}

func runGeoSearch(t *testing.T, router *Router, s *store.Store, args ...string) string {
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

// GEOSEARCH requires BOTH a centre clause (FROMMEMBER/FROMLONLAT) and an extent
// clause (BYRADIUS/BYBOX). The handler guarded only ArgCount() < 4 and parsed
// the rest in a loop with a catch-all `default: i++`, so a query that merely
// omitted BYRADIUS left radius at zero. `dist <= radius` then FABRICATED a
// match for a member sitting exactly on the centre (0 <= 0) and reported "no
// matches" for everything else -- a wrong answer where Redis answers with a
// syntax error, leaving a dropped BYRADIUS indistinguishable from a genuine
// empty result.
func TestGeoSearchRequiresRadiusClause(t *testing.T) {
	router, s := newGeoSearchRouter()
	runGeoSearch(t, router, s, "GEOADD", "cities", "13.361389", "38.115556", "Palermo")

	// Centre sits exactly on Palermo: the zero radius made this match.
	got := runGeoSearch(t, router, s, "GEOSEARCH", "cities",
		"FROMLONLAT", "13.361389", "38.115556")
	if len(got) == 0 || got[0] != '-' {
		t.Fatalf("GEOSEARCH without a radius clause replied %q, want a syntax error; "+
			"the zero radius fabricated a match at the exact centre", got)
	}
}

// A centre clause is equally mandatory; without one, fromLon/fromLat stay 0,0.
func TestGeoSearchRequiresCentreClause(t *testing.T) {
	router, s := newGeoSearchRouter()
	runGeoSearch(t, router, s, "GEOADD", "cities", "13.361389", "38.115556", "Palermo")

	got := runGeoSearch(t, router, s, "GEOSEARCH", "cities", "BYRADIUS", "20000", "km")
	if len(got) == 0 || got[0] != '-' {
		t.Fatalf("GEOSEARCH without a centre clause replied %q, want a syntax error", got)
	}
}

// Control: every well-formed combination still matches, and a genuine empty
// result still returns an empty array (not an error) -- that path is legitimate
// and must not be turned into a rejection.
func TestGeoSearchWellFormedStillWorks(t *testing.T) {
	t.Run("FROMLONLAT + BYRADIUS matches", func(t *testing.T) {
		router, s := newGeoSearchRouter()
		runGeoSearch(t, router, s, "GEOADD", "cities", "13.361389", "38.115556", "Palermo")
		got := runGeoSearch(t, router, s, "GEOSEARCH", "cities",
			"FROMLONLAT", "13.361389", "38.115556", "BYRADIUS", "200", "km")
		if !bytes.HasPrefix([]byte(got), []byte("*1\r\n")) {
			t.Errorf("well-formed GEOSEARCH = %q, want a one-element array", got)
		}
		if !bytes.Contains([]byte(got), []byte("Palermo")) {
			t.Errorf("expected Palermo in the result, got %q", got)
		}
	})

	t.Run("FROMMEMBER + BYBOX matches", func(t *testing.T) {
		router, s := newGeoSearchRouter()
		runGeoSearch(t, router, s, "GEOADD", "cities", "13.361389", "38.115556", "Palermo")
		got := runGeoSearch(t, router, s, "GEOSEARCH", "cities",
			"FROMMEMBER", "Palermo", "BYBOX", "400", "400", "km")
		if !bytes.HasPrefix([]byte(got), []byte("*1\r\n")) {
			t.Errorf("well-formed BYBOX GEOSEARCH = %q, want a one-element array", got)
		}
	})

	t.Run("genuine empty result is not an error", func(t *testing.T) {
		router, s := newGeoSearchRouter()
		runGeoSearch(t, router, s, "GEOADD", "cities", "13.361389", "38.115556", "Palermo")
		got := runGeoSearch(t, router, s, "GEOSEARCH", "cities",
			"FROMLONLAT", "0", "0", "BYRADIUS", "10", "km")
		if got != "*0\r\n" {
			t.Errorf("a valid query that matches nothing = %q, want \"*0\\r\\n\"", got)
		}
	})
}
