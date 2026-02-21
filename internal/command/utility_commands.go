package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterUtilityCommands(router *Router) {
	router.Register(&CommandDef{Name: "RL.CREATE", Handler: cmdRLCREATE})
	router.Register(&CommandDef{Name: "RL.ALLOW", Handler: cmdRLALLOW})
	router.Register(&CommandDef{Name: "RL.GET", Handler: cmdRLGET})
	router.Register(&CommandDef{Name: "RL.DELETE", Handler: cmdRLDELETE})
	router.Register(&CommandDef{Name: "RL.RESET", Handler: cmdRLRESET})

	router.Register(&CommandDef{Name: "LOCK.TRY", Handler: cmdLOCKTRY})
	router.Register(&CommandDef{Name: "LOCK.ACQUIRE", Handler: cmdLOCKACQUIRE})
	router.Register(&CommandDef{Name: "LOCK.RELEASE", Handler: cmdLOCKRELEASE})
	router.Register(&CommandDef{Name: "LOCK.RENEW", Handler: cmdLOCKRENEW})
	router.Register(&CommandDef{Name: "LOCK.INFO", Handler: cmdLOCKINFO})
	router.Register(&CommandDef{Name: "LOCK.ISLOCKED", Handler: cmdLOCKISLOCKED})

	router.Register(&CommandDef{Name: "ID.CREATE", Handler: cmdIDCREATE})
	router.Register(&CommandDef{Name: "ID.NEXT", Handler: cmdIDNEXT})
	router.Register(&CommandDef{Name: "ID.NEXTN", Handler: cmdIDNEXTN})
	router.Register(&CommandDef{Name: "ID.CURRENT", Handler: cmdIDCURRENT})
	router.Register(&CommandDef{Name: "ID.SET", Handler: cmdIDSET})
	router.Register(&CommandDef{Name: "ID.DELETE", Handler: cmdIDDELETE})

	router.Register(&CommandDef{Name: "SNOWFLAKE.NEXT", Handler: cmdSNOWFLAKENEXT})
	router.Register(&CommandDef{Name: "SNOWFLAKE.PARSE", Handler: cmdSNOWFLAKEPARSE})
}

func cmdRLCREATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	maxTokens := parseInt64(ctx.ArgString(1))
	refillRate := parseInt64(ctx.ArgString(2))
	intervalMs := parseInt64(ctx.ArgString(3))

	store.GlobalRateLimiter.Create(key, int(maxTokens), int(refillRate), time.Duration(intervalMs)*time.Millisecond)

	return ctx.WriteOK()
}

func cmdRLALLOW(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	tokens := parseInt64(ctx.ArgString(1))

	allowed, remaining, resetTime := store.GlobalRateLimiter.Allow(key, int(tokens))

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(utilityBoolToInt(allowed)),
		resp.IntegerValue(int64(remaining)),
		resp.IntegerValue(resetTime.Unix()),
	})
}

func cmdRLGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tokens, maxTokens, refillRate, interval, exists := store.GlobalRateLimiter.Get(key)
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("tokens"),
		resp.IntegerValue(int64(tokens)),
		resp.BulkString("max_tokens"),
		resp.IntegerValue(int64(maxTokens)),
		resp.BulkString("refill_rate"),
		resp.IntegerValue(int64(refillRate)),
		resp.BulkString("interval_ms"),
		resp.IntegerValue(interval.Milliseconds()),
	})
}

func cmdRLDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	if store.GlobalRateLimiter.Delete(key) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdRLRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	if store.GlobalRateLimiter.Reset(key) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKTRY(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	token := ctx.ArgString(2)
	ttlMs := parseInt64(ctx.ArgString(3))

	if store.GlobalDistributedLock.TryLock(key, holder, token, time.Duration(ttlMs)*time.Millisecond) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	token := ctx.ArgString(2)
	ttlMs := parseInt64(ctx.ArgString(3))
	timeoutMs := parseInt64(ctx.ArgString(4))

	if store.GlobalDistributedLock.Lock(key, holder, token, time.Duration(ttlMs)*time.Millisecond, time.Duration(timeoutMs)*time.Millisecond) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	token := ctx.ArgString(2)

	if store.GlobalDistributedLock.Unlock(key, holder, token) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKRENEW(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	token := ctx.ArgString(2)
	ttlMs := parseInt64(ctx.ArgString(3))

	if store.GlobalDistributedLock.Renew(key, holder, token, time.Duration(ttlMs)*time.Millisecond) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	holder, expiresAt, exists := store.GlobalDistributedLock.GetHolder(key)
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("holder"),
		resp.BulkString(holder),
		resp.BulkString("expires_at"),
		resp.IntegerValue(expiresAt.Unix()),
		resp.BulkString("ttl_ms"),
		resp.IntegerValue(time.Until(expiresAt).Milliseconds()),
	})
}

func cmdLOCKISLOCKED(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	if store.GlobalDistributedLock.IsLocked(key) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdIDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	start := parseInt64(ctx.ArgString(1))
	increment := parseInt64(ctx.ArgString(2))

	prefix := ""
	suffix := ""
	padding := 0

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "PREFIX":
			if i+1 < ctx.ArgCount() {
				prefix = ctx.ArgString(i + 1)
				i++
			}
		case "SUFFIX":
			if i+1 < ctx.ArgCount() {
				suffix = ctx.ArgString(i + 1)
				i++
			}
		case "PADDING":
			if i+1 < ctx.ArgCount() {
				padding = int(parseInt64(ctx.ArgString(i + 1)))
				i++
			}
		}
	}

	store.GlobalIDGenerator.Create(name, start, increment, prefix, suffix, padding)

	return ctx.WriteOK()
}

func cmdIDNEXT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	id, _, exists := store.GlobalIDGenerator.Next(name)
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sequence not found"))
	}

	return ctx.WriteBulkString(id)
}

func cmdIDNEXTN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	count := parseInt64(ctx.ArgString(1))

	ids, _, exists := store.GlobalIDGenerator.NextN(name, int(count))
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sequence not found"))
	}

	results := make([]*resp.Value, len(ids))
	for i, id := range ids {
		results[i] = resp.BulkString(id)
	}

	return ctx.WriteArray(results)
}

func cmdIDCURRENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	id, num, exists := store.GlobalIDGenerator.Current(name)
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(id),
		resp.IntegerValue(num),
	})
}

func cmdIDSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))

	if store.GlobalIDGenerator.Set(name, value) {
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR sequence not found"))
}

func cmdIDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalIDGenerator.Delete(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

var snowflakeGen = store.NewSnowflakeIDGenerator(1)

func cmdSNOWFLAKENEXT(ctx *Context) error {
	nodeID := int64(1)
	if ctx.ArgCount() >= 1 {
		nodeID = parseInt64(ctx.ArgString(0))
	}

	gen := store.NewSnowflakeIDGenerator(nodeID)
	id := gen.Next()

	return ctx.WriteBulkString(strconv.FormatInt(id, 10))
}

func cmdSNOWFLAKEPARSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	idStr := ctx.ArgString(0)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	gen := store.NewSnowflakeIDGenerator(0)
	parsed := gen.Parse(id)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("timestamp"),
		resp.IntegerValue(parsed["timestamp"]),
		resp.BulkString("node_id"),
		resp.IntegerValue(parsed["node_id"]),
		resp.BulkString("sequence"),
		resp.IntegerValue(parsed["sequence"]),
		resp.BulkString("datetime"),
		resp.BulkString(time.UnixMilli(parsed["timestamp"]).UTC().Format(time.RFC3339)),
	})
}

func utilityBoolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
