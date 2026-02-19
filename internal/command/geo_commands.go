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
	case "km":
		dist = dist
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
	switch unit {
	case "km":
	case "mi":
		radius = radius / 0.621371
	case "ft":
		radius = radius / 3280.84
	default:
		radius = radius / 1000
	}

	geo := getGeo(ctx, key)
	if geo == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0)
	for member, point := range geo.Points {
		dist := store.Haversine(lon, lat, point.Lon, point.Lat)
		if dist <= radius {
			results = append(results, resp.BulkString(member))
		}
	}

	return ctx.WriteArray(results)
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
	switch unit {
	case "km":
	case "mi":
		radius = radius / 0.621371
	case "ft":
		radius = radius / 3280.84
	default:
		radius = radius / 1000
	}

	geo := getGeo(ctx, key)
	if geo == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	centerPoint, exists := geo.Get(member)
	if !exists {
		return ctx.WriteError(errors.New("ERR member not found"))
	}

	results := make([]*resp.Value, 0)
	for m, point := range geo.Points {
		dist := store.Haversine(centerPoint.Lon, centerPoint.Lat, point.Lon, point.Lat)
		if dist <= radius {
			results = append(results, resp.BulkString(m))
		}
	}

	return ctx.WriteArray(results)
}
