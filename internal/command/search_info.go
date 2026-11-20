package command

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/search"
)

// RediSearch's query statistics, for INFO search: the queries run, their
// time, the errors they met (a standalone server counts them as the
// coordinator's) and the dialects used.
var searchQueryStats struct {
	commands, processed, nanos atomic.Int64
	syntaxErrors, argErrors    atomic.Int64
	dialects                   atomic.Uint32
}

// searchVersion is the RediSearch release INFO search reports, the one
// Redis 8.2.11 bundles.
const searchVersion = "8.2.13"

// countSearchQuery wraps FT.SEARCH, FT.AGGREGATE, FT.PROFILE, FT.CURSOR and
// FT.SPELLCHECK with RediSearch's accounting: a query that ran counts, with
// its time, and marks its dialect used; one refused counts as a syntax or
// an argument error. FT.CURSOR counts its reads, FT.SPELLCHECK only its
// dialect.
func countSearchQuery(name string, h func(*Context) error) func(*Context) error {
	return func(ctx *Context) error {
		start := time.Now()
		errs := ctx.Writer.Errors()
		ctx.lastError = nil
		err := h(ctx)
		failed := err != nil || ctx.Writer.Errors() != errs
		switch name {
		case "FT.CURSOR":
			if !failed && strings.EqualFold(ctx.ArgString(0), "READ") {
				searchQueryStats.commands.Add(1)
				searchQueryStats.processed.Add(1)
				searchQueryStats.nanos.Add(int64(time.Since(start)))
			}
			return err
		case "FT.SPELLCHECK":
			if !failed {
				markSearchDialect(ctx, 2)
			}
			return err
		}
		if failed {
			msg := ""
			if err != nil {
				msg = err.Error()
			} else if ctx.lastError != nil {
				msg = ctx.lastError.Error()
			}
			switch {
			case strings.HasPrefix(msg, "Syntax error"):
				searchQueryStats.syntaxErrors.Add(1)
			case strings.HasPrefix(msg, "No such index") || strings.Contains(msg, "no such index") || msg == "":
			default:
				searchQueryStats.argErrors.Add(1)
			}
			return err
		}
		searchQueryStats.commands.Add(1)
		searchQueryStats.processed.Add(1)
		searchQueryStats.nanos.Add(int64(time.Since(start)))
		from := 2
		if name == "FT.PROFILE" {
			for i := 1; i < ctx.ArgCount(); i++ {
				if strings.EqualFold(ctx.ArgString(i), "QUERY") {
					from = i + 2
					break
				}
			}
		}
		markSearchDialect(ctx, from)
		return err
	}
}

// markSearchDialect records the dialect of the query whose options start at
// argument from: its last DIALECT, or the default one.
func markSearchDialect(ctx *Context, from int) {
	d := search.DefaultDialect()
	for i := from; i+1 < ctx.ArgCount(); i++ {
		if strings.EqualFold(ctx.ArgString(i), "DIALECT") {
			if n, err := strconv.Atoi(ctx.ArgString(i + 1)); err == nil {
				d = n
			}
		}
	}
	if d >= 1 && d <= 4 {
		searchQueryStats.dialects.Or(1 << (d - 1))
	}
}

// infoSearch writes RediSearch's INFO sections. As RediSearch does, with no
// index and search-_info-on-zero-indexes off only the version, the index
// counts and the configuration are given.
func infoSearch(w *infoWriter, ctx *Context) {
	indexes := searchIndexesFor(ctx).All()
	w.section("search_version")
	w.field("search_version", searchVersion)
	w.field("search_redis_version", infoRedisVersion+" - oss")
	w.end()

	var docs, used, smallest, largest int64
	for i, idx := range indexes {
		n, _, _ := idx.Summary(ctx.Store)
		docs += int64(n)
		m := idx.MemoryUsage()
		used += m
		if i == 0 || m < smallest {
			smallest = m
		}
		largest = max(largest, m)
	}
	w.section("search_indexes")
	w.int("search_number_of_indexes", int64(len(indexes)))
	for _, name := range []string{"search_number_of_active_indexes", "search_number_of_active_indexes_running_queries",
		"search_number_of_active_indexes_indexing", "search_total_active_write_threads"} {
		w.int(name, 0)
	}
	w.field("search_total_indexing_time", "0")
	w.int("search_total_num_docs_in_indexes", docs)
	w.end()

	configs := configValues(ctx)
	if len(indexes) > 0 || configs["search-_info-on-zero-indexes"] == "yes" {
		infoSearchFields(w, indexes)
		mb := func(n int64) string { return strconv.FormatFloat(float64(n)/(1<<20), 'g', 17, 64) }
		w.section("search_memory")
		w.int("search_used_memory_indexes", used)
		w.field("search_used_memory_indexes_human", mb(used))
		w.int("search_smallest_memory_index", smallest)
		w.field("search_smallest_memory_index_human", mb(smallest))
		w.int("search_largest_memory_index", largest)
		w.field("search_largest_memory_index_human", mb(largest))
		w.int("search_used_memory_vector_index", 0)
		w.end()

		idle, total := searchIndexesFor(ctx).CursorTotals()
		w.section("search_cursors")
		w.int("search_global_idle_user", int64(idle))
		w.int("search_global_idle_internal", 0)
		w.int("search_global_total_user", int64(total))
		w.int("search_global_total_internal", 0)
		w.end()

		zeros := func(section string, names ...string) {
			w.section(section)
			for _, n := range names {
				w.int(n, 0)
			}
			w.end()
		}
		zeros("search_garbage_collector", "search_gc_bytes_collected", "search_gc_total_cycles", "search_gc_total_ms_run",
			"search_gc_total_docs_not_collected", "search_gc_marked_deleted_vectors")

		st := &searchQueryStats
		w.section("search_queries")
		w.int("search_total_queries_processed", st.processed.Load())
		w.int("search_total_query_commands", st.commands.Load())
		w.int("search_total_query_execution_time_ms", st.nanos.Load()/int64(time.Millisecond))
		w.int("search_total_active_queries", 0)
		w.end()

		zeros("search_warnings_and_errors", "search_errors_indexing_failures", "search_errors_for_index_with_max_failures",
			"search_OOM_indexing_failures_indexes_count", "search_shard_total_query_errors_syntax",
			"search_shard_total_query_errors_arguments", "search_shard_total_query_errors_timeout",
			"search_shard_total_query_warnings_timeout", "search_shard_total_query_warnings_max_prefix_expansions")
		w.section("search_coordinator_warnings_and_errors")
		w.int("search_coord_total_query_errors_syntax", st.syntaxErrors.Load())
		w.int("search_coord_total_query_errors_arguments", st.argErrors.Load())
		w.int("search_coord_total_query_errors_timeout", 0)
		w.int("search_coord_total_query_warnings_timeout", 0)
		w.int("search_coord_total_query_warnings_max_prefix_expansions", 0)
		w.end()
		zeros("search_multi_threading", "search_uv_threads_running_queries", "search_uv_threads_running_topology_update",
			"search_active_worker_threads", "search_active_coord_threads", "search_workers_low_priority_pending_jobs",
			"search_workers_high_priority_pending_jobs", "search_workers_admin_priority_pending_jobs",
			"search_coord_high_priority_pending_jobs")

		w.section("search_dialect_statistics")
		dialects := st.dialects.Load()
		for d := 1; d <= 4; d++ {
			w.int("search_dialect_"+strconv.Itoa(d), int64(dialects>>(d-1)&1))
		}
		w.end()
	}

	ft := func(name string) string {
		v, _ := search.ConfigValue(name)
		return v
	}
	onOff := func(b bool) string {
		if b {
			return "ON"
		}
		return "OFF"
	}
	maxAgg := ft("MAXAGGREGATERESULTS")
	if maxAgg == "unlimited" {
		maxAgg = searchMaxResults
	}
	w.section("search_runtime_configurations")
	w.field("search_extension_load", ft("EXTLOAD"))
	w.field("search_friso_ini", ft("FRISOINI"))
	w.field("search_default_scorer", ft("DEFAULT_SCORER"))
	w.field("search_enableGC", onOff(ft("NOGC") != "true"))
	w.field("search_minimal_term_prefix", ft("MINPREFIX"))
	w.field("search_minimal_stem_length", ft("MINSTEMLEN"))
	w.field("search_maximal_prefix_expansions", ft("MAXPREFIXEXPANSIONS"))
	w.field("search_query_timeout_ms", ft("TIMEOUT"))
	w.field("search_timeout_policy", ft("ON_TIMEOUT"))
	w.field("search_cursor_read_size", "1000")
	w.field("search_cursor_max_idle_time", ft("CURSOR_MAX_IDLE"))
	w.field("search_max_doc_table_size", ft("MAXDOCTABLESIZE"))
	w.field("search_max_search_results", ft("MAXSEARCHRESULTS"))
	w.field("search_max_aggregate_results", maxAgg)
	w.field("search_gc_scan_size", ft("GCSCANSIZE"))
	w.field("search_min_phonetic_term_length", ft("MIN_PHONETIC_TERM_LEN"))
	w.field("search_bm25std_tanh_factor", ft("BM25STD_TANH_FACTOR"))
	w.field("search_info_on_zero_indexes", onOff(configs["search-_info-on-zero-indexes"] == "yes"))
	w.end()
}

// infoSearchFields is RediSearch's fields_statistics section: the fields of
// every index by type, and the field values indexed.
func infoSearchFields(w *infoWriter, indexes []*search.Index) {
	type counts struct{ n, sortable, noIndex, caseSensitive, flat, hnsw int64 }
	var by [search.FieldGeoShape + 1]counts
	for _, idx := range indexes {
		for _, f := range idx.Fields {
			c := &by[f.Type]
			c.n++
			if f.Sortable {
				c.sortable++
			}
			if f.NoIndex {
				c.noIndex++
			}
			if f.CaseSensitive {
				c.caseSensitive++
			}
			if f.Type == search.FieldVector && f.Vector != nil {
				if strings.EqualFold(f.Vector.Algorithm, "HNSW") {
					c.hnsw++
				} else {
					c.flat++
				}
			}
		}
	}
	w.section("search_fields_statistics")
	for _, t := range []struct {
		typ   search.FieldType
		field string
		label string
	}{
		{search.FieldText, "search_fields_text", "Text"},
		{search.FieldNumeric, "search_fields_numeric", "Numeric"},
		{search.FieldTag, "search_fields_tag", "Tag"},
		{search.FieldGeo, "search_fields_geo", "Geo"},
		{search.FieldVector, "search_fields_vector", "Vector"},
		{search.FieldGeoShape, "search_fields_geoshape", "Geoshape"},
	} {
		c := by[t.typ]
		if c.n == 0 {
			continue
		}
		parts := []string{t.label + "=" + strconv.FormatInt(c.n, 10)}
		add := func(name string, v int64) {
			if v > 0 {
				parts = append(parts, name+"="+strconv.FormatInt(v, 10))
			}
		}
		if t.typ == search.FieldVector {
			add("Flat", c.flat)
			add("HNSW", c.hnsw)
		} else {
			add("Sortable", c.sortable)
			add("NoIndex", c.noIndex)
			if t.typ == search.FieldTag {
				add("CaseSensitive", c.caseSensitive)
			}
		}
		parts = append(parts, "IndexErrors=0")
		w.field(t.field, strings.Join(parts, ","))
	}
	for _, t := range []struct {
		name string
		typ  search.FieldType
	}{
		{"text", search.FieldText}, {"tag", search.FieldTag}, {"numeric", search.FieldNumeric},
		{"geo", search.FieldGeo}, {"geoshape", search.FieldGeoShape}, {"vector", search.FieldVector},
	} {
		w.int("search_total_indexing_ops_"+t.name+"_fields", search.IndexingOps(t.typ))
	}
	w.end()
}
