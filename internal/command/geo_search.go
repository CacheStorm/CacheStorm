package command

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// This file ports Redis' georadiusGeneric (geo.c) and the geohash helpers it
// uses (geohash.c, geohash_helper.c): a search scans the sorted set by the
// score ranges of the geohash box around the center and its neighbors, in
// Redis' order, so unsorted replies and ANY pick the same members.

const (
	geoRadiusCoords = 1 << iota
	geoRadiusMember
	geoRadiusNoStore
	geoSearchVariant
	geoSearchStoreVariant
)

const (
	geoEarthRadius = 6372797.560856
	geoMercatorMax = 20037726.37
	geoDegToRad    = math.Pi / 180.0
	geoStepMax     = 26
	geoLongMin     = -180.0
	geoLongMax     = 180.0
	geoSortNone    = 0
	geoSortAsc     = 1
	geoSortDesc    = 2
	geoShapeRadius = 0
	geoShapeBox    = 1
)

type geoHashBits struct {
	bits uint64
	step uint
}

func (h geoHashBits) zero() bool { return h.bits == 0 && h.step == 0 }

type geoArea struct {
	hash           geoHashBits
	lonMin, lonMax float64
	latMin, latMax float64
}

type geoShape struct {
	kind          int
	lon, lat      float64
	conversion    float64
	radius        float64
	width, height float64
}

func geoInterleave64(lat, lon uint32) uint64 {
	var out uint64
	for bit := uint(0); bit < 32; bit++ {
		out |= uint64((lat>>bit)&1) << (2 * bit)
		out |= uint64((lon>>bit)&1) << (2*bit + 1)
	}
	return out
}

func geoDeinterleave64(bits uint64) (lat, lon uint32) {
	for bit := uint(0); bit < 32; bit++ {
		lat |= uint32((bits>>(2*bit))&1) << bit
		lon |= uint32((bits>>(2*bit+1))&1) << bit
	}
	return lat, lon
}

// geoEncode is geohashEncode over the WGS84 ranges.
func geoEncode(lon, lat float64, step uint) (geoHashBits, bool) {
	if lon > geoLongMax || lon < geoLongMin || lat > store.GeoLatitudeLimit || lat < -store.GeoLatitudeLimit {
		return geoHashBits{}, false
	}
	latOffset := (lat - -store.GeoLatitudeLimit) / (store.GeoLatitudeLimit - -store.GeoLatitudeLimit)
	lonOffset := (lon - geoLongMin) / (geoLongMax - geoLongMin)
	latOffset *= float64(uint64(1) << step)
	lonOffset *= float64(uint64(1) << step)
	return geoHashBits{bits: geoInterleave64(uint32(latOffset), uint32(lonOffset)), step: step}, true
}

// geoDecode is geohashDecode: the area a hash covers.
func geoDecode(h geoHashBits) geoArea {
	ilat, ilon := geoDeinterleave64(h.bits)
	latScale := store.GeoLatitudeLimit - -store.GeoLatitudeLimit
	lonScale := geoLongMax - geoLongMin
	div := float64(uint64(1) << h.step)
	return geoArea{
		hash:   h,
		latMin: -store.GeoLatitudeLimit + (float64(ilat)*1.0/div)*latScale,
		latMax: -store.GeoLatitudeLimit + ((float64(ilat)+1)*1.0/div)*latScale,
		lonMin: geoLongMin + (float64(ilon)*1.0/div)*lonScale,
		lonMax: geoLongMin + ((float64(ilon)+1)*1.0/div)*lonScale,
	}
}

func geoMoveX(h *geoHashBits, d int) {
	x := h.bits & 0xaaaaaaaaaaaaaaaa
	y := h.bits & 0x5555555555555555
	zz := uint64(0x5555555555555555) >> (64 - h.step*2)
	if d > 0 {
		x += zz + 1
	} else {
		x |= zz
		x -= zz + 1
	}
	x &= uint64(0xaaaaaaaaaaaaaaaa) >> (64 - h.step*2)
	h.bits = x | y
}

func geoMoveY(h *geoHashBits, d int) {
	x := h.bits & 0xaaaaaaaaaaaaaaaa
	y := h.bits & 0x5555555555555555
	zz := uint64(0xaaaaaaaaaaaaaaaa) >> (64 - h.step*2)
	if d > 0 {
		y += zz + 1
	} else {
		y |= zz
		y -= zz + 1
	}
	y &= uint64(0x5555555555555555) >> (64 - h.step*2)
	h.bits = x | y
}

// geoNeighbors returns the hash's neighbors in the order Redis scans them:
// north, south, east, west, north-east, north-west, south-east, south-west.
func geoNeighbors(h geoHashBits) [8]geoHashBits {
	moved := func(dx, dy int) geoHashBits {
		n := h
		if dx != 0 {
			geoMoveX(&n, dx)
		}
		if dy != 0 {
			geoMoveY(&n, dy)
		}
		return n
	}
	return [8]geoHashBits{moved(0, 1), moved(0, -1), moved(1, 0), moved(-1, 0), moved(1, 1), moved(-1, 1), moved(1, -1), moved(-1, -1)}
}

func geoEstimateSteps(rangeMeters, lat float64) uint {
	if rangeMeters == 0 {
		return geoStepMax
	}
	step := 1
	for rangeMeters < geoMercatorMax {
		rangeMeters *= 2
		step++
	}
	step -= 2
	if lat > 66 || lat < -66 {
		step--
		if lat > 80 || lat < -80 {
			step--
		}
	}
	return uint(min(max(step, 1), geoStepMax))
}

// geoAreas is geohashCalculateAreasByShapeWGS84: the center box and its
// neighbors, with the ones the shape cannot reach zeroed.
func geoAreas(s *geoShape) [9]geoHashBits {
	height, width := s.radius, s.radius
	if s.kind == geoShapeBox {
		height, width = s.height/2, s.width/2
	}
	height *= s.conversion
	width *= s.conversion
	latDelta := height / geoEarthRadius / geoDegToRad
	lonDeltaTop := width / geoEarthRadius / math.Cos((s.lat+latDelta)*geoDegToRad) / geoDegToRad
	lonDeltaBottom := width / geoEarthRadius / math.Cos((s.lat-latDelta)*geoDegToRad) / geoDegToRad
	minLon, maxLon := s.lon-lonDeltaTop, s.lon+lonDeltaTop
	if s.lat < 0 {
		minLon, maxLon = s.lon-lonDeltaBottom, s.lon+lonDeltaBottom
	}
	minLat, maxLat := s.lat-latDelta, s.lat+latDelta

	radiusMeters := s.radius
	if s.kind == geoShapeBox {
		radiusMeters = math.Sqrt((s.width/2)*(s.width/2) + (s.height/2)*(s.height/2))
	}
	radiusMeters *= s.conversion
	steps := geoEstimateSteps(radiusMeters, s.lat)

	hash, _ := geoEncode(s.lon, s.lat, steps)
	neighbors := geoNeighbors(hash)
	area := geoDecode(hash)
	north, south, east, west := geoDecode(neighbors[0]), geoDecode(neighbors[1]), geoDecode(neighbors[2]), geoDecode(neighbors[3])
	if steps > 1 && (north.latMax < maxLat || south.latMin > minLat || east.lonMax < maxLon || west.lonMin > minLon) {
		steps--
		hash, _ = geoEncode(s.lon, s.lat, steps)
		neighbors = geoNeighbors(hash)
		area = geoDecode(hash)
	}
	if steps >= 2 {
		const n, so, e, w, ne, nw, se, sw = 0, 1, 2, 3, 4, 5, 6, 7
		if area.latMin < minLat {
			neighbors[so], neighbors[sw], neighbors[se] = geoHashBits{}, geoHashBits{}, geoHashBits{}
		}
		if area.latMax > maxLat {
			neighbors[n], neighbors[ne], neighbors[nw] = geoHashBits{}, geoHashBits{}, geoHashBits{}
		}
		if area.lonMin < minLon {
			neighbors[w], neighbors[sw], neighbors[nw] = geoHashBits{}, geoHashBits{}, geoHashBits{}
		}
		if area.lonMax > maxLon {
			neighbors[e], neighbors[se], neighbors[ne] = geoHashBits{}, geoHashBits{}, geoHashBits{}
		}
	}
	var out [9]geoHashBits
	out[0] = hash
	copy(out[1:], neighbors[:])
	return out
}

func geoAlign52(h geoHashBits) float64 {
	return float64(h.bits << (52 - h.step*2))
}

func geoLatDistance(lat1, lat2 float64) float64 {
	return geoEarthRadius * math.Abs(lat2*geoDegToRad-lat1*geoDegToRad)
}

// geoContains reports whether point is inside the shape, and its distance
// from the center in meters, as geoAppendIfWithinShape decides.
func (s *geoShape) contains(p store.GeoPoint) (float64, bool) {
	center := store.GeoPoint{Lon: s.lon, Lat: s.lat}
	if s.kind == geoShapeRadius {
		d := store.GeoDistanceMeters(center, p)
		return d, d <= s.radius*s.conversion
	}
	if geoLatDistance(p.Lat, s.lat) > s.height*s.conversion/2 {
		return 0, false
	}
	if store.GeoDistanceMeters(p, store.GeoPoint{Lon: s.lon, Lat: p.Lat}) > s.width*s.conversion/2 {
		return 0, false
	}
	return store.GeoDistanceMeters(center, p), true
}

type geoHit struct {
	member string
	score  float64
	dist   float64
	point  store.GeoPoint
}

// geoMembersInAreas is membersOfAllNeighbors: the members inside the shape,
// box by box, stopping once limit were found when limit is not 0.
func geoMembersInAreas(z *store.SortedSetValue, areas *[9]geoHashBits, s *geoShape, limit int) []geoHit {
	var hits []geoHit
	last := 0
	for i, box := range areas {
		if box.zero() {
			continue
		}
		if last != 0 && box == areas[last] {
			continue
		}
		if len(hits) > 0 && limit > 0 && len(hits) >= limit {
			break
		}
		lo := geoAlign52(box)
		box.bits++
		hi := geoAlign52(box)
		for _, e := range z.GetByScoreRange(lo, false, hi, true, false) {
			p := store.GeoPointFromScore(e.Score)
			d, ok := s.contains(p)
			if !ok {
				continue
			}
			hits = append(hits, geoHit{member: e.Member, score: e.Score, dist: d, point: p})
			if limit > 0 && len(hits) >= limit {
				break
			}
		}
		last = i
	}
	return hits
}

// geoSearchZSet returns the sorted set a geo command reads at key, or nil.
func geoSearchZSet(ctx *Context, key string) (*store.SortedSetValue, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil
	}
	switch v := entry.Value.(type) {
	case *store.SortedSetValue:
		return v, nil
	case *store.GeoValue:
		z, _ := v.SortedSet()
		return z, nil
	}
	return nil, store.ErrWrongType
}

func geoParseFloat(arg string) (float64, bool) {
	f, err := strconv.ParseFloat(arg, 64)
	if err != nil || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

func geoParseLonLat(ctx *Context, i int, s *geoShape) error {
	lon, ok1 := geoParseFloat(ctx.ArgString(i))
	lat, ok2 := geoParseFloat(ctx.ArgString(i + 1))
	if !ok1 || !ok2 {
		return ErrNotFloat
	}
	if lon < geoLongMin || lon > geoLongMax || lat < -store.GeoLatitudeLimit || lat > store.GeoLatitudeLimit {
		return errors.New("ERR invalid longitude,latitude pair " + geoCoordinateString(lon) + "," + geoCoordinateString(lat))
	}
	s.lon, s.lat = lon, lat
	return nil
}

func geoParseUnit(arg string) (float64, error) {
	scale, ok := geoUnitMeters(strings.ToLower(arg))
	if !ok {
		return 0, errUnsupportedGeoUnit
	}
	return scale, nil
}

func geoParseRadius(ctx *Context, i int, s *geoShape) error {
	r, ok := geoParseFloat(ctx.ArgString(i))
	if !ok {
		return errors.New("ERR need numeric radius")
	}
	if r < 0 {
		return errors.New("ERR radius cannot be negative")
	}
	conv, err := geoParseUnit(ctx.ArgString(i + 1))
	if err != nil {
		return err
	}
	s.kind, s.radius, s.conversion = geoShapeRadius, r, conv
	return nil
}

func geoParseBox(ctx *Context, i int, s *geoShape) error {
	w, ok := geoParseFloat(ctx.ArgString(i))
	if !ok {
		return errors.New("ERR need numeric width")
	}
	h, ok := geoParseFloat(ctx.ArgString(i + 1))
	if !ok {
		return errors.New("ERR need numeric height")
	}
	if h < 0 || w < 0 {
		return errors.New("ERR height or width cannot be negative")
	}
	conv, err := geoParseUnit(ctx.ArgString(i + 2))
	if err != nil {
		return err
	}
	s.kind, s.width, s.height, s.conversion = geoShapeBox, w, h, conv
	return nil
}

func geoMemberPoint(z *store.SortedSetValue, member string, s *geoShape) error {
	z.RLock()
	score, ok := z.GetScore(member)
	z.RUnlock()
	if !ok {
		return errors.New("ERR could not decode requested zset member")
	}
	p := store.GeoPointFromScore(score)
	s.lon, s.lat = p.Lon, p.Lat
	return nil
}

func cmdGEORADIUS(ctx *Context) error {
	flags := geoRadiusCoords
	if ctx.Command == "GEORADIUS_RO" {
		flags |= geoRadiusNoStore
	}
	return geoSearchGeneric(ctx, 0, flags)
}

func cmdGEORADIUSBYMEMBER(ctx *Context) error {
	flags := geoRadiusMember
	if ctx.Command == "GEORADIUSBYMEMBER_RO" {
		flags |= geoRadiusNoStore
	}
	return geoSearchGeneric(ctx, 0, flags)
}

func cmdGEOSEARCH(ctx *Context) error {
	return geoSearchGeneric(ctx, 0, geoSearchVariant)
}

func cmdGEOSEARCHSTORE(ctx *Context) error {
	return geoSearchGeneric(ctx, 1, geoSearchVariant|geoSearchStoreVariant)
}

// geoSearchGeneric is Redis' georadiusGeneric; src is the index of the key
// searched among the arguments.
func geoSearchGeneric(ctx *Context, src int, flags int) error {
	ctx.writeEffectsHandled = true
	ctx.PropagateAs()
	z, err := geoSearchZSet(ctx, ctx.ArgString(src))
	if err != nil {
		return ctx.WriteError(err)
	}

	var shape geoShape
	var base int
	storeKey, hasStore, storeDist := "", false, false
	switch {
	case flags&geoRadiusCoords != 0:
		base = 5
		if err := geoParseLonLat(ctx, 1, &shape); err != nil {
			return ctx.WriteError(err)
		}
		if err := geoParseRadius(ctx, 3, &shape); err != nil {
			return ctx.WriteError(err)
		}
	case flags&geoRadiusMember != 0:
		base = 4
		if z != nil {
			if err := geoMemberPoint(z, ctx.ArgString(1), &shape); err != nil {
				return ctx.WriteError(err)
			}
			if err := geoParseRadius(ctx, 2, &shape); err != nil {
				return ctx.WriteError(err)
			}
		}
	default:
		base = 1
		if flags&geoSearchStoreVariant != 0 {
			base = 2
			storeKey, hasStore = ctx.ArgString(0), true
		}
	}

	withDist, withHash, withCoord := false, false, false
	fromMember, fromLoc, byRadius, byBox := false, false, false, false
	sortOrder, anyLimit, count := geoSortNone, false, int64(0)
	n := ctx.ArgCount()
	for i := base; i < n; i++ {
		arg := strings.ToLower(ctx.ArgString(i))
		remaining := n - i
		switch {
		case arg == "withdist":
			withDist = true
		case arg == "withhash":
			withHash = true
		case arg == "withcoord":
			withCoord = true
		case arg == "any":
			anyLimit = true
		case arg == "asc":
			sortOrder = geoSortAsc
		case arg == "desc":
			sortOrder = geoSortDesc
		case arg == "count" && remaining > 1:
			c, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if c <= 0 {
				return ctx.WriteError(errors.New("ERR COUNT must be > 0"))
			}
			count = c
			i++
		case (arg == "store" || arg == "storedist") && remaining > 1 && flags&(geoRadiusNoStore|geoSearchVariant) == 0:
			storeKey, hasStore, storeDist = ctx.ArgString(i+1), true, arg == "storedist"
			i++
		case arg == "storedist" && flags&geoSearchStoreVariant != 0:
			storeDist = true
		case arg == "frommember" && remaining > 1 && flags&geoSearchVariant != 0 && !fromLoc:
			if z != nil {
				if err := geoMemberPoint(z, ctx.ArgString(i+1), &shape); err != nil {
					return ctx.WriteError(err)
				}
			}
			fromMember = true
			i++
		case arg == "fromlonlat" && remaining > 2 && flags&geoSearchVariant != 0 && !fromMember:
			if err := geoParseLonLat(ctx, i+1, &shape); err != nil {
				return ctx.WriteError(err)
			}
			fromLoc = true
			i += 2
		case arg == "byradius" && remaining > 2 && flags&geoSearchVariant != 0 && !byBox:
			if err := geoParseRadius(ctx, i+1, &shape); err != nil {
				return ctx.WriteError(err)
			}
			byRadius = true
			i += 2
		case arg == "bybox" && remaining > 3 && flags&geoSearchVariant != 0 && !byRadius:
			if err := geoParseBox(ctx, i+1, &shape); err != nil {
				return ctx.WriteError(err)
			}
			byBox = true
			i += 3
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if hasStore && (withDist || withHash || withCoord) {
		what := "STORE option in GEORADIUS"
		if flags&geoSearchStoreVariant != 0 {
			what = "GEOSEARCHSTORE"
		}
		return ctx.WriteError(errors.New("ERR " + what + " is not compatible with WITHDIST, WITHHASH and WITHCOORD options"))
	}
	name := strings.ToLower(ctx.Command)
	if flags&geoSearchVariant != 0 && !fromMember && !fromLoc {
		return ctx.WriteError(errors.New("ERR exactly one of FROMMEMBER or FROMLONLAT can be specified for " + name))
	}
	if flags&geoSearchVariant != 0 && !byRadius && !byBox {
		return ctx.WriteError(errors.New("ERR exactly one of BYRADIUS and BYBOX can be specified for " + name))
	}
	if anyLimit && count == 0 {
		return ctx.WriteError(errors.New("ERR the ANY argument requires COUNT argument"))
	}

	if z == nil {
		if hasStore {
			return publishGeoResult(ctx, storeKey, nil)
		}
		return ctx.WriteArray([]*resp.Value{})
	}
	if count != 0 && sortOrder == geoSortNone && !anyLimit {
		sortOrder = geoSortAsc
	}

	limit := 0
	if anyLimit {
		limit = int(min(count, math.MaxInt32))
	}
	areas := geoAreas(&shape)
	z.RLock()
	hits := geoMembersInAreas(z, &areas, &shape, limit)
	z.RUnlock()

	returned := len(hits)
	if count > 0 && int64(returned) > count {
		returned = int(count)
	}
	if sortOrder != geoSortNone {
		cmp := func(x, y *geoHit) int {
			r := 0
			if x.dist > y.dist {
				r = 1
			} else if x.dist < y.dist {
				r = -1
			}
			if sortOrder == geoSortDesc {
				r = -r
			}
			return r
		}
		// Redis sorts with the C library's qsort, a stable merge sort,
		// when every member is returned, and with pqsort otherwise.
		if returned == len(hits) {
			sort.SliceStable(hits, func(i, j int) bool { return cmp(&hits[i], &hits[j]) < 0 })
		} else {
			geoPartialSort(hits, cmp, 0, returned-1)
		}
	}
	hits = hits[:returned]

	if hasStore {
		if len(hits) == 0 {
			return publishGeoResult(ctx, storeKey, nil)
		}
		dest := &store.SortedSetValue{Members: make(map[string]float64, len(hits))}
		for _, h := range hits {
			score := h.score
			if storeDist {
				score = store.GeoDistanceMetersExact(store.GeoPoint{Lon: shape.lon, Lat: shape.lat}, h.point) / shape.conversion
			}
			dest.Add(h.member, score)
		}
		return publishGeoResult(ctx, storeKey, dest)
	}

	out := make([]*resp.Value, 0, len(hits))
	for _, h := range hits {
		if !withDist && !withHash && !withCoord {
			out = append(out, resp.BulkString(h.member))
			continue
		}
		entry := []*resp.Value{resp.BulkString(h.member)}
		if withDist {
			entry = append(entry, resp.BulkString(strconv.FormatFloat(h.dist/shape.conversion, 'f', 4, 64)))
		}
		if withHash {
			entry = append(entry, resp.IntegerValue(int64(h.score)))
		}
		if withCoord {
			entry = append(entry, resp.ArrayValue([]*resp.Value{
				resp.DoubleValue(geoPositionString(h.point.Lon)),
				resp.DoubleValue(geoPositionString(h.point.Lat)),
			}))
		}
		out = append(out, resp.ArrayValue(entry))
	}
	return ctx.WriteArray(out)
}

// geoPartialSort is Redis' pqsort (a Bentley-McIlroy quicksort that only
// sorts the partitions overlapping positions lo..hi), so ties among the
// returned members are broken as Redis breaks them.
func geoPartialSort(a []geoHit, cmp func(x, y *geoHit) int, lo, hi int) {
	geoPQSort(a, 0, len(a), cmp, lo, hi)
}

func geoMed3(a []geoHit, i, j, k int, cmp func(x, y *geoHit) int) int {
	if cmp(&a[i], &a[j]) < 0 {
		if cmp(&a[j], &a[k]) < 0 {
			return j
		}
		if cmp(&a[i], &a[k]) < 0 {
			return k
		}
		return i
	}
	if cmp(&a[j], &a[k]) > 0 {
		return j
	}
	if cmp(&a[i], &a[k]) < 0 {
		return i
	}
	return k
}

func geoVecSwap(a []geoHit, i, j, n int) {
	for ; n > 0; n-- {
		a[i], a[j] = a[j], a[i]
		i++
		j++
	}
}

func geoPQSort(a []geoHit, base, n int, cmp func(x, y *geoHit) int, lo, hi int) {
	for {
		if n < 7 {
			for pm := base + 1; pm < base+n; pm++ {
				for pl := pm; pl > base && cmp(&a[pl-1], &a[pl]) > 0; pl-- {
					a[pl], a[pl-1] = a[pl-1], a[pl]
				}
			}
			return
		}
		pm := base + n/2
		if n > 7 {
			pl, pn := base, base+n-1
			if n > 40 {
				d := n / 8
				pl = geoMed3(a, pl, pl+d, pl+2*d, cmp)
				pm = geoMed3(a, pm-d, pm, pm+d, cmp)
				pn = geoMed3(a, pn-2*d, pn-d, pn, cmp)
			}
			pm = geoMed3(a, pl, pm, pn, cmp)
		}
		a[base], a[pm] = a[pm], a[base]
		pa, pb := base+1, base+1
		pc, pd := base+n-1, base+n-1
		for {
			for pb <= pc {
				r := cmp(&a[pb], &a[base])
				if r > 0 {
					break
				}
				if r == 0 {
					a[pa], a[pb] = a[pb], a[pa]
					pa++
				}
				pb++
			}
			for pb <= pc {
				r := cmp(&a[pc], &a[base])
				if r < 0 {
					break
				}
				if r == 0 {
					a[pc], a[pd] = a[pd], a[pc]
					pd--
				}
				pc--
			}
			if pb > pc {
				break
			}
			a[pb], a[pc] = a[pc], a[pb]
			pb++
			pc--
		}
		pn := base + n
		r := min(pa-base, pb-pa)
		geoVecSwap(a, base, pb-r, r)
		r = min(pd-pc, pn-pd-1)
		geoVecSwap(a, pb, pn-r, r)
		if r = pb - pa; r > 1 {
			if !geoRangeMisses(lo, hi, base, base+r-1) {
				geoPQSort(a, base, r, cmp, lo, hi)
			}
		}
		r = pd - pc
		if r <= 1 {
			return
		}
		base, n = pn-r, r
		if geoRangeMisses(lo, hi, base, base+r-1) {
			return
		}
	}
}

// geoRangeMisses reports whether positions lo..hi lie entirely outside
// first..last.
func geoRangeMisses(lo, hi, first, last int) bool {
	return (lo < first && hi < first) || (lo > last && hi > last)
}
