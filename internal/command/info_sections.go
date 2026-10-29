package command

import (
	"os"
	"runtime"
	"runtime/metrics"
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
	TrackingClients  int64
	TrackingKeys     int64
	TrackingItems    int64
	TrackingPrefixes int64
	SyncFull         int64
	SyncPartialOK    int64
	SyncPartialErr   int64
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

func infoServer(w *infoWriter, st *ServerStatus, port int, mode string) {
	uptime := int64(time.Since(st.Started) / time.Second)
	if st.Started.IsZero() {
		uptime = 0
	}
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	globalConfig.mu.RLock()
	bind := globalConfig.bind
	globalConfig.mu.RUnlock()
	listener := "name=tcp"
	if fields := strings.Fields(bind); len(fields) > 0 {
		for _, addr := range fields {
			listener += ",bind=" + addr
		}
	} else {
		listener += ",bind=*"
	}
	listener += ",port=" + strconv.Itoa(port)
	w.section("Server")
	w.field("redis_version", infoRedisVersion)
	w.field("redis_git_sha1", "00000000")
	w.field("redis_git_dirty", "0")
	w.field("redis_build_id", "0")
	w.field("redis_mode", mode)
	w.field("cachestorm_version", version.Version)
	w.field("os", runtime.GOOS+" "+runtime.GOARCH)
	w.field("arch_bits", "64")
	w.field("monotonic_clock", "POSIX clock_gettime")
	w.field("multiplexing_api", "goroutines")
	w.field("atomicvar_api", "go")
	w.field("gcc_version", "0.0.0")
	w.field("go_version", runtime.Version())
	w.int("process_id", int64(os.Getpid()))
	w.field("process_supervised", "no")
	w.field("run_id", st.RunID)
	w.int("tcp_port", int64(port))
	w.int("server_time_usec", time.Now().UnixMicro())
	w.int("uptime_in_seconds", uptime)
	w.int("uptime_in_days", uptime/86400)
	w.field("hz", "10")
	w.field("configured_hz", "10")
	w.int("lru_clock", time.Now().Unix()&(1<<24-1))
	w.field("executable", exe)
	w.field("config_file", st.ConfigFile)
	w.field("io_threads_active", "0")
	w.field("listener0", listener)
	w.end()
}

func infoClients(w *infoWriter, st *ServerStatus, ctx *Context) {
	var watched, blockingKeys int64
	if dbs := ctx.databases(); dbs != nil {
		for i := 0; i < dbs.Count(); i++ {
			if db := dbs.Existing(i); db != nil {
				watched += db.WatchedKeys()
				blockingKeys += int64(db.KeyNotifier().WaitedKeys())
			}
		}
	}
	_, _, _, pubsubClients := ctx.Store.GetPubSub().Counts()
	w.section("Clients")
	w.int("connected_clients", st.ConnectedClients)
	w.field("cluster_connections", "0")
	w.int("maxclients", st.MaxClients)
	w.field("client_recent_max_input_buffer", "0")
	w.field("client_recent_max_output_buffer", "0")
	w.int("blocked_clients", st.BlockedClients)
	w.int("tracking_clients", st.TrackingClients)
	w.int("pubsub_clients", int64(pubsubClients))
	w.int("watching_clients", watchingClients.Load())
	w.field("clients_in_timeout_table", "0")
	w.int("total_watched_keys", watched)
	w.int("total_blocking_keys", blockingKeys)
	w.field("total_blocking_keys_on_nokey", "0")
	w.end()
}

func infoMemory(w *infoWriter, ctx *Context) {
	used := ctx.Store.MemUsage()
	if dbs := ctx.databases(); dbs != nil {
		used = dbs.MemUsage()
	}
	limit, policy := ctx.Store.MemoryLimit()
	ms := readGoMemory()
	mem := readMemoryFigures(ctx, used, ms)
	scripts, evalBytes, functions, libraries, functionBytes := scriptMemory(ctx)
	ratio := func(a, b int64) string {
		if b <= 0 {
			return "0.00"
		}
		return strconv.FormatFloat(float64(a)/float64(b), 'f', 2, 64)
	}
	allocated, active, resident := int64(ms.heapAlloc), int64(ms.heapAlloc+ms.heapUnused), int64(ms.mapped)
	datasetPerc := "0.00%"
	if used > 0 {
		datasetPerc = "100.00%"
	}
	w.section("Memory")
	w.int("used_memory", used)
	w.field("used_memory_human", humanBytes(used))
	w.int("used_memory_rss", mem.rss)
	w.field("used_memory_rss_human", humanBytes(mem.rss))
	w.int("used_memory_peak", mem.peak)
	w.field("used_memory_peak_human", humanBytes(mem.peak))
	w.field("used_memory_peak_perc", mem.peakPerc())
	w.field("used_memory_overhead", "0")
	w.field("used_memory_startup", "0")
	w.int("used_memory_dataset", used)
	w.field("used_memory_dataset_perc", datasetPerc)
	w.int("used_memory_heap", allocated)
	w.field("used_memory_heap_human", humanBytes(allocated))
	w.int("allocator_allocated", allocated)
	w.int("allocator_active", active)
	w.int("allocator_resident", resident)
	w.field("allocator_muzzy", "0")
	w.int("total_system_memory", mem.system)
	w.field("total_system_memory_human", humanBytes(mem.system))
	w.field("used_memory_lua", "0")
	w.field("used_memory_vm_eval", "0")
	w.field("used_memory_lua_human", "0B")
	w.int("used_memory_scripts_eval", evalBytes)
	w.int("number_of_cached_scripts", scripts)
	w.int("number_of_functions", functions)
	w.int("number_of_libraries", libraries)
	w.field("used_memory_vm_functions", "0")
	w.field("used_memory_vm_total", "0")
	w.field("used_memory_vm_total_human", "0B")
	w.int("used_memory_functions", functionBytes)
	w.int("used_memory_scripts", evalBytes+functionBytes)
	w.field("used_memory_scripts_human", humanBytes(evalBytes+functionBytes))
	w.int("maxmemory", limit)
	w.field("maxmemory_human", humanBytes(limit))
	w.field("maxmemory_policy", policy.String())
	w.field("allocator_frag_ratio", ratio(active, allocated))
	w.int("allocator_frag_bytes", active-allocated)
	w.field("allocator_rss_ratio", ratio(resident, active))
	w.int("allocator_rss_bytes", resident-active)
	w.field("rss_overhead_ratio", ratio(mem.rss, resident))
	w.int("rss_overhead_bytes", mem.rss-resident)
	w.field("mem_fragmentation_ratio", strconv.FormatFloat(mem.fragmentation(), 'f', 2, 64))
	w.int("mem_fragmentation_bytes", mem.rss-used)
	w.field("mem_not_counted_for_evict", "0")
	w.field("mem_replication_backlog", "0")
	w.field("mem_total_replication_buffers", "0")
	w.field("mem_clients_slaves", "0")
	w.field("mem_clients_normal", "0")
	w.field("mem_cluster_links", "0")
	w.field("mem_aof_buffer", "0")
	w.field("mem_allocator", "go")
	w.field("mem_overhead_db_hashtable_rehashing", "0")
	w.field("active_defrag_running", "0")
	w.field("lazyfree_pending_objects", "0")
	w.field("lazyfreed_objects", "0")
	w.int("gc_cycles", int64(ms.numGC))
	w.end()
}

// scriptMemory reports the scripts EVAL cached and the functions loaded,
// with the bytes of their source.
func scriptMemory(ctx *Context) (scripts, evalBytes, functions, libraries, functionBytes int64) {
	scriptEngineMu.RLock()
	e := scriptEngine
	scriptEngineMu.RUnlock()
	if e != nil {
		e.mu.RLock()
		for _, body := range e.scripts {
			scripts++
			evalBytes += int64(len(body))
		}
		e.mu.RUnlock()
	}
	if r := functionRegistryFor(ctx); r != nil {
		r.mu.RLock()
		for _, lib := range r.libraries {
			libraries++
			functions += int64(len(lib.Functions))
			functionBytes += int64(len(lib.Code))
		}
		r.mu.RUnlock()
	}
	return scripts, evalBytes, functions, libraries, functionBytes
}

func infoStats(w *infoWriter, st *ServerStatus, ctx *Context) {
	m := store.GlobalMetrics
	channels, patterns, shards, _ := ctx.Store.GetPubSub().Counts()
	zero := func(names ...string) {
		for _, n := range names {
			w.field(n, "0")
		}
	}
	w.section("Stats")
	w.int("total_connections_received", m.TotalConnections.Load())
	w.int("total_commands_processed", m.TotalCommands.Load())
	w.int("instantaneous_ops_per_sec", st.OpsPerSec)
	w.int("total_net_input_bytes", m.TotalBytesIn.Load())
	w.int("total_net_output_bytes", m.TotalBytesOut.Load())
	zero("total_net_repl_input_bytes", "total_net_repl_output_bytes")
	w.field("instantaneous_input_kbps", strconv.FormatFloat(st.InputKbps, 'f', 2, 64))
	w.field("instantaneous_output_kbps", strconv.FormatFloat(st.OutputKbps, 'f', 2, 64))
	w.field("instantaneous_input_repl_kbps", "0.00")
	w.field("instantaneous_output_repl_kbps", "0.00")
	w.int("rejected_connections", st.RejectedConns)
	w.int("sync_full", st.SyncFull)
	w.int("sync_partial_ok", st.SyncPartialOK)
	w.int("sync_partial_err", st.SyncPartialErr)
	w.int("expired_subkeys", m.ExpiredSubkeys.Load())
	w.int("expired_keys", m.TotalExpirations.Load())
	w.field("expired_stale_perc", "0.00")
	zero("expired_time_cap_reached_count", "expire_cycle_cpu_milliseconds")
	w.int("evicted_keys", m.TotalEvictions.Load())
	zero("evicted_clients", "evicted_scripts", "total_eviction_exceeded_time", "current_eviction_exceeded_time")
	w.int("keyspace_hits", m.TotalHits.Load())
	w.int("keyspace_misses", m.TotalMisses.Load())
	w.int("pubsub_channels", int64(channels))
	w.int("pubsub_patterns", int64(patterns))
	w.int("pubsubshard_channels", int64(shards))
	zero("latest_fork_usec", "total_forks", "migrate_cached_sockets", "slave_expires_tracked_keys",
		"active_defrag_hits", "active_defrag_misses", "active_defrag_key_hits", "active_defrag_key_misses",
		"total_active_defrag_time", "current_active_defrag_time")
	w.int("tracking_total_keys", st.TrackingKeys)
	w.int("tracking_total_items", st.TrackingItems)
	w.int("tracking_total_prefixes", st.TrackingPrefixes)
	w.field("unexpected_error_replies", "0")
	w.int("total_error_replies", commandStats.totalErrors.Load())
	w.field("dump_payload_sanitizations", "0")
	w.int("total_reads_processed", m.ReadEvents.Load())
	w.int("total_writes_processed", m.WriteEvents.Load())
	zero("io_threaded_reads_processed", "io_threaded_writes_processed",
		"client_query_buffer_limit_disconnections", "client_output_buffer_limit_disconnections",
		"reply_buffer_shrinks", "reply_buffer_expands", "eventloop_cycles", "eventloop_duration_sum",
		"eventloop_duration_cmd_sum", "instantaneous_eventloop_cycles_per_sec", "instantaneous_eventloop_duration_usec")
	w.int("acl_access_denied_auth", aclDenied.auth.Load())
	w.int("acl_access_denied_cmd", aclDenied.cmd.Load())
	w.int("acl_access_denied_key", aclDenied.key.Load())
	w.int("acl_access_denied_channel", aclDenied.channel.Load())
	w.end()
}

// memoryFigures are the process-level memory numbers INFO memory, MEMORY
// STATS and MEMORY DOCTOR share.
type memoryFigures struct {
	used, rss, peak, system int64
}

// goMemory is what INFO reads from the Go runtime, through runtime/metrics,
// which unlike runtime.ReadMemStats does not stop the world.
type goMemory struct {
	heapAlloc, heapUnused, mapped, numGC uint64
}

func readGoMemory() goMemory {
	s := []metrics.Sample{
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/memory/classes/total:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
		{Name: "/gc/cycles/total:gc-cycles"},
		{Name: "/memory/classes/heap/unused:bytes"},
	}
	metrics.Read(s)
	u := func(i int) uint64 {
		if s[i].Value.Kind() == metrics.KindUint64 {
			return s[i].Value.Uint64()
		}
		return 0
	}
	return goMemory{heapAlloc: u(0), mapped: u(1) - u(2), numGC: u(3), heapUnused: u(4)}
}

func readMemoryFigures(ctx *Context, used int64, ms goMemory) memoryFigures {
	m := memoryFigures{used: used, peak: ctx.Store.MemoryPeak(), system: totalSystemMemory()}
	if rss, ok := processRSS(); ok {
		m.rss = rss
	} else {
		m.rss = int64(ms.mapped)
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
