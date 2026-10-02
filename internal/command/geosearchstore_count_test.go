package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execGeo runs the real router handler for cmd against s and returns the reply.
func execGeo(s *store.Store, r *Router, cmd string, args ...string) string {
	handler, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered"
	}
	var buf bytes.Buffer
	ctx := NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(&buf))
	if err := handler.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// geoLen reports how many members the destination key actually holds,
// independent of the integer the command replied with.
func geoLen(s *store.Store, key string) int {
	entry, exists := s.Get(key)
	if !exists {
		return -1
	}
	geo, ok := entry.Value.(*store.GeoValue)
	if !ok {
		return -2
	}
	return len(geo.Points)
}

// TestProofGeoSearchStoreCountLimits is the round proof.
//
// Contract (Redis GEOSEARCHSTORE): the command accepts the same options as
// GEOSEARCH, including "COUNT count" which limits how many matches are stored
// in the destination key. The integer it returns is the number of items stored.
//
// Defect: cmdGEOSEARCHSTORE's option loop had a single catch-all
// `case "ASC", "DESC", "COUNT": i++` that consumed the keyword and DISCARDED
// it — unlike its sibling cmdGEOSEARCH, which parses COUNT and truncates. So
// `COUNT 1` stored every match in the radius and still reported that larger
// count.
//
// Controls: the same command WITHOUT COUNT must store and report every match,
// and the read-only sibling GEOSEARCH must honour COUNT correctly — both pass
// before and after the fix.
func TestProofGeoSearchStoreCountLimits(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterGeoCommands(r)
	RegisterSortedSetCommands(r) // ZCARD, for the STOREDIST control

	// Three points on a line near lon 13.0, lat 38.0, all inside a 200 km box.
	if got := execGeo(s, r, "GEOADD", "src", "13.0", "38.0", "near", "13.01", "38.0", "mid", "13.05", "38.0", "far"); got != ":3\r\n" {
		t.Fatalf("CONTROL 0 broken harness: GEOADD = %q, want \":3\\r\\n\"", got)
	}

	// ---- CONTROL 1: GEOSEARCHSTORE WITHOUT COUNT stores every match.
	if got := execGeo(s, r, "GEOSEARCHSTORE", "d:all", "src", "FROMLONLAT", "13.0", "38.0", "BYRADIUS", "200", "km"); got != ":3\r\n" {
		t.Fatalf("CONTROL 1 broken harness: plain GEOSEARCHSTORE = %q, want \":3\\r\\n\"", got)
	}
	if n := geoLen(s, "d:all"); n != 3 {
		t.Fatalf("CONTROL 1 broken harness: plain GEOSEARCHSTORE stored %d members, want 3", n)
	}
	t.Log("CONTROL 1 ok: without COUNT all 3 matches stored")

	// ---- CONTROL 2: the sibling GEOSEARCH honours COUNT (fixed in an
	// earlier round, and it is the in-repo basis for the expected behaviour).
	// A bare-member GEOSEARCH reply is "*<n>\r\n$len\r\nmember\r\n" repeated,
	// so assert on the array header and that the single member is the NEAREST.
	got2 := execGeo(s, r, "GEOSEARCH", "src", "FROMLONLAT", "13.0", "38.0", "BYRADIUS", "200", "km", "COUNT", "1")
	if !strings.HasPrefix(got2, "*1\r\n") {
		t.Fatalf("CONTROL 2 broken harness: GEOSEARCH COUNT 1 = %q, want a 1-element array", got2)
	}
	if !strings.Contains(got2, "near") || strings.Contains(got2, "far") {
		t.Fatalf("CONTROL 2 broken harness: GEOSEARCH COUNT 1 = %q, want the nearest member \"near\"", got2)
	}
	t.Log("CONTROL 2 ok: sibling GEOSEARCH honours COUNT")

	// ---- THE DEFECT: GEOSEARCHSTORE with COUNT 1 must store exactly 1.
	got := execGeo(s, r, "GEOSEARCHSTORE", "d:cnt", "src", "FROMLONLAT", "13.0", "38.0", "BYRADIUS", "200", "km", "COUNT", "1")
	stored := geoLen(s, "d:cnt")

	if got != ":1\r\n" || stored != 1 {
		t.Fatalf("FAIL: GEOSEARCHSTORE ... COUNT 1 reported %q and stored %d member(s); "+
			"want \":1\\r\\n\" and 1 stored (COUNT was parsed and discarded)",
			strings.TrimSpace(got), stored)
	}
	t.Log("PASS: COUNT 1 stored exactly one member")

	// ---- SECONDARY: COUNT 2 must store 2, and the destination must contain
	// the two NEAREST members — COUNT implies nearest-first ordering.
	if got := execGeo(s, r, "GEOSEARCHSTORE", "d:cnt2", "src", "FROMLONLAT", "13.0", "38.0", "BYRADIUS", "200", "km", "COUNT", "2"); got != ":2\r\n" {
		t.Fatalf("SECONDARY: GEOSEARCHSTORE COUNT 2 = %q, want \":2\\r\\n\"", strings.TrimSpace(got))
	}
	if n := geoLen(s, "d:cnt2"); n != 2 {
		t.Fatalf("SECONDARY FAIL: COUNT 2 stored %d members, want 2", n)
	}
	d2, _ := s.Get("d:cnt2")
	if _, ok := d2.Value.(*store.GeoValue).Points["far"]; ok {
		t.Fatalf("SECONDARY FAIL: COUNT 2 kept \"far\" (the most distant member); "+
			"COUNT implies nearest-first, so \"far\" must be dropped")
	}
	t.Log("PASS: COUNT 2 stored the two nearest members")

	// ---- THIRDARY BOUNDARY: COUNT larger than the match count keeps all.
	if got := execGeo(s, r, "GEOSEARCHSTORE", "d:big", "src", "FROMLONLAT", "13.0", "38.0", "BYRADIUS", "200", "km", "COUNT", "99"); got != ":3\r\n" {
		t.Fatalf("BOUNDARY: GEOSEARCHSTORE COUNT 99 = %q, want \":3\\r\\n\" (all matches)", strings.TrimSpace(got))
	}
	if n := geoLen(s, "d:big"); n != 3 {
		t.Fatalf("BOUNDARY FAIL: COUNT 99 stored %d members, want 3", n)
	}
	t.Log("PASS: COUNT beyond the match count keeps every member")

	// ---- FOURTHARY: STOREDIST control still builds a zset destination.
	if got := execGeo(s, r, "GEOSEARCHSTORE", "d:dist", "src", "FROMLONLAT", "13.0", "38.0", "BYRADIUS", "200", "km", "STOREDIST"); got != ":3\r\n" {
		t.Fatalf("STOREDIST control broken harness: = %q, want \":3\\r\\n\"", got)
	}
	if got := execGeo(s, r, "ZCARD", "d:dist"); got != ":3\r\n" {
		t.Fatalf("STOREDIST control broken harness: ZCARD = %q, want \":3\\r\\n\"", got)
	}
	t.Log("PASS: STOREDIST destination still holds all 3 as a zset")
}