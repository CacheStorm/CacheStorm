package command

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
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
	case "RELOAD", "LOADAOF":
		return ctx.WriteError(fmt.Errorf("ERR DEBUG %s is not supported by this server", subCmd))
	case "DIGEST":
		return ctx.WriteBulkString(datasetDigest(ctx))
	case "SEGFAULT":
		return ctx.WriteError(ErrSegfault)
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
	// hash-max-listpack-entries is 512 in Redis 7.4, unlike the set and
	// zset limits.
	encMaxHashListpackEntries = 512
	encMaxListpackValue       = 64
	encMaxIntsetEntries       = 512
	encMaxListpackBytes       = 8192
	encMaxEmbstrLen           = 44
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
		if len(val.Fields) > encMaxHashListpackEntries {
			return "hashtable"
		}
		for f, x := range val.Fields {
			if len(f) > encMaxListpackValue || len(x) > encMaxListpackValue {
				return "hashtable"
			}
		}
		if val.FieldTTLsUsed() {
			return "listpackex"
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

const objectHelp = `OBJECT <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
ENCODING <key>
    Return the kind of internal representation used in order to store the value
    associated with a <key>.
FREQ <key>
    Return the access frequency index of the <key>. The returned integer is
    proportional to the logarithm of the recent access frequency of the key.
IDLETIME <key>
    Return the idle time of the <key>, that is the approximated number of
    seconds elapsed since the last access to the key.
REFCOUNT <key>
    Return the number of references of the value associated with the specified
    <key>.
HELP
    Print this help.`

func cmdOBJECT(ctx *Context) error {
	if ctx.ArgCount() == 1 && strings.EqualFold(ctx.ArgString(0), "HELP") {
		return ctx.WriteArray(helpReply(objectHelp))
	}
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
		return ctx.Writer.WriteError("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try OBJECT HELP.")
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
		return ctx.WriteBulkString(memoryDoctorReport(ctx))
	case "PURGE":
		debug.FreeOSMemory()
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
	m, keys := currentMemoryFigures(ctx)
	perKey := int64(0)
	if keys > 0 {
		perKey = m.used / keys
	}
	peakPerc := 100.0
	if m.peak > 0 {
		peakPerc = float64(m.used) * 100 / float64(m.peak)
	}
	double := func(f float64) *resp.Value { return resp.DoubleValue(strconv.FormatFloat(f, 'f', -1, 64)) }
	return ctx.Writer.WriteValue(resp.PairsValue([]*resp.Value{
		resp.BulkString("peak.allocated"), resp.IntegerValue(m.peak),
		resp.BulkString("total.allocated"), resp.IntegerValue(m.used),
		resp.BulkString("keys.count"), resp.IntegerValue(keys),
		resp.BulkString("keys.bytes-per-key"), resp.IntegerValue(perKey),
		resp.BulkString("dataset.bytes"), resp.IntegerValue(m.used),
		resp.BulkString("peak.percentage"), double(peakPerc),
		resp.BulkString("allocator.resident"), resp.IntegerValue(m.rss),
		resp.BulkString("fragmentation"), double(m.fragmentation()),
		resp.BulkString("fragmentation.bytes"), resp.IntegerValue(m.rss - m.used),
	}))
}

// currentMemoryFigures reads the memory figures and the key count across
// every database.
func currentMemoryFigures(ctx *Context) (memoryFigures, int64) {
	used, keys := ctx.Store.MemUsage(), ctx.Store.KeyCount()
	if dbs := ctx.databases(); dbs != nil {
		used, keys = dbs.MemUsage(), dbs.KeyCount()
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return readMemoryFigures(ctx, used, &ms), keys
}

// memoryDoctorReport is MEMORY DOCTOR: Redis' checks for an empty instance,
// a past peak and a large RSS overhead, plus a maxmemory that is nearly
// reached with nothing to evict.
func memoryDoctorReport(ctx *Context) string {
	m, _ := currentMemoryFigures(ctx)
	if m.used < 5<<20 {
		return "Hi Sam, this instance is empty or is using very little memory, my issues detector can't be used in these conditions. " +
			"Please, leave for your mission on Earth and fill it with some data. The new Sam and I will be back to our programming as soon as I finished rebooting."
	}
	var issues []string
	if m.peak > m.used*3/2 {
		issues = append(issues, " * Peak memory: In the past this instance used more than 150% the memory that is currently using ("+humanBytes(m.peak)+
			" against "+humanBytes(m.used)+"). The Go runtime returns freed memory to the operating system gradually, so the RSS can stay high for a while after a peak. "+
			"If the peak was occasional and you want the memory back now, run MEMORY PURGE.")
	}
	if m.fragmentation() > 1.4 && m.rss-m.used > 64<<20 {
		issues = append(issues, " * High total RSS: This instance has an RSS of "+humanBytes(m.rss)+" for "+humanBytes(m.used)+" of data, a ratio of "+
			strconv.FormatFloat(m.fragmentation(), 'f', 2, 64)+". This is usually due to a memory peak (see above) or to garbage the Go runtime has not collected yet. "+
			"MEMORY PURGE runs a collection and returns free memory to the operating system; a lower GOGC trades CPU for a smaller heap.")
	}
	if limit, policy := ctx.Store.MemoryLimit(); limit > 0 && m.used > limit*9/10 && policy == store.EvictionNoEviction {
		issues = append(issues, " * Near maxmemory: "+humanBytes(m.used)+" of the "+humanBytes(limit)+
			" maxmemory is used and the policy is noeviction, so writes will soon be refused with OOM. Raise maxmemory or choose an eviction policy.")
	}
	if len(issues) == 0 {
		return "Hi Sam, I can't find any memory issue in your instance. I can only account for what occurs on this base."
	}
	return "Sam, I detected a few issues in this CacheStorm instance memory implants:\n\n" + strings.Join(issues, "\n\n") +
		"\n\nI'm here to keep you safe, Sam. I want to help you.\n"
}
