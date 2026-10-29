package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var errUnsupportedGeoUnit = errors.New("ERR unsupported unit provided. please use M, KM, FT, MI")

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

// geoPoints looks up the positions of members of the geo set at key; ok is
// false when the key is missing.
func geoPoints(ctx *Context, key string, members ...string) (points map[string]store.GeoPoint, ok bool, err error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, false, nil
	}
	points = make(map[string]store.GeoPoint, len(members))
	switch value := entry.Value.(type) {
	case *store.GeoValue:
		for _, member := range members {
			if point, found := value.Get(member); found {
				points[member] = point
			}
		}
	case *store.SortedSetValue:
		value.RLock()
		for _, member := range members {
			if score, found := value.Members[member]; found {
				points[member] = store.GeoPointFromScore(score)
			}
		}
		value.RUnlock()
	default:
		return nil, false, store.ErrWrongType
	}
	return points, true, nil
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
			maxLen := 0
			for i := range points {
				if zset == nil {
					maxLen = max(maxLen, len(ctx.Arg(argIdx+i*3+2)))
				}
			}
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
				target := zset
				if _, shared := value.(*store.SortedSetValue); shared {
					target.Lock()
					defer target.Unlock()
				} else if target == nil {
					target = &store.SortedSetValue{Members: make(map[string]float64, len(updates))}
				}
				target.NoteAdd(len(points), maxLen)
				for member, score := range updates {
					target.Add(member, score)
				}
				return target
			}}, nil
		})
		if err != nil {
			return ctx.WriteError(err)
		}
	}
	return geoAddReply(ctx, key, added, changed, ch)
}

func geoAddReply(ctx *Context, key string, added, changed int, ch bool) error {
	if added+changed == 0 {
		ctx.PropagateAs()
	} else {
		ctx.notify(store.NotifyZSet, "zadd", key)
		ctx.Store.KeyNotifier().NotifyKey(key)
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
	geo, exists, err := geoPoints(ctx, ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteNull()
	}
	a, okA := geo[ctx.ArgString(1)]
	b, okB := geo[ctx.ArgString(2)]
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

	members := make([]string, ctx.ArgCount()-1)
	for i := range members {
		members[i] = ctx.ArgString(i + 1)
	}
	geo, exists, err := geoPoints(ctx, key, members...)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		results := make([]*resp.Value, ctx.ArgCount()-1)
		for i := range results {
			results[i] = resp.NullValue()
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		point, exists := geo[member]
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

	members := make([]string, ctx.ArgCount()-1)
	for i := range members {
		members[i] = ctx.ArgString(i + 1)
	}
	geo, exists, err := geoPoints(ctx, key, members...)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		results := make([]*resp.Value, ctx.ArgCount()-1)
		for i := range results {
			results[i] = resp.NullArray()
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		point, exists := geo[member]
		if !exists {
			results = append(results, resp.NullArray())
		} else {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.DoubleValue(geoPositionString(point.Lon)),
				resp.DoubleValue(geoPositionString(point.Lat)),
			}))
		}
	}

	return ctx.WriteArray(results)
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
