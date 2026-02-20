package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterGeoCommands(router *Router) {
	router.Register(&CommandDef{Name: "GEOADD", Handler: cmdGEOADD})
	router.Register(&CommandDef{Name: "GEODIST", Handler: cmdGEODIST})
	router.Register(&CommandDef{Name: "GEOHASH", Handler: cmdGEOHASH})
	router.Register(&CommandDef{Name: "GEOPOS", Handler: cmdGEOPOS})
	router.Register(&CommandDef{Name: "GEORADIUS", Handler: cmdGEORADIUS})
	router.Register(&CommandDef{Name: "GEORADIUSBYMEMBER", Handler: cmdGEORADIUSBYMEMBER})
	router.Register(&CommandDef{Name: "GEOSEARCH", Handler: cmdGEOSEARCH})
	router.Register(&CommandDef{Name: "GEOSEARCHSTORE", Handler: cmdGEOSEARCHSTORE})
}

func getOrCreateGeo(ctx *Context, key string) *store.GeoValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		geo := store.NewGeoValue()
		ctx.Store.Set(key, geo, store.SetOptions{})
		return geo
	}

	if geo, ok := entry.Value.(*store.GeoValue); ok {
		return geo
	}
	return nil
}

func getGeo(ctx *Context, key string) *store.GeoValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil
	}

	if geo, ok := entry.Value.(*store.GeoValue); ok {
		return geo
	}
	return nil
}

func cmdGEOADD(ctx *Context) error {
	if ctx.ArgCount() < 4 || (ctx.ArgCount()-1)%3 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	nx := false
	xx := false
	argIdx := 1

	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		if arg == "NX" {
			nx = true
			argIdx++
		} else if arg == "XX" {
			xx = true
			argIdx++
		} else if arg == "CH" {
			argIdx++
		} else {
			break
		}
	}

	remaining := ctx.ArgCount() - argIdx
	if remaining < 3 || remaining%3 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	geo := getOrCreateGeo(ctx, key)
	if geo == nil {
		return ctx.WriteError(store.ErrWrongType)
	}

	added := 0
	for i := argIdx; i < ctx.ArgCount(); i += 3 {
		lon, err1 := strconv.ParseFloat(ctx.ArgString(i), 64)
		lat, err2 := strconv.ParseFloat(ctx.ArgString(i+1), 64)
		if err1 != nil || err2 != nil {
			return ctx.WriteError(ErrNotFloat)
		}

		if lon < -180 || lon > 180 || lat < -85.05112878 || lat > 85.05112878 {
			return ctx.WriteError(errors.New("ERR invalid longitude/latitude"))
		}

		member := ctx.ArgString(i + 2)

		_, exists := geo.Get(member)
		if xx && !exists {
			continue
		}
		if nx && exists {
			continue
		}

		geo.Add(member, lon, lat)
		added++
	}

	return ctx.WriteInteger(int64(added))
}

func cmdGEODIST(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	member1 := ctx.ArgString(1)
	member2 := ctx.ArgString(2)

	unit := "m"
	if ctx.ArgCount() > 3 {
		unit = strings.ToLower(ctx.ArgString(3))
	}

	geo := getGeo(ctx, key)
	if geo == nil {
		return ctx.WriteNull()
	}

	dist := geo.Distance(member1, member2)
	if dist < 0 {
		return ctx.WriteNull()
	}

	switch unit {
	case "m":
		dist = dist * 1000
	case "km":
	case "mi":
		dist = dist * 0.621371
	case "ft":
		dist = dist * 3280.84
	default:
		dist = dist * 1000
	}

	return ctx.WriteBulkString(strconv.FormatFloat(dist, 'f', -1, 64))
}

func cmdGEOHASH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	geo := getGeo(ctx, key)
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
			hash := store.EncodeGeohash(point.Lon, point.Lat)
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

	geo := getGeo(ctx, key)
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
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(strconv.FormatFloat(point.Lon, 'f', -1, 64)),
				resp.BulkString(strconv.FormatFloat(point.Lat, 'f', -1, 64)),
			}))
		}
	}

	return ctx.WriteArray(results)
}

func cmdGEORADIUS(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	lon, err1 := strconv.ParseFloat(ctx.ArgString(1), 64)
	lat, err2 := strconv.ParseFloat(ctx.ArgString(2), 64)
	radius, err3 := strconv.ParseFloat(ctx.ArgString(3), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ctx.WriteError(ErrNotFloat)
	}

	unit := strings.ToLower(ctx.ArgString(4))
	radiusKm := radius
	switch unit {
	case "km":
	case "mi":
		radiusKm = radius * 1.60934
	case "ft":
		radiusKm = radius / 3280.84
	case "m":
		radiusKm = radius / 1000
	}

	withCoord := false
	withDist := false
	withHash := false
	count := 0
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
		case "ASC":
			sortOrder = "ASC"
		case "DESC":
			sortOrder = "DESC"
		case "STORE":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeKey = ctx.ArgString(i)
		case "STOREDIST":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeDistKey = ctx.ArgString(i)
		}
	}

	geo := getGeo(ctx, key)
	if geo == nil {
		if storeKey != "" || storeDistKey != "" {
			ctx.Store.Delete(storeKey)
			ctx.Store.Delete(storeDistKey)
			return ctx.WriteInteger(0)
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

	for member, point := range geo.Points {
		dist := store.Haversine(lon, lat, point.Lon, point.Lat)
		if dist <= radiusKm {
			hash := store.EncodeGeohashInt(point.Lon, point.Lat)
			results = append(results, result{member: member, dist: dist, point: point, hash: hash})
		}
	}

	if sortOrder == "ASC" {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].dist < results[i].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	} else if sortOrder == "DESC" {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].dist > results[i].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	}

	if count > 0 && count < len(results) {
		results = results[:count]
	}

	if storeKey != "" {
		if len(results) == 0 {
			ctx.Store.Delete(storeKey)
			return ctx.WriteInteger(0)
		}
		destGeo := store.NewGeoValue()
		for _, r := range results {
			destGeo.Add(r.member, r.point.Lon, r.point.Lat)
		}
		ctx.Store.Set(storeKey, destGeo, store.SetOptions{})
		return ctx.WriteInteger(int64(len(results)))
	}

	if storeDistKey != "" {
		if len(results) == 0 {
			ctx.Store.Delete(storeDistKey)
			return ctx.WriteInteger(0)
		}
		destZset := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, r := range results {
			switch unit {
			case "m":
				destZset.Members[r.member] = r.dist * 1000
			case "km":
				destZset.Members[r.member] = r.dist
			case "mi":
				destZset.Members[r.member] = r.dist * 0.621371
			case "ft":
				destZset.Members[r.member] = r.dist * 3280.84
			}
		}
		ctx.Store.Set(storeDistKey, destZset, store.SetOptions{})
		return ctx.WriteInteger(int64(len(results)))
	}

	respResults := make([]*resp.Value, 0, len(results))
	for _, r := range results {
		if !withCoord && !withDist && !withHash {
			respResults = append(respResults, resp.BulkString(r.member))
		} else {
			entry := []*resp.Value{resp.BulkString(r.member)}
			if withDist {
				var dist float64
				switch unit {
				case "m":
					dist = r.dist * 1000
				case "km":
					dist = r.dist
				case "mi":
					dist = r.dist * 0.621371
				case "ft":
					dist = r.dist * 3280.84
				}
				entry = append(entry, resp.BulkString(strconv.FormatFloat(dist, 'f', -1, 64)))
			}
			if withHash {
				entry = append(entry, resp.IntegerValue(int64(r.hash)))
			}
			if withCoord {
				entry = append(entry, resp.ArrayValue([]*resp.Value{
					resp.BulkString(strconv.FormatFloat(r.point.Lon, 'f', -1, 64)),
					resp.BulkString(strconv.FormatFloat(r.point.Lat, 'f', -1, 64)),
				}))
			}
			respResults = append(respResults, resp.ArrayValue(entry))
		}
	}

	return ctx.WriteArray(respResults)
}

func cmdGEORADIUSBYMEMBER(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	member := ctx.ArgString(1)
	radius, err1 := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err1 != nil {
		return ctx.WriteError(ErrNotFloat)
	}

	unit := strings.ToLower(ctx.ArgString(3))
	radiusKm := radius
	switch unit {
	case "km":
	case "mi":
		radiusKm = radius * 1.60934
	case "ft":
		radiusKm = radius / 3280.84
	case "m":
		radiusKm = radius / 1000
	}

	withCoord := false
	withDist := false
	withHash := false
	count := 0
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
		case "ASC":
			sortOrder = "ASC"
		case "DESC":
			sortOrder = "DESC"
		case "STORE":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeKey = ctx.ArgString(i)
		case "STOREDIST":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeDistKey = ctx.ArgString(i)
		}
	}

	geo := getGeo(ctx, key)
	if geo == nil {
		if storeKey != "" || storeDistKey != "" {
			ctx.Store.Delete(storeKey)
			ctx.Store.Delete(storeDistKey)
			return ctx.WriteInteger(0)
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	centerPoint, exists := geo.Get(member)
	if !exists {
		return ctx.WriteError(errors.New("ERR member not found"))
	}

	type result struct {
		member string
		dist   float64
		point  store.GeoPoint
		hash   uint64
	}
	results := make([]result, 0)

	for m, point := range geo.Points {
		dist := store.Haversine(centerPoint.Lon, centerPoint.Lat, point.Lon, point.Lat)
		if dist <= radiusKm {
			hash := store.EncodeGeohashInt(point.Lon, point.Lat)
			results = append(results, result{member: m, dist: dist, point: point, hash: hash})
		}
	}

	if sortOrder == "ASC" {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].dist < results[i].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	} else if sortOrder == "DESC" {
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[j].dist > results[i].dist {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	}

	if count > 0 && count < len(results) {
		results = results[:count]
	}

	if storeKey != "" {
		if len(results) == 0 {
			ctx.Store.Delete(storeKey)
			return ctx.WriteInteger(0)
		}
		destGeo := store.NewGeoValue()
		for _, r := range results {
			destGeo.Add(r.member, r.point.Lon, r.point.Lat)
		}
		ctx.Store.Set(storeKey, destGeo, store.SetOptions{})
		return ctx.WriteInteger(int64(len(results)))
	}

	if storeDistKey != "" {
		if len(results) == 0 {
			ctx.Store.Delete(storeDistKey)
			return ctx.WriteInteger(0)
		}
		destZset := &store.SortedSetValue{Members: make(map[string]float64)}
		for _, r := range results {
			switch unit {
			case "m":
				destZset.Members[r.member] = r.dist * 1000
			case "km":
				destZset.Members[r.member] = r.dist
			case "mi":
				destZset.Members[r.member] = r.dist * 0.621371
			case "ft":
				destZset.Members[r.member] = r.dist * 3280.84
			}
		}
		ctx.Store.Set(storeDistKey, destZset, store.SetOptions{})
		return ctx.WriteInteger(int64(len(results)))
	}

	respResults := make([]*resp.Value, 0, len(results))
	for _, r := range results {
		if !withCoord && !withDist && !withHash {
			respResults = append(respResults, resp.BulkString(r.member))
		} else {
			entry := []*resp.Value{resp.BulkString(r.member)}
			if withDist {
				var dist float64
				switch unit {
				case "m":
					dist = r.dist * 1000
				case "km":
					dist = r.dist
				case "mi":
					dist = r.dist * 0.621371
				case "ft":
					dist = r.dist * 3280.84
				}
				entry = append(entry, resp.BulkString(strconv.FormatFloat(dist, 'f', -1, 64)))
			}
			if withHash {
				entry = append(entry, resp.IntegerValue(int64(r.hash)))
			}
			if withCoord {
				entry = append(entry, resp.ArrayValue([]*resp.Value{
					resp.BulkString(strconv.FormatFloat(r.point.Lon, 'f', -1, 64)),
					resp.BulkString(strconv.FormatFloat(r.point.Lat, 'f', -1, 64)),
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

	var fromLon, fromLat, radius float64
	var hasFromMember bool
	var fromMember string
	var unit string = "km"

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
			if err1 != nil || err2 != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			hasFromMember = false
			i += 3
		case "BYRADIUS":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			unit = strings.ToLower(ctx.ArgString(i + 2))
			i += 3
		case "BYBOX":
			if i+4 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			unit = strings.ToLower(ctx.ArgString(i + 3))
			i += 5
		case "ASC", "DESC", "COUNT", "WITHCOORD", "WITHDIST", "WITHHASH":
			i++
		default:
			i++
		}
	}

	geo := getGeo(ctx, key)
	if geo == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	if hasFromMember {
		centerPoint, exists := geo.Get(fromMember)
		if !exists {
			return ctx.WriteArray([]*resp.Value{})
		}
		fromLon = centerPoint.Lon
		fromLat = centerPoint.Lat
	}

	switch unit {
	case "mi":
		radius = radius / 0.621371
	case "ft":
		radius = radius / 3280.84
	case "m":
		radius = radius / 1000
	}

	results := make([]*resp.Value, 0)
	for member, point := range geo.Points {
		dist := store.Haversine(fromLon, fromLat, point.Lon, point.Lat)
		if dist <= radius {
			results = append(results, resp.BulkString(member))
		}
	}

	return ctx.WriteArray(results)
}

func cmdGEOSEARCHSTORE(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	destKey := ctx.ArgString(0)
	srcKey := ctx.ArgString(1)

	var fromLon, fromLat, radius float64
	var hasFromMember bool
	var fromMember string
	var unit string = "km"

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
			if err1 != nil || err2 != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			hasFromMember = false
			i += 3
		case "BYRADIUS":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			unit = strings.ToLower(ctx.ArgString(i + 2))
			i += 3
		case "BYBOX":
			if i+4 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			radius, err = strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return ctx.WriteError(ErrNotFloat)
			}
			unit = strings.ToLower(ctx.ArgString(i + 3))
			i += 5
		case "ASC", "DESC", "COUNT", "STOREDIST":
			i++
		default:
			i++
		}
	}

	geo := getGeo(ctx, srcKey)
	if geo == nil {
		ctx.Store.Delete(destKey)
		return ctx.WriteInteger(0)
	}

	if hasFromMember {
		centerPoint, exists := geo.Get(fromMember)
		if !exists {
			ctx.Store.Delete(destKey)
			return ctx.WriteInteger(0)
		}
		fromLon = centerPoint.Lon
		fromLat = centerPoint.Lat
	}

	switch unit {
	case "mi":
		radius = radius / 0.621371
	case "ft":
		radius = radius / 3280.84
	case "m":
		radius = radius / 1000
	}

	destGeo := getOrCreateGeo(ctx, destKey)
	if destGeo == nil {
		return ctx.WriteError(store.ErrWrongType)
	}

	count := 0
	for member, point := range geo.Points {
		dist := store.Haversine(fromLon, fromLat, point.Lon, point.Lat)
		if dist <= radius {
			destGeo.Add(member, point.Lon, point.Lat)
			count++
		}
	}

	if count == 0 {
		ctx.Store.Delete(destKey)
	}

	return ctx.WriteInteger(int64(count))
}
