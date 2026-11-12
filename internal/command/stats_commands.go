package command

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterStatsCommands(router *Router) {
	router.Register(&CommandDef{Name: "TDIGEST.CREATE", Handler: cmdTDIGESTCREATE})
	router.Register(&CommandDef{Name: "TDIGEST.ADD", Handler: cmdTDIGESTADD})
	router.Register(&CommandDef{Name: "TDIGEST.QUANTILE", Handler: cmdTDIGESTQUANTILE})
	router.Register(&CommandDef{Name: "TDIGEST.CDF", Handler: cmdTDIGESTCDF})
	router.Register(&CommandDef{Name: "TDIGEST.MEAN", Handler: cmdTDIGESTMEAN})
	router.Register(&CommandDef{Name: "TDIGEST.MIN", Handler: cmdTDIGESTMIN})
	router.Register(&CommandDef{Name: "TDIGEST.MAX", Handler: cmdTDIGESTMAX})
	router.Register(&CommandDef{Name: "TDIGEST.INFO", Handler: cmdTDIGESTINFO})
	router.Register(&CommandDef{Name: "TDIGEST.RESET", Handler: cmdTDIGESTRESET})
	router.Register(&CommandDef{Name: "TDIGEST.MERGE", Handler: cmdTDIGESTMERGE})
	router.Register(&CommandDef{Name: "TDIGEST.RANK", Handler: cmdTDIGESTRANK})
	router.Register(&CommandDef{Name: "TDIGEST.REVRANK", Handler: cmdTDIGESTREVRANK})
	router.Register(&CommandDef{Name: "TDIGEST.BYRANK", Handler: cmdTDIGESTBYRANK})
	router.Register(&CommandDef{Name: "TDIGEST.BYREVRANK", Handler: cmdTDIGESTBYREVRANK})
	router.Register(&CommandDef{Name: "TDIGEST.TRIMMED_MEAN", Handler: cmdTDIGESTTRIMMEDMEAN})

	router.Register(&CommandDef{Name: "SAMPLE.CREATE", Handler: cmdSAMPLECREATE})
	router.Register(&CommandDef{Name: "SAMPLE.ADD", Handler: cmdSAMPLEADD})
	router.Register(&CommandDef{Name: "SAMPLE.GET", Handler: cmdSAMPLEGET})
	router.Register(&CommandDef{Name: "SAMPLE.RESET", Handler: cmdSAMPLERESET})
	router.Register(&CommandDef{Name: "SAMPLE.INFO", Handler: cmdSAMPLEINFO})

	router.Register(&CommandDef{Name: "HISTOGRAM.CREATE", Handler: cmdHISTOGRAMCREATE})
	router.Register(&CommandDef{Name: "HISTOGRAM.ADD", Handler: cmdHISTOGRAMADD})
	router.Register(&CommandDef{Name: "HISTOGRAM.GET", Handler: cmdHISTOGRAMGET})
	router.Register(&CommandDef{Name: "HISTOGRAM.MEAN", Handler: cmdHISTOGRAMMEAN})
	router.Register(&CommandDef{Name: "HISTOGRAM.RESET", Handler: cmdHISTOGRAMRESET})
	router.Register(&CommandDef{Name: "HISTOGRAM.INFO", Handler: cmdHISTOGRAMINFO})
}

var (
	errTDigestNoKey           = errors.New("ERR T-Digest: key does not exist")
	errTDigestKeyExists       = errors.New("ERR T-Digest: key already exists")
	errTDigestCompression     = errors.New("ERR T-Digest: error parsing compression parameter")
	errTDigestValue           = errors.New("ERR T-Digest: error parsing val parameter")
	errTDigestQuantile        = errors.New("ERR T-Digest: quantile should be in [0,1]")
	errTDigestRank            = errors.New("ERR T-Digest: rank needs to be non negative")
	errTDigestNumKeys         = errors.New("ERR T-Digest: numkeys needs to be a positive integer")
	errTDigestNumKeysParse    = errors.New("ERR T-Digest: error parsing numkeys")
	errTDigestCut             = errors.New("ERR T-Digest: low_cut_percentile and high_cut_percentile should be in [0,1]")
	errTDigestCutOrder        = errors.New("ERR T-Digest: low_cut_percentile should be lower than high_cut_percentile")
	errTDigestFinite          = errors.New("ERR T-Digest: val parameter needs to be a finite number")
	errTDigestCompressionSign = errors.New("ERR T-Digest: compression parameter needs to be a positive integer")
	errTDigestQuantileParse   = errors.New("ERR T-Digest: error parsing quantile")
	errTDigestCDFParse        = errors.New("ERR T-Digest: error parsing cdf")
	errTDigestValueParse      = errors.New("ERR T-Digest: error parsing value")
	errTDigestRankParse       = errors.New("ERR T-Digest: error parsing rank")
	errTDigestLowCutParse     = errors.New("ERR T-Digest: error parsing low_cut_percentile")
	errTDigestHighCutParse    = errors.New("ERR T-Digest: error parsing high_cut_percentile")
	errTDigestKeyword         = errors.New("ERR T-Digest: wrong keyword")
)

const tdigestDefaultCompression = 100

// tdigestDouble is a double as RedisBloom replies with it, formatted by
// Redis' fpconv (Grisu2), which is not always the shortest form.
func tdigestDouble(ctx *Context, v float64) *resp.Value {
	if ctx.Writer.IsRESP3() {
		return resp.DoubleValue(fpconvDtoa(v))
	}
	return resp.BulkString(fpconvDtoa(v))
}

func parseTDigestCompression(b []byte) (int64, error) {
	c, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil || c > 1<<20 {
		return 0, errTDigestCompression
	}
	if c <= 0 {
		return 0, errTDigestCompressionSign
	}
	return c, nil
}

// getTDigest returns the t-digest at key, nil when the key is absent.
func getTDigest(ctx *Context, key string) (*tdigestSketch, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	td, ok := entry.Value.(*tdigestSketch)
	if !ok {
		return nil, store.ErrWrongType
	}
	return td, nil
}

// withTDigest runs fn on the t-digest the command names; a TDIGEST.ADD or
// TDIGEST.RESET changes it in place.
func withTDigest(ctx *Context, fn func(td *tdigestSketch) *resp.Value) error {
	key := ctx.ArgString(0)
	td, err := getTDigest(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if td == nil {
		return ctx.WriteError(errTDigestNoKey)
	}
	td.mu.Lock()
	reply := fn(td)
	td.mu.Unlock()
	if ctx.Command == "TDIGEST.ADD" || ctx.Command == "TDIGEST.RESET" {
		valueWritten(ctx, key)
	}
	return ctx.WriteValue(reply)
}

func parseTDigestFloats(ctx *Context, from int, errParse error) ([]float64, error) {
	vals := make([]float64, 0, ctx.ArgCount()-from)
	for i := from; i < ctx.ArgCount(); i++ {
		v, err := strconv.ParseFloat(ctx.ArgString(i), 64)
		if err != nil || math.IsNaN(v) {
			return nil, errParse
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func cmdTDIGESTCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	compression := int64(tdigestDefaultCompression)
	if ctx.ArgCount() == 3 {
		if !strings.EqualFold(ctx.ArgString(1), "COMPRESSION") {
			return ctx.WriteError(errTDigestKeyword)
		}
		c, err := parseTDigestCompression(ctx.Arg(2))
		if err != nil {
			return ctx.WriteError(err)
		}
		compression = c
	}

	if existing, err := getTDigest(ctx, key); err != nil {
		return ctx.WriteError(err)
	} else if existing != nil {
		return ctx.WriteError(errTDigestKeyExists)
	}
	if err := ctx.Store.Set(key, newTDigestSketch(compression), store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTDIGESTADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	values, err := parseTDigestFloats(ctx, 1, errTDigestValue)
	if err != nil {
		return ctx.WriteError(err)
	}
	for _, v := range values {
		if math.IsInf(v, 0) {
			return ctx.WriteError(errTDigestFinite)
		}
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		for _, v := range values {
			if err := td.addLocked(v, 1); err != nil {
				return resp.ErrorValue(err.Error())
			}
		}
		return resp.SimpleString("OK")
	})
}

func cmdTDIGESTQUANTILE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	qs, err := parseTDigestFloats(ctx, 1, errTDigestQuantileParse)
	if err != nil {
		return ctx.WriteError(err)
	}
	for _, q := range qs {
		if q < 0 || q > 1 {
			return ctx.WriteError(errTDigestQuantile)
		}
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		vals := make([]float64, len(qs))
		for i := 0; i < len(qs); i++ {
			start := i
			for i < len(qs)-1 && qs[i] <= qs[i+1] {
				i++
			}
			td.quantilesLocked(qs[start:i+1], vals[start:i+1])
		}
		out := make([]*resp.Value, len(qs))
		for i, v := range vals {
			out[i] = tdigestDouble(ctx, v)
		}
		return resp.ArrayValue(out)
	})
}

func cmdTDIGESTCDF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	vals, err := parseTDigestFloats(ctx, 1, errTDigestCDFParse)
	if err != nil {
		return ctx.WriteError(err)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		out := make([]*resp.Value, len(vals))
		for i, v := range vals {
			out[i] = tdigestDouble(ctx, td.cdfLocked(v))
		}
		return resp.ArrayValue(out)
	})
}

func tdigestRankCmd(ctx *Context, reverse bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	vals, err := parseTDigestFloats(ctx, 1, errTDigestValueParse)
	if err != nil {
		return ctx.WriteError(err)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		out := make([]*resp.Value, len(vals))
		for i, v := range vals {
			out[i] = resp.IntegerValue(td.rankLocked(v, reverse))
		}
		return resp.ArrayValue(out)
	})
}

func cmdTDIGESTRANK(ctx *Context) error {
	return tdigestRankCmd(ctx, false)
}

func cmdTDIGESTREVRANK(ctx *Context) error {
	return tdigestRankCmd(ctx, true)
}

func tdigestByRankCmd(ctx *Context, reverse bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ranks := make([]float64, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		r, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
		if err != nil {
			return ctx.WriteError(errTDigestRankParse)
		}
		if r < 0 {
			return ctx.WriteError(errTDigestRank)
		}
		ranks = append(ranks, float64(r))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		out := make([]*resp.Value, len(ranks))
		for i, r := range ranks {
			out[i] = tdigestDouble(ctx, td.byRankLocked(r, reverse))
		}
		return resp.ArrayValue(out)
	})
}

func cmdTDIGESTBYRANK(ctx *Context) error {
	return tdigestByRankCmd(ctx, false)
}

func cmdTDIGESTBYREVRANK(ctx *Context) error {
	return tdigestByRankCmd(ctx, true)
}

func cmdTDIGESTTRIMMEDMEAN(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	low, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil || math.IsNaN(low) {
		return ctx.WriteError(errTDigestLowCutParse)
	}
	high, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil || math.IsNaN(high) {
		return ctx.WriteError(errTDigestHighCutParse)
	}
	if low < 0 || low > 1 || high < 0 || high > 1 {
		return ctx.WriteError(errTDigestCut)
	}
	if low >= high {
		return ctx.WriteError(errTDigestCutOrder)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		return tdigestDouble(ctx, td.trimmedMeanLocked(low, high))
	})
}

func cmdTDIGESTMEAN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		return tdigestDouble(ctx, td.meanLocked())
	})
}

func cmdTDIGESTMIN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		if td.total() == 0 {
			return tdigestDouble(ctx, math.NaN())
		}
		return tdigestDouble(ctx, td.min)
	})
}

func cmdTDIGESTMAX(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		if td.total() == 0 {
			return tdigestDouble(ctx, math.NaN())
		}
		return tdigestDouble(ctx, td.max)
	})
}

func cmdTDIGESTINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		capacity := int64(td.capacity())
		label := resp.SimpleString
		return resp.PairsValue([]*resp.Value{
			label("Compression"), resp.IntegerValue(td.compression),
			label("Capacity"), resp.IntegerValue(capacity),
			label("Merged nodes"), resp.IntegerValue(int64(td.mergedNodes)),
			label("Unmerged nodes"), resp.IntegerValue(int64(td.unmergedNodes())),
			label("Merged weight"), resp.IntegerValue(td.mergedWeight),
			label("Unmerged weight"), resp.IntegerValue(td.unmergedWeight),
			label("Observations"), resp.IntegerValue(td.total()),
			label("Total compressions"), resp.IntegerValue(td.totalCompressions),
			label("Memory usage"), resp.IntegerValue(8 + 16*capacity),
		})
	})
}

func cmdTDIGESTRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		td.resetLocked()
		return resp.SimpleString("OK")
	})
}

// cmdTDIGESTMERGE is RedisBloom's TDIGEST.MERGE: a new digest of the
// destination's compression (or COMPRESSION, or the largest source's) takes
// the destination's centroids unless OVERRIDE, then each source's in order.
func cmdTDIGESTMERGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	destKey := ctx.ArgString(0)
	dest, err := getTDigest(ctx, destKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	numKeys, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(errTDigestNumKeysParse)
	}
	if numKeys <= 0 {
		return ctx.WriteError(errTDigestNumKeys)
	}
	if numKeys > int64(ctx.ArgCount()-2) {
		return ctx.WriteError(arityError(ctx.Command))
	}
	var compression int64
	if dest != nil {
		compression = dest.compression
	}
	useMax, override := dest == nil, false
	if rest := ctx.Args[2+numKeys:]; len(rest) > 0 {
		compressionAt, overrideAt := -1, -1
		for i, a := range rest {
			switch {
			case compressionAt < 0 && strings.EqualFold(string(a), "COMPRESSION"):
				compressionAt = i
			case overrideAt < 0 && strings.EqualFold(string(a), "OVERRIDE"):
				overrideAt = i
			}
		}
		if compressionAt >= 0 {
			useMax = false
			if compressionAt+1 >= len(rest) {
				return ctx.WriteError(arityError(ctx.Command))
			}
			if compression, err = parseTDigestCompression(rest[compressionAt+1]); err != nil {
				return ctx.WriteError(err)
			}
		}
		if overrideAt >= 0 {
			override = true
			if compressionAt < 0 {
				useMax, compression = true, 0
			}
		}
		if compressionAt < 0 && overrideAt < 0 {
			return ctx.WriteError(errTDigestKeyword)
		}
	}
	sources := make([]*tdigestSketch, numKeys)
	for i := range sources {
		k := ctx.ArgString(2 + i)
		if k == destKey {
			if dest == nil {
				return ctx.WriteError(errTDigestNoKey)
			}
			sources[i] = dest
		} else {
			src, err := getTDigest(ctx, k)
			if err != nil {
				return ctx.WriteError(err)
			}
			if src == nil {
				return ctx.WriteError(errTDigestNoKey)
			}
			sources[i] = src
		}
		if useMax {
			compression = max(compression, sources[i].compression)
		}
	}
	merged := newTDigestSketch(compression)
	if dest != nil && !override {
		dest.mu.Lock()
		err := merged.mergeFrom(dest)
		dest.mu.Unlock()
		if err != nil {
			return ctx.WriteError(err)
		}
	}
	for _, src := range sources {
		src.mu.Lock()
		err := merged.mergeFrom(src)
		src.mu.Unlock()
		if err != nil {
			return ctx.WriteError(err)
		}
	}
	if err := ctx.Store.Set(destKey, merged, store.SetOptions{KeepTTL: true}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SAMPLE and HISTOGRAM keep each object in a hash key marked by a field
// naming its kind. Being keys they are per database, persisted, replicated
// and removed by DEL and FLUSHALL; changes are recorded as HSET and DEL, so
// a reservoir's random replacement replays as the slot it chose.

const (
	sampleMarker    = "__sample"
	histogramMarker = "__histogram"
)

func parseFloatList(ctx *Context, from int) ([]float64, error) {
	args := make([]string, ctx.ArgCount()-from)
	for i := range args {
		args[i] = ctx.ArgString(from + i)
	}
	return parseFloatArgs(args...)
}

// --- SAMPLE ---

// A sampler's marker holds its reservoir size, count the values offered,
// and the fields 0, 1, ... the reservoir's slots.

type sampler struct {
	size, count int64
}

func readSampler(ctx *Context, key string) (*sampler, error) {
	h, err := readMarkedHash(ctx, key, sampleMarker)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, errors.New("ERR sampler not found")
	}
	h.RLock()
	size, _ := h.Get(sampleMarker)
	count, _ := h.Get("count")
	h.RUnlock()
	rec := &record{vals: map[string]string{"size": string(size), "count": string(count)}}
	sm := &sampler{rec.int("size"), rec.int("count")}
	if rec.bad || sm.size <= 0 || sm.count < 0 {
		return nil, store.ErrWrongType
	}
	return sm, nil
}

func (sm *sampler) slots() int64 { return min(sm.size, sm.count) }

// SAMPLE.CREATE key size makes an empty reservoir of size slots, replacing
// one at key. A size of 0 or less means 1000.
func cmdSAMPLECREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if size <= 0 {
		size = 1000
	}
	if size > maxExtensionSlots {
		return ctx.WriteError(fmt.Errorf("ERR size must be at most %d", maxExtensionSlots))
	}
	if err := createMarked(ctx, ctx.ArgString(0), sampleMarker, sampleMarker, i64(size), "count", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SAMPLE.ADD key value [value ...] offers the values to the reservoir: how
// many were offered.
func cmdSAMPLEADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	values, err := parseFloatList(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	sm, err := readSampler(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	slots := map[int64]float64{}
	for _, v := range values {
		sm.count++
		if sm.count <= sm.size {
			slots[sm.count-1] = v
		} else if j := rand.Int64N(sm.count); j < sm.size {
			slots[j] = v
		}
	}
	pairs := []string{"count", i64(sm.count)}
	for slot, v := range slots {
		pairs = append(pairs, i64(slot), f64(v))
	}
	if err := updateRecord(ctx, key, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(values)))
}

// SAMPLE.GET key: the values in the reservoir.
func cmdSAMPLEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	sm, err := readSampler(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	r, err := readMarked(ctx, key, sampleMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, sm.slots())
	for i := range out {
		v, ok := r[strconv.Itoa(i)]
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		out[i] = resp.BulkString(v)
	}
	return ctx.WriteArray(out)
}

// SAMPLE.RESET key empties the reservoir.
func cmdSAMPLERESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	sm, err := readSampler(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := replaceRecord(ctx, key, sampleMarker, i64(sm.size), "count", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSAMPLEINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	sm, err := readSampler(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"), resp.IntegerValue(sm.slots()),
		resp.BulkString("total_count"), resp.IntegerValue(sm.count),
	})
}

// --- HISTOGRAM ---

// A histogram holds its min, max, width, count and sum, and each bucket
// that has values as the field b<index>. Values below min fall in bucket
// -1 and values from max on in the last bucket.

type histogram struct {
	min, max, width, sum float64
	count                int64
	buckets              map[int64]int64
}

func readHistogram(ctx *Context, key string) (*histogram, error) {
	r, err := loadMarked(ctx, key, histogramMarker, "histogram")
	if err != nil {
		return nil, err
	}
	rec := &record{vals: r}
	h := &histogram{min: rec.float("min"), max: rec.float("max"), width: rec.float("width"), sum: rec.float("sum"), count: rec.int("count"), buckets: map[int64]int64{}}
	for f, v := range r {
		idx, ok := strings.CutPrefix(f, "b")
		if !ok {
			continue
		}
		i, err1 := strconv.ParseInt(idx, 10, 64)
		n, err2 := strconv.ParseInt(v, 10, 64)
		if err1 != nil || err2 != nil {
			return nil, store.ErrWrongType
		}
		h.buckets[i] = n
	}
	if rec.bad || h.width <= 0 {
		return nil, store.ErrWrongType
	}
	return h, nil
}

func (h *histogram) bucket(v float64) int64 {
	switch {
	case v < h.min:
		return -1
	case v >= h.max:
		return int64((h.max - h.min) / h.width)
	}
	return int64((v - h.min) / h.width)
}

func (h *histogram) mean() string {
	if h.count == 0 {
		return "0"
	}
	return f64(h.sum / float64(h.count))
}

// HISTOGRAM.CREATE key min max [width] makes an empty histogram, replacing
// one at key. The width defaults to 1.
func cmdHISTOGRAMCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 && ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	b, err := parseFloatList(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	width := 1.0
	if len(b) == 3 {
		width = b[2]
	}
	if b[0] >= b[1] || width <= 0 {
		return ctx.WriteError(errors.New("ERR min must be below max and width positive"))
	}
	if (b[1]-b[0])/width > maxExtensionSlots {
		return ctx.WriteError(fmt.Errorf("ERR at most %d buckets", maxExtensionSlots))
	}
	if err := createMarked(ctx, ctx.ArgString(0), histogramMarker, histogramMarker, "1", "min", f64(b[0]), "max", f64(b[1]), "width", f64(width), "count", "0", "sum", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// HISTOGRAM.ADD key value [value ...] counts the values: how many.
func cmdHISTOGRAMADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	values, err := parseFloatList(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	h, err := readHistogram(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	touched := map[int64]bool{}
	for _, v := range values {
		b := h.bucket(v)
		h.buckets[b]++
		touched[b] = true
		h.count++
		h.sum += v
	}
	if math.IsInf(h.sum, 0) {
		return ctx.WriteError(errors.New("ERR sum would overflow"))
	}
	pairs := []string{"count", i64(h.count), "sum", f64(h.sum)}
	for b := range touched {
		pairs = append(pairs, "b"+i64(b), i64(h.buckets[b]))
	}
	if err := updateRecord(ctx, key, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(values)))
}

// HISTOGRAM.GET key: each bucket's range and count, lowest first.
func cmdHISTOGRAMGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, err := readHistogram(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	idx := make([]int64, 0, len(h.buckets))
	for b := range h.buckets {
		idx = append(idx, b)
	}
	sort.Slice(idx, func(i, j int) bool { return idx[i] < idx[j] })
	out := make([]*resp.Value, 0, 2*len(idx))
	for _, b := range idx {
		lo, hi := h.min+float64(b)*h.width, h.min+float64(b+1)*h.width
		out = append(out, resp.BulkString("["+f64(lo)+","+f64(hi)+")"), resp.IntegerValue(h.buckets[b]))
	}
	return ctx.WriteArray(out)
}

func cmdHISTOGRAMMEAN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, err := readHistogram(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(h.mean())
}

// HISTOGRAM.RESET key empties the histogram, keeping its range.
func cmdHISTOGRAMRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	h, err := readHistogram(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := replaceRecord(ctx, key, histogramMarker, "1", "min", f64(h.min), "max", f64(h.max), "width", f64(h.width), "count", "0", "sum", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdHISTOGRAMINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, err := readHistogram(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("min"), resp.BulkString(f64(h.min)),
		resp.BulkString("max"), resp.BulkString(f64(h.max)),
		resp.BulkString("bucket_width"), resp.BulkString(f64(h.width)),
		resp.BulkString("count"), resp.IntegerValue(h.count),
		resp.BulkString("mean"), resp.BulkString(h.mean()),
	})
}
