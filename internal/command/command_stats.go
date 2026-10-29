package command

import (
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Redis' per-command statistics: INFO commandstats, errorstats and
// latencystats, and LATENCY HISTOGRAM. Latencies go into an HdrHistogram
// laid out like the one Redis keeps per command: 1 ns to 1 s, two
// significant digits.
const (
	histSubBucketHalfMag = 7
	histSubBucketHalf    = 1 << histSubBucketHalfMag
	histSubBucketMask    = 2*histSubBucketHalf - 1
	histBuckets          = 23
	histCountsLen        = (histBuckets + 1) * histSubBucketHalf
	histMinValue         = 1
	histMaxValue         = 1_000_000_000
	// errorStatsLimit is how many distinct error codes are tracked, as in
	// Redis.
	errorStatsLimit = 128
)

type latencyHistogram struct {
	total  atomic.Int64
	counts [histCountsLen]atomic.Int64
}

func histIndex(v int64) int {
	pow2ceiling := 64 - bits.LeadingZeros64(uint64(v)|histSubBucketMask)
	bucket := pow2ceiling - (histSubBucketHalfMag + 1)
	sub := int(v >> uint(bucket))
	return (bucket+1)<<histSubBucketHalfMag + sub - histSubBucketHalf
}

func histValueAt(idx int) int64 {
	bucket := idx>>histSubBucketHalfMag - 1
	sub := int64(idx&(histSubBucketHalf-1)) + histSubBucketHalf
	if bucket < 0 {
		sub -= histSubBucketHalf
		bucket = 0
	}
	return sub << uint(bucket)
}

// histHighestEquivalent is the largest value counted in the same slot as v.
func histHighestEquivalent(v int64) int64 {
	pow2ceiling := 64 - bits.LeadingZeros64(uint64(v)|histSubBucketMask)
	bucket := pow2ceiling - (histSubBucketHalfMag + 1)
	sub := v >> uint(bucket)
	size := int64(1) << uint(bucket)
	if sub >= 2*histSubBucketHalf {
		size <<= 1
	}
	return sub<<uint(bucket) + size - 1
}

func (h *latencyHistogram) record(ns int64) {
	ns = min(max(ns, histMinValue), histMaxValue)
	h.counts[histIndex(ns)].Add(1)
	h.total.Add(1)
}

// percentile is hdr_value_at_percentile.
func (h *latencyHistogram) percentile(p float64) int64 {
	want := max(int64(min(p, 100)/100*float64(h.total.Load())+0.5), 1)
	var seen int64
	for i := range h.counts {
		seen += h.counts[i].Load()
		if seen >= want {
			v := histValueAt(i)
			if p == 0 {
				return v
			}
			return histHighestEquivalent(v)
		}
	}
	return 0
}

// cdf is the reply LATENCY HISTOGRAM gives for one command: the cumulative
// counts at power-of-two microsecond steps, as Redis' fillCommandCDF walks
// its histogram with a logarithmic iterator starting at 1024 ns.
func (h *latencyHistogram) cdf() *resp.Value {
	total := h.total.Load()
	var points []*resp.Value
	var cumulative, previous int64
	i := 0
	for level := int64(1024); ; level *= 2 {
		idx := histIndex(level)
		if idx >= histCountsLen {
			break
		}
		for ; i <= idx; i++ {
			cumulative += h.counts[i].Load()
		}
		if cumulative > previous {
			points = append(points, resp.IntegerValue(histHighestEquivalent(level)/1000), resp.IntegerValue(cumulative))
		}
		previous = cumulative
		if cumulative >= total {
			break
		}
	}
	return resp.PairsValue([]*resp.Value{
		resp.BulkString("calls"), resp.IntegerValue(total),
		resp.BulkString("histogram_usec"), resp.PairsValue(points),
	})
}

type commandStat struct {
	name     string
	calls    atomic.Int64
	usec     atomic.Int64
	rejected atomic.Int64
	failed   atomic.Int64
	// hist is allocated on the first call, as Redis allocates a command's
	// histogram.
	hist atomic.Pointer[latencyHistogram]
}

var commandStats = struct {
	// all maps "CMD" or "CMD|SUB" to its statistics. The map is never
	// modified once published; a new command copies it under mu.
	all atomic.Pointer[map[string]*commandStat]
	mu  sync.Mutex

	tracking    atomic.Bool
	percentiles atomic.Pointer[[]float64]

	errMu       sync.Mutex
	errors      map[string]int64
	totalErrors atomic.Int64
}{}

func init() {
	commandStats.all.Store(&map[string]*commandStat{})
	commandStats.tracking.Store(true)
	commandStats.percentiles.Store(&[]float64{50, 99, 99.9})
	commandStats.errors = make(map[string]int64)
	resp.SetErrorHook(countErrorReply)
}

// statKey names the statistics a call of cmd counts towards: "CMD", or
// "CMD|SUB" for a container command. An unknown subcommand counts nowhere.
func statKey(cmd string, args [][]byte) string {
	if !hasSubcommands(cmd) || len(args) == 0 {
		return cmd
	}
	key := cmd + "|" + strings.ToUpper(string(args[0]))
	if _, ok := redisSubcommandCategories[key]; !ok {
		return ""
	}
	return key
}

func commandStatFor(cmd string, args [][]byte) *commandStat {
	key := statKey(cmd, args)
	if key == "" {
		return nil
	}
	if st := (*commandStats.all.Load())[key]; st != nil {
		return st
	}
	commandStats.mu.Lock()
	defer commandStats.mu.Unlock()
	cur := *commandStats.all.Load()
	if st := cur[key]; st != nil {
		return st
	}
	next := make(map[string]*commandStat, len(cur)+1)
	for k, v := range cur {
		next[k] = v
	}
	st := &commandStat{name: strings.ToLower(key)}
	next[key] = st
	commandStats.all.Store(&next)
	return st
}

// recordCall counts one execution of cmd that took d and whether it
// answered with an error.
func recordCall(cmd string, args [][]byte, d time.Duration, failed bool) {
	st := commandStatFor(cmd, args)
	if st == nil {
		return
	}
	usec := int64(d / time.Microsecond)
	st.calls.Add(1)
	st.usec.Add(usec)
	if failed {
		st.failed.Add(1)
	}
	if !commandStats.tracking.Load() {
		return
	}
	h := st.hist.Load()
	if h == nil {
		st.hist.CompareAndSwap(nil, new(latencyHistogram))
		h = st.hist.Load()
	}
	h.record(usec * 1000)
}

// RecordRejected counts a call of cmd the server refused before it ran.
func RecordRejected(cmd string, args [][]byte) { recordRejected(cmd, args) }

// recordRejected counts a call of cmd refused before it ran.
func recordRejected(cmd string, args [][]byte) {
	if st := commandStatFor(cmd, args); st != nil {
		st.rejected.Add(1)
	}
}

// countErrorReply counts an error reply under its code, the text before the
// first space within its first 32 bytes, or ERR.
func countErrorReply(msg string) {
	commandStats.totalErrors.Add(1)
	code := "ERR"
	if i := strings.IndexByte(msg[:min(len(msg), 32)], ' '); i >= 0 {
		code = msg[:i]
	}
	commandStats.errMu.Lock()
	defer commandStats.errMu.Unlock()
	if _, ok := commandStats.errors[code]; ok || len(commandStats.errors) < errorStatsLimit {
		commandStats.errors[code]++
	}
}

// resetCommandStats is CONFIG RESETSTAT's part for the command and error
// statistics.
func resetCommandStats() {
	commandStats.mu.Lock()
	commandStats.all.Store(&map[string]*commandStat{})
	commandStats.mu.Unlock()
	commandStats.errMu.Lock()
	commandStats.errors = make(map[string]int64)
	commandStats.errMu.Unlock()
	commandStats.totalErrors.Store(0)
}

func sortedCommandStats() []*commandStat {
	all := *commandStats.all.Load()
	out := make([]*commandStat, 0, len(all))
	for _, st := range all {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func infoCommandStats(w *infoWriter) {
	w.section("Commandstats")
	for _, st := range sortedCommandStats() {
		calls, usec := st.calls.Load(), st.usec.Load()
		perCall := 0.0
		if calls > 0 {
			perCall = float64(usec) / float64(calls)
		}
		w.field("cmdstat_"+st.name, "calls="+strconv.FormatInt(calls, 10)+
			",usec="+strconv.FormatInt(usec, 10)+
			",usec_per_call="+strconv.FormatFloat(perCall, 'f', 2, 64)+
			",rejected_calls="+strconv.FormatInt(st.rejected.Load(), 10)+
			",failed_calls="+strconv.FormatInt(st.failed.Load(), 10))
	}
	w.end()
}

func infoErrorStats(w *infoWriter) {
	commandStats.errMu.Lock()
	codes := make([]string, 0, len(commandStats.errors))
	for code := range commandStats.errors {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	counts := make([]int64, len(codes))
	for i, code := range codes {
		counts[i] = commandStats.errors[code]
	}
	commandStats.errMu.Unlock()
	w.section("Errorstats")
	for i, code := range codes {
		w.field("errorstat_"+code, "count="+strconv.FormatInt(counts[i], 10))
	}
	w.end()
}

func infoLatencyStats(w *infoWriter) {
	percentiles := *commandStats.percentiles.Load()
	w.section("Latencystats")
	for _, st := range sortedCommandStats() {
		h := st.hist.Load()
		if h == nil {
			continue
		}
		var b strings.Builder
		for i, p := range percentiles {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString("p" + strconv.FormatFloat(p, 'f', -1, 64) + "=")
			b.WriteString(strconv.FormatFloat(float64(h.percentile(p))/1000, 'f', 3, 64))
		}
		w.field("latency_percentiles_usec_"+st.name, b.String())
	}
	w.end()
}

// latencyHistogramReply answers LATENCY HISTOGRAM [command ...].
func latencyHistogramReply(ctx *Context) error {
	all := *commandStats.all.Load()
	var pairs []*resp.Value
	add := func(st *commandStat) {
		if st == nil {
			return
		}
		if h := st.hist.Load(); h != nil {
			pairs = append(pairs, resp.BulkString(st.name), h.cdf())
		}
	}
	if ctx.ArgCount() == 1 {
		for _, st := range sortedCommandStats() {
			add(st)
		}
		return ctx.Writer.WriteValue(resp.PairsValue(pairs))
	}
	for i := 1; i < ctx.ArgCount(); i++ {
		name := strings.ToUpper(ctx.ArgString(i))
		add(all[name])
		if !hasSubcommands(name) {
			continue
		}
		var subs []*commandStat
		for key, st := range all {
			if strings.HasPrefix(key, name+"|") {
				subs = append(subs, st)
			}
		}
		sort.Slice(subs, func(i, j int) bool { return subs[i].name < subs[j].name })
		for _, st := range subs {
			add(st)
		}
	}
	return ctx.Writer.WriteValue(resp.PairsValue(pairs))
}

// parseLatencyPercentiles parses latency-tracking-info-percentiles: numbers
// from 0 to 100 separated by spaces.
func parseLatencyPercentiles(value string) ([]float64, string) {
	fields := strings.Fields(value)
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		p, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, "Invalid latency-tracking-info-percentiles parameters"
		}
		if p < 0 || p > 100 {
			return nil, "latency-tracking-info-percentiles parameters should sit between [0.0,100.0]"
		}
		out = append(out, p)
	}
	return out, ""
}

func formatLatencyPercentiles() string {
	ps := *commandStats.percentiles.Load()
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = strconv.FormatFloat(p, 'f', -1, 64)
	}
	return strings.Join(parts, " ")
}
