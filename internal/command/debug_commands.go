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

	// Cap at 300 seconds to prevent indefinite blocking
	const maxSleepSeconds = 300.0
	if seconds < 0 {
		seconds = 0
	}
	if seconds > maxSleepSeconds {
		seconds = maxSleepSeconds
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
	fmt.Fprintf(&sb, "%p", entry.Value)
	sb.WriteString(" refcount:1 ")
	sb.WriteString("encoding:")
	sb.WriteString(getEncoding(entry.Value))
	sb.WriteString(" serializedlength:")
	sb.WriteString(strconv.FormatInt(int64(len(key)), 10))
	sb.WriteString(" lru:")
	sb.WriteString(strconv.FormatInt(entry.LastAccess.Load()/1000000, 10))
	sb.WriteString(" lru_seconds_idle:")
	sb.WriteString(strconv.FormatInt(int64(time.Since(time.Unix(0, entry.LastAccess.Load())).Seconds()), 10))

	return ctx.WriteSimpleString(sb.String())
}

// Redis 7.4's default thresholds for its compact encodings.
const (
	encMaxListpackEntries = 128
	encMaxListpackValue   = 64
	encMaxIntsetEntries   = 512
	encMaxListpackBytes   = 8192
	encMaxEmbstrLen       = 44
)

// getEncoding reports the encoding Redis would use for v under its default
// configuration. CacheStorm has one representation per type; the name is
// derived from the value's current size and contents, so unlike Redis, whose
// conversions are one-way, a collection that grew and shrank again reports
// its compact encoding.
func getEncoding(v store.Value) string {
	switch val := v.(type) {
	case *store.StringValue:
		if _, ok := parseRedisInt(val.Data); ok {
			return "int"
		}
		if len(val.Data) <= encMaxEmbstrLen {
			return "embstr"
		}
		return "raw"
	case *store.HashValue:
		val.RLock()
		defer val.RUnlock()
		if len(val.Fields) > encMaxListpackEntries {
			return "hashtable"
		}
		for f, x := range val.Fields {
			if len(f) > encMaxListpackValue || len(x) > encMaxListpackValue {
				return "hashtable"
			}
		}
		return "listpack"
	case *store.ListValue:
		val.RLock()
		defer val.RUnlock()
		total := 0
		for _, el := range val.Elements {
			total += len(el) + 2
		}
		if total <= encMaxListpackBytes {
			return "listpack"
		}
		return "quicklist"
	case *store.SetValue:
		val.RLock()
		defer val.RUnlock()
		if len(val.Members) <= encMaxIntsetEntries {
			allInts := true
			for m := range val.Members {
				if _, ok := parseRedisInt([]byte(m)); !ok {
					allInts = false
					break
				}
			}
			if allInts {
				return "intset"
			}
		}
		if len(val.Members) > encMaxListpackEntries {
			return "hashtable"
		}
		for m := range val.Members {
			if len(m) > encMaxListpackValue {
				return "hashtable"
			}
		}
		return "listpack"
	case *store.SortedSetValue:
		val.RLock()
		defer val.RUnlock()
		if len(val.Members) > encMaxListpackEntries {
			return "skiplist"
		}
		for m := range val.Members {
			if len(m) > encMaxListpackValue {
				return "skiplist"
			}
		}
		return "listpack"
	case *store.StreamValue:
		return "stream"
	default:
		return "raw"
	}
}

var errObjectFreqNoLFU = errors.New("ERR An LFU maxmemory policy is not selected, access frequency not tracked. " + //nolint:staticcheck // Redis reply text
	"Please note that when switching between policies at runtime LRU and LFU data will take some time to adjust.")

func cmdOBJECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))
	key := ctx.ArgString(1)

	entry, exists := ctx.Store.GetShard(key).Get(key)
	if !exists {
		return ctx.WriteNull()
	}
	if entry.IsExpired() {
		entry, exists = ctx.Store.Get(key)
		if !exists {
			return ctx.WriteNull()
		}
	}

	switch subCmd {
	case "ENCODING":
		return ctx.WriteBulkString(getEncoding(entry.Value))
	case "IDLETIME":
		idleSeconds := int64(time.Since(time.Unix(0, entry.LastAccess.Load())).Seconds())
		return ctx.WriteInteger(idleSeconds)
	case "FREQ":
		if !ctx.Store.UsesLFU() {
			return ctx.WriteError(errObjectFreqNoLFU)
		}
		return ctx.WriteInteger(int64(entry.AccessCount.Load()))
	case "REFCOUNT":
		return ctx.WriteInteger(1)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'. Try OBJECT HELP.", ctx.ArgString(0)))
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
		var diagnosis strings.Builder
		diagnosis.WriteString("CacheStorm Memory Doctor:\n\n")

		memUsage := ctx.Store.MemUsage()
		keyCount := ctx.Store.KeyCount()

		fmt.Fprintf(&diagnosis, "Total memory usage: %d bytes\n", memUsage)
		fmt.Fprintf(&diagnosis, "Total keys: %d\n", keyCount)

		if keyCount > 0 {
			avgSize := memUsage / keyCount
			fmt.Fprintf(&diagnosis, "Average key size: %d bytes\n", avgSize)

			if avgSize > 10000 {
				diagnosis.WriteString("\nWarning: Average key size is high.\n")
				diagnosis.WriteString("Consider using smaller values or compression.\n")
			}
		}

		if memUsage > 100*1024*1024 {
			diagnosis.WriteString("\nWarning: Memory usage exceeds 100MB.\n")
			diagnosis.WriteString("Consider setting maxmemory limit or using eviction policy.\n")
		}

		diagnosis.WriteString("\nNo critical issues detected.\n")
		return ctx.WriteBulkString(diagnosis.String())
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
		if arg == "SAMPLES" {
			// A SAMPLES with no value must be reported, not dropped. Folding the
			// bounds check into the outer condition (`... && i+1 < ArgCount()`)
			// made a trailing SAMPLES fall through silently, so the client got a
			// byte count and success while the documented option was ignored.
			// This was the last handler still using the shape that cmdXRANGE's
			// COUNT was fixed for in round 12.
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
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
		results = append(results, resp.BulkString(k), resp.BulkString(v))
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
