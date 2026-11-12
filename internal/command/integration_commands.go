package command

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterIntegrationCommands(router *Router) {
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.CREATE", Handler: cmdCIRCUITBREAKERCREATE})
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.STATE", Handler: cmdCIRCUITBREAKERSTATE})
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.TRIP", Handler: cmdCIRCUITBREAKERTRIP})
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.RESET", Handler: cmdCIRCUITBREAKERRESET})

	router.Register(&CommandDef{Name: "RATELIMIT.CREATE", Handler: cmdRATELIMITCREATE})
	router.Register(&CommandDef{Name: "RATELIMIT.CHECK", Handler: cmdRATELIMITCHECK})
	router.Register(&CommandDef{Name: "RATELIMIT.RESET", Handler: cmdRATELIMITRESET})
	router.Register(&CommandDef{Name: "RATELIMIT.DELETE", Handler: cmdRATELIMITDELETE})

	router.Register(&CommandDef{Name: "CACHE.LOCK", Handler: cmdCACHELOCK})
	router.Register(&CommandDef{Name: "CACHE.UNLOCK", Handler: cmdCACHEUNLOCK})
	router.Register(&CommandDef{Name: "CACHE.LOCKED", Handler: cmdCACHELOCKED})
	router.Register(&CommandDef{Name: "CACHE.REFRESH", Handler: cmdCACHEREFRESH})

	router.Register(&CommandDef{Name: "ARRAY.PUSH", Handler: cmdARRAYPUSH})
	router.Register(&CommandDef{Name: "ARRAY.POP", Handler: cmdARRAYPOP})
	router.Register(&CommandDef{Name: "ARRAY.SHIFT", Handler: cmdARRAYSHIFT})
	router.Register(&CommandDef{Name: "ARRAY.UNSHIFT", Handler: cmdARRAYUNSHIFT})
	router.Register(&CommandDef{Name: "ARRAY.SLICE", Handler: cmdARRAYSLICE})
	router.Register(&CommandDef{Name: "ARRAY.SPLICE", Handler: cmdARRAYSPLICE})
	router.Register(&CommandDef{Name: "ARRAY.REVERSE", Handler: cmdARRAYREVERSE})
	router.Register(&CommandDef{Name: "ARRAY.SORT", Handler: cmdARRAYSORT})
	router.Register(&CommandDef{Name: "ARRAY.UNIQUE", Handler: cmdARRAYUNIQUE})
	router.Register(&CommandDef{Name: "ARRAY.MERGE", Handler: cmdARRAYMERGE})
	router.Register(&CommandDef{Name: "ARRAY.INTERSECT", Handler: cmdARRAYINTERSECT})
	router.Register(&CommandDef{Name: "ARRAY.DIFF", Handler: cmdARRAYDIFF})
	router.Register(&CommandDef{Name: "ARRAY.INDEXOF", Handler: cmdARRAYINDEXOF})
	router.Register(&CommandDef{Name: "ARRAY.LASTINDEXOF", Handler: cmdARRAYLASTINDEXOF})
	router.Register(&CommandDef{Name: "ARRAY.INCLUDES", Handler: cmdARRAYINCLUDES})

	router.Register(&CommandDef{Name: "OBJECT.KEYS", Handler: cmdOBJECTKEYS})
	router.Register(&CommandDef{Name: "OBJECT.VALUES", Handler: cmdOBJECTVALUES})
	router.Register(&CommandDef{Name: "OBJECT.ENTRIES", Handler: cmdOBJECTENTRIES})
	router.Register(&CommandDef{Name: "OBJECT.FROMENTRIES", Handler: cmdOBJECTFROMENTRIES})
	router.Register(&CommandDef{Name: "OBJECT.MERGE", Handler: cmdOBJECTMERGE})
	router.Register(&CommandDef{Name: "OBJECT.PICK", Handler: cmdOBJECTPICK})
	router.Register(&CommandDef{Name: "OBJECT.OMIT", Handler: cmdOBJECTOMIT})
	router.Register(&CommandDef{Name: "OBJECT.HAS", Handler: cmdOBJECTHAS})
	router.Register(&CommandDef{Name: "OBJECT.GET", Handler: cmdOBJECTGET})
	router.Register(&CommandDef{Name: "OBJECT.SET", Handler: cmdOBJECTSET})
	router.Register(&CommandDef{Name: "OBJECT.DELETE", Handler: cmdOBJECTDELETE})

	router.Register(&CommandDef{Name: "MATH.ADD", Handler: cmdMATHADD})
	router.Register(&CommandDef{Name: "MATH.SUB", Handler: cmdMATHSUB})
	router.Register(&CommandDef{Name: "MATH.MUL", Handler: cmdMATHMUL})
	router.Register(&CommandDef{Name: "MATH.DIV", Handler: cmdMATHDIV})
	router.Register(&CommandDef{Name: "MATH.MOD", Handler: cmdMATHMOD})
	router.Register(&CommandDef{Name: "MATH.POW", Handler: cmdMATHPOW})
	router.Register(&CommandDef{Name: "MATH.SQRT", Handler: cmdMATHSQRT})
	router.Register(&CommandDef{Name: "MATH.ABS", Handler: cmdMATHABS})
	router.Register(&CommandDef{Name: "MATH.MIN", Handler: cmdMATHMIN})
	router.Register(&CommandDef{Name: "MATH.MAX", Handler: cmdMATHMAX})
	router.Register(&CommandDef{Name: "MATH.FLOOR", Handler: cmdMATHFLOOR})
	router.Register(&CommandDef{Name: "MATH.CEIL", Handler: cmdMATHCEIL})
	router.Register(&CommandDef{Name: "MATH.ROUND", Handler: cmdMATHROUND})
	router.Register(&CommandDef{Name: "MATH.RANDOM", Handler: cmdMATHRANDOM})
	router.Register(&CommandDef{Name: "MATH.SUM", Handler: cmdMATHSUM})
	router.Register(&CommandDef{Name: "MATH.AVG", Handler: cmdMATHAVG})
	router.Register(&CommandDef{Name: "MATH.MEDIAN", Handler: cmdMATHMEDIAN})
	router.Register(&CommandDef{Name: "MATH.STDDEV", Handler: cmdMATHSTDDEV})

	router.Register(&CommandDef{Name: "GEO.ENCODE", Handler: cmdGEOENCODE})
	router.Register(&CommandDef{Name: "GEO.DECODE", Handler: cmdGEODECODE})
	router.Register(&CommandDef{Name: "GEO.DISTANCE", Handler: cmdGEODISTANCE})
	router.Register(&CommandDef{Name: "GEO.BOUNDINGBOX", Handler: cmdGEOBOUNDINGBOX})

	router.Register(&CommandDef{Name: "CAPTCHA.GENERATE", Handler: cmdCAPTCHAGENERATE})
	router.Register(&CommandDef{Name: "CAPTCHA.VERIFY", Handler: cmdCAPTCHAVERIFY})

	router.Register(&CommandDef{Name: "SEQUENCE.NEXT", Handler: cmdSEQUENCENEXT})
	router.Register(&CommandDef{Name: "SEQUENCE.CURRENT", Handler: cmdSEQUENCECURRENT})
	router.Register(&CommandDef{Name: "SEQUENCE.RESET", Handler: cmdSEQUENCERESET})
	router.Register(&CommandDef{Name: "SEQUENCE.SET", Handler: cmdSEQUENCESET})
}

func cmdMATHADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		result += parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(result)
}

func cmdMATHSUB(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		result -= parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(result)
}

func cmdMATHMUL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		result *= parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(result)
}

func cmdMATHDIV(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		divisor := parseInt64(ctx.ArgString(i))
		if divisor == 0 {
			return ctx.WriteError(fmt.Errorf("ERR division by zero"))
		}
		result /= divisor
	}

	return ctx.WriteInteger(result)
}

func cmdMATHMOD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	a := parseInt64(ctx.ArgString(0))
	b := parseInt64(ctx.ArgString(1))

	if b == 0 {
		return ctx.WriteError(fmt.Errorf("ERR division by zero"))
	}

	return ctx.WriteInteger(a % b)
}

func cmdMATHPOW(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	base := parseInt64(ctx.ArgString(0))
	exp := parseInt64(ctx.ArgString(1))

	// Squaring gives the product exp multiplications would, wrapping the
	// same way, in log(exp) steps instead of exp.
	result := int64(1)
	for ; exp > 0; exp >>= 1 {
		if exp&1 == 1 {
			result *= base
		}
		base *= base
	}

	return ctx.WriteInteger(result)
}

func cmdMATHSQRT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	if n < 0 {
		return ctx.WriteError(fmt.Errorf("ERR negative number"))
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.6f", math.Sqrt(n)))
}

func cmdMATHABS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseInt64(ctx.ArgString(0))
	if n < 0 {
		n = -n
	}

	return ctx.WriteInteger(n)
}

func cmdMATHMIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	min := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		val := parseInt64(ctx.ArgString(i))
		if val < min {
			min = val
		}
	}

	return ctx.WriteInteger(min)
}

func cmdMATHMAX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	max := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		val := parseInt64(ctx.ArgString(i))
		if val > max {
			max = val
		}
	}

	return ctx.WriteInteger(max)
}

func cmdMATHFLOOR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	result := int64(math.Floor(n))

	return ctx.WriteInteger(result)
}

func cmdMATHCEIL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	result := int64(n)
	if float64(result) < n {
		result++
	}

	return ctx.WriteInteger(result)
}

func cmdMATHROUND(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	result := int64(math.Round(n))

	return ctx.WriteInteger(result)
}

func cmdMATHRANDOM(ctx *Context) error {
	min := int64(0)
	max := int64(100)

	if ctx.ArgCount() >= 2 {
		min = parseInt64(ctx.ArgString(0))
		max = parseInt64(ctx.ArgString(1))
	}

	if max < min {
		return ctx.WriteError(fmt.Errorf("ERR max must not be less than min"))
	}
	span := uint64(max - min)
	if span == math.MaxUint64 {
		return ctx.WriteInteger(int64(rand.Uint64()))
	}
	return ctx.WriteInteger(min + int64(rand.Uint64()%(span+1)))
}

func cmdMATHSUM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sum int64
	for i := 0; i < ctx.ArgCount(); i++ {
		sum += parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(sum)
}

func cmdMATHAVG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sum int64
	count := ctx.ArgCount()
	for i := 0; i < count; i++ {
		sum += parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", float64(sum)/float64(count)))
}

func cmdMATHMEDIAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	values := make([]int64, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		values[i] = parseInt64(ctx.ArgString(i))
	}

	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[i] > values[j] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}

	n := len(values)
	if n%2 == 1 {
		return ctx.WriteInteger(values[n/2])
	}

	median := float64(values[n/2-1]+values[n/2]) / 2
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", median))
}

func cmdMATHSTDDEV(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sum int64
	n := ctx.ArgCount()
	values := make([]int64, n)

	for i := 0; i < n; i++ {
		values[i] = parseInt64(ctx.ArgString(i))
		sum += values[i]
	}

	mean := float64(sum) / float64(n)
	var variance float64

	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}

	variance /= float64(n)
	stddev := sqrtFloat(variance)

	return ctx.WriteBulkString(fmt.Sprintf("%.6f", stddev))
}

func sqrtFloat(x float64) float64 {
	if x < 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func cmdGEOENCODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	lat := parseFloatExt([]byte(ctx.ArgString(0)))
	lon := parseFloatExt([]byte(ctx.ArgString(1)))

	geohash := encodeGeohash(lat, lon)

	return ctx.WriteBulkString(geohash)
}

func encodeGeohash(lat, lon float64) string {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	latMin, latMax := -90.0, 90.0
	lonMin, lonMax := -180.0, 180.0

	var bits uint64
	for i := 0; i < 60; i++ {
		if i%2 == 0 {
			mid := (lonMin + lonMax) / 2
			if lon >= mid {
				bits |= 1 << (59 - i)
				lonMin = mid
			} else {
				lonMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if lat >= mid {
				bits |= 1 << (59 - i)
				latMin = mid
			} else {
				latMax = mid
			}
		}
	}

	result := ""
	for i := 0; i < 12; i++ {
		idx := (bits >> uint(55-i*5)) & 0x1F
		result += string(base32[idx])
	}

	return result
}

func cmdGEODECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	geohash := ctx.ArgString(0)

	lat, lon := decodeGeohash(geohash)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("lat"),
		resp.BulkString(fmt.Sprintf("%.6f", lat)),
		resp.BulkString("lon"),
		resp.BulkString(fmt.Sprintf("%.6f", lon)),
	})
}

func decodeGeohash(geohash string) (float64, float64) {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	decodeMap := make(map[rune]int)
	for i, c := range base32 {
		decodeMap[c] = i
	}

	var bits uint64
	n := 0
	for _, c := range geohash {
		if n == 12 {
			break
		}
		if val, ok := decodeMap[c]; ok {
			bits = (bits << 5) | uint64(val)
			n++
		}
	}
	bits <<= uint(60 - 5*n)

	latMin, latMax := -90.0, 90.0
	lonMin, lonMax := -180.0, 180.0

	for i := 0; i < 5*n; i++ {
		if i%2 == 0 {
			mid := (lonMin + lonMax) / 2
			if bits&(1<<uint(59-i)) != 0 {
				lonMin = mid
			} else {
				lonMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if bits&(1<<uint(59-i)) != 0 {
				latMin = mid
			} else {
				latMax = mid
			}
		}
	}

	return (latMin + latMax) / 2, (lonMin + lonMax) / 2
}

func cmdGEODISTANCE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	lat1 := parseFloatExt([]byte(ctx.ArgString(0)))
	lon1 := parseFloatExt([]byte(ctx.ArgString(1)))
	lat2 := parseFloatExt([]byte(ctx.ArgString(2)))
	lon2 := parseFloatExt([]byte(ctx.ArgString(3)))

	dist := haversine(lat1, lon1, lat2, lon2)

	unit := "km"
	if ctx.ArgCount() >= 5 {
		unit = ctx.ArgString(4)
	}

	switch unit {
	case "m", "meters":
		dist *= 1000
	case "mi", "miles":
		dist *= 0.621371
	case "ft", "feet":
		dist *= 3280.84
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", dist))
}

func cmdGEOBOUNDINGBOX(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	lat := parseFloatExt([]byte(ctx.ArgString(0)))
	lon := parseFloatExt([]byte(ctx.ArgString(1)))
	radiusKm := parseFloatExt([]byte(ctx.ArgString(2)))

	latOffset := radiusKm / 111.0
	lonOffset := radiusKm / (111.0 * cosFloat(lat*0.01745329252))

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("min_lat"),
		resp.BulkString(fmt.Sprintf("%.6f", lat-latOffset)),
		resp.BulkString("min_lon"),
		resp.BulkString(fmt.Sprintf("%.6f", lon-lonOffset)),
		resp.BulkString("max_lat"),
		resp.BulkString(fmt.Sprintf("%.6f", lat+latOffset)),
		resp.BulkString("max_lon"),
		resp.BulkString(fmt.Sprintf("%.6f", lon+lonOffset)),
	})
}

func cosFloat(x float64) float64 {
	return sinFloat(x + 1.57079632679)
}

func sinFloat(x float64) float64 {
	result := x
	term := x
	for i := 1; i < 10; i++ {
		term *= -x * x / float64(2*i*(2*i+1))
		result += term
	}
	return result
}
