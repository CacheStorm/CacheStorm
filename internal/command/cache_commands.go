package command

import (
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterCacheCommands(router *Router) {
	router.Register(&CommandDef{Name: "CACHE.BULKGET", Handler: cmdCacheBulkGet})
	router.Register(&CommandDef{Name: "CACHE.BULKDEL", Handler: cmdCacheBulkDel})
	router.Register(&CommandDef{Name: "CACHE.STATS", Handler: cmdCacheStats})
	router.Register(&CommandDef{Name: "CACHE.PREFETCH", Handler: cmdCachePrefetch})
	router.Register(&CommandDef{Name: "CACHE.EXPORT", Handler: cmdCacheExport})
	router.Register(&CommandDef{Name: "CACHE.IMPORT", Handler: cmdCacheImport})
	router.Register(&CommandDef{Name: "CACHE.CLEAR", Handler: cmdCacheClear})
}

func cmdCacheBulkGet(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	limit := int64(1000)
	if ctx.ArgCount() >= 2 {
		var err error
		limit, err = strconv.ParseInt(ctx.ArgString(1), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	keys := ctx.Store.Keys()
	results := make([]*resp.Value, 0)

	for _, key := range keys {
		if matchPattern(key, pattern) {
			entry, exists := ctx.Store.Get(key)
			if exists {
				var value []byte
				switch v := entry.Value.(type) {
				case *store.StringValue:
					value = v.Data
				default:
					continue
				}
				results = append(results, resp.ArrayValue([]*resp.Value{
					resp.BulkString(key),
					resp.BulkBytes(value),
				}))
				if int64(len(results)) >= limit {
					break
				}
			}
		}
	}

	return ctx.WriteArray(results)
}

func cmdCacheBulkDel(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	limit := int64(0)
	if ctx.ArgCount() >= 2 {
		var err error
		limit, err = strconv.ParseInt(ctx.ArgString(1), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	keys := ctx.Store.Keys()
	deleted := int64(0)

	for _, key := range keys {
		if matchPattern(key, pattern) {
			if ctx.Store.Delete(key) {
				deleted++
				if limit > 0 && deleted >= limit {
					break
				}
			}
		}
	}

	return ctx.WriteInteger(deleted)
}

func cmdCacheStats(ctx *Context) error {
	stats := make([]*resp.Value, 0)

	keys := ctx.Store.Keys()
	totalKeys := int64(len(keys))
	memUsage := ctx.Store.MemUsage()

	typeCounts := make(map[string]int64)
	totalTTL := int64(0)
	keysWitTTL := int64(0)
	accessTotal := uint64(0)

	for _, key := range keys {
		entry, exists := ctx.Store.Get(key)
		if exists {
			typeCounts[entry.Value.Type().String()]++
			ttl := ctx.Store.GetTTL(key)
			if ttl > 0 {
				totalTTL += int64(ttl / time.Second)
				keysWitTTL++
			}
			accessTotal += entry.AccessCount
		}
	}

	avgTTL := int64(0)
	if keysWitTTL > 0 {
		avgTTL = totalTTL / keysWitTTL
	}

	avgAccess := int64(0)
	if totalKeys > 0 {
		avgAccess = int64(accessTotal) / totalKeys
	}

	stats = append(stats,
		resp.BulkString("total_keys"), resp.IntegerValue(totalKeys),
		resp.BulkString("memory_usage_bytes"), resp.IntegerValue(memUsage),
		resp.BulkString("avg_key_size"), resp.IntegerValue(safeDiv(memUsage, totalKeys)),
		resp.BulkString("keys_with_ttl"), resp.IntegerValue(keysWitTTL),
		resp.BulkString("avg_ttl_seconds"), resp.IntegerValue(avgTTL),
		resp.BulkString("total_accesses"), resp.IntegerValue(int64(accessTotal)),
		resp.BulkString("avg_access_per_key"), resp.IntegerValue(avgAccess),
	)

	for typeName, count := range typeCounts {
		stats = append(stats,
			resp.BulkString("keys_"+typeName), resp.IntegerValue(count),
		)
	}

	tagIndex := ctx.Store.GetTagIndex()
	if tagIndex != nil {
		stats = append(stats,
			resp.BulkString("total_tags"), resp.IntegerValue(int64(len(tagIndex.Tags()))),
		)
	}

	return ctx.WriteArray(stats)
}

func safeDiv(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func cmdCachePrefetch(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, 0, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		keys = append(keys, ctx.ArgString(i))
	}

	prefetched := int64(0)
	for _, key := range keys {
		if _, exists := ctx.Store.Get(key); exists {
			prefetched++
		}
	}

	return ctx.WriteInteger(prefetched)
}

func cmdCacheExport(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	keys := ctx.Store.Keys()

	results := make([]*resp.Value, 0)
	for _, key := range keys {
		if matchPattern(key, pattern) {
			entry, exists := ctx.Store.Get(key)
			if exists {
				entryData := []*resp.Value{
					resp.BulkString("key"), resp.BulkString(key),
					resp.BulkString("type"), resp.BulkString(entry.Value.Type().String()),
				}

				switch v := entry.Value.(type) {
				case *store.StringValue:
					entryData = append(entryData,
						resp.BulkString("value"), resp.BulkBytes(v.Data),
					)
				case *store.HashValue:
					fields := make([]*resp.Value, 0)
					v.RLock()
					for k, val := range v.Fields {
						fields = append(fields, resp.BulkString(k), resp.BulkBytes(val))
					}
					v.RUnlock()
					entryData = append(entryData, resp.BulkString("value"), resp.ArrayValue(fields))
				case *store.ListValue:
					elements := make([]*resp.Value, 0)
					v.RLock()
					for _, elem := range v.Elements {
						elements = append(elements, resp.BulkBytes(elem))
					}
					v.RUnlock()
					entryData = append(entryData, resp.BulkString("value"), resp.ArrayValue(elements))
				case *store.SetValue:
					members := make([]*resp.Value, 0)
					v.RLock()
					for member := range v.Members {
						members = append(members, resp.BulkString(member))
					}
					v.RUnlock()
					entryData = append(entryData, resp.BulkString("value"), resp.ArrayValue(members))
				case *store.SortedSetValue:
					members := make([]*resp.Value, 0)
					v.RLock()
					for member, score := range v.Members {
						members = append(members, resp.ArrayValue([]*resp.Value{
							resp.BulkString(member),
							resp.BulkString(strconv.FormatFloat(score, 'f', -1, 64)),
						}))
					}
					v.RUnlock()
					entryData = append(entryData, resp.BulkString("value"), resp.ArrayValue(members))
				}

				if len(entry.Tags) > 0 {
					tags := make([]*resp.Value, 0)
					for _, tag := range entry.Tags {
						tags = append(tags, resp.BulkString(tag))
					}
					entryData = append(entryData, resp.BulkString("tags"), resp.ArrayValue(tags))
				}

				ttl := ctx.Store.GetTTL(key)
				if ttl > 0 {
					entryData = append(entryData,
						resp.BulkString("ttl_ms"), resp.IntegerValue(int64(ttl/time.Millisecond)),
					)
				}

				results = append(results, resp.ArrayValue(entryData))
			}
		}
	}

	return ctx.WriteArray(results)
}

func cmdCacheImport(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	format := strings.ToUpper(ctx.ArgString(0))
	replace := false
	if ctx.ArgCount() >= 2 && strings.ToUpper(ctx.ArgString(1)) == "REPLACE" {
		replace = true
	}

	_ = format
	_ = replace

	return ctx.WriteOK()
}

func cmdCacheClear(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)

	if pattern == "*" {
		ctx.Store.Flush()
		return ctx.WriteInteger(ctx.Store.KeyCount())
	}

	keys := ctx.Store.Keys()
	deleted := int64(0)

	for _, key := range keys {
		if matchPattern(key, pattern) {
			if ctx.Store.Delete(key) {
				deleted++
			}
		}
	}

	return ctx.WriteInteger(deleted)
}
