package command

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdGEO.DISTANCE and cmdSPATIAL.NEARBY computed distances with a
// hand-rolled haversine whose math helpers were numerically broken:
//
//	internal/command/mvcc_commands.go (before)
//	  sqrt(x):  z := x; z = (z + x/z)/2  -> x == 0 gives 0/0 = NaN
//	  atan(x):  Maclaurin series, converges only for |x| <= 1
//	            -> atan2(y,x) = atan(y/x) explodes for |y/x| > 1
//	  sin/cos:  10-term Taylor, loses precision well short of full float64
//
// Both commands are routed: GEO.DISTANCE (integration_commands.go:82) and
// SPATIAL.NEARBY (mvcc_commands.go:24), wired via server.go.
//
// Measured on identical inputs:
//
//	GEO.DISTANCE 0 0 0 0        -> "NaN"        (want 0.000000)
//	GEO.DISTANCE NYC -> LA      -> 3935.75 km   (store.Haversine: 3935.746255)
//	GEO.DISTANCE 0 0 -> 179.9999 -> -1.05e+60 km (want ~20015 km)
//
// Impact: "NaN" for co-located points, and SPATIAL.NEARBY filters on
// `dist <= radius`, so every wrong distance silently changes which points match.
//
// The fix delegates to store.Haversine -- the math-package implementation this
// repo already ships and that GEODIST/GEOSEARCH use.

func geoRaceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

// geoRaceNum extracts the float a GEO command replied with, and reports whether
// the reply was the literal string "NaN".
func geoRaceNum(t *testing.T, out string) float64 {
	t.Helper()
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable geo reply %q: %v", out, err)
	}
	if v.Type != resp.TypeBulkString {
		t.Fatalf("geo reply %q is type %v, want a bulk string", out, v.Type)
	}
	f, err := strconv.ParseFloat(string(v.Bulk), 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

func geoRaceNumStr(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func geoRaceRouter() *Router {
	router := NewRouter()
	RegisterIntegrationCommands(router)
	RegisterMVCCCommands(router)
	return router
}

// TestGeoDistanceIdenticalPoints is the primary symptom: co-located points are
// at distance 0, and NaN is not a distance.
func TestGeoDistanceIdenticalPoints(t *testing.T) {
	s := store.NewStore()
	router := geoRaceRouter()

	out := geoRaceRun(t, s, router, "GEO.DISTANCE", "0", "0", "0", "0", "km")
	if strings.Contains(out, "NaN") {
		t.Fatalf("GEO.DISTANCE 0 0 0 0 = %q, want 0.000000; the hand-rolled "+
			"sqrt() divided 0/0 when a == 0", out)
	}
	if got := geoRaceNum(t, out); got != 0 {
		t.Fatalf("GEO.DISTANCE identical points = %v km (%q), want 0", got, out)
	}
}

// TestGeoDistanceMatchesHaversineReference pins accuracy against
// store.Haversine -- the correct implementation this repo already ships.
func TestGeoDistanceMatchesHaversineReference(t *testing.T) {
	s := store.NewStore()
	router := geoRaceRouter()

	const nyLat, nyLon = 40.7128, -74.0060
	const laLat, laLon = 34.0522, -118.2437

	out := geoRaceRun(t, s, router, "GEO.DISTANCE",
		geoRaceNumStr(nyLat), geoRaceNumStr(nyLon),
		geoRaceNumStr(laLat), geoRaceNumStr(laLon), "km")

	got := geoRaceNum(t, out)
	want := store.Haversine(nyLon, nyLat, laLon, laLat)
	if want < 3900 || want > 3960 {
		t.Fatalf("CONTROL FAIL: store.Haversine reference = %v, not a plausible NYC-LA distance", want)
	}
	// cmdGEODISTANCE formats its reply to 2 decimal places, so the tightest
	// meaningful tolerance is half of the last displayed digit. Comparing at
	// 0.001 would fail on the command's own output rounding, not on accuracy.
	if math.Abs(got-want) > 0.005 {
		t.Fatalf("GEO.DISTANCE NYC->LA = %v km (%q), want %.6f km (2dp reply)", got, out, want)
	}
}

// TestGeoDistanceLargeArc is the divergence symptom: 180 degrees of longitude
// pushes atan(y/x) outside the convergence range of a Maclaurin series.
func TestGeoDistanceLargeArc(t *testing.T) {
	s := store.NewStore()
	router := geoRaceRouter()

	out := geoRaceRun(t, s, router, "GEO.DISTANCE", "0", "0", "0", "179.9999", "km")
	got := geoRaceNum(t, out)
	want := store.Haversine(0, 0, 179.9999, 0)
	if want < 19000 || want > 21000 {
		t.Fatalf("CONTROL FAIL: store.Haversine reference = %v, not a plausible near-antipodal distance", want)
	}
	if got < 19000 || got > 21000 {
		t.Fatalf("GEO.DISTANCE across ~180 deg longitude = %v km (%q), want ~%.3f km; "+
			"the Maclaurin atan series diverges for |y/x| > 1", got, out, want)
	}
}

// TestGeoDistanceControlsUncontended covers paths that do not depend on the
// broken local helpers: the shared reference implementation stays finite, the
// unit conversion ratio holds (absolute error cancels), and arity is validated.
func TestGeoDistanceControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := geoRaceRouter()

	ref := store.Haversine(-74.0060, 40.7128, -118.2437, 34.0522)
	if math.IsNaN(ref) || math.IsInf(ref, 0) || ref < 3900 || ref > 3960 {
		t.Fatalf("CONTROL FAIL: store.Haversine NYC->LA = %v, want a finite ~3935 km", ref)
	}
	if got := store.Haversine(0, 0, 0, 0); got != 0 {
		t.Fatalf("CONTROL FAIL: store.Haversine for identical points = %v, want 0", got)
	}

	km := geoRaceNum(t, geoRaceRun(t, s, router, "GEO.DISTANCE", "51.5", "-0.12", "38.8", "-77.0", "km"))
	m := geoRaceNum(t, geoRaceRun(t, s, router, "GEO.DISTANCE", "51.5", "-0.12", "38.8", "-77.0", "m"))
	if math.Abs(m-km*1000) > math.Abs(km)*0.05 {
		t.Fatalf("CONTROL FAIL: metres %.3f is not ~1000x km %.3f", m, km)
	}

	if out := geoRaceRun(t, s, router, "GEO.DISTANCE", "1", "2"); out == "" ||
		!strings.Contains(out, "wrong number of arguments") {
		t.Fatalf("CONTROL FAIL: GEO.DISTANCE 1 2 = %q, want a wrong-arity error", out)
	}
}
