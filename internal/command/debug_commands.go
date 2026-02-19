package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var ErrSegfault = errors.New("ERR SEGFAULT not allowed in production")

func RegisterDebugCommands(router *Router) {
	router.Register(&CommandDef{Name: "DEBUG", Handler: cmdDEBUG})
	router.Register(&CommandDef{Name: "OBJECT", Handler: cmdOBJECT})
	router.Register(&CommandDef{Name: "MEMORY", Handler: cmdMEMORY})
}

func cmdDEBUG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "SLEEP":
		return cmdDebugSleep(ctx)
	case "OBJECT":
		return cmdDebugObject(ctx)
	case "RELOAD":
		return ctx.WriteOK()
	case "LOADAOF":
		return ctx.WriteOK()
	case "DIGEST":
		return ctx.WriteBulkString("0000000000000000000000000000000000000000")
	case "SEGFAULT":
		return ctx.WriteError(ErrSegfault)
	case "DSNAPSHOT":
		return ctx.WriteOK()
	case "STRUCTSIZE":
		return ctx.WriteInteger(104)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown DEBUG subcommand '%s'", subCmd))
	}
}

func cmdDebugSleep(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	seconds, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	time.Sleep(time.Duration(seconds * float64(time.Second)))
	return ctx.WriteOK()
}

func cmdDebugObject(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(1)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	var sb strings.Builder
	sb.WriteString("Value at:")
	sb.WriteString(fmt.Sprintf("%p", entry.Value))
	sb.WriteString(" refcount:1 ")
	sb.WriteString("encoding:")
	sb.WriteString(getEncoding(entry.Value))
	sb.WriteString(" serializedlength:")
	sb.WriteString(strconv.FormatInt(int64(len(key)), 10))
	sb.WriteString(" lru:")
	sb.WriteString(strconv.FormatInt(entry.LastAccess/1000000, 10))
	sb.WriteString(" lru_seconds_idle:")
	sb.WriteString(strconv.FormatInt(int64(time.Since(time.Unix(0, entry.LastAccess)).Seconds()), 10))

	return ctx.WriteSimpleString(sb.String())
}

func getEncoding(v store.Value) string {
	switch v.(type) {
	case *store.StringValue:
		return "embstr"
	case *store.HashValue:
		return "hashtable"
	case *store.ListValue:
		return "quicklist"
	case *store.SetValue:
		return "hashtable"
	case *store.SortedSetValue:
		return "skiplist"
	default:
		return "raw"
	}
}

func cmdOBJECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))
	key := ctx.ArgString(1)

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNull()
	}

	switch subCmd {
	case "ENCODING":
		return ctx.WriteSimpleString(getEncoding(entry.Value))
	case "IDLETIME":
		idleSeconds := int64(time.Since(time.Unix(0, entry.LastAccess)).Seconds())
		return ctx.WriteInteger(idleSeconds)
	case "FREQ":
		return ctx.WriteInteger(int64(entry.AccessCount))
	case "REFCOUNT":
		return ctx.WriteInteger(1)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown OBJECT subcommand '%s'", subCmd))
	}
}

func cmdMEMORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "USAGE":
		return cmdMemoryUsage(ctx)
	case "STATS":
		return cmdMemoryStats(ctx)
	case "MALLOC-STATS":
		return ctx.WriteSimpleString("allocator: go runtime")
	case "DOCTOR":
		return ctx.WriteSimpleString("Sam said that everything is fine.")
	case "PURGE":
		return ctx.WriteOK()
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown MEMORY subcommand '%s'", subCmd))
	}
}

func cmdMemoryUsage(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(1)
	samples := 0

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if arg == "SAMPLES" && i+1 < ctx.ArgCount() {
			var err error
			samples, err = strconv.Atoi(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i++
		}
	}

	_ = samples

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteInteger(entry.MemoryUsage())
}

func cmdMemoryStats(ctx *Context) error {
	var results []*resp.Value

	addResult := func(k, v string) {
		results = append(results, resp.BulkString(k))
		results = append(results, resp.BulkString(v))
	}

	addResult("peak.allocated", strconv.FormatInt(ctx.Store.MemUsage(), 10))
	addResult("total.allocated", strconv.FormatInt(ctx.Store.MemUsage(), 10))
	addResult("keys.count", strconv.FormatInt(ctx.Store.KeyCount(), 10))
	addResult("keys.bytes-per-key", func() string {
		count := ctx.Store.KeyCount()
		if count == 0 {
			return "0"
		}
		return strconv.FormatInt(ctx.Store.MemUsage()/count, 10)
	}())

	return ctx.WriteArray(results)
}
