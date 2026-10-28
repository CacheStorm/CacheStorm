package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var errUnsupportedGeoUnit = errors.New("ERR unsupported unit provided. please use M, KM, FT, MI")

func geoUnitValid(unit string) bool {
	switch unit {
	case "m", "km", "ft", "mi":
		return true
	}
	return false
}

func RegisterGeoCommands(router *Router) {
	router.Register(&CommandDef{Name: "GEOADD", Handler: cmdGEOADD})
	router.Register(&CommandDef{Name: "GEODIST", Handler: cmdGEODIST})
	router.Register(&CommandDef{Name: "GEOHASH", Handler: cmdGEOHASH})
	router.Register(&CommandDef{Name: "GEOPOS", Handler: cmdGEOPOS})
	router.Register(&CommandDef{Name: "GEORADIUS", Handler: cmdGEORADIUS})
	router.Register(&CommandDef{Name: "GEORADIUSBYMEMBER", Handler: cmdGEORADIUSBYMEMBER})
	router.Register(&CommandDef{Name: "GEORADIUS_RO", Handler: cmdGEORADIUS})
	router.Register(&CommandDef{Name: "GEORADIUSBYMEMBER_RO", Handler: cmdGEORADIUSBYMEMBER})
	router.Register(&CommandDef{Name: "GEOSEARCH", Handler: cmdGEOSEARCH})
	router.Register(&CommandDef{Name: "GEOSEARCHSTORE", Handler: cmdGEOSEARCHSTORE})
}

type geoReadValue struct {
	*store.GeoValue
	scores map[string]float64
}

func getGeo(ctx *Context, key string) (*geoReadValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	result := &geoReadValue{GeoValue: store.NewGeoValue(), scores: make(map[string]float64)}
	switch value := entry.Value.(type) {
	case *store.GeoValue:
		result.GeoValue = value
		value.RLock()
		for member, point := range value.Points {
			result.scores[member] = store.GeoScore(point.Lon, point.Lat)
		}
		value.RUnlock()
	case *store.SortedSetValue:
		value.RLock()
		for member, score := range value.Members {
			result.Points[member] = store.GeoPointFromScore(score)
			result.scores[member] = score
		}
		value.RUnlock()
	default:
		return nil, store.ErrWrongType
	}
	return result, nil
}

func cmdGEOADD(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	nx := false
	xx := false
	ch := false
	argIdx := 1

options:
	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		switch arg {
		case "NX":
			nx = true
			argIdx++
		case "XX":
			xx = true
			argIdx++
		case "CH":
			ch = true
			argIdx++
		default:
			break options
		}
	}

	remaining := ctx.ArgCount() - argIdx
	if remaining < 3 || remaining%3 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	historical := ctx.Replaying || ctx.LegacyReplay
	if nx && xx && !historical {
		return ctx.WriteError(ErrSyntaxError)
	}
	if historical && xx && !ctx.Store.Exists(key) {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	points := make([]store.GeoPoint, 0, remaining/3)
	for i := argIdx; i < ctx.ArgCount(); i += 3 {
		lon, err1 := strconv.ParseFloat(ctx.ArgString(i), 64)
		lat, err2 := strconv.ParseFloat(ctx.ArgString(i+1), 64)
		if err1 != nil || err2 != nil || (!historical && (math.IsNaN(lon) || math.IsNaN(lat))) {
			return ctx.WriteError(ErrNotFloat)
		}
		if lon < -180 || lon > 180 || lat < -85.05112878 || lat > 85.05112878 {
			return ctx.WriteError(fmt.Errorf("ERR invalid longitude,latitude pair %s,%s", geoCoordinateString(lon), geoCoordinateString(lat)))
		}
		points = append(points, store.GeoPoint{Lon: lon, Lat: lat})
	}
	added, changed := 0, 0
	nonfinite := false
	for _, point := range points {
		nonfinite = nonfinite || math.IsNaN(point.Lon) || math.IsNaN(point.Lat)
	}
	if entry, ok := ctx.Store.Get(key); ok {
		if legacy, ok := entry.Value.(*store.GeoValue); ok {
			_, valid := legacy.SortedSet()
			nonfinite = nonfinite || !valid
		}
	}
	if nonfinite && historical {
		var err error
		added, changed, err = applyHistoricalGeoAdd(ctx, key, points, argIdx, nx, xx)
		if err != nil {
			return ctx.WriteError(err)
		}
	} else {
		if nonfinite {
			return ctx.WriteError(ErrNotFloat)
		}
		err := ctx.Store.ApplyValuePlan(key, func(value store.Value) (store.ValuePlan, error) {
			added, changed = 0, 0
			var zset *store.SortedSetValue
			switch current := value.(type) {
			case nil:
			case *store.SortedSetValue:
				zset = current
			case *store.GeoValue:
				var ok bool
				zset, ok = current.SortedSet()
				if !ok {
					return store.ValuePlan{}, ErrNotFloat
				}
			default:
				return store.ValuePlan{}, store.ErrWrongType
			}
			if zset == nil && xx {
				return store.ValuePlan{}, nil
			}
			size := int64(48)
			if zset != nil {
				size = zset.SizeOf()
				zset.RLock()
				defer zset.RUnlock()
			}
			updates := make(map[string]float64, len(points))
			for i, point := range points {
				member := ctx.ArgString(argIdx + i*3 + 2)
				score := store.GeoScore(point.Lon, point.Lat)
				existing, exists := updates[member]
				if !exists && zset != nil {
					existing, exists = zset.Members[member]
				}
				if (xx && !exists) || (nx && exists) {
					continue
				}
				if exists {
					if existing == score {
						continue
					}
					changed++
				} else {
					added++
					size += int64(len(member)) + 96
				}
				updates[member] = score
			}
			if added+changed == 0 {
				return store.ValuePlan{}, nil
			}
			return store.ValuePlan{Size: size, Apply: func() store.Value {
				result := &store.SortedSetValue{Members: make(map[string]float64)}
				if zset != nil {
					zset.RLock()
					for member, score := range zset.Members {
						result.Add(member, score)
					}
					zset.RUnlock()
				}
				for member, score := range updates {
					result.Add(member, score)
				}
				return result
			}}, nil
		})
		if err != nil {
			return ctx.WriteError(err)
		}
	}
	if added+changed == 0 {
		ctx.PropagateAs()
	} else {
		ctx.notify(store.NotifyZSet, "zadd", key)
	}

	if ch {
		return ctx.WriteInteger(int64(added + changed))
	}
	return ctx.WriteInteger(int64(added))
}

func applyHistoricalGeoAdd(ctx *Context, key string, points []store.GeoPoint, argIdx int, nx, xx bool) (int, int, error) {
	added, changed := 0, 0
	err := ctx.Store.ApplyValuePlan(key, func(value store.Value) (store.ValuePlan, error) {
		added, changed = 0, 0
		var geo *store.GeoValue
		if value != nil {
			switch current := value.(type) {
			case *store.GeoValue:
				geo = current
			case *store.SortedSetValue:
				geo = store.NewGeoValue()
				current.RLock()
				for member, score := range current.Members {
					point := store.GeoPointFromScore(score)
					geo.Add(member, point.Lon, point.Lat)
				}
				current.RUnlock()
			default:
				return store.ValuePlan{}, store.ErrWrongType
			}
		}
		if geo == nil && xx {
			return store.ValuePlan{}, nil
		}
		size := int64(48)
		if geo != nil {
			size = geo.SizeOf()
			geo.RLock()
			defer geo.RUnlock()
		}
		updates := make(map[string]store.GeoPoint, len(points))
		for i, point := range points {
			member := ctx.ArgString(argIdx + i*3 + 2)
			existing, exists := updates[member]
			if !exists && geo != nil {
				existing, exists = geo.Points[member]
			}
			if (xx && !exists) || (nx && exists) {
				continue
			}
			if exists {
				if existing == point {
					continue
				}
				changed++
			} else {
				added++
				size += int64(len(member)) + 32
			}
			updates[member] = point
		}
		if added+changed == 0 {
			return store.ValuePlan{}, nil
		}
		return store.ValuePlan{Size: size, Apply: func() store.Value {
			next := store.NewGeoValue()
			if geo != nil {
				geo.RLock()
				for member, point := range geo.Points {
					next.Points[member] = point
				}
				geo.RUnlock()
			}
			for member, point := range updates {
				next.Points[member] = point
			}
			return next
		}}, nil
	})
	return added, changed, err
}

func geoCoordinateString(value float64) string {
	if math.IsInf(value, 1) {
		return "inf"
	}
	if math.IsInf(value, -1) {
		return "-inf"
	}
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func cmdGEODIST(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if ctx.ArgCount() > 4 {
		return ctx.WriteError(ErrSyntaxError)
	}
	unit := "m"
	if ctx.ArgCount() == 4 {
		unit = strings.ToLower(ctx.ArgString(3))
	}
	scale, ok := geoUnitMeters(unit)
	if !ok {
		return ctx.WriteError(errUnsupportedGeoUnit)
	}
	geo, err := getGeo(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if geo == nil {
		return ctx.WriteNull()
	}
	a, okA := geo.Get(ctx.ArgString(1))
	b, okB := geo.Get(ctx.ArgString(2))
	if !okA || !okB {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(strconv.FormatFloat(store.GeoDistanceMeters(a, b)/scale, 'f', 4, 64))
}
func geoUnitMeters(unit string) (float64, bool) {
	switch unit {
	case "m":
		return 1, true
	case "km":
		return 1000, true
	case "mi":
		return 1609.34, true
	case "ft":
		return 0.3048, true
	}
	return 0, false
}

func cmdGEOHASH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	geo, err := getGeo(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if geo == nil {
		results := make([]*resp.Value, ctx.ArgCount()-1)
		for i := range results {
			results[i] = resp.NullValue()
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		point, exists := geo.Get(member)
		if !exists {
			results = append(results, resp.NullValue())
		} else {
			hash := store.GeoHashString(point)
			results = append(results, resp.BulkString(hash))
		}
	}

	return ctx.WriteArray(results)
}

func cmdGEOPOS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	geo, err := getGeo(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if geo == nil {
		results := make([]*resp.Value, ctx.ArgCount()-1)
		for i := range results {
			results[i] = resp.NullArray()
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		point, exists := geo.Get(member)
		if !exists {
			results = append(results, resp.NullArray())
		} else {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(geoPositionString(point.Lon)),
				resp.BulkString(geoPositionString(point.Lat)),
			}))
		}
	}

	return ctx.WriteArray(results)
}

func cmdGEORADIUS(ctx *Context) error {
	ctx.writeEffectsHandled = true
	ctx.PropagateAs()
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	geo, err := getGeo(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	lon, err1 := strconv.ParseFloat(ctx.ArgString(1), 64)
	lat, err2 := strconv.ParseFloat(ctx.ArgString(2), 64)
	radius, err3 := strconv.ParseFloat(ctx.ArgString(3), 64)
	if err1 != nil || err2 != nil || math.IsNaN(lon) || math.IsNaN(lat) {
		return ctx.WriteError(ErrNotFloat)
	}
	if err3 != nil || math.IsNaN(radius) {
		return ctx.WriteError(errors.New("ERR need numeric radius"))
	}

	if radius < 0 {
		return ctx.WriteError(errors.New("ERR radius cannot be negative"))
	}
	if lon < -180 || lon > 180 || lat < -store.GeoLatitudeLimit || lat > store.GeoLatitudeLimit {
		return ctx.WriteError(fmt.Errorf("ERR invalid longitude,latitude pair %s,%s", geoCoordinateString(lon), geoCoordinateString(lat)))
	}
	unit := strings.ToLower(ctx.ArgString(4))
	radiusKm := radius
	switch unit {
	case "km":
	case "mi":
		radiusKm = radius * 1.60934
	case "ft":
		radiusKm = radius * 0.0003048
	case "m":
		radiusKm = radius / 1000
	default:
		return ctx.WriteError(errUnsupportedGeoUnit)
	}

	withCoord := false
	withDist := false
	withHash := false
	count := 0
	anyCount := false
	sortOrder := ""
	storeKey := ""
	storeDistKey := ""

	for i := 5; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "WITHCOORD":
			withCoord = true
		case "WITHDIST":
			withDist = true
		case "WITHHASH":
			withHash = true
		case "ANY":
			anyCount = true
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if count <= 0 {
				return ctx.WriteError(errors.New("ERR COUNT must be > 0"))
			}
		case "ASC":
			sortOrder = "ASC"
		case "DESC":
			sortOrder = "DESC"
		case "STORE":
			if strings.HasSuffix(ctx.Command, "_RO") {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeKey = ctx.ArgString(i)
			storeDistKey = ""
		case "STOREDIST":
			if strings.HasSuffix(ctx.Command, "_RO") {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeDistKey = ctx.ArgString(i)
			storeKey = ""
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if anyCount && count == 0 {
		return ctx.WriteError(errors.New("ERR the ANY argument requires COUNT argument"))
	}
	if (storeKey != "" || storeDistKey != "") && (withCoord || withDist || withHash) {
		name := "STORE"
		if storeDistKey != "" {
			name = "STOREDIST"
		}
		return ctx.WriteError(fmt.Errorf("ERR %s is not compatible with WITHDIST, WITHHASH and WITHCOORD options", name))
	}
	if geo == nil {
		if storeKey != "" || storeDistKey != "" {
			if storeKey != "" {
				return publishGeoResult(ctx, storeKey, nil)
			}
			return publishGeoResult(ctx, storeDistKey, nil)
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	type result struct {
		member string
		dist   float64
		point  store.GeoPoint
		hash   uint64
	}
	results := make([]result, 0)

	geo.RLock()
	for member, point := range geo.Points {
		dist := store.GeoDistanceMeters(store.GeoPoint{Lon: lon, Lat: lat}, point)
		if dist <= radiusKm*1000 {
			hash := uint64(geo.scores[member])
			results = append(results, result{member: member, dist: dist, point: point, hash: hash})
		}
	}
	geo.RUnlock()

	if sortOrder == "ASC" || sortOrder == "DESC" || count > 0 {
		sort.Slice(results, func(i, j int) bool {
			if sortOrder == "DESC" {
				return results[i].dist > results[j].dist
			}
			return results[i].dist < results[j].dist
		})
	}

	if count > 0 && count < len(results) {
		results = results[:count]
	}

	if storeKey != "" {
		if len(results) == 0 {
			return publishGeoResult(ctx, storeKey, nil)
		}
		destGeo := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, r := range results {
			destGeo.Add(r.member, geo.scores[r.member])
		}
		return publishGeoResult(ctx, storeKey, destGeo)
	}

	if storeDistKey != "" {
		if len(results) == 0 {
			return publishGeoResult(ctx, storeDistKey, nil)
		}
		destZset := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, r := range results {
			destZset.Add(r.member, geoDistanceValue(r.dist, unit))
		}
		return publishGeoResult(ctx, storeDistKey, destZset)
	}

	respResults := make([]*resp.Value, 0, len(results))
	for _, r := range results {
		if !withCoord && !withDist && !withHash {
			respResults = append(respResults, resp.BulkString(r.member))
		} else {
			entry := []*resp.Value{resp.BulkString(r.member)}
			if withDist {
				entry = append(entry, resp.BulkString(strconv.FormatFloat(geoDistanceValue(r.dist, unit), 'f', 4, 64)))
			}
			if withHash {
				entry = append(entry, resp.IntegerValue(int64(r.hash)))
			}
			if withCoord {
				entry = append(entry, resp.ArrayValue([]*resp.Value{
					resp.BulkString(geoPositionString(r.point.Lon)),
					resp.BulkString(geoPositionString(r.point.Lat)),
				}))
			}
			respResults = append(respResults, resp.ArrayValue(entry))
		}
	}

	return ctx.WriteArray(respResults)
}

func cmdGEORADIUSBYMEMBER(ctx *Context) error {
	ctx.writeEffectsHandled = true
	ctx.PropagateAs()
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	geo, err := getGeo(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	member := ctx.ArgString(1)
	radius, err1 := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err1 != nil || math.IsNaN(radius) {
		return ctx.WriteError(errors.New("ERR need numeric radius"))
	}

	if radius < 0 {
		return ctx.WriteError(errors.New("ERR radius cannot be negative"))
	}
	unit := strings.ToLower(ctx.ArgString(3))
	radiusKm := radius
	switch unit {
	case "km":
	case "mi":
		radiusKm = radius * 1.60934
	case "ft":
		radiusKm = radius * 0.0003048
	case "m":
		radiusKm = radius / 1000
	default:
		return ctx.WriteError(errUnsupportedGeoUnit)
	}

	withCoord := false
	withDist := false
	withHash := false
	count := 0
	anyCount := false
	sortOrder := ""
	storeKey := ""
	storeDistKey := ""

	for i := 4; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "WITHCOORD":
			withCoord = true
		case "WITHDIST":
			withDist = true
		case "WITHHASH":
			withHash = true
		case "ANY":
			anyCount = true
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if count <= 0 {
				return ctx.WriteError(errors.New("ERR COUNT must be > 0"))
			}
		case "ASC":
			sortOrder = "ASC"
		case "DESC":
			sortOrder = "DESC"
		case "STORE":
			if strings.HasSuffix(ctx.Command, "_RO") {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeKey = ctx.ArgString(i)
			storeDistKey = ""
		case "STOREDIST":
			if strings.HasSuffix(ctx.Command, "_RO") {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeDistKey = ctx.ArgString(i)
			storeKey = ""
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if anyCount && count == 0 {
		return ctx.WriteError(errors.New("ERR the ANY argument requires COUNT argument"))
	}
	if (storeKey != "" || storeDistKey != "") && (withCoord || withDist || withHash) {
		name := "STORE"
		if storeDistKey != "" {
			name = "STOREDIST"
		}
		return ctx.WriteError(fmt.Errorf("ERR %s is not compatible with WITHDIST, WITHHASH and WITHCOORD options", name))
	}
	if geo == nil {
		if storeKey != "" || storeDistKey != "" {
			if storeKey != "" {
				return publishGeoResult(ctx, storeKey, nil)
			}
			return publishGeoResult(ctx, storeDistKey, nil)
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	centerPoint, exists := geo.Get(member)
	if !exists {
		return ctx.WriteError(errors.New("ERR could not decode requested zset member"))
	}

	type result struct {
		member string
		dist   float64
		point  store.GeoPoint
		hash   uint64
	}
	results := make([]result, 0)

	geo.RLock()
	for m, point := range geo.Points {
		dist := store.GeoDistanceMeters(centerPoint, point)
		if dist <= radiusKm*1000 {
			hash := uint64(geo.scores[m])
			results = append(results, result{member: m, dist: dist, point: point, hash: hash})
		}
	}
	geo.RUnlock()

	if sortOrder == "ASC" || sortOrder == "DESC" || count > 0 {
		sort.Slice(results, func(i, j int) bool {
			if sortOrder == "DESC" {
				return results[i].dist > results[j].dist
			}
			return results[i].dist < results[j].dist
		})
	}

	if count > 0 && count < len(results) {
		results = results[:count]
	}

	if storeKey != "" {
		if len(results) == 0 {
			return publishGeoResult(ctx, storeKey, nil)
		}
		destGeo := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, r := range results {
			destGeo.Add(r.member, geo.scores[r.member])
		}
		return publishGeoResult(ctx, storeKey, destGeo)
	}

	if storeDistKey != "" {
		if len(results) == 0 {
			return publishGeoResult(ctx, storeDistKey, nil)
		}
		destZset := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, r := range results {
			destZset.Add(r.member, geoDistanceValue(r.dist, unit))
		}
		return publishGeoResult(ctx, storeDistKey, destZset)
	}

	respResults := make([]*resp.Value, 0, len(results))
	for _, r := range results {
		if !withCoord && !withDist && !withHash {
			respResults = append(respResults, resp.BulkString(r.member))
		} else {
			entry := []*resp.Value{resp.BulkString(r.member)}
			if withDist {
				entry = append(entry, resp.BulkString(strconv.FormatFloat(geoDistanceValue(r.dist, unit), 'f', 4, 64)))
			}
			if withHash {
				entry = append(entry, resp.IntegerValue(int64(r.hash)))
			}
			if withCoord {
				entry = append(entry, resp.ArrayValue([]*resp.Value{
					resp.BulkString(geoPositionString(r.point.Lon)),
					resp.BulkString(geoPositionString(r.point.Lat)),
				}))
			}
			respResults = append(respResults, resp.ArrayValue(entry))
		}
	}

	return ctx.WriteArray(respResults)
}

func cmdGEOSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	geo, err := getGeo(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	var fromLon, fromLat, radius float64
	var boxHeight float64
	var byBox bool
	var hasFromMember bool
	var fromMember string
	var unit = "km"
	var withCoord, withDist, withHash bool
	var count int
	anyCount := false
	var sortOrder string
	// A centre clause (FROMMEMBER/FROMLONLAT) and an extent clause
	// (BYRADIUS/BYBOX) are both mandatory. Without this, radius stayed at its
	// zero value, so `dist <= radius` fabricated a match for a member sitting
	// exactly on the centre (0 <= 0) and returned "no matches" for everything
	// else -- a wrong answer where Redis answers with a syntax error, leaving
	// a dropped BYRADIUS indistinguishable from a genuine empty result.
	var hasFromLonLat bool
	var hasExtent bool

	i := 1
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "FROMMEMBER":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			fromMember = ctx.ArgString(i + 1)
			hasFromMember = true
			i += 2
		case "FROMLONLAT":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err1, err2 error
			fromLon, err1 = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			fromLat, err2 = strconv.ParseFloat(ctx.ArgString(i+2), 64)
			if err1 != nil || err2 != nil || math.IsNaN(fromLon) || math.IsNaN(fromLat) {
				return ctx.WriteError(ErrNotFloat)
			}
			if fromLon < -180 || fromLon > 180 || fromLat < -store.GeoLatitudeLimit || fromLat > store.GeoLatitudeLimit {
				return ctx.WriteError(fmt.Errorf("ERR invalid longitude,latitude pair %s,%s", geoCoordinateString(fromLon), geoCoordinateString(fromLat)))
			}
			hasFromMember = false
			hasFromLonLat = true
			i += 3
		case "BYRADIUS":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil || math.IsNaN(radius) {
				return ctx.WriteError(errors.New("ERR need numeric radius"))
			}
			if radius < 0 {
				return ctx.WriteError(errors.New("ERR radius cannot be negative"))
			}
			unit = strings.ToLower(ctx.ArgString(i + 2))
			if !geoUnitValid(unit) {
				return ctx.WriteError(errUnsupportedGeoUnit)
			}
			hasExtent = true
			i += 3
		case "BYBOX":
			if i+3 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			boxHeight, err = strconv.ParseFloat(ctx.ArgString(i+2), 64)
			if err != nil || math.IsNaN(radius) || math.IsNaN(boxHeight) {
				return ctx.WriteError(ErrNotFloat)
			}
			if radius < 0 || boxHeight < 0 {
				return ctx.WriteError(errors.New("ERR height or width cannot be negative"))
			}
			byBox = true
			unit = strings.ToLower(ctx.ArgString(i + 3))
			if !geoUnitValid(unit) {
				return ctx.WriteError(errUnsupportedGeoUnit)
			}
			hasExtent = true
			i += 4
		case "WITHCOORD":
			withCoord = true
			i++
		case "WITHDIST":
			withDist = true
			i++
		case "WITHHASH":
			withHash = true
			i++
		case "ANY":
			anyCount = true
			i++
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if count <= 0 {
				return ctx.WriteError(errors.New("ERR COUNT must be > 0"))
			}
			i++
		case "ASC":
			sortOrder = "ASC"
			i++
		case "DESC":
			sortOrder = "DESC"
			i++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if anyCount && count == 0 {
		return ctx.WriteError(errors.New("ERR the ANY argument requires COUNT argument"))
	}
	// Both clause families are mandatory; reject before evaluating anything.
	if !hasFromMember && !hasFromLonLat {
		return ctx.WriteError(ErrSyntaxError)
	}
	if !hasExtent {
		return ctx.WriteError(ErrSyntaxError)
	}

	if geo == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	if hasFromMember {
		centerPoint, exists := geo.Get(fromMember)
		if !exists {
			return ctx.WriteError(errors.New("ERR could not decode requested zset member"))
		}
		fromLon = centerPoint.Lon
		fromLat = centerPoint.Lat
	}

	switch unit {
	case "mi":
		radius *= 1.60934
	case "ft":
		radius *= 0.0003048
	case "m":
		radius /= 1000
	}

	type result struct {
		member string
		dist   float64
		point  store.GeoPoint
		hash   uint64
	}

	results := make([]result, 0)
	geo.RLock()
	for member, point := range geo.Points {
		dist := store.GeoDistanceMeters(store.GeoPoint{Lon: fromLon, Lat: fromLat}, point)
		if geoInsideShape(store.GeoPoint{Lon: fromLon, Lat: fromLat}, point, dist, radius*1000, boxHeight, unit, byBox) {
			results = append(results, result{member, dist, point, uint64(geo.scores[member])})
		}
	}
	geo.RUnlock()

	if sortOrder == "DESC" {
		sort.Slice(results, func(i, j int) bool { return results[i].dist > results[j].dist })
	} else if sortOrder == "ASC" || count > 0 {
		sort.Slice(results, func(i, j int) bool { return results[i].dist < results[j].dist })
	}

	if count > 0 && count < len(results) {
		results = results[:count]
	}

	respResults := make([]*resp.Value, 0, len(results))
	for _, r := range results {
		if !withCoord && !withDist && !withHash {
			respResults = append(respResults, resp.BulkString(r.member))
			continue
		}
		entry := []*resp.Value{resp.BulkString(r.member)}
		if withDist {
			entry = append(entry, resp.BulkString(strconv.FormatFloat(geoDistanceValue(r.dist, unit), 'f', 4, 64)))
		}
		if withHash {
			entry = append(entry, resp.IntegerValue(int64(r.hash)))
		}
		if withCoord {
			entry = append(entry, resp.ArrayValue([]*resp.Value{
				resp.BulkString(geoPositionString(r.point.Lon)),
				resp.BulkString(geoPositionString(r.point.Lat)),
			}))
		}
		respResults = append(respResults, resp.ArrayValue(entry))
	}

	return ctx.WriteArray(respResults)
}

func cmdGEOSEARCHSTORE(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	destKey := ctx.ArgString(0)
	srcKey := ctx.ArgString(1)
	geo, err := getGeo(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	var fromLon, fromLat, radius float64
	var boxHeight float64
	var byBox bool
	var hasFromMember bool
	var fromMember string
	var unit = "km"
	var storeDist bool
	var count int
	anyCount := false
	var sortOrder string
	// A centre clause (FROMMEMBER/FROMLONLAT) and an extent clause
	// (BYRADIUS/BYBOX) are both mandatory. Without this the extent stayed at
	// its zero value, the search matched nothing, and the "nothing matched"
	// path DELETED the destination key while replying :0 -- a call that merely
	// omitted BYRADIUS silently destroyed data instead of being rejected.
	var hasFromLonLat bool
	var hasExtent bool

	i := 2
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "FROMMEMBER":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			fromMember = ctx.ArgString(i + 1)
			hasFromMember = true
			i += 2
		case "FROMLONLAT":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err1, err2 error
			fromLon, err1 = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			fromLat, err2 = strconv.ParseFloat(ctx.ArgString(i+2), 64)
			if err1 != nil || err2 != nil || math.IsNaN(fromLon) || math.IsNaN(fromLat) {
				return ctx.WriteError(ErrNotFloat)
			}
			if fromLon < -180 || fromLon > 180 || fromLat < -store.GeoLatitudeLimit || fromLat > store.GeoLatitudeLimit {
				return ctx.WriteError(fmt.Errorf("ERR invalid longitude,latitude pair %s,%s", geoCoordinateString(fromLon), geoCoordinateString(fromLat)))
			}
			hasFromMember = false
			hasFromLonLat = true
			i += 3
		case "BYRADIUS":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil || math.IsNaN(radius) {
				return ctx.WriteError(errors.New("ERR need numeric radius"))
			}
			if radius < 0 {
				return ctx.WriteError(errors.New("ERR radius cannot be negative"))
			}
			unit = strings.ToLower(ctx.ArgString(i + 2))
			if !geoUnitValid(unit) {
				return ctx.WriteError(errUnsupportedGeoUnit)
			}
			hasExtent = true
			i += 3
		case "BYBOX":
			if i+3 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			boxHeight, err = strconv.ParseFloat(ctx.ArgString(i+2), 64)
			if err != nil || math.IsNaN(radius) || math.IsNaN(boxHeight) {
				return ctx.WriteError(ErrNotFloat)
			}
			if radius < 0 || boxHeight < 0 {
				return ctx.WriteError(errors.New("ERR height or width cannot be negative"))
			}
			byBox = true
			unit = strings.ToLower(ctx.ArgString(i + 3))
			if !geoUnitValid(unit) {
				return ctx.WriteError(errUnsupportedGeoUnit)
			}
			hasExtent = true
			i += 4
		case "STOREDIST":
			storeDist = true
			i++
		case "ASC":
			sortOrder = "ASC"
			i++
		case "DESC":
			sortOrder = "DESC"
			i++
		case "ANY":
			anyCount = true
			i++
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if count <= 0 {
				return ctx.WriteError(errors.New("ERR COUNT must be > 0"))
			}
			i++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if anyCount && count == 0 {
		return ctx.WriteError(errors.New("ERR the ANY argument requires COUNT argument"))
	}
	// Both clauses are mandatory; reject before touching the destination.
	if !hasFromMember && !hasFromLonLat {
		return ctx.WriteError(ErrSyntaxError)
	}
	if !hasExtent {
		return ctx.WriteError(ErrSyntaxError)
	}

	if geo == nil {
		return publishGeoResult(ctx, destKey, nil)
	}

	if hasFromMember {
		centerPoint, exists := geo.Get(fromMember)
		if !exists {
			return ctx.WriteError(errors.New("ERR could not decode requested zset member"))
		}
		fromLon = centerPoint.Lon
		fromLat = centerPoint.Lat
	}

	switch unit {
	case "mi":
		radius *= 1.60934
	case "ft":
		radius *= 0.0003048
	case "m":
		radius /= 1000
	}

	type match struct {
		member string
		dist   float64
		point  store.GeoPoint
	}

	matches := make([]match, 0)
	geo.RLock()
	for member, point := range geo.Points {
		dist := store.GeoDistanceMeters(store.GeoPoint{Lon: fromLon, Lat: fromLat}, point)
		if geoInsideShape(store.GeoPoint{Lon: fromLon, Lat: fromLat}, point, dist, radius*1000, boxHeight, unit, byBox) {
			matches = append(matches, match{member, dist, point})
		}
	}
	geo.RUnlock()

	// COUNT implies nearest-first, exactly as in the read-only sibling
	// GEOSEARCH: the map iteration above is unordered, so an untruncated
	// "first N" would keep an arbitrary N rather than the closest N.
	if sortOrder == "DESC" {
		sort.Slice(matches, func(i, j int) bool { return matches[i].dist > matches[j].dist })
	} else if sortOrder == "ASC" || count > 0 {
		sort.Slice(matches, func(i, j int) bool { return matches[i].dist < matches[j].dist })
	}

	if count > 0 && count < len(matches) {
		matches = matches[:count]
	}

	stored := len(matches)
	if stored == 0 {
		return publishGeoResult(ctx, destKey, nil)
	}

	var destValue *store.SortedSetValue
	if storeDist {
		destZset := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, m := range matches {
			destZset.Add(m.member, geoDistanceValue(m.dist, unit))
		}
		destValue = destZset
	} else {
		destGeo := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, m := range matches {
			destGeo.Add(m.member, geo.scores[m.member])
		}
		destValue = destGeo
	}

	return publishGeoResult(ctx, destKey, destValue)
}

func geoPositionString(value float64) string {
	text := strconv.FormatFloat(value, 'f', 17, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "-0" {
		return "0"
	}
	return text
}

func publishGeoResult(ctx *Context, key string, value *store.SortedSetValue) error {
	ctx.writeEffectsHandled = true
	if value == nil || len(value.Members) == 0 {
		if ctx.Store.Delete(key) {
			ctx.PropagateAs(ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB})
			ctx.notify(store.NotifyGeneric, "del", key)
		} else {
			ctx.PropagateAs()
		}
		return ctx.WriteInteger(0)
	}
	payload, err := persistence.EncodeDump(value)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := ctx.Store.Set(key, value, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	ctx.PropagateAs(ExecutedCommand{Name: "RESTORE", Args: [][]byte{[]byte(key), []byte("0"), payload, []byte("REPLACE")}, DB: ctx.DB})
	if ctx.Command == "GEOSEARCHSTORE" {
		ctx.notify(store.NotifyZSet, "geosearchstore", key)
	} else {
		ctx.notify(store.NotifyZSet, "georadiusstore", key)
	}
	return ctx.WriteInteger(int64(len(value.Members)))
}

func geoDistanceValue(meters float64, unit string) float64 {
	scale, _ := geoUnitMeters(unit)
	return meters / scale
}

func geoInsideShape(center, point store.GeoPoint, distance, width, height float64, unit string, box bool) bool {
	if !box {
		return distance <= width
	}
	scale, _ := geoUnitMeters(unit)
	if store.GeoDistanceMeters(store.GeoPoint{Lon: point.Lon, Lat: center.Lat}, point) > height*scale/2 {
		return false
	}
	return store.GeoDistanceMeters(store.GeoPoint{Lon: center.Lon, Lat: point.Lat}, point) <= width/2
}
