package store

import (
	"fmt"
	"math"
	"sync"
)

type GeoPoint struct {
	Lon float64
	Lat float64
}

type GeoValue struct {
	mu     sync.RWMutex
	Points map[string]GeoPoint
}

func (v *GeoValue) Lock()    { v.mu.Lock() }
func (v *GeoValue) Unlock()  { v.mu.Unlock() }
func (v *GeoValue) RLock()   { v.mu.RLock() }
func (v *GeoValue) RUnlock() { v.mu.RUnlock() }

func NewGeoValue() *GeoValue {
	return &GeoValue{
		Points: make(map[string]GeoPoint),
	}
}

func (v *GeoValue) Type() DataType { return DataTypeGeo }
func (v *GeoValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return int64(len(v.Points))*24 + 48
}
func (v *GeoValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for member, point := range v.Points {
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s: (%.6f, %.6f)", member, point.Lon, point.Lat)
	}
	return result
}
func (v *GeoValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := NewGeoValue()
	for k, p := range v.Points {
		cloned.Points[k] = p
	}
	return cloned
}

func (v *GeoValue) Add(member string, lon, lat float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Points[member] = GeoPoint{Lon: lon, Lat: lat}
}

func (v *GeoValue) Get(member string) (GeoPoint, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	p, ok := v.Points[member]
	return p, ok
}

func (v *GeoValue) Remove(members ...string) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	removed := 0
	for _, m := range members {
		if _, exists := v.Points[m]; exists {
			delete(v.Points, m)
			removed++
		}
	}
	return removed
}

func (v *GeoValue) Distance(from, to string) float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	p1, ok1 := v.Points[from]
	p2, ok2 := v.Points[to]
	if !ok1 || !ok2 {
		return -1
	}
	return Haversine(p1.Lon, p1.Lat, p2.Lon, p2.Lat)
}

func Haversine(lon1, lat1, lon2, lat2 float64) float64 {
	const earthRadius = 6371

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	a = math.Max(0, math.Min(1, a))
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func EncodeGeohash(lon, lat float64) string {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	var bits uint64
	var precision = 12

	minLon, maxLon := -180.0, 180.0
	minLat, maxLat := -90.0, 90.0

	for i := 0; i < precision*5; i++ {
		if i%2 == 0 {
			mid := (minLon + maxLon) / 2
			if lon >= mid {
				bits = bits*2 + 1
				minLon = mid
			} else {
				bits *= 2
				maxLon = mid
			}
		} else {
			mid := (minLat + maxLat) / 2
			if lat >= mid {
				bits = bits*2 + 1
				minLat = mid
			} else {
				bits *= 2
				maxLat = mid
			}
		}
	}

	result := make([]byte, precision)
	for i := 0; i < precision; i++ {
		result[precision-1-i] = base32[bits%32]
		bits /= 32
	}

	return string(result)
}

func EncodeGeohashInt(lon, lat float64) uint64 {
	var bits uint64
	minLon, maxLon := -180.0, 180.0
	minLat, maxLat := -90.0, 90.0

	for i := 0; i < 52; i++ {
		if i%2 == 0 {
			mid := (minLon + maxLon) / 2
			if lon >= mid {
				bits = bits*2 + 1
				minLon = mid
			} else {
				bits *= 2
				maxLon = mid
			}
		} else {
			mid := (minLat + maxLat) / 2
			if lat >= mid {
				bits = bits*2 + 1
				minLat = mid
			} else {
				bits *= 2
				maxLat = mid
			}
		}
	}

	return bits
}
