package store

import (
	"math"
	"testing"
)

func TestGeoCodecRedisFixtures(t *testing.T) {
	for _, tc := range []struct {
		point GeoPoint
		score float64
		hash  string
	}{
		{GeoPoint{Lon: 13.361389, Lat: 38.115556}, 3479099956230698, "sqc8b49rny0"},
		{GeoPoint{Lon: 15.087269, Lat: 37.502669}, 3479447370796909, "sqdtr74hyu0"},
		{GeoPoint{}, 3377699720527872, "s0000000000"},
	} {
		if score := GeoScore(tc.point.Lon, tc.point.Lat); score != tc.score {
			t.Fatalf("score=%v want=%v", score, tc.score)
		}
		point := GeoPointFromScore(tc.score)
		if GeoScore(point.Lon, point.Lat) != tc.score || GeoHashString(point) != tc.hash {
			t.Fatalf("roundtrip/hash for %+v: %+v %q", tc.point, point, GeoHashString(point))
		}
	}
	a, b := GeoPointFromScore(3479099956230698), GeoPointFromScore(3479447370796909)
	if d := GeoDistanceMeters(a, b); math.Abs(d-166274.15156960032) > 1e-7 {
		t.Fatalf("distance=%v", d)
	}
}

func FuzzGeoCodec(f *testing.F) {
	for _, point := range []GeoPoint{{}, {Lon: 180, Lat: GeoLatitudeLimit}, {Lon: -180, Lat: -GeoLatitudeLimit}, {Lon: 13.361389, Lat: 38.115556}} {
		f.Add(point.Lon, point.Lat)
	}
	f.Fuzz(func(t *testing.T, lon, lat float64) {
		if math.IsNaN(lon) || math.IsNaN(lat) || lon < -180 || lon > 180 || lat < -GeoLatitudeLimit || lat > GeoLatitudeLimit {
			t.Skip()
		}
		score := GeoScore(lon, lat)
		point := GeoPointFromScore(score)
		if math.IsNaN(point.Lon) || math.IsNaN(point.Lat) || point.Lon < -180 || point.Lon > 180 || point.Lat < -GeoLatitudeLimit || point.Lat > GeoLatitudeLimit || GeoScore(point.Lon, point.Lat) != score {
			t.Fatalf("invalid codec roundtrip: %+v -> %v -> %+v", GeoPoint{Lon: lon, Lat: lat}, score, point)
		}
	})
}
