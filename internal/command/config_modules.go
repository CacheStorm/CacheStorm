package command

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 8 exposes its bundled modules' settings as CONFIG parameters:
// search-* (RediSearch, the same settings FT.CONFIG reads and writes),
// ts-* (RedisTimeSeries) and bf-*/cf-* (RedisBloom). Their values,
// ranges and errors are Redis 8.2's; the ones that set defaults for new
// series and filters take effect.

// searchMaxResults is RediSearch's ceiling for MAXSEARCHRESULTS and
// MAXAGGREGATERESULTS, which FT.CONFIG shows as unlimited.
const searchMaxResults = "2147483648"

type searchConfig struct {
	name string
	kind configKind
	def  string
	lo   int64
	hi   int64
	imm  bool
	ft   string
}

const configLongMaxStr = math.MaxInt64

var searchConfigs = []searchConfig{
	{"search-_bg-index-mem-pct-thr", cfgInt, "100", 0, 100, false, "_BG_INDEX_MEM_PCT_THR"},
	{"search-_bg-index-oom-pause-time", cfgInt, "0", 0, math.MaxUint32, false, "_BG_INDEX_OOM_PAUSE_TIME"},
	{"search-_free-resource-on-thread", cfgBool, "yes", 0, 0, false, "_FREE_RESOURCE_ON_THREAD"},
	{"search-_info-on-zero-indexes", cfgBool, "no", 0, 0, false, ""},
	{"search-_numeric-compress", cfgBool, "no", 0, 0, false, "_NUMERIC_COMPRESS"},
	{"search-_numeric-ranges-parents", cfgInt, "0", 0, 2, false, "_NUMERIC_RANGES_PARENTS"},
	{"search-_print-profile-clock", cfgBool, "yes", 0, 0, false, "_PRINT_PROFILE_CLOCK"},
	{"search-_prioritize-intersect-union-children", cfgBool, "no", 0, 0, false, "_PRIORITIZE_INTERSECT_UNION_CHILDREN"},
	{"search-bg-index-sleep-duration-us", cfgInt, "1", 1, 999999, false, "BG_INDEX_SLEEP_DURATION_US"},
	{"search-bg-index-sleep-gap", cfgInt, "100", 0, 0, true, "BG_INDEX_SLEEP_GAP"},
	{"search-bm25std-tanh-factor", cfgInt, "4", 1, 10000, false, "BM25STD_TANH_FACTOR"},
	{"search-cursor-max-idle", cfgInt, "300000", 1, configLongMaxStr, false, "CURSOR_MAX_IDLE"},
	{"search-default-dialect", cfgInt, "1", 1, 4, false, "DEFAULT_DIALECT"},
	{"search-default-scorer", cfgSpecial, "BM25STD", 0, 0, false, "DEFAULT_SCORER"},
	{"search-enable-unstable-features", cfgBool, "no", 0, 0, false, "ENABLE_UNSTABLE_FEATURES"},
	{"search-ext-load", cfgString, "", 0, 0, true, "EXTLOAD"},
	{"search-fork-gc-clean-threshold", cfgInt, "100", 1, configLongMaxStr, false, "FORK_GC_CLEAN_THRESHOLD"},
	{"search-fork-gc-retry-interval", cfgInt, "5", 1, configLongMaxStr, false, "FORK_GC_RETRY_INTERVAL"},
	{"search-fork-gc-run-interval", cfgInt, "30", 1, configLongMaxStr, false, "FORK_GC_RUN_INTERVAL"},
	{"search-fork-gc-sleep-before-exit", cfgInt, "0", 0, configLongMaxStr, false, "FORKGC_SLEEP_BEFORE_EXIT"},
	{"search-friso-ini", cfgString, "", 0, 0, true, "FRISOINI"},
	{"search-gc-scan-size", cfgInt, "100", 0, 0, true, "GCSCANSIZE"},
	{"search-index-cursor-limit", cfgInt, "128", 0, configLongMaxStr, false, "INDEX_CURSOR_LIMIT"},
	{"search-indexer-yield-every-ops", cfgInt, "1000", 1, math.MaxUint32, false, "INDEXER_YIELD_EVERY_OPS"},
	{"search-max-aggregate-results", cfgInt, searchMaxResults, 0, 1 << 31, false, "MAXAGGREGATERESULTS"},
	{"search-max-doctablesize", cfgInt, "1000000", 0, 0, true, "MAXDOCTABLESIZE"},
	{"search-max-prefix-expansions", cfgInt, "200", 1, configLongMaxStr, false, "MAXPREFIXEXPANSIONS"},
	{"search-max-search-results", cfgInt, "1000000", 0, 1 << 31, false, "MAXSEARCHRESULTS"},
	{"search-min-operation-workers", cfgInt, "4", 0, 16, false, "MIN_OPERATION_WORKERS"},
	{"search-min-phonetic-term-len", cfgInt, "3", 1, configLongMaxStr, false, "MIN_PHONETIC_TERM_LEN"},
	{"search-min-prefix", cfgInt, "2", 1, configLongMaxStr, false, "MINPREFIX"},
	{"search-min-stem-len", cfgInt, "4", 2, math.MaxUint32, false, "MINSTEMLEN"},
	{"search-monitor-expiration", cfgBool, "yes", 0, 0, false, ""},
	{"search-multi-text-slop", cfgInt, "100", 0, 0, true, "MULTI_TEXT_SLOP"},
	{"search-no-gc", cfgBool, "no", 0, 0, true, "NOGC"},
	{"search-no-mem-pools", cfgBool, "no", 0, 0, true, "NO_MEM_POOLS"},
	{"search-on-timeout", cfgEnum, "return", 0, 0, false, "ON_TIMEOUT"},
	{"search-partial-indexed-docs", cfgBool, "no", 0, 0, true, "PARTIAL_INDEXED_DOCS"},
	{"search-raw-docid-encoding", cfgBool, "no", 0, 0, true, "RAW_DOCID_ENCODING"},
	{"search-tiered-hnsw-buffer-limit", cfgInt, "1024", 0, 0, true, "TIERED_HNSW_BUFFER_LIMIT"},
	{"search-timeout", cfgInt, "500", 1, configLongMaxStr, false, "TIMEOUT"},
	{"search-union-iterator-heap", cfgInt, "20", 1, configLongMaxStr, false, "UNION_ITERATOR_HEAP"},
	{"search-vss-max-resize", cfgInt, "0", 0, math.MaxUint32, false, "VSS_MAX_RESIZE"},
	{"search-workers", cfgInt, "0", 0, 16, false, "WORKERS"},
	{"search-workers-priority-bias-threshold", cfgInt, "1", 0, 0, true, "WORKERS_PRIORITY_BIAS_THRESHOLD"},
}

var searchScorers = []string{"TFIDF", "TFIDF.DOCNORM", "BM25", "BM25STD", "BM25STD.TANH", "BM25STD.NORM", "DISMAX", "DOCSCORE", "HAMMING"}

// searchModuleValue is an FT.CONFIG value as the module parameter shows it.
func searchModuleValue(sc searchConfig) string {
	v, ok := search.ConfigValue(sc.ft)
	if !ok {
		return ""
	}
	switch {
	case sc.kind == cfgBool:
		return boolStr(v == "true")
	case v == "unlimited":
		return searchMaxResults
	}
	return v
}

// searchFTValue is a module parameter value as FT.CONFIG holds it.
func searchFTValue(sc searchConfig, v string) string {
	switch {
	case sc.kind == cfgBool:
		if v == "yes" {
			return "true"
		}
		return "false"
	case (sc.ft == "MAXSEARCHRESULTS" || sc.ft == "MAXAGGREGATERESULTS") && v == searchMaxResults:
		return "unlimited"
	}
	return v
}

func searchParam(sc searchConfig) *configParam {
	p := &configParam{kind: sc.kind, def: sc.def, lo: sc.lo, hi: sc.hi, immutable: sc.imm, strict: true}
	if sc.kind == cfgEnum {
		p.enum = []string{"return", "fail"}
	}
	if sc.ft == "" {
		return p
	}
	p.get = func() string { return searchModuleValue(sc) }
	if sc.kind == cfgSpecial {
		p.check = func(name, value string) (string, error) {
			unstable, _ := search.ConfigValue("ENABLE_UNSTABLE_FEATURES")
			if unstable != "true" {
				return "", ConfigSetError(name, "`search-default-scorer` is unavailable when `search-enable-unstable-features` is off. Enable it with `CONFIG SET search-enable-unstable-features true`")
			}
			for _, s := range searchScorers {
				if s == value {
					return value, nil
				}
			}
			return "", ConfigSetError(name, "Invalid default scorer value")
		}
	}
	p.applyText = func(v string) {
		ft := searchFTValue(sc, v)
		search.SetConfigValue(sc.ft, ft)
		if sc.ft == "MAXPREFIXEXPANSIONS" {
			search.SetConfigValue("MAXEXPANSIONS", ft)
		}
	}
	return p
}

// moduleDefaults are the RedisBloom and RedisTimeSeries settings new
// filters and series take.
var moduleDefaults = struct {
	sync.RWMutex
	bfErrorRate       float64
	bfInitialSize     int64
	bfExpansion       int64
	cfBucketSize      int64
	cfInitialSize     int64
	cfMaxIterations   int64
	cfExpansion       int64
	cfMaxExpansions   int64
	tsRetention       int64
	tsChunkSize       int64
	tsDuplicatePolicy string
	tsUncompressed    bool
	tsIgnoreTimeDiff  int64
	tsIgnoreValDiff   float64
	tsCompaction      []tsPolicyRule
}{
	bfErrorRate: bloomDefaultErrorRate, bfInitialSize: bloomDefaultCapacity, bfExpansion: bloomDefaultExpansion,
	cfBucketSize: 2, cfInitialSize: 1024, cfMaxIterations: 20, cfExpansion: 1, cfMaxExpansions: 32,
	tsChunkSize: store.TSDefaultChunkSize,
}

var redisBloomInt = regexp.MustCompile(`^(0|-?[1-9][0-9]*)$`)

// bloomIntParam is a RedisBloom integer setting, which takes plain decimal
// integers only.
func bloomIntParam(def string, lo, hi int64, set func(int64)) *configParam {
	return &configParam{kind: cfgString, def: def, check: func(name, value string) (string, error) {
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || !redisBloomInt.MatchString(value) {
			return "", ConfigSetError(name, "Invalid value for `"+name+"`")
		}
		if v < lo || v > hi {
			return "", ConfigSetError(name, fmt.Sprintf("Value for `%s` must be in the range [%d, %d]", name, lo, hi))
		}
		return value, nil
	}, applyText: func(v string) {
		n, _ := strconv.ParseInt(v, 10, 64) //nolint:errcheck // checked above
		moduleDefaults.Lock()
		set(n)
		moduleDefaults.Unlock()
	}}
}

var tsAggregators = map[string]bool{"avg": true, "sum": true, "min": true, "max": true, "range": true, "count": true, "first": true,
	"last": true, "std.p": true, "std.s": true, "var.p": true, "var.s": true, "twa": true}

var tsDurationUnits = []struct {
	unit string
	ms   int64
}{{"d", 86400000}, {"h", 3600000}, {"M", 60000}, {"s", 1000}, {"m", 1}}

// parseTSDuration reads a compaction policy duration: digits and one of
// m (milliseconds), s, M (minutes), h or d.
func parseTSDuration(s string) (int64, bool) {
	if len(s) < 2 {
		return 0, false
	}
	digits, unit := s[:len(s)-1], s[len(s)-1:]
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n < 0 || strings.ContainsAny(digits, "+-") {
		return 0, false
	}
	for _, u := range tsDurationUnits {
		if u.unit == unit {
			return n * u.ms, true
		}
	}
	return 0, false
}

func formatTSDuration(ms int64) string {
	for _, u := range tsDurationUnits {
		if ms%u.ms == 0 {
			return strconv.FormatInt(ms/u.ms, 10) + u.unit
		}
	}
	return strconv.FormatInt(ms, 10) + "m"
}

// checkTSCompactionPolicy validates RedisTimeSeries' compaction policy,
// rules of agg:bucket:retention[:align] split by ';', and shows it with
// each duration in its largest whole unit.
func checkTSCompactionPolicy(name, value string) (string, error) {
	var rules []string
	for _, rule := range strings.Split(value, ";") {
		if rule == "" {
			continue
		}
		parts := strings.Split(rule, ":")
		if len(parts) < 3 || len(parts) > 4 || !tsAggregators[parts[0]] {
			return "", ConfigSetError(name, "Invalid compaction policy: "+value)
		}
		bucket, okB := parseTSDuration(parts[1])
		retention, okR := parseTSDuration(parts[2])
		if parts[2] == "0" {
			retention, okR = 0, true
		}
		if !okB || !okR || bucket == 0 {
			return "", ConfigSetError(name, "Invalid compaction policy: "+value)
		}
		shown := parts[0] + ":" + formatTSDuration(bucket) + ":" + formatTSDuration(retention)
		if len(parts) == 4 {
			align, ok := parseTSDuration(parts[3])
			if parts[3] == "0" {
				align, ok = 0, true
			}
			if !ok {
				return "", ConfigSetError(name, "Invalid compaction policy: "+value)
			}
			if align != 0 {
				shown += ":" + formatTSDuration(align)
			}
		}
		rules = append(rules, shown)
	}
	return strings.Join(rules, ";"), nil
}

func init() {
	for _, sc := range searchConfigs {
		configParams[sc.name] = searchParam(sc)
	}
	md := &moduleDefaults
	configParams["bf-error-rate"] = &configParam{kind: cfgString, def: "0.01", check: func(name, value string) (string, error) {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil || strings.TrimSpace(value) != value {
			return "", ConfigSetError(name, "Invalid value for `bf-error-rate`")
		}
		if v <= 0 || v >= 1 {
			return "", ConfigSetError(name, "Value for `bf-error-rate` must be in the range (0.000000, 1.000000)")
		}
		return value, nil
	}, applyText: func(v string) {
		f, _ := strconv.ParseFloat(v, 64) //nolint:errcheck // checked above
		md.Lock()
		md.bfErrorRate = f
		md.Unlock()
	}}
	configParams["bf-expansion-factor"] = bloomIntParam("2", 0, 32768, func(v int64) { md.bfExpansion = v })
	configParams["bf-initial-size"] = bloomIntParam("100", 1, 1<<30, func(v int64) { md.bfInitialSize = v })
	configParams["cf-bucket-size"] = bloomIntParam("2", 1, 255, func(v int64) { md.cfBucketSize = v })
	configParams["cf-expansion-factor"] = bloomIntParam("1", 0, 32768, func(v int64) { md.cfExpansion = v })
	configParams["cf-initial-size"] = bloomIntParam("1024", 4, 1<<30, func(v int64) { md.cfInitialSize = v })
	configParams["cf-max-expansions"] = bloomIntParam("32", 1, 65536, func(v int64) { md.cfMaxExpansions = v })
	configParams["cf-max-iterations"] = bloomIntParam("20", 1, 65535, func(v int64) { md.cfMaxIterations = v })

	configParams["ts-num-threads"] = &configParam{kind: cfgInt, def: "3", immutable: true}
	configParams["ts-retention-policy"] = &configParam{kind: cfgInt, def: "0", lo: 0, hi: math.MaxInt64, strict: true, apply: func(v int64) {
		md.Lock()
		md.tsRetention = v
		md.Unlock()
	}}
	configParams["ts-ignore-max-time-diff"] = &configParam{kind: cfgInt, def: "0", lo: 0, hi: math.MaxInt64, strict: true, apply: func(v int64) {
		md.Lock()
		md.tsIgnoreTimeDiff = v
		md.Unlock()
	}}
	configParams["ts-chunk-size-bytes"] = &configParam{kind: cfgSpecial, def: "4096", check: func(name, value string) (string, error) {
		if !redisBloomInt.MatchString(value) {
			return "", ConfigSetError(name, "argument couldn't be parsed into an integer")
		}
		v, err := ParseConfigInt(name, value, 48, 1048576)
		if err != nil {
			return "", err
		}
		if v%8 != 0 {
			return "", ConfigSetError(name, "TSDB: CHUNK_SIZE value must be a multiple of 8 in the range [48 .. 1048576]")
		}
		return value, nil
	}, applyText: func(v string) {
		n, _ := strconv.ParseInt(v, 10, 64) //nolint:errcheck // checked above
		md.Lock()
		md.tsChunkSize = n
		md.Unlock()
	}}
	configParams["ts-ignore-max-val-diff"] = &configParam{kind: cfgString, def: "0.000000", check: func(name, value string) (string, error) {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil && !strings.HasSuffix(err.Error(), "value out of range") {
			return "", ConfigSetError(name, "Invalid value for `ts-ignore-max-val-diff`")
		}
		if v < 0 || math.IsInf(v, 0) || v != v {
			// RedisTimeSeries writes the message into 256 bytes.
			msg := fmt.Sprintf("Invalid value for `ts-ignore-max-val-diff`. Value must be in the range [0.000000 .. %f]", math.MaxFloat64)
			return "", ConfigSetError(name, msg[:255])
		}
		return fmt.Sprintf("%f", v), nil
	}, applyText: func(v string) {
		f, _ := strconv.ParseFloat(v, 64) //nolint:errcheck // checked above
		md.Lock()
		md.tsIgnoreValDiff = f
		md.Unlock()
	}}
	configParams["ts-duplicate-policy"] = &configParam{kind: cfgString, def: "block", check: func(name, value string) (string, error) {
		p, ok := store.ParseTSDuplicatePolicy(value)
		if !ok {
			return "", ConfigSetError(name, "Invalid duplicate policy: "+value)
		}
		return strings.ToLower(p), nil
	}, applyText: func(v string) {
		md.Lock()
		md.tsDuplicatePolicy = strings.ToUpper(v)
		if v == "block" {
			md.tsDuplicatePolicy = ""
		}
		md.Unlock()
	}}
	// RedisTimeSeries cannot go back to compressed once set uncompressed.
	configParams["ts-encoding"] = &configParam{kind: cfgString, def: "compressed", check: func(name, value string) (string, error) {
		switch strings.ToLower(value) {
		case "compressed", "uncompressed":
		default:
			return "", ConfigSetError(name, "Invalid encoding: "+value)
		}
		md.RLock()
		defer md.RUnlock()
		if md.tsUncompressed || strings.EqualFold(value, "uncompressed") {
			return "uncompressed", nil
		}
		return "compressed", nil
	}, applyText: func(v string) {
		md.Lock()
		md.tsUncompressed = v == "uncompressed"
		md.Unlock()
	}}
	configParams["ts-compaction-policy"] = &configParam{kind: cfgString, def: "", check: checkTSCompactionPolicy, applyText: func(v string) {
		rules := parseTSCompactionPolicy(v)
		md.Lock()
		md.tsCompaction = rules
		md.Unlock()
	}}
}

// tsCreateDefaults returns the settings a new series takes.
func tsCreateDefaults(ts *store.TimeSeriesValue) {
	md := &moduleDefaults
	md.RLock()
	defer md.RUnlock()
	ts.ChunkSize = md.tsChunkSize
	ts.Uncompressed = md.tsUncompressed
	ts.DuplicatePolicy = md.tsDuplicatePolicy
	ts.IgnoreMaxTimeDiff, ts.IgnoreMaxValDiff = md.tsIgnoreTimeDiff, md.tsIgnoreValDiff
	if md.tsRetention > 0 {
		ts.SetRetention(time.Duration(min(md.tsRetention, maxExpireAtMillis)) * time.Millisecond)
	}
}

// bloomDefaults are the capacity, error rate and expansion a filter BF.ADD
// creates takes.
func bloomDefaults() (int64, float64, int64) {
	md := &moduleDefaults
	md.RLock()
	defer md.RUnlock()
	return md.bfInitialSize, md.bfErrorRate, md.bfExpansion
}

// cuckooDefaults are the capacity, bucket size, iterations and expansion
// a cuckoo filter CF.ADD creates takes, and its limit of expansions.
func cuckooDefaults() (capacity, bucketSize, maxIterations, expansion, maxExpansions int64) {
	md := &moduleDefaults
	md.RLock()
	defer md.RUnlock()
	return md.cfInitialSize, md.cfBucketSize, md.cfMaxIterations, md.cfExpansion, md.cfMaxExpansions
}

type tsPolicyRule struct {
	agg                      string
	bucket, retention, align int64
}

// parseTSCompactionPolicy reads a policy checkTSCompactionPolicy showed.
func parseTSCompactionPolicy(v string) []tsPolicyRule {
	var out []tsPolicyRule
	for _, rule := range strings.Split(v, ";") {
		parts := strings.Split(rule, ":")
		if len(parts) < 3 {
			continue
		}
		r := tsPolicyRule{agg: parts[0]}
		r.bucket, _ = parseTSDuration(parts[1])    //nolint:errcheck // checked when set
		r.retention, _ = parseTSDuration(parts[2]) //nolint:errcheck // checked when set
		if len(parts) == 4 {
			r.align, _ = parseTSDuration(parts[3]) //nolint:errcheck // checked when set
		}
		out = append(out, r)
	}
	return out
}

// applyCompactionPolicy gives a series TS.ADD or TS.INCRBY created the
// compactions ts-compaction-policy names, as RedisTimeSeries does: a
// series key_AGG_bucket[_align] for each rule whose key is free, with the
// rule's retention and the source's labels.
func applyCompactionPolicy(ctx *Context, key string, src *store.TimeSeriesValue) {
	md := &moduleDefaults
	md.RLock()
	rules := md.tsCompaction
	md.RUnlock()
	for _, rule := range rules {
		agg, ok := store.ParseTSAggregation(rule.agg)
		if !ok {
			continue
		}
		dest := fmt.Sprintf("%s_%s_%d", key, strings.ToUpper(rule.agg), rule.bucket)
		if rule.align != 0 {
			dest += fmt.Sprintf("_%d", rule.align)
		}
		if ctx.Store.Exists(dest) {
			continue
		}
		ts := store.NewTimeSeriesValue(0)
		tsCreateDefaults(ts)
		ts.DuplicatePolicy = store.TSNoPolicy
		ts.SetRetention(time.Duration(min(rule.retention, maxExpireAtMillis)) * time.Millisecond)
		var keys, values []string
		for _, l := range src.LabelList() {
			keys, values = append(keys, l[0]), append(values, l[1])
		}
		keys = append(keys, "aggregation", "time_bucket")
		values = append(values, strings.ToUpper(rule.agg), strconv.FormatInt(rule.bucket, 10))
		ts.SetLabels(keys, values)
		ts.SourceKey = key
		if err := ctx.Store.Set(dest, ts, store.SetOptions{}); err != nil {
			continue
		}
		src.Lock()
		src.Rules = append(src.Rules, store.TSRule{Dest: dest, Aggregation: agg, Bucket: rule.bucket, Align: rule.align})
		src.Unlock()
		valueWritten(ctx, dest)
	}
}
