package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterCacheWarmingCommands(router *Router) {
	router.Register(&CommandDef{Name: "WARM.PRELOAD", Handler: cmdWARMPRELOAD})
	router.Register(&CommandDef{Name: "WARM.PREFETCH", Handler: cmdWARMPREFETCH})
	router.Register(&CommandDef{Name: "WARM.INVALIDATE", Handler: cmdWARMINVALIDATE})
	router.Register(&CommandDef{Name: "WARM.STATUS", Handler: cmdWARMSTATUS})
	router.Register(&CommandDef{Name: "BATCH.GET", Handler: cmdBATCHGET})
	router.Register(&CommandDef{Name: "BATCH.SET", Handler: cmdBATCHSET})
	router.Register(&CommandDef{Name: "BATCH.DEL", Handler: cmdBATCHDEL})
	router.Register(&CommandDef{Name: "BATCH.MGET", Handler: cmdBATCHMGET})
	router.Register(&CommandDef{Name: "BATCH.MSET", Handler: cmdBATCHMSET})
	router.Register(&CommandDef{Name: "BATCH.MDEL", Handler: cmdBATCHMDEL})
	router.Register(&CommandDef{Name: "BATCH.EXEC", Handler: cmdBATCHEXEC})
	router.Register(&CommandDef{Name: "PIPELINE.EXEC", Handler: cmdPIPELINEEXEC})
	router.Register(&CommandDef{Name: "KEY.RENAME", Handler: cmdKEYRENAME})
	router.Register(&CommandDef{Name: "KEY.RENAMENX", Handler: cmdKEYRENAMENX})
	router.Register(&CommandDef{Name: "KEY.COPY", Handler: cmdKEYCOPY})
	router.Register(&CommandDef{Name: "KEY.MOVE", Handler: cmdKEYMOVE})
	router.Register(&CommandDef{Name: "KEY.DUMP", Handler: cmdKEYDUMP})
	router.Register(&CommandDef{Name: "KEY.RESTORE", Handler: cmdKEYRESTORE})
	router.Register(&CommandDef{Name: "KEY.OBJECT", Handler: cmdKEYOBJECT})
	router.Register(&CommandDef{Name: "KEY.ENCODE", Handler: cmdKEYENCODE})
	router.Register(&CommandDef{Name: "KEY.FREQ", Handler: cmdKEYFREQ})
	router.Register(&CommandDef{Name: "KEY.IDLETIME", Handler: cmdKEYIDLETIME})
	router.Register(&CommandDef{Name: "KEY.REFCOUNT", Handler: cmdKEYREFCOUNT})
}

type WarmCacheStatus struct {
	Preloaded   int64
	Prefetched  int64
	Invalidated int64
	LastWarm    time.Time
}

var warmStatus WarmCacheStatus

func cmdWARMPRELOAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	preloaded := 0
	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		if ctx.Store.Exists(key) {
			preloaded++
		}
	}

	warmStatus.Preloaded += int64(preloaded)
	warmStatus.LastWarm = time.Now()

	return ctx.WriteInteger(int64(preloaded))
}

func cmdWARMPREFETCH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	prefetched := 0
	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		if entry, ok := ctx.Store.Get(key); ok && entry != nil {
			prefetched++
		}
	}

	warmStatus.Prefetched += int64(prefetched)
	warmStatus.LastWarm = time.Now()

	return ctx.WriteInteger(int64(prefetched))
}

func cmdWARMINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	invalidated := 0
	for i := 0; i < ctx.ArgCount(); i++ {
		if ctx.Store.Delete(ctx.ArgString(i)) {
			invalidated++
		}
	}

	warmStatus.Invalidated += int64(invalidated)

	return ctx.WriteInteger(int64(invalidated))
}

func cmdWARMSTATUS(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("preloaded"),
		resp.IntegerValue(warmStatus.Preloaded),
		resp.BulkString("prefetched"),
		resp.IntegerValue(warmStatus.Prefetched),
		resp.BulkString("invalidated"),
		resp.IntegerValue(warmStatus.Invalidated),
		resp.BulkString("last_warm"),
		resp.BulkString(warmStatus.LastWarm.Format(time.RFC3339)),
	})
}

func cmdBATCHGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		entry, ok := ctx.Store.Get(key)
		if !ok || entry == nil {
			results[i] = resp.NullValue()
		} else {
			results[i] = resp.BulkString(entry.Value.String())
		}
	}

	return ctx.WriteArray(results)
}

func cmdBATCHSET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	for i := 0; i < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		val := ctx.ArgString(i + 1)
		ctx.Store.Set(key, &store.StringValue{Data: []byte(val)}, store.SetOptions{})
	}

	return ctx.WriteOK()
}

func cmdBATCHDEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	deleted := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		if ctx.Store.Delete(ctx.ArgString(i)) {
			deleted++
		}
	}

	return ctx.WriteInteger(deleted)
}

func cmdBATCHMGET(ctx *Context) error {
	return cmdBATCHGET(ctx)
}

func cmdBATCHMSET(ctx *Context) error {
	return cmdBATCHSET(ctx)
}

func cmdBATCHMDEL(ctx *Context) error {
	return cmdBATCHDEL(ctx)
}

func cmdBATCHEXEC(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, 0)
	i := 0

	for i < ctx.ArgCount() {
		cmd := strings.ToUpper(ctx.ArgString(i))
		i++

		switch cmd {
		case "GET":
			if i >= ctx.ArgCount() {
				return ctx.WriteError(fmt.Errorf("ERR missing key for GET"))
			}
			key := ctx.ArgString(i)
			i++
			entry, ok := ctx.Store.Get(key)
			if !ok || entry == nil {
				results = append(results, resp.NullValue())
			} else {
				results = append(results, resp.BulkString(entry.Value.String()))
			}

		case "SET":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(fmt.Errorf("ERR missing key/value for SET"))
			}
			key := ctx.ArgString(i)
			val := ctx.ArgString(i + 1)
			i += 2
			ctx.Store.Set(key, &store.StringValue{Data: []byte(val)}, store.SetOptions{})
			results = append(results, resp.SimpleString("OK"))

		case "DEL":
			if i >= ctx.ArgCount() {
				return ctx.WriteError(fmt.Errorf("ERR missing key for DEL"))
			}
			key := ctx.ArgString(i)
			i++
			deleted := ctx.Store.Delete(key)
			if deleted {
				results = append(results, resp.IntegerValue(1))
			} else {
				results = append(results, resp.IntegerValue(0))
			}

		case "EXISTS":
			if i >= ctx.ArgCount() {
				return ctx.WriteError(fmt.Errorf("ERR missing key for EXISTS"))
			}
			key := ctx.ArgString(i)
			i++
			if ctx.Store.Exists(key) {
				results = append(results, resp.IntegerValue(1))
			} else {
				results = append(results, resp.IntegerValue(0))
			}

		default:
			return ctx.WriteError(fmt.Errorf("ERR unsupported batch command: %s", cmd))
		}
	}

	return ctx.WriteArray(results)
}

func cmdPIPELINEEXEC(ctx *Context) error {
	return cmdBATCHEXEC(ctx)
}

func cmdKEYRENAME(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	entry, ok := ctx.Store.Get(oldKey)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR no such key"))
	}

	ctx.Store.Set(newKey, entry.Value, store.SetOptions{})
	ctx.Store.Delete(oldKey)

	return ctx.WriteOK()
}

func cmdKEYRENAMENX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	entry, ok := ctx.Store.Get(oldKey)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR no such key"))
	}

	if ctx.Store.Exists(newKey) {
		return ctx.WriteInteger(0)
	}

	ctx.Store.Set(newKey, entry.Value, store.SetOptions{})
	ctx.Store.Delete(oldKey)

	return ctx.WriteInteger(1)
}

func cmdKEYCOPY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)

	entry, ok := ctx.Store.Get(srcKey)
	if !ok {
		return ctx.WriteInteger(0)
	}

	replace := false
	if ctx.ArgCount() >= 3 {
		replace = strings.ToUpper(ctx.ArgString(2)) == "REPLACE"
	}

	if ctx.Store.Exists(dstKey) && !replace {
		return ctx.WriteInteger(0)
	}

	ctx.Store.Set(dstKey, entry.Value.Clone(), store.SetOptions{})

	return ctx.WriteInteger(1)
}

func cmdKEYMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	namespace := ctx.ArgString(1)

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteInteger(0)
	}

	nsKey := namespace + ":" + key
	ctx.Store.Set(nsKey, entry.Value, store.SetOptions{})
	ctx.Store.Delete(key)

	return ctx.WriteInteger(1)
}

func cmdKEYDUMP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	dataType := entry.Value.Type()
	dump := fmt.Sprintf("%d:%s", dataType, entry.Value.String())

	return ctx.WriteBulkString(dump)
}

func cmdKEYRESTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	_ = ctx.ArgString(1)

	replace := false
	ttl := int64(0)

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "REPLACE":
			replace = true
		case "TTL":
			if i+1 < ctx.ArgCount() {
				ttl = parseInt64(ctx.ArgString(i + 1))
				i++
			}
		}
	}

	if ctx.Store.Exists(key) && !replace {
		return ctx.WriteError(fmt.Errorf("ERR key already exists"))
	}

	var opts store.SetOptions
	if ttl > 0 {
		opts.TTL = time.Duration(ttl) * time.Millisecond
	}

	ctx.Store.Set(key, &store.StringValue{Data: []byte("restored")}, opts)

	return ctx.WriteOK()
}

func cmdKEYOBJECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subcmd := strings.ToUpper(ctx.ArgString(0))
	key := ctx.ArgString(1)

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	switch subcmd {
	case "ENCODING":
		return ctx.WriteBulkString("raw")
	case "IDLETIME":
		return ctx.WriteInteger(0)
	case "REFCOUNT":
		return ctx.WriteInteger(1)
	case "FREQ":
		return ctx.WriteInteger(int64(entry.AccessCount))
	case "TYPE":
		return ctx.WriteBulkString(entry.Value.Type().String())
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand"))
	}
}

func cmdKEYENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(entry.Value.Type().String())
}

func cmdKEYFREQ(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteInteger(int64(entry.AccessCount))
}

func cmdKEYIDLETIME(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	idleNs := time.Now().UnixNano() - entry.LastAccess
	idleSec := idleNs / 1e9

	return ctx.WriteInteger(idleSec)
}

func cmdKEYREFCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	if !ctx.Store.Exists(key) {
		return ctx.WriteNull()
	}

	return ctx.WriteInteger(1)
}
