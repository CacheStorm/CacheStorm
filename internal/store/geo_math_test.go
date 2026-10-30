package store

import (
	"math"
	"testing"
)

func TestGeoTrigRoundsLikeGlibc(t *testing.T) {
	// Values glibc returns and Go's math package misses by one unit.
	if got := geoCos(37 * (math.Pi / 180)); got != 0.7986355100472928 {
		t.Fatalf("cos(37 deg) = %v", got)
	}
	if got := geoCos(math.Pi / 2); got != 6.123233995736766e-17 {
		t.Fatalf("cos(pi/2) = %v", got)
	}
	if got := geoSin(math.Pi); got != 1.2246467991473532e-16 {
		t.Fatalf("sin(pi) = %v", got)
	}
	for _, x := range []float64{0, 0.5, -1.2, 3.1, -3.14159, 1e-300} {
		if d := math.Abs(geoSin(x) - math.Sin(x)); d > 2*ulp(math.Sin(x)) {
			t.Fatalf("sin(%v) = %v, math.Sin %v", x, geoSin(x), math.Sin(x))
		}
		if d := math.Abs(geoCos(x) - math.Cos(x)); d > 2*ulp(math.Cos(x)) {
			t.Fatalf("cos(%v) = %v, math.Cos %v", x, geoCos(x), math.Cos(x))
		}
	}
	// glibc's asin, which Go's math.Asin misses near 1 by several units.
	for x, want := range map[float64]float64{0: 0, 0.5000001: 0.5235988910683566, 0.75: 0.848062078981481, 0.9: 1.1197695149986342, 0.99: 1.4292568534704693, 0.999999: 1.5693821131146521, 1: math.Pi / 2} {
		if got := geoAsin(x); got != want {
			t.Fatalf("asin(%v) = %v, want %v", x, got, want)
		}
	}
}

func TestGeoDistanceMatchesRedis(t *testing.T) {
	// GEOSEARCHSTORE ... FROMLONLAT 15 37 ... STOREDIST to Palermo, in km.
	palermo := GeoPoint{Lon: 13.36138933897018433, Lat: 38.11555639549629859}
	if got := GeoDistanceMetersExact(GeoPoint{Lon: 15, Lat: 37}, palermo) / 1000; got != 190.44242984775784 {
		t.Fatalf("distance = %v", got)
	}
}

func ulp(x float64) float64 {
	return math.Nextafter(math.Abs(x), math.Inf(1)) - math.Abs(x)
}

func BenchmarkGeoDistanceMeters(b *testing.B) {
	p, q := GeoPoint{Lon: 15, Lat: 37}, GeoPoint{Lon: 13.36, Lat: 38.11}
	b.Run("fast", func(b *testing.B) {
		for b.Loop() {
			GeoDistanceMeters(p, q)
		}
	})
	b.Run("exact", func(b *testing.B) {
		for b.Loop() {
			GeoDistanceMetersExact(p, q)
		}
	})
}
