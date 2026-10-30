package store

import (
	"math"
)

const GeoLatitudeLimit = 85.05112878

func GeoScore(lon, lat float64) float64 {
	return float64(geoInterleave(uint32((lat+GeoLatitudeLimit)/(2*GeoLatitudeLimit)*(1<<26)), uint32((lon+180)/360*(1<<26))))
}

func geoInterleave(lat, lon uint32) uint64 {
	var result uint64
	for bit := uint(0); bit < 32; bit++ {
		result |= uint64((lat>>bit)&1) << (2 * bit)
		result |= uint64((lon>>bit)&1) << (2*bit + 1)
	}
	return result
}

func GeoPointFromScore(score float64) GeoPoint {
	var bits uint64
	if score >= 0 && score < math.Exp2(64) {
		bits = uint64(score)
	} else if score < 0 && score > -math.Exp2(63) {
		bits = -uint64(-score)
	} else if score < 0 {
		bits = 1 << 63
	}
	var lat, lon uint32
	for bit := uint(0); bit < 32; bit++ {
		lat |= uint32((bits>>(2*bit))&1) << bit
		lon |= uint32((bits>>(2*bit+1))&1) << bit
	}
	minLon := -180 + float64(lon)/(1<<26)*360
	maxLon := -180 + float64(lon+1)/(1<<26)*360
	minLat := -GeoLatitudeLimit + float64(lat)/(1<<26)*(2*GeoLatitudeLimit)
	maxLat := -GeoLatitudeLimit + float64(lat+1)/(1<<26)*(2*GeoLatitudeLimit)
	return GeoPoint{Lon: max(-180, min(180, (minLon+maxLon)/2)), Lat: max(-GeoLatitudeLimit, min(GeoLatitudeLimit, (minLat+maxLat)/2))}
}

func GeoHashString(point GeoPoint) string {
	bits := geoInterleave(uint32((point.Lat+90)/180*(1<<26)), uint32((point.Lon+180)/360*(1<<26)))
	const alphabet = "0123456789bcdefghjkmnpqrstuvwxyz"
	result := make([]byte, 11)
	for i := 0; i < 10; i++ {
		result[i] = alphabet[(bits>>uint(47-5*i))&31]
	}
	result[10] = '0'
	return string(result)
}

func GeoDistanceMeters(a, b GeoPoint) float64 {
	return geoDistance(a, b, math.Sin, math.Cos, math.Asin)
}

// GeoDistanceMetersExact is GeoDistanceMeters to the last bit of Redis'
// result, as STOREDIST stores it; see geo_math.go.
func GeoDistanceMetersExact(a, b GeoPoint) float64 {
	return geoDistance(a, b, geoSin, geoCos, geoAsin)
}

// geoDistance is Redis' geohashGetDistance.
func geoDistance(a, b GeoPoint, sin, cos, asin func(float64) float64) float64 {
	const radius = 6372797.560856
	const radians = math.Pi / 180
	v := sin((b.Lon*radians - a.Lon*radians) / 2)
	if v == 0 {
		return radius * math.Abs(b.Lat*radians-a.Lat*radians)
	}
	u := sin((b.Lat*radians - a.Lat*radians) / 2)
	// The conversions keep the compiler from fusing these into FMAs, which
	// Redis' build does not use.
	x := float64(u*u) + float64(float64(float64(cos(a.Lat*radians)*cos(b.Lat*radians))*v)*v)
	return 2 * radius * asin(math.Sqrt(min(1, max(0, x))))
}

func (v *GeoValue) SortedSet() (*SortedSetValue, bool) {
	v.RLock()
	defer v.RUnlock()
	result := &SortedSetValue{Members: make(map[string]float64, len(v.Points))}
	for member, point := range v.Points {
		if math.IsNaN(point.Lon) || math.IsNaN(point.Lat) || point.Lon < -180 || point.Lon > 180 || point.Lat < -GeoLatitudeLimit || point.Lat > GeoLatitudeLimit {
			return nil, false
		}
		result.Add(member, GeoScore(point.Lon, point.Lat))
	}
	return result, true
}
