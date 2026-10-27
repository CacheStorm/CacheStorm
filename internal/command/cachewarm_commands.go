package command

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterCacheWarmingCommands(router *Router) {
	router.Register(&CommandDef{Name: "WARM.INVALIDATE", Handler: cmdWARMINVALIDATE})
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
	router.Register(&CommandDef{Name: "KEY.FREQ", Handler: cmdKEYFREQ})
	router.Register(&CommandDef{Name: "KEY.IDLETIME", Handler: cmdKEYIDLETIME})
}

type WarmCacheStatus struct {
	Preloaded   int64
	Prefetched  int64
	Invalidated int64
	LastWarm    time.Time
}

var (
	warmStatus   WarmCacheStatus
	warmStatusMu sync.Mutex
)

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

	warmStatusMu.Lock()
	warmStatus.Invalidated += int64(invalidated)
	warmStatusMu.Unlock()

	return ctx.WriteInteger(int64(invalidated))
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

	_, err := ctx.Store.Transfer(oldKey, ctx.Store, newKey, store.TransferOptions{Replace: true})
	if err == store.ErrKeyNotFound {
		return ctx.WriteError(fmt.Errorf("ERR no such key"))
	}
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdKEYRENAMENX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	changed, err := ctx.Store.Transfer(oldKey, ctx.Store, newKey, store.TransferOptions{})
	if err == store.ErrKeyNotFound {
		return ctx.WriteError(fmt.Errorf("ERR no such key"))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		return ctx.WriteInteger(0)
	}

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

	nsKey := namespace + ":" + key
	changed, err := ctx.Store.Transfer(key, ctx.Store, nsKey, store.TransferOptions{Replace: true})
	if err == store.ErrKeyNotFound {
		return ctx.WriteInteger(0)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(1)
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

	return ctx.WriteInteger(int64(entry.AccessCount.Load()))
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

	idleNs := time.Now().UnixNano() - entry.LastAccess.Load()
	idleSec := idleNs / 1e9

	return ctx.WriteInteger(idleSec)
}
