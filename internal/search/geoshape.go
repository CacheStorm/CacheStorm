package search

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
)

// GEOSHAPE fields hold a WKT POINT or POLYGON (with holes); queries ask
// whether a document's shape is WITHIN, CONTAINS, INTERSECTS or is
// DISJOINT from a shape given as a parameter, with the OGC meaning
// RediSearch's boost.geometry gives them: touching boundaries intersect,
// a shape on another's boundary only is not within it, and a point is
// neither within nor contains another point. Shapes are compared in the
// plane, FLAT as given and SPHERICAL in longitude and latitude degrees.

type geoPoint struct{ x, y float64 }

// geoShape is a point (rings empty) or a polygon: rings[0] the outer
// ring, the rest holes, each closed (first point repeated last).
type geoShape struct {
	pt    geoPoint
	rings [][]geoPoint
	// sph takes the shape on the sphere (a SPHERICAL field).
	sph bool
}

func (s *geoShape) ringLocate(p geoPoint, ring []geoPoint) int {
	if s.sph {
		return sphRingLocate(p, ring)
	}
	return ringLocate(p, ring)
}

func (s *geoShape) segmentsIntersect(a, b, c, d geoPoint) bool {
	if s.sph {
		return sphSegmentsIntersect(a, b, c, d)
	}
	return segmentsIntersect(a, b, c, d)
}

// along is the point a fraction t along s's edge from a to b.
func (s *geoShape) along(a, b geoPoint, t float64) geoPoint {
	if s.sph {
		return slerp(a, b, t)
	}
	return geoPoint{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t}
}

func (s *geoShape) isPoint() bool { return len(s.rings) == 0 }

// Geoshape predicates, numbered as FT.EXPLAIN prints them.
const (
	geoContains   = 1
	geoWithin     = 2
	geoDisjoint   = 3
	geoIntersects = 4
)

var geoPredicates = map[string]int{"CONTAINS": geoContains, "WITHIN": geoWithin, "DISJOINT": geoDisjoint, "INTERSECTS": geoIntersects}

var errGeometryType = errors.New("Error querying geoshape index: unknown geometry type") //nolint:staticcheck // RediSearch's reply text

// parseWKT reads a WKT POINT or POLYGON.
func parseWKT(s string) (*geoShape, error) {
	t := strings.TrimSpace(s)
	upper := strings.ToUpper(t)
	switch {
	case strings.HasPrefix(upper, "POINT"):
		body, ok := wktBody(t[len("POINT"):])
		if !ok {
			return nil, errGeometryType
		}
		pts, ok := wktPoints(body)
		if !ok || len(pts) != 1 {
			return nil, errGeometryType
		}
		return &geoShape{pt: pts[0]}, nil
	case strings.HasPrefix(upper, "POLYGON"):
		body, ok := wktBody(t[len("POLYGON"):])
		if !ok {
			return nil, errGeometryType
		}
		var rings [][]geoPoint
		for body = strings.TrimSpace(body); body != ""; {
			if body[0] != '(' {
				return nil, errGeometryType
			}
			end := strings.IndexByte(body, ')')
			if end < 0 {
				return nil, errGeometryType
			}
			pts, ok := wktPoints(body[1:end])
			if !ok {
				return nil, errGeometryType
			}
			if len(pts) > 0 && pts[0] != pts[len(pts)-1] {
				pts = append(pts, pts[0]) // closed, as boost.geometry's correct does
			}
			if len(pts) < 4 {
				return nil, errGeometryType
			}
			rings = append(rings, pts)
			body = strings.TrimSpace(body[end+1:])
			body = strings.TrimSpace(strings.TrimPrefix(body, ","))
		}
		if len(rings) == 0 {
			return nil, errGeometryType
		}
		return &geoShape{rings: rings}, nil
	}
	return nil, errGeometryType
}

// wktBody returns what is between a geometry's outer parentheses.
func wktBody(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return "", false
	}
	return s[1 : len(s)-1], true
}

// wktPoints reads "x y, x y, ...".
func wktPoints(s string) ([]geoPoint, bool) {
	var out []geoPoint
	for _, part := range strings.Split(s, ",") {
		f := strings.Fields(part)
		if len(f) != 2 {
			return nil, false
		}
		x, err1 := strconv.ParseFloat(f[0], 64)
		y, err2 := strconv.ParseFloat(f[1], 64)
		if err1 != nil || err2 != nil || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			return nil, false
		}
		out = append(out, geoPoint{x, y})
	}
	return out, len(out) > 0
}

// Where a point lies relative to a shape.
const (
	geoOutside = iota
	geoBoundary
	geoInside
)

func cross(o, a, b geoPoint) float64 { return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x) }

func onSegment(p, a, b geoPoint) bool {
	return cross(a, b, p) == 0 && math.Min(a.x, b.x) <= p.x && p.x <= math.Max(a.x, b.x) &&
		math.Min(a.y, b.y) <= p.y && p.y <= math.Max(a.y, b.y)
}

// ringLocate tells where p is relative to a closed ring.
func ringLocate(p geoPoint, ring []geoPoint) int {
	in := false
	for i := 0; i+1 < len(ring); i++ {
		a, b := ring[i], ring[i+1]
		if onSegment(p, a, b) {
			return geoBoundary
		}
		if (a.y > p.y) != (b.y > p.y) && p.x < (b.x-a.x)*(p.y-a.y)/(b.y-a.y)+a.x {
			in = !in
		}
	}
	if in {
		return geoInside
	}
	return geoOutside
}

// locate tells where p is relative to a polygon: on a ring is the
// boundary, inside a hole is outside.
func (s *geoShape) locate(p geoPoint) int {
	if s.isPoint() {
		if p == s.pt {
			return geoBoundary
		}
		return geoOutside
	}
	switch s.ringLocate(p, s.rings[0]) {
	case geoOutside:
		return geoOutside
	case geoBoundary:
		return geoBoundary
	}
	for _, h := range s.rings[1:] {
		switch s.ringLocate(p, h) {
		case geoInside:
			return geoOutside
		case geoBoundary:
			return geoBoundary
		}
	}
	return geoInside
}

// segmentsIntersect reports whether segments ab and cd share a point.
func segmentsIntersect(a, b, c, d geoPoint) bool {
	d1, d2 := cross(c, d, a), cross(c, d, b)
	d3, d4 := cross(a, b, c), cross(a, b, d)
	if (d1 > 0 && d2 < 0 || d1 < 0 && d2 > 0) && (d3 > 0 && d4 < 0 || d3 < 0 && d4 > 0) {
		return true
	}
	return onSegment(a, c, d) || onSegment(b, c, d) || onSegment(c, a, b) || onSegment(d, a, b)
}

// splitParams are the parameters along ab, from 0 to 1, where ab meets
// the shape's rings: ab's pieces between them lie wholly on one side.
func (s *geoShape) splitParams(a, b geoPoint) []float64 {
	if s.sph {
		ts := s.sphSplitParams(a, b)
		sort.Float64s(ts)
		return ts
	}
	ts := []float64{0, 1}
	for _, r := range s.rings {
		for i := 0; i+1 < len(r); i++ {
			c, d := r[i], r[i+1]
			den := (b.x-a.x)*(d.y-c.y) - (b.y-a.y)*(d.x-c.x)
			if den != 0 {
				t := ((c.x-a.x)*(d.y-c.y) - (c.y-a.y)*(d.x-c.x)) / den
				u := ((c.x-a.x)*(b.y-a.y) - (c.y-a.y)*(b.x-a.x)) / den
				if t > 0 && t < 1 && u >= 0 && u <= 1 {
					ts = append(ts, t)
				}
				continue
			}
			// Collinear: the ring's segment ends split ab.
			for _, q := range []geoPoint{c, d} {
				if onSegment(q, a, b) {
					if t := segParam(a, b, q); t > 0 && t < 1 {
						ts = append(ts, t)
					}
				}
			}
		}
	}
	sort.Float64s(ts)
	return ts
}

func segParam(a, b, q geoPoint) float64 {
	if math.Abs(b.x-a.x) >= math.Abs(b.y-a.y) {
		return (q.x - a.x) / (b.x - a.x)
	}
	return (q.y - a.y) / (b.y - a.y)
}

// pieces calls fn with a point inside each piece of a's rings, split
// where they meet other's rings; fn returning false stops.
func (s *geoShape) pieces(other *geoShape, fn func(geoPoint) bool) bool {
	for _, r := range s.rings {
		for i := 0; i+1 < len(r); i++ {
			a, b := r[i], r[i+1]
			ts := other.splitParams(a, b)
			for j := 0; j+1 < len(ts); j++ {
				if ts[j+1]-ts[j] <= 0 {
					continue
				}
				m := (ts[j] + ts[j+1]) / 2
				if !fn(s.along(a, b, m)) {
					return false
				}
			}
		}
	}
	return true
}

func (s *geoShape) intersects(o *geoShape) bool {
	switch {
	case s.isPoint():
		return o.locate(s.pt) != geoOutside
	case o.isPoint():
		return s.locate(o.pt) != geoOutside
	}
	for _, r := range s.rings {
		for i := 0; i+1 < len(r); i++ {
			for _, q := range o.rings {
				for j := 0; j+1 < len(q); j++ {
					if s.segmentsIntersect(r[i], r[i+1], q[j], q[j+1]) {
						return true
					}
				}
			}
		}
	}
	return o.locate(s.rings[0][0]) != geoOutside || s.locate(o.rings[0][0]) != geoOutside
}

// within reports whether s lies in o's closure with their interiors
// meeting.
func (s *geoShape) within(o *geoShape) bool {
	if o.isPoint() {
		return false
	}
	if s.isPoint() {
		return o.locate(s.pt) == geoInside
	}
	// Every piece of s's rings is in o (inside or on its boundary)...
	for _, r := range s.rings {
		for _, p := range r {
			if o.locate(p) == geoOutside {
				return false
			}
		}
	}
	if !s.pieces(o, func(p geoPoint) bool { return o.locate(p) != geoOutside }) {
		return false
	}
	// ...and no piece of o's boundary (its outside, or a hole) runs
	// through s's interior.
	if !o.pieces(s, func(p geoPoint) bool { return s.locate(p) != geoInside }) {
		return false
	}
	for _, r := range o.rings {
		for _, p := range r {
			if s.locate(p) == geoInside {
				return false
			}
		}
	}
	return true
}

// geoshapeMatch evaluates a predicate of a document's shape against the
// query's.
func geoshapeMatch(pred int, doc, q *geoShape) bool {
	switch pred {
	case geoWithin:
		return doc.within(q)
	case geoContains:
		return q.within(doc)
	case geoIntersects:
		return doc.intersects(q)
	case geoDisjoint:
		return !doc.intersects(q)
	}
	return false
}

// prepareShapes parses the query shapes of n's geoshape nodes, in their
// fields' coordinate systems.
func (idx *Index) prepareShapes(n *qnode) error {
	if n == nil {
		return nil
	}
	if n.typ == qGeoShape && n.shape == nil {
		s, err := parseWKT(n.str)
		if err != nil {
			return err
		}
		if f := idx.Field(n.field); f != nil && f.CoordSystem == "SPHERICAL" {
			s.sph = true
		}
		if why := s.invalidity(); why != "" {
			return errors.New("Error querying geoshape index: invalid geometry: " + why) //nolint:staticcheck // RediSearch's reply text
		}
		n.shape = s
	}
	for _, c := range n.children {
		if err := idx.prepareShapes(c); err != nil {
			return err
		}
	}
	return nil
}

// valid reports whether a polygon is valid as boost.geometry's is_valid
// requires before RediSearch indexes it.
func (s *geoShape) valid() bool { return s.invalidity() == "" }

// invalidity is why boost.geometry's is_valid refuses a polygon, in its
// words, checked in its order: too few points, too few distinct points, a
// ring folding back on itself (a spike), rings crossing, holes outside the
// outer ring; "" for a valid shape.
func (s *geoShape) invalidity() string {
	if s.isPoint() {
		return ""
	}
	for k, raw := range s.rings {
		if len(raw) < 4 {
			return "Geometry has too few points"
		}
		r := dedupe(raw)
		if len(r) < 4 {
			return "Geometry has wrong topological dimension"
		}
		// boost.geometry's correct first: an outer ring clockwise, holes
		// counter-clockwise, which numbers the segments it reports.
		if a := s.area(r); k == 0 && a > 0 || k > 0 && a < 0 {
			r = reversed(r)
		}
		n := len(r) - 1 // segments
		// A spike: the ring turns back on itself at an apex, looked for
		// from the second point, the first last.
		for step := 1; step <= n; step++ {
			i := step % n
			prev, apex, next := r[(i+n-1)%n], r[i], r[i+1]
			if s.spike(prev, apex, next) {
				return "Geometry has spikes. A spike point was found with apex at (" + gfmt(apex.x) + ", " + gfmt(apex.y) + ")"
			}
		}
		if s.area(r) == 0 {
			// No net area left after correct: a ring that crosses
			// itself into equal and opposite lobes.
			return "Geometry has wrong orientation"
		}
		// Rings crossing: boost numbers the segments of the corrected
		// ring as given, repeated points (empty segments) included.
		cr := raw
		if a := s.area(raw); k == 0 && a > 0 || k > 0 && a < 0 {
			cr = reversed(raw)
		}
		m := len(cr) - 1
		// neighbors reports whether only empty segments separate i and j.
		neighbors := func(i, j int) bool {
			for x := i + 1; x < j; x++ {
				if cr[x] != cr[x+1] {
					goto wrap
				}
			}
			return true
		wrap:
			for x := (j + 1) % m; x != i; x = (x + 1) % m {
				if cr[x] != cr[x+1] {
					return false
				}
			}
			return true
		}
		for i := 0; i < m; i++ {
			if cr[i] == cr[i+1] {
				continue
			}
			for j := i + 1; j < m; j++ {
				if cr[j] == cr[j+1] || neighbors(i, j) {
					continue
				}
				a, b, c, d := cr[i], cr[i+1], cr[j], cr[j+1]
				if s.segmentsIntersect(a, b, c, d) {
					x, y := s.crossingPoint(a, b, c, d)
					// How the segments meet: crossing inside both (i), an
					// end of one inside the other (m), ends of both (t).
					method := "i"
					endA := s.onSegment(a, c, d) || s.onSegment(b, c, d)
					endC := s.onSegment(c, a, b) || s.onSegment(d, a, b)
					switch {
					case endA && endC:
						method = "t"
					case endA || endC:
						method = "m"
					}
					for _, q := range []geoPoint{a, b, c, d} {
						if (q == a || q == b) && s.onSegment(q, c, d) || (q == c || q == d) && s.onSegment(q, a, b) {
							x, y = q.x, q.y
							break
						}
					}
					ops := "u/i"
					if s.leftOf(a, b, d) {
						ops = "i/u"
					}
					return "Geometry has invalid self-intersections. A self-intersection point was found at (" + gfmt(x) + ", " + gfmt(y) +
						"); method: " + method + "; operations: " + ops + "; segment IDs {source, multi, ring, segment}: {0, -1, " + strconv.Itoa(k-1) + ", " + strconv.Itoa(i) +
						"}/{0, -1, " + strconv.Itoa(k-1) + ", " + strconv.Itoa(j) + "}"
				}
			}
		}
		if k > 0 {
			outer := &geoShape{rings: s.rings[:1], sph: s.sph}
			for _, p := range r {
				if outer.locate(p) == geoOutside {
					return "Geometry has interior rings defined outside the outer boundary"
				}
			}
		}
	}
	return ""
}

// spike reports whether the ring turns back on itself at apex.
func (s *geoShape) spike(prev, apex, next geoPoint) bool {
	if !s.sph {
		return cross(apex, prev, next) == 0 && (prev.x-apex.x)*(next.x-apex.x)+(prev.y-apex.y)*(next.y-apex.y) > 0
	}
	P, A, N := toVec(prev), toVec(apex), toVec(next)
	if math.Abs(A.cross(P).unit().dot(N)) > sphEps {
		return false
	}
	// The directions from the apex to its neighbours agree.
	tp, tn := P.add(A.scale(-A.dot(P))), N.add(A.scale(-A.dot(N)))
	return tp.dot(tn) > 0
}

func (s *geoShape) onSegment(p, a, b geoPoint) bool {
	if s.sph {
		return sphOnArc(p, a, b)
	}
	return onSegment(p, a, b)
}

// leftOf reports whether d is left of the edge from a to b.
func (s *geoShape) leftOf(a, b, d geoPoint) bool {
	if s.sph {
		return toVec(a).cross(toVec(b)).dot(toVec(d)) > 0
	}
	return cross(a, b, d) > 0
}

func (s *geoShape) crossingPoint(a, b, c, d geoPoint) (float64, float64) {
	if s.sph {
		if x, ok := sphCrossing(a, b, c, d); ok {
			p := toLonLat(x)
			return p.x, p.y
		}
		return c.x, c.y
	}
	return crossingPoint(a, b, c, d)
}

// crossingPoint is where segments ab and cd cross (their lines meet).
func crossingPoint(a, b, c, d geoPoint) (float64, float64) {
	den := (b.x-a.x)*(d.y-c.y) - (b.y-a.y)*(d.x-c.x)
	if den == 0 {
		return c.x, c.y
	}
	t := ((c.x-a.x)*(d.y-c.y) - (c.y-a.y)*(d.x-c.x)) / den
	return a.x + t*(b.x-a.x), a.y + t*(b.y-a.y)
}

// gfmt prints a coordinate as a C++ stream does by default.
func gfmt(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }

func reversed(r []geoPoint) []geoPoint {
	out := make([]geoPoint, len(r))
	for i, p := range r {
		out[len(r)-1-i] = p
	}
	return out
}

func dedupe(r []geoPoint) []geoPoint {
	out := make([]geoPoint, 0, len(r))
	for i, p := range r {
		if i == 0 || p != r[i-1] {
			out = append(out, p)
		}
	}
	return out
}

// area is a ring's signed area, positive counter-clockwise: on the
// sphere the sum of each edge's band, in square degrees' sign only.
func (s *geoShape) area(r []geoPoint) float64 {
	if !s.sph {
		return ringArea(r)
	}
	a := 0.0
	for i := 0; i+1 < len(r); i++ {
		dl := wrapLon(r[i+1].x-r[i].x) * math.Pi / 180
		a -= dl * (2 + math.Sin(r[i].y*math.Pi/180) + math.Sin(r[i+1].y*math.Pi/180)) / 2
	}
	if math.Abs(a) < 1e-15 {
		return 0
	}
	return a
}

func ringArea(r []geoPoint) float64 {
	a := 0.0
	for i := 0; i+1 < len(r); i++ {
		a += r[i].x*r[i+1].y - r[i+1].x*r[i].y
	}
	return a / 2
}
