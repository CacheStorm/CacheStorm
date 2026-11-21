package command

import (
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

// parseSetCardArgs parses SUNIONCARD's and SDIFFCARD's
// numkeys key [key ...] [APPROX] [LIMIT limit], with Redis 8.10's replies;
// APPROX is accepted only when approxOK.
func parseSetCardArgs(ctx *Context, approxOK bool) (keys []string, approx bool, limit int64, err error) {
	n := ctx.ArgCount()
	numKeys, perr := redisAtoi64(ctx.ArgString(0))
	if perr != nil || numKeys < 1 {
		return nil, false, 0, errNumKeysPositive
	}
	if numKeys > int64(n-1) {
		return nil, false, 0, errSetTooManyKeys
	}
	for j := 1 + int(numKeys); j < n; j++ {
		switch opt := ctx.ArgString(j); {
		case strings.EqualFold(opt, "LIMIT") && j+1 < n:
			v, lerr := redisAtoi64(ctx.ArgString(j + 1))
			if lerr != nil || v < 0 {
				return nil, false, 0, errLimitNegative
			}
			limit = v
			j++
		case approxOK && strings.EqualFold(opt, "APPROX"):
			approx = true
		default:
			return nil, false, 0, ErrSyntaxError
		}
	}
	keys = make([]string, numKeys)
	for i := range keys {
		keys[i] = ctx.ArgString(i + 1)
	}
	return keys, approx, limit, nil
}

// capCard is a cardinality under LIMIT (0 is no limit).
func capCard(n, limit int64) int64 {
	if limit > 0 && n > limit {
		return limit
	}
	return n
}

// cmdSUNIONCARD is SUNIONCARD numkeys key [key ...] [APPROX] [LIMIT limit]:
// the size of the sets' union, or with APPROX the HyperLogLog estimate of it
// PFADD of every member would give, at most LIMIT.
func cmdSUNIONCARD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	keys, approx, limit, err := parseSetCardArgs(ctx, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	src, err := openSetOpSources(ctx, keys)
	if err != nil {
		return ctx.WriteError(err)
	}
	defer src.close()
	if approx {
		var hll store.HLLState
		for _, set := range src.sets {
			if set == nil {
				continue
			}
			for m := range set.All() {
				hll.Add([]byte(m))
			}
		}
		return ctx.WriteInteger(capCard(hll.Count(), limit))
	}
	seen := make(map[string]struct{})
	for _, set := range src.sets {
		if set == nil {
			continue
		}
		for m := range set.All() {
			seen[m] = struct{}{}
			if limit > 0 && int64(len(seen)) >= limit {
				return ctx.WriteInteger(limit)
			}
		}
	}
	return ctx.WriteInteger(int64(len(seen)))
}

// cmdSDIFFCARD is SDIFFCARD numkeys key [key ...] [LIMIT limit]: the number
// of members of the first set in none of the others, at most LIMIT.
func cmdSDIFFCARD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	keys, _, limit, err := parseSetCardArgs(ctx, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	src, err := openSetOpSources(ctx, keys)
	if err != nil {
		return ctx.WriteError(err)
	}
	defer src.close()
	first := src.sets[0]
	if first == nil {
		return ctx.WriteInteger(0)
	}
	for _, other := range src.sets[1:] {
		if other == first {
			return ctx.WriteInteger(0)
		}
	}
	count := int64(0)
	for m := range first.All() {
		found := false
		for _, other := range src.sets[1:] {
			if other != nil && other.Has(m) {
				found = true
				break
			}
		}
		if !found {
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}
	return ctx.WriteInteger(count)
}
