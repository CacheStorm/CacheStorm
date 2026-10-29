package command

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
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
	case "POPULATE":
		return cmdDebugPopulate(ctx)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown DEBUG subcommand '%s'", subCmd))
	}
}

// cmdDebugPopulate is DEBUG POPULATE count [prefix [size]]: it creates the
// string keys prefix:0 .. prefix:count-1 holding "value:N", padded with zero
// bytes or cut to size when size is given, and skips keys that exist.
func cmdDebugPopulate(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 4 {
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand or wrong number of arguments for '%s'. Try DEBUG HELP.", ctx.ArgString(0))) //nolint:staticcheck // Redis reply text
	}
	count, err := debugPositive(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	prefix := "key"
	if ctx.ArgCount() > 2 {
		prefix = ctx.ArgString(2)
	}
	var size int64
	if ctx.ArgCount() > 3 {
		if size, err = debugPositive(ctx.ArgString(3)); err != nil {
			return ctx.WriteError(err)
		}
	}
	for j := int64(0); j < count; j++ {
		key := prefix + ":" + strconv.FormatInt(j, 10)
		val := []byte("value:" + strconv.FormatInt(j, 10))
		if size > 0 {
			padded := make([]byte, size)
			copy(padded, val)
			val = padded
		}
		err := ctx.Store.Set(key, &store.StringValue{Data: val}, store.SetOptions{NX: true})
		if err != nil && !errors.Is(err, store.ErrKeyExists) {
			return ctx.WriteError(err)
		}
	}
	return ctx.WriteOK()
}

func debugPositive(arg string) (int64, error) {
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || n < 0 {
		return 0, errors.New("ERR value is out of range, must be positive")
	}
	return n, nil
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
	// sharedIntegers is OBJ_SHARED_INTEGERS: Redis keeps one shared object
	// for each integer below it, reported with refcount INT_MAX.
	sharedIntegers = 10000
)

// getEncoding reports the encoding Redis would use for v under its default
// configuration. CacheStorm has one representation per type; the name is
// derived from the value's current size and contents, so unlike Redis, whose
// conversions are one-way, a collection that grew and shrank again reports
// its compact encoding.
func getEncoding(v store.Value) string {
	switch val := v.(type) {
	case *store.StringValue:
		if val.Enc == store.StringEncRaw {
			return "raw"
		}
		if _, ok := parseRedisInt(val.Data); ok && val.Enc != store.StringEncNoInt {
			return "int"
		}
		if len(val.Data) <= encMaxEmbstrLen {
			return "embstr"
		}
		return "raw"
	case *store.HashValue:
		val.RLock()
		defer val.RUnlock()
		if val.Big() {
			return "hashtable"
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
		if enc := val.Encoding(); enc != "" {
			return enc
		}
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
		if val.Skiplist() {
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
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("object"))
	}
	if !checkRedisSubcommand(ctx, "object", nil) {
		return nil
	}
	if strings.EqualFold(ctx.ArgString(0), "HELP") {
		return ctx.WriteArray(helpReply(objectHelp))
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
		if sv, ok := entry.Value.(*store.StringValue); ok && ctx.Store.SharesIntegers() {
			if n, ok := parseRedisInt(sv.Data); ok && n >= 0 && n < sharedIntegers {
				return ctx.WriteInteger(math.MaxInt32)
			}
		}
		return ctx.WriteInteger(1)
	default:
		return ctx.Writer.WriteError("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try OBJECT HELP.")
	}
}

func cmdMEMORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if !checkRedisSubcommand(ctx, "memory", nil) {
		return nil
	}

	switch strings.ToUpper(ctx.ArgString(0)) {
	case "USAGE":
		return cmdMemoryUsage(ctx)
	case "STATS":
		return cmdMemoryStats(ctx)
	case "MALLOC-STATS":
		return ctx.WriteSimpleString("allocator: go runtime")
	case "DOCTOR":
		return ctx.WriteVerbatim(memoryDoctorReport(ctx))
	case "PURGE":
		debug.FreeOSMemory()
		return ctx.WriteOK()
	default:
		return ctx.WriteArray(helpReply(memoryHelp))
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
			continue
		}
		return ctx.WriteError(ErrSyntaxError)
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
	ms := readGoMemory()
	_, evalBytes, _, _, functionBytes := scriptMemory(ctx)
	allocated, active, resident := int64(ms.heapAlloc), int64(ms.heapAlloc+ms.heapUnused), int64(ms.mapped)
	ratio := func(a, b int64) float64 {
		if b <= 0 {
			return 0
		}
		return float64(a) / float64(b)
	}
	double := func(f float64) *resp.Value { return resp.DoubleValue(strconv.FormatFloat(f, 'f', -1, 64)) }
	integer := resp.IntegerValue
	out := []*resp.Value{
		resp.BulkString("peak.allocated"), integer(m.peak),
		resp.BulkString("total.allocated"), integer(m.used),
		resp.BulkString("startup.allocated"), integer(0),
		resp.BulkString("replication.backlog"), integer(0),
		resp.BulkString("clients.slaves"), integer(0),
		resp.BulkString("clients.normal"), integer(0),
		resp.BulkString("cluster.links"), integer(0),
		resp.BulkString("aof.buffer"), integer(0),
		resp.BulkString("lua.caches"), integer(evalBytes),
		resp.BulkString("functions.caches"), integer(functionBytes),
	}
	// The per-database overhead is what Redis' dictionaries would take for
	// the same keys: a bucket array sized to a power of two and an entry per
	// key.
	dictBytes := func(n int64) int64 {
		if n == 0 {
			return 0
		}
		return 8<<bits.Len64(uint64(n-1)) + 24*n
	}
	var overhead int64
	ctx.databases().ForEach(func(index int, s *store.Store) {
		n := s.KeyCount()
		if n == 0 {
			return
		}
		expires, _ := s.ExpiryStats()
		main, exp := dictBytes(n), dictBytes(expires)
		overhead += main + exp
		out = append(out, resp.BulkString("db."+strconv.Itoa(index)), resp.PairsValue([]*resp.Value{
			resp.BulkString("overhead.hashtable.main"), integer(main),
			resp.BulkString("overhead.hashtable.expires"), integer(exp),
		}))
	})
	dataset := max(m.used-overhead, 0)
	perKey, datasetPerc, peakPerc := int64(0), 0.0, 100.0
	if keys > 0 {
		perKey = m.used / keys
	}
	if m.used > 0 {
		datasetPerc = float64(dataset) * 100 / float64(m.used)
	}
	if m.peak > 0 {
		peakPerc = float64(m.used) * 100 / float64(m.peak)
	}
	out = append(out,
		resp.BulkString("overhead.db.hashtable.lut"), integer(0),
		resp.BulkString("overhead.db.hashtable.rehashing"), integer(0),
		resp.BulkString("overhead.total"), integer(overhead),
		resp.BulkString("db.dict.rehashing.count"), integer(0),
		resp.BulkString("keys.count"), integer(keys),
		resp.BulkString("keys.bytes-per-key"), integer(perKey),
		resp.BulkString("dataset.bytes"), integer(dataset),
		resp.BulkString("dataset.percentage"), double(datasetPerc),
		resp.BulkString("peak.percentage"), double(peakPerc),
		resp.BulkString("allocator.allocated"), integer(allocated),
		resp.BulkString("allocator.active"), integer(active),
		resp.BulkString("allocator.resident"), integer(resident),
		resp.BulkString("allocator.muzzy"), integer(0),
		resp.BulkString("allocator-fragmentation.ratio"), double(ratio(active, allocated)),
		resp.BulkString("allocator-fragmentation.bytes"), integer(active-allocated),
		resp.BulkString("allocator-rss.ratio"), double(ratio(resident, active)),
		resp.BulkString("allocator-rss.bytes"), integer(resident-active),
		resp.BulkString("rss-overhead.ratio"), double(ratio(m.rss, resident)),
		resp.BulkString("rss-overhead.bytes"), integer(m.rss-resident),
		resp.BulkString("fragmentation"), double(m.fragmentation()),
		resp.BulkString("fragmentation.bytes"), integer(m.rss-m.used),
	)
	return ctx.Writer.WriteValue(resp.PairsValue(out))
}

// currentMemoryFigures reads the memory figures and the key count across
// every database.
func currentMemoryFigures(ctx *Context) (memoryFigures, int64) {
	used, keys := ctx.Store.MemUsage(), ctx.Store.KeyCount()
	if dbs := ctx.databases(); dbs != nil {
		used, keys = dbs.MemUsage(), dbs.KeyCount()
	}
	return readMemoryFigures(ctx, used, readGoMemory()), keys
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
