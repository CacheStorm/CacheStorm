package command

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

func cmdDUMP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entry, ok := ctx.Store.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteNullBulkString()
	}
	payload, err := persistence.EncodeDump(entry.Value)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(payload)
}

func cmdRESTORE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if (ctx.LegacyReplay || ctx.Replaying) && strings.HasPrefix(ctx.ArgString(2), "CACHSTORM001") {
		return cmdRESTORELegacy(ctx)
	}
	commandNow := time.Now().UnixMilli()
	replace, absolute := false, false
	idle, freq := int64(-1), int64(-1)
	for i := 3; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "REPLACE":
			replace = true
		case "ABSTTL":
			absolute = true
		case "IDLETIME":
			if freq != -1 || i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			n, ok := parseRedisInt(ctx.Arg(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 {
				return ctx.WriteError(errors.New("ERR Invalid IDLETIME value, must be >= 0"))
			}
			idle = n
		case "FREQ":
			if idle != -1 || i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			n, ok := parseRedisInt(ctx.Arg(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 || n > 255 {
				return ctx.WriteError(errors.New("ERR Invalid FREQ value, must be >= 0 and <= 255"))
			}
			freq = n
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	key := ctx.ArgString(0)
	if !replace && ctx.Store.Exists(key) {
		return ctx.Writer.WriteError("BUSYKEY Target key name already exists.")
	}
	ttl, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if ttl < 0 {
		return ctx.WriteError(errors.New("ERR Invalid TTL value, must be >= 0"))
	}
	value, err := persistence.DecodeDump(ctx.Arg(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	deadline := ttl
	if ttl > 0 && !absolute {
		if ttl > math.MaxInt64-commandNow {
			return ctx.WriteError(errInvalidExpireIn("restore"))
		}
		deadline += commandNow
	}
	if ttl > 0 && deadline <= commandNow && !ctx.Replaying {
		if ctx.Store.Delete(key) {
			ctx.propagateDelete(key)
		} else {
			ctx.PropagateAs()
		}
		return ctx.WriteOK()
	}
	if err := ctx.Store.Set(key, value, store.SetOptions{NX: !replace, ExpireAtMS: deadline}); err != nil {
		if errors.Is(err, store.ErrKeyExists) {
			return ctx.Writer.WriteError("BUSYKEY Target key name already exists.")
		}
		return ctx.WriteError(err)
	}
	if entry, exists := ctx.Store.GetShard(key).Get(key); exists {
		if freq >= 0 && ctx.Store.UsesLFU() {
			entry.AccessCount.Store(uint64(freq))
		}
		if idle >= 0 && !ctx.Store.UsesLFU() {
			entry.LastAccess.Store(time.Now().Add(-time.Duration(min(idle, int64(1<<24)-1)) * time.Second).UnixNano())
		}
	}
	args := [][]byte{[]byte(key), strconv.AppendInt(nil, deadline, 10), ctx.Arg(2), []byte("REPLACE"), []byte("ABSTTL")}
	if idle >= 0 {
		args = append(args, []byte("IDLETIME"), strconv.AppendInt(nil, idle, 10))
	}
	if freq >= 0 {
		args = append(args, []byte("FREQ"), strconv.AppendInt(nil, freq, 10))
	}
	ctx.PropagateAs(ExecutedCommand{Name: "RESTORE", Args: args, DB: ctx.DB})
	return ctx.WriteOK()
}
