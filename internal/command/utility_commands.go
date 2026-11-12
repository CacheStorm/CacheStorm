package command

import (
	"fmt"
	"strconv"
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

// parseRateLimitDuration parses a duration argument expressed in
// milliseconds. The value must be a positive integer.
//
// The shared parseInt64 helper returns 0 for any input it cannot read, so a
// typo'd argument was indistinguishable from a legitimate zero. Zero is
// exactly the value a rate limiter cannot survive: the refill arithmetic in
// store.RateLimiter.Allow computes elapsed/Interval, so a zero interval makes
// the next RL.ALLOW divide by zero and panic.
//
// A negative interval is refused for the same reason it is not usable: it
// makes every refill subtract instead of add.
func parseRateLimitDuration(arg, name string) (int64, error) {
	ms, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ERR %s is not an integer or out of range", name)
	}
	if ms <= 0 {
		return 0, fmt.Errorf("ERR %s must be positive", name)
	}
	return ms, nil
}

// maxIDBatch bounds how many IDs one ID.NEXTN hands out, so a single call
// cannot loop for hours or exhaust memory building the reply.
const maxIDBatch = 1 << 16

// maxIDPadding bounds the zero padding of a sequence's IDs; an int64 has at
// most 19 digits.
const maxIDPadding = 64

func cmdSNOWFLAKENEXT(ctx *Context) error {
	nodeID := int64(1)
	if ctx.ArgCount() >= 1 {
		nodeID = parseInt64(ctx.ArgString(0))
	}

	// The generator carries the intra-millisecond sequence counter that makes
	// the IDs unique. It used to be constructed here on every call, so each
	// call started a fresh sequence at zero and two SNOWFLAKE.NEXT replies
	// landing in the same millisecond returned byte-identical IDs. Reuse the
	// process-wide generator so the sequence keeps advancing.
	gen := store.GetSnowflakeIDGenerator(nodeID)
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
