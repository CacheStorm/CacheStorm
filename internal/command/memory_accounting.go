package command

import (
	"errors"
	"runtime/metrics"
	"strings"
	"sync/atomic"
	"time"
)

// errOOM is Redis' reply to a command that may grow memory while used memory
// is over maxmemory.
var errOOM = errors.New("OOM command not allowed when used memory > 'maxmemory'.") //nolint:staticcheck // Redis reply text, clients match it verbatim

// errExecAbortOOM is the reply to an EXEC refused for memory: Redis discards
// the transaction and reports why.
var errExecAbortOOM = errors.New("EXECABORT Transaction discarded because of: " + errOOM.Error()) //nolint:staticcheck // Redis reply text

var (
	readonlyCommands = make(map[string]bool)
	denyOOMCommands  = make(map[string]bool)
)

func init() {
	for name, spec := range redisCommandSpecs {
		for _, f := range spec.Flags {
			switch f {
			case "readonly":
				readonlyCommands[name] = true
			case "denyoom":
				denyOOMCommands[name] = true
			}
		}
	}
}

// reaccountWrittenKeys charges the memory the command's keys gained or lost
// in place. Store.Set accounts whole values only, so without this a list,
// hash, set, sorted set or stream grown by RPUSH, HSET, SADD, ZADD or XADD
// was never charged and maxmemory did not bound it. Every command Redis does
// not flag readonly is covered, including CacheStorm's own; a key that did
// not change costs a read-locked lookup.
func reaccountWrittenKeys(ctx *Context) {
	if ctx.Store == nil || readonlyCommands[ctx.Command] {
		return
	}
	ctx.Store.Reaccount(ctx.CommandKeys()...)
}

// overMemoryFor reports whether cmd must be refused because used memory is
// over maxmemory and eviction could not bring it back, as Redis refuses
// commands flagged denyoom before running them.
//
// An extension command that may grow memory is refused the same way, and
// also when the live heap is past the limit: its registry is not counted in
// used_memory, so it could otherwise grow until the process runs out of
// memory.
func overMemoryFor(ctx *Context, cmd string) bool {
	if ctx.Store == nil {
		return false
	}
	if denyOOMCommands[cmd] {
		return ctx.Store.CheckWriteMemory() != nil
	}
	if !extensionMayGrow(cmd) {
		return false
	}
	mt := ctx.Store.MemoryTracker()
	if mt == nil || mt.Max() <= 0 {
		return false
	}
	return ctx.Store.CheckWriteMemory() != nil || heapOverLimit(mt.Max())
}

// execDeniedForMemory reports whether an EXEC must be refused for memory: a
// transaction that queued any denyoom command is, like Redis does.
func execDeniedForMemory(ctx *Context) bool {
	if ctx.Store == nil || ctx.Transaction == nil {
		return false
	}
	for _, qc := range ctx.Transaction.GetQueued() {
		if overMemoryFor(ctx, qc.cmd) {
			return true
		}
	}
	return false
}

// extensionKeepVerbs are the extension command verbs (the part after the
// last dot) that read or remove state. Every other extension command may
// grow it, and most keep it in registries outside the keyspace, which
// used_memory does not count.
var extensionKeepVerbs = map[string]bool{
	"GET": true, "MGET": true, "GETALL": true, "BULKGET": true, "LIST": true, "INFO": true,
	"STATUS": true, "STATS": true, "CHECK": true, "COUNT": true, "LEN": true, "LENGTH": true,
	"SIZE": true, "CARD": true, "EXISTS": true, "MEXISTS": true, "PEEK": true, "SEARCH": true,
	"QUERY": true, "READ": true, "RANGE": true, "REVRANGE": true, "KEYS": true, "VALUES": true,
	"MIN": true, "MAX": true, "SUM": true, "AVG": true, "MEAN": true, "MEDIAN": true,
	"CURRENT": true, "STATE": true, "HISTORY": true, "METRICS": true, "USAGE": true,
	"VERSION": true, "VALIDATE": true, "VERIFY": true, "EXPORT": true, "TTL": true, "TYPE": true,
	"ENCODE": true, "DECODE": true, "POP": true, "DEL": true, "DELETE": true, "MDEL": true,
	"BULKDEL": true, "REMOVE": true, "CLEAR": true, "PURGE": true, "RESET": true,
	"RESETALL": true, "RELEASE": true, "UNLOCK": true, "REVOKE": true, "CANCEL": true,
	"DEREGISTER": true, "DROPINDEX": true, "TRIM": true, "FLUSH": true, "CLEANUP": true,
	"DELNODE": true, "DELEDGE": true, "ALIASDEL": true, "ACK": true, "CLOSE": true,
}

// extensionMayGrow reports whether cmd is an extension command (MODULE.VERB,
// not one of Redis') that may grow memory.
func extensionMayGrow(cmd string) bool {
	dot := strings.LastIndexByte(cmd, '.')
	if dot < 0 {
		return false
	}
	if _, redis := redisCommandSpecs[cmd]; redis {
		return false
	}
	return !extensionKeepVerbs[cmd[dot+1:]]
}

// heapSample caches the live heap size so the check costs a runtime/metrics
// read at most every heapSampleEvery.
var heapSample struct {
	at   atomic.Int64
	live atomic.Uint64
}

const heapSampleEvery = int64(100 * time.Millisecond)

func liveHeap() uint64 {
	now := time.Now().UnixNano()
	if last := heapSample.at.Load(); now-last >= heapSampleEvery && heapSample.at.CompareAndSwap(last, now) {
		s := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
		metrics.Read(s)
		if s[0].Value.Kind() == metrics.KindUint64 {
			heapSample.live.Store(s[0].Value.Uint64())
		}
	}
	return heapSample.live.Load()
}

// heapOverLimit reports whether the live heap is past the room maxmemory
// leaves for the dataset (the Go memory limit the server derives from it),
// which only state used_memory does not count can cause.
func heapOverLimit(max int64) bool {
	return max > 0 && liveHeap() > uint64(max+max/4+64<<20)
}
