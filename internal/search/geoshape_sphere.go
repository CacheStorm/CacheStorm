package search

import "math"

// SPHERICAL geoshapes: longitude and latitude in degrees, edges along great
// circles, as RediSearch's boost.geometry spherical strategies take them.

type vec3 struct{ x, y, z float64 }

func toVec(p geoPoint) vec3 {
	lon, lat := p.x*math.Pi/180, p.y*math.Pi/180
	return vec3{math.Cos(lat) * math.Cos(lon), math.Cos(lat) * math.Sin(lon), math.Sin(lat)}
}

func toLonLat(v vec3) geoPoint {
	return geoPoint{math.Atan2(v.y, v.x) * 180 / math.Pi, math.Atan2(v.z, math.Hypot(v.x, v.y)) * 180 / math.Pi}
}

func (a vec3) cross(b vec3) vec3 {
	return vec3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}
func (a vec3) dot(b vec3) float64   { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec3) norm() float64        { return math.Sqrt(a.dot(a)) }
func (a vec3) scale(f float64) vec3 { return vec3{a.x * f, a.y * f, a.z * f} }
func (a vec3) add(b vec3) vec3      { return vec3{a.x + b.x, a.y + b.y, a.z + b.z} }

func (a vec3) unit() vec3 {
	if n := a.norm(); n > 0 {
		return a.scale(1 / n)
	}
	return a
}

func angleBetween(a, b vec3) float64 { return math.Atan2(a.cross(b).norm(), a.dot(b)) }

const sphEps = 1e-12

// sphOnArc reports whether p lies on the great-circle arc from a to b.
func sphOnArc(p, a, b geoPoint) bool {
	if p == a || p == b {
		return true
	}
	P, A, B := toVec(p), toVec(a), toVec(b)
	n := A.cross(B)
	if n.norm() < sphEps {
		return angleBetween(P, A) < sphEps
	}
	n = n.unit()
	if math.Abs(n.dot(P)) > sphEps {
		return false
	}
	return A.cross(P).dot(n) >= -sphEps && P.cross(B).dot(n) >= -sphEps
}

// sphCrossing is where arcs ab and cd cross away from their ends, if they
// do.
func sphCrossing(a, b, c, d geoPoint) (vec3, bool) {
	A, B, C, D := toVec(a), toVec(b), toVec(c), toVec(d)
	n1, n2 := A.cross(B), C.cross(D)
	l := n1.cross(n2)
	if l.norm() < sphEps {
		return vec3{}, false
	}
	l = l.unit()
	for _, x := range []vec3{l, l.scale(-1)} {
		if A.cross(x).dot(n1) > 0 && x.cross(B).dot(n1) > 0 && C.cross(x).dot(n2) > 0 && x.cross(D).dot(n2) > 0 {
			return x, true
		}
	}
	return vec3{}, false
}

func sphSegmentsIntersect(a, b, c, d geoPoint) bool {
	if sphOnArc(a, c, d) || sphOnArc(b, c, d) || sphOnArc(c, a, b) || sphOnArc(d, a, b) {
		return true
	}
	_, ok := sphCrossing(a, b, c, d)
	return ok
}

// wrapLon brings a longitude difference into (-180, 180].
func wrapLon(d float64) float64 {
	for d <= -180 {
		d += 360
	}
	for d > 180 {
		d -= 360
	}
	return d
}

// sphRingLocate tells where p is relative to a ring of great-circle arcs:
// a ray from p north along its meridian crosses the ring an odd number of
// times when p is inside.
func sphRingLocate(p geoPoint, ring []geoPoint) int {
	in := false
	for i := 0; i+1 < len(ring); i++ {
		a, b := ring[i], ring[i+1]
		if sphOnArc(p, a, b) {
			return geoBoundary
		}
		da, db := wrapLon(a.x-p.x), wrapLon(b.x-p.x)
		if (da > 0) == (db > 0) || math.Abs(da-db) >= 180 {
			continue
		}
		n := toVec(a).cross(toVec(b))
		if n.z == 0 {
			continue
		}
		lon := p.x * math.Pi / 180
		lat := math.Atan(-(n.x*math.Cos(lon)+n.y*math.Sin(lon))/n.z) * 180 / math.Pi
		if lat > p.y {
			in = !in
		}
	}
	if in {
		return geoInside
	}
	return geoOutside
}

// sphSplitParams are the fractions along arc ab where it meets s's rings.
func (s *geoShape) sphSplitParams(a, b geoPoint) []float64 {
	A, B := toVec(a), toVec(b)
	total := angleBetween(A, B)
	ts := []float64{0, 1}
	if total == 0 {
		return ts
	}
	at := func(x vec3) {
		if t := angleBetween(A, x) / total; t > 0 && t < 1 {
			ts = append(ts, t)
		}
	}
	for _, r := range s.rings {
		for i := 0; i+1 < len(r); i++ {
			c, d := r[i], r[i+1]
			if x, ok := sphCrossing(a, b, c, d); ok {
				at(x)
			}
			for _, q := range []geoPoint{c, d} {
				if sphOnArc(q, a, b) {
					at(toVec(q))
				}
			}
		}
	}
	return ts
}

// slerp is the point a fraction t along the arc from a to b.
func slerp(a, b geoPoint, t float64) geoPoint {
	A, B := toVec(a), toVec(b)
	w := angleBetween(A, B)
	if w == 0 {
		return a
	}
	s := math.Sin(w)
	return toLonLat(A.scale(math.Sin((1-t)*w) / s).add(B.scale(math.Sin(t*w) / s)))
}
