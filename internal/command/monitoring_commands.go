package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterMonitoringCommands(router *Router) {
	router.Register(&CommandDef{Name: "METRICS.GET", Handler: cmdMETRICSGET})
	router.Register(&CommandDef{Name: "METRICS.RESET", Handler: cmdMETRICSRESET})
	router.Register(&CommandDef{Name: "METRICS.CMD", Handler: cmdMETRICSCMD})

	router.Register(&CommandDef{Name: "SLOWLOG.GET", Handler: cmdSLOWLOGGET})
	router.Register(&CommandDef{Name: "SLOWLOG.LEN", Handler: cmdSLOWLOGLEN})
	router.Register(&CommandDef{Name: "SLOWLOG.RESET", Handler: cmdSLOWLOGRESET})
	router.Register(&CommandDef{Name: "SLOWLOG.CONFIG", Handler: cmdSLOWLOGCONFIG})

	router.Register(&CommandDef{Name: "STATS.KEYSPACE", Handler: cmdSTATSKEYSPACE})
	router.Register(&CommandDef{Name: "STATS.MEMORY", Handler: cmdSTATSMEMORY})
	router.Register(&CommandDef{Name: "STATS.CPU", Handler: cmdSTATSCPU})
	router.Register(&CommandDef{Name: "STATS.CLIENTS", Handler: cmdSTATSCLIENTS})
	router.Register(&CommandDef{Name: "STATS.ALL", Handler: cmdSTATSALL})

	router.Register(&CommandDef{Name: "HEALTH.CHECK", Handler: cmdHEALTHCHECK})
	router.Register(&CommandDef{Name: "HEALTH.LIVENESS", Handler: cmdHEALTHLIVENESS})
	router.Register(&CommandDef{Name: "HEALTH.READINESS", Handler: cmdHEALTHREADINESS})
}

var slowLogThreshold = 10 * time.Millisecond

func cmdMETRICSGET(ctx *Context) error {
	snapshot := store.GlobalMetrics.Snapshot()

	results := make([]*resp.Value, 0)
	for k, v := range snapshot {
		if k == "command_stats" {
			continue
		}
		results = append(results, resp.BulkString(k))
		switch val := v.(type) {
		case int64:
			results = append(results, resp.IntegerValue(val))
		case float64:
			results = append(results, resp.BulkString(fmt.Sprintf("%.2f", val)))
		default:
			results = append(results, resp.BulkString(fmt.Sprintf("%v", val)))
		}
	}

	return ctx.WriteArray(results)
}

func cmdMETRICSRESET(ctx *Context) error {
	store.GlobalMetrics.Reset()
	return ctx.WriteOK()
}

func cmdMETRICSCMD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	cmd := strings.ToUpper(ctx.ArgString(0))
	stats := store.GlobalMetrics.GetCommandStats(cmd)
	if stats == nil {
		return ctx.WriteNull()
	}

	latency := stats["latency"].(map[string]interface{})

	results := []*resp.Value{
		resp.BulkString("count"),
		resp.IntegerValue(stats["count"].(int64)),
		resp.BulkString("avg_ns"),
		resp.IntegerValue(latency["avg"].(int64)),
		resp.BulkString("min_ns"),
		resp.IntegerValue(latency["min"].(int64)),
		resp.BulkString("max_ns"),
		resp.IntegerValue(latency["max"].(int64)),
	}

	return ctx.WriteArray(results)
}

func cmdSLOWLOGGET(ctx *Context) error {
	n := 10
	if ctx.ArgCount() >= 1 {
		n = int(parseInt64(ctx.ArgString(0)))
	}

	entries := store.GlobalSlowLog.Get(n)

	results := make([]*resp.Value, len(entries))
	for i, entry := range entries {
		args := make([]*resp.Value, len(entry.Args))
		for j, arg := range entry.Args {
			args[j] = resp.BulkString(arg)
		}

		results[i] = resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(entry.ID),
			resp.IntegerValue(entry.Timestamp.Unix()),
			resp.IntegerValue(entry.Duration.Microseconds()),
			resp.ArrayValue([]*resp.Value{
				resp.BulkString(entry.Command),
			}),
			resp.BulkString(entry.ClientIP),
			resp.BulkString(""),
		})
	}

	return ctx.WriteArray(results)
}

func cmdSLOWLOGLEN(ctx *Context) error {
	return ctx.WriteInteger(int64(store.GlobalSlowLog.Len()))
}

func cmdSLOWLOGRESET(ctx *Context) error {
	store.GlobalSlowLog.Clear()
	return ctx.WriteOK()
}

func cmdSLOWLOGCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	setting := strings.ToUpper(ctx.ArgString(0))
	value := ctx.ArgString(1)

	switch setting {
	case "THRESHOLD":
		ms := parseInt64(value)
		slowLogThreshold = time.Duration(ms) * time.Millisecond
		return ctx.WriteOK()
	case "MAXLEN":
		maxLen := int(parseInt64(value))
		store.GlobalSlowLog.MaxSize = maxLen
		return ctx.WriteOK()
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown slowlog setting"))
	}
}

func cmdSTATSKEYSPACE(ctx *Context) error {
	totalKeys := ctx.Store.KeyCount()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("total_keys"),
		resp.IntegerValue(totalKeys),
		resp.BulkString("string_keys"),
		resp.IntegerValue(0),
		resp.BulkString("hash_keys"),
		resp.IntegerValue(0),
		resp.BulkString("list_keys"),
		resp.IntegerValue(0),
		resp.BulkString("set_keys"),
		resp.IntegerValue(0),
		resp.BulkString("zset_keys"),
		resp.IntegerValue(0),
		resp.BulkString("expires"),
		resp.IntegerValue(0),
		resp.BulkString("avg_ttl"),
		resp.IntegerValue(0),
	})
}

func cmdSTATSMEMORY(ctx *Context) error {
	usedMemory := ctx.Store.MemUsage()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("used_memory"),
		resp.IntegerValue(usedMemory),
		resp.BulkString("used_memory_human"),
		resp.BulkString(formatBytes(usedMemory)),
		resp.BulkString("peak_memory"),
		resp.IntegerValue(0),
		resp.BulkString("total_allocated"),
		resp.IntegerValue(0),
		resp.BulkString("fragmentation_ratio"),
		resp.BulkString("0.00"),
	})
}

func cmdSTATSCPU(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("used_cpu_sys"),
		resp.BulkString("0.00"),
		resp.BulkString("used_cpu_user"),
		resp.BulkString("0.00"),
	})
}

func cmdSTATSCLIENTS(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("connected_clients"),
		resp.IntegerValue(store.GlobalMetrics.ActiveConnections.Load()),
		resp.BulkString("total_connections"),
		resp.IntegerValue(store.GlobalMetrics.TotalConnections.Load()),
		resp.BulkString("blocked_clients"),
		resp.IntegerValue(0),
	})
}

func cmdSTATSALL(ctx *Context) error {
	metrics := store.GlobalMetrics.Snapshot()
	totalKeys := ctx.Store.KeyCount()
	usedMemory := ctx.Store.MemUsage()

	results := []*resp.Value{
		resp.BulkString("uptime_seconds"),
		resp.IntegerValue(metrics["uptime_seconds"].(int64)),
		resp.BulkString("total_connections"),
		resp.IntegerValue(metrics["total_connections"].(int64)),
		resp.BulkString("active_connections"),
		resp.IntegerValue(metrics["active_connections"].(int64)),
		resp.BulkString("total_commands"),
		resp.IntegerValue(metrics["total_commands"].(int64)),
		resp.BulkString("total_reads"),
		resp.IntegerValue(metrics["total_reads"].(int64)),
		resp.BulkString("total_writes"),
		resp.IntegerValue(metrics["total_writes"].(int64)),
		resp.BulkString("total_hits"),
		resp.IntegerValue(metrics["total_hits"].(int64)),
		resp.BulkString("total_misses"),
		resp.IntegerValue(metrics["total_misses"].(int64)),
		resp.BulkString("hit_rate"),
		resp.BulkString(fmt.Sprintf("%.2f", metrics["hit_rate"])),
		resp.BulkString("total_keys"),
		resp.IntegerValue(totalKeys),
		resp.BulkString("used_memory"),
		resp.IntegerValue(usedMemory),
		resp.BulkString("used_memory_human"),
		resp.BulkString(formatBytes(usedMemory)),
		resp.BulkString("bytes_in"),
		resp.IntegerValue(metrics["bytes_in"].(int64)),
		resp.BulkString("bytes_out"),
		resp.IntegerValue(metrics["bytes_out"].(int64)),
	}

	return ctx.WriteArray(results)
}

func cmdHEALTHCHECK(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("status"),
		resp.BulkString("ok"),
		resp.BulkString("timestamp"),
		resp.BulkString(time.Now().UTC().Format(time.RFC3339)),
		resp.BulkString("uptime_seconds"),
		resp.IntegerValue(int64(time.Since(store.GlobalMetrics.StartTime).Seconds())),
	})
}

func cmdHEALTHLIVENESS(ctx *Context) error {
	return ctx.WriteSimpleString("OK")
}

func cmdHEALTHREADINESS(ctx *Context) error {
	return ctx.WriteSimpleString("OK")
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func init() {
	_ = slowLogThreshold.String()
}
