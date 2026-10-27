package command

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/version"
)

// infoRedisVersion is the Redis version CacheStorm is compatible with, which
// INFO reports as redis_version so clients that gate features on it work.
const infoRedisVersion = "7.4.0"

// ServerStatus is the live server state INFO reports.
type ServerStatus struct {
	RunID            string
	Started          time.Time
	ConfigFile       string
	ConnectedClients int64
	BlockedClients   int64
	MaxClients       int64
	OpsPerSec        int64
	InputKbps        float64
	OutputKbps       float64
	RejectedConns    int64
}

// SetServerStatus installs the function INFO reads the live server state
// from.
func (r *Router) SetServerStatus(fn func() ServerStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serverStatus = fn
}

func (r *Router) getServerStatus() ServerStatus {
	r.mu.RLock()
	fn := r.serverStatus
	r.mu.RUnlock()
	if fn == nil {
		return ServerStatus{Started: store.GlobalMetrics.StartTime}
	}
	return fn()
}

type infoWriter struct {
	sb strings.Builder
}

func (w *infoWriter) section(name string) {
	w.sb.WriteString("# ")
	w.sb.WriteString(name)
	w.sb.WriteString("\r\n")
}

func (w *infoWriter) field(name, value string) {
	w.sb.WriteString(name)
	w.sb.WriteByte(':')
	w.sb.WriteString(value)
	w.sb.WriteString("\r\n")
}

func (w *infoWriter) int(name string, v int64) { w.field(name, strconv.FormatInt(v, 10)) }

func (w *infoWriter) end() { w.sb.WriteString("\r\n") }

// humanBytes formats n as Redis' bytesToHuman does.
func humanBytes(n int64) string {
	f := float64(n)
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + "B"
	case n < 1<<20:
		return strconv.FormatFloat(f/(1<<10), 'f', 2, 64) + "K"
	case n < 1<<30:
		return strconv.FormatFloat(f/(1<<20), 'f', 2, 64) + "M"
	case n < 1<<40:
		return strconv.FormatFloat(f/(1<<30), 'f', 2, 64) + "G"
	default:
		return strconv.FormatFloat(f/(1<<40), 'f', 2, 64) + "T"
	}
}

func infoServer(w *infoWriter, st ServerStatus, port int, mode string) {
	uptime := int64(time.Since(st.Started) / time.Second)
	if st.Started.IsZero() {
		uptime = 0
	}
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	w.section("Server")
	w.field("redis_version", infoRedisVersion)
	w.field("redis_git_sha1", "00000000")
	w.field("redis_git_dirty", "0")
	w.field("redis_mode", mode)
	w.field("cachestorm_version", version.Version)
	w.field("os", runtime.GOOS+" "+runtime.GOARCH)
	w.field("arch_bits", "64")
	w.field("multiplexing_api", "goroutines")
	w.field("go_version", runtime.Version())
	w.int("process_id", int64(os.Getpid()))
	w.field("run_id", st.RunID)
	w.int("tcp_port", int64(port))
	w.int("server_time_usec", time.Now().UnixMicro())
	w.int("uptime_in_seconds", uptime)
	w.int("uptime_in_days", uptime/86400)
	w.field("hz", "10")
	w.field("executable", exe)
	w.field("config_file", st.ConfigFile)
	w.end()
}

func infoClients(w *infoWriter, st ServerStatus) {
	w.section("Clients")
	w.int("connected_clients", st.ConnectedClients)
	w.int("maxclients", st.MaxClients)
	w.int("blocked_clients", st.BlockedClients)
	w.end()
}

func infoMemory(w *infoWriter, ctx *Context) {
	used := ctx.Store.MemUsage()
	if dbs := ctx.databases(); dbs != nil {
		used = dbs.MemUsage()
	}
	limit, policy := ctx.Store.MemoryLimit()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	mem := readMemoryFigures(ctx, used, &ms)
	w.section("Memory")
	w.int("used_memory", used)
	w.field("used_memory_human", humanBytes(used))
	w.int("used_memory_rss", mem.rss)
	w.field("used_memory_rss_human", humanBytes(mem.rss))
	w.int("used_memory_peak", mem.peak)
	w.field("used_memory_peak_human", humanBytes(mem.peak))
	w.field("used_memory_peak_perc", mem.peakPerc())
	w.int("used_memory_heap", int64(ms.HeapAlloc))
	w.field("used_memory_heap_human", humanBytes(int64(ms.HeapAlloc)))
	w.int("total_system_memory", mem.system)
	w.field("total_system_memory_human", humanBytes(mem.system))
	w.int("maxmemory", limit)
	w.field("maxmemory_human", humanBytes(limit))
	w.field("maxmemory_policy", policy.String())
	w.field("mem_fragmentation_ratio", strconv.FormatFloat(mem.fragmentation(), 'f', 2, 64))
	w.int("mem_fragmentation_bytes", mem.rss-used)
	w.field("mem_allocator", "go")
	w.int("gc_cycles", int64(ms.NumGC))
	w.end()
}

func infoStats(w *infoWriter, st ServerStatus) {
	m := store.GlobalMetrics
	w.section("Stats")
	w.int("total_connections_received", m.TotalConnections.Load())
	w.int("total_commands_processed", m.TotalCommands.Load())
	w.int("instantaneous_ops_per_sec", st.OpsPerSec)
	w.int("total_net_input_bytes", m.TotalBytesIn.Load())
	w.int("total_net_output_bytes", m.TotalBytesOut.Load())
	w.field("instantaneous_input_kbps", strconv.FormatFloat(st.InputKbps, 'f', 2, 64))
	w.field("instantaneous_output_kbps", strconv.FormatFloat(st.OutputKbps, 'f', 2, 64))
	w.int("rejected_connections", st.RejectedConns)
	w.int("expired_subkeys", m.ExpiredSubkeys.Load())
	w.int("expired_keys", m.TotalExpirations.Load())
	w.int("evicted_keys", m.TotalEvictions.Load())
	w.int("keyspace_hits", m.TotalHits.Load())
	w.int("keyspace_misses", m.TotalMisses.Load())
	w.int("total_error_replies", m.TotalErrors.Load())
	w.int("total_reads_processed", m.TotalReads.Load())
	w.int("total_writes_processed", m.TotalWrites.Load())
	w.end()
}

// memoryFigures are the process-level memory numbers INFO memory, MEMORY
// STATS and MEMORY DOCTOR share.
type memoryFigures struct {
	used, rss, peak, system int64
}

func readMemoryFigures(ctx *Context, used int64, ms *runtime.MemStats) memoryFigures {
	m := memoryFigures{used: used, peak: ctx.Store.MemoryPeak(), system: totalSystemMemory()}
	if rss, ok := processRSS(); ok {
		m.rss = rss
	} else {
		m.rss = int64(ms.Sys - ms.HeapReleased)
	}
	if m.peak < used {
		m.peak = used
	}
	return m
}

func (m memoryFigures) peakPerc() string {
	if m.peak == 0 {
		return "100.00%"
	}
	return strconv.FormatFloat(float64(m.used)*100/float64(m.peak), 'f', 2, 64) + "%"
}

// fragmentation is RSS over used memory, as Redis reports it. The Go runtime
// and the server's own structures are in RSS but not in used memory, so a
// nearly empty server reads high.
func (m memoryFigures) fragmentation() float64 {
	if m.used <= 0 {
		return 0
	}
	return float64(m.rss) / float64(m.used)
}
