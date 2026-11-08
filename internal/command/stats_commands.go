package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

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
	samplers     = make(map[string]*store.ReservoirSampler)
	samplersMu   sync.RWMutex
	histograms   = make(map[string]*store.Histogram)
	histogramsMu sync.RWMutex
)

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

func cmdSAMPLECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))

	if size <= 0 {
		size = 1000
	}
	// The reservoir is allocated at full size up front.
	if size > maxExtensionSlots {
		return ctx.WriteError(fmt.Errorf("ERR size must be at most %d", maxExtensionSlots))
	}

	samplersMu.Lock()
	samplers[key] = store.NewReservoirSampler(size)
	samplersMu.Unlock()

	return ctx.WriteOK()
}

func cmdSAMPLEADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	samplersMu.RLock()
	rs, exists := samplers[key]
	samplersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sampler not found"))
	}

	count := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		v, err := parseFloat(ctx.Arg(i))
		if err != nil {
			continue
		}
		rs.Add(v)
		count++
	}

	return ctx.WriteInteger(int64(count))
}

func cmdSAMPLEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	samplersMu.RLock()
	rs, exists := samplers[key]
	samplersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sampler not found"))
	}

	data := rs.Get()
	results := make([]*resp.Value, len(data))
	for i, v := range data {
		results[i] = resp.BulkString(strconv.FormatFloat(v, 'f', -1, 64))
	}

	return ctx.WriteArray(results)
}

func cmdSAMPLERESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	samplersMu.RLock()
	rs, exists := samplers[key]
	samplersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sampler not found"))
	}

	rs.Reset()

	return ctx.WriteOK()
}

func cmdSAMPLEINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	samplersMu.RLock()
	rs, exists := samplers[key]
	samplersMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sampler not found"))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"),
		resp.IntegerValue(int64(rs.Size())),
		resp.BulkString("total_count"),
		resp.IntegerValue(rs.TotalCount()),
	})
}

func cmdHISTOGRAMCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	min, err := parseFloat(ctx.Arg(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	max, err := parseFloat(ctx.Arg(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	bucketWidth := 1.0

	if ctx.ArgCount() >= 4 {
		bw, err := parseFloat(ctx.Arg(3))
		if err == nil && bw > 0 {
			bucketWidth = bw
		}
	}

	histogramsMu.Lock()
	histograms[key] = store.NewHistogram(min, max, bucketWidth)
	histogramsMu.Unlock()

	return ctx.WriteOK()
}

func cmdHISTOGRAMADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	// The histogram is mutated under the registry write lock. The readers
	// (HISTOGRAM.INFO) read its fields under the registry read lock, so the
	// two must exclude each other: adding to the histogram with no lock at
	// all raced with every reader of the same histogram.
	histogramsMu.Lock()
	h, exists := histograms[key]
	if !exists {
		histogramsMu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR histogram not found"))
	}

	count := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		v, err := parseFloat(ctx.Arg(i))
		if err != nil {
			continue
		}
		h.Add(v)
		count++
	}
	histogramsMu.Unlock()

	return ctx.WriteInteger(int64(count))
}

func cmdHISTOGRAMGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	histogramsMu.RLock()
	h, exists := histograms[key]
	histogramsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR histogram not found"))
	}

	buckets := h.Get()
	results := make([]*resp.Value, 0)
	for k, v := range buckets {
		results = append(results, resp.BulkString(k), resp.IntegerValue(v))
	}

	return ctx.WriteArray(results)
}

func cmdHISTOGRAMMEAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	histogramsMu.RLock()
	h, exists := histograms[key]
	histogramsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR histogram not found"))
	}

	return ctx.WriteBulkString(strconv.FormatFloat(h.Mean(), 'f', -1, 64))
}

func cmdHISTOGRAMRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	histogramsMu.Lock()
	h, exists := histograms[key]

	if !exists {
		histogramsMu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR histogram not found"))
	}

	h.Reset()

	histogramsMu.Unlock()
	return ctx.WriteOK()
}

func cmdHISTOGRAMINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	histogramsMu.RLock()
	h, exists := histograms[key]

	if !exists {
		reply := fmt.Errorf("ERR histogram not found")
		histogramsMu.RUnlock()
		return ctx.WriteError(reply)
	}

	reply := []*resp.Value{
		resp.BulkString("min"),
		resp.BulkString(strconv.FormatFloat(h.Min, 'f', -1, 64)),
		resp.BulkString("max"),
		resp.BulkString(strconv.FormatFloat(h.Max, 'f', -1, 64)),
		resp.BulkString("bucket_width"),
		resp.BulkString(strconv.FormatFloat(h.BucketWidth, 'f', -1, 64)),
		resp.BulkString("count"),
		resp.IntegerValue(h.Count),
		resp.BulkString("mean"),
		resp.BulkString(strconv.FormatFloat(h.Mean(), 'f', -1, 64)),
	}
	histogramsMu.RUnlock()
	return ctx.WriteArray(reply)
}
