package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
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
	tdigests     = make(map[string]*tdigestSketch)
	tdigestsMu   sync.RWMutex
	samplers     = make(map[string]*store.ReservoirSampler)
	samplersMu   sync.RWMutex
	histograms   = make(map[string]*store.Histogram)
	histogramsMu sync.RWMutex
)

var (
	errTDigestNoKey       = errors.New("ERR T-Digest: key does not exist")
	errTDigestKeyExists   = errors.New("ERR T-Digest: key already exists")
	errTDigestCompression = errors.New("ERR T-Digest: error parsing compression parameter")
	errTDigestValue       = errors.New("ERR T-Digest: error parsing val parameter")
	errTDigestQuantile    = errors.New("ERR T-Digest: quantile should be in [0,1]")
	errTDigestRank        = errors.New("ERR T-Digest: rank needs to be non negative")
	errTDigestNumKeys     = errors.New("ERR T-Digest: numkeys needs to be a positive integer")
	errTDigestCut         = errors.New("ERR T-Digest: low_cut_percentile and high_cut_percentile should be in [0,1]")
	errTDigestCutOrder    = errors.New("ERR T-Digest: low_cut_percentile should be lower than high_cut_percentile")
)

const tdigestDefaultCompression = 100

type tdigestSketch struct {
	mu                sync.Mutex
	compression       int64
	means             []float64
	weights           []float64
	pendingMeans      []float64
	pendingWeights    []float64
	mergedWeight      float64
	unmergedWeight    float64
	min               float64
	max               float64
	totalCompressions int64
}

func newTDigestSketch(compression int64) *tdigestSketch {
	td := &tdigestSketch{compression: compression}
	td.resetLocked()
	return td
}

func (td *tdigestSketch) capacity() int {
	return int(6*td.compression + 10)
}

func (td *tdigestSketch) resetLocked() {
	td.means = td.means[:0]
	td.weights = td.weights[:0]
	td.pendingMeans = td.pendingMeans[:0]
	td.pendingWeights = td.pendingWeights[:0]
	td.mergedWeight = 0
	td.unmergedWeight = 0
	td.min = math.Inf(1)
	td.max = math.Inf(-1)
	td.totalCompressions = 0
}

func (td *tdigestSketch) total() float64 {
	return td.mergedWeight + td.unmergedWeight
}

func (td *tdigestSketch) addLocked(v, w float64) {
	if v < td.min {
		td.min = v
	}
	if v > td.max {
		td.max = v
	}
	td.pendingMeans = append(td.pendingMeans, v)
	td.pendingWeights = append(td.pendingWeights, w)
	td.unmergedWeight += w
	if len(td.means)+len(td.pendingMeans) >= td.capacity() {
		td.compressLocked()
	}
}

func (td *tdigestSketch) scaleK(q float64) float64 {
	return float64(td.compression) / (2 * math.Pi) * math.Asin(2*q-1)
}

func (td *tdigestSketch) compressLocked() {
	if len(td.pendingMeans) == 0 {
		return
	}
	n := len(td.means) + len(td.pendingMeans)
	means := append(append(make([]float64, 0, n), td.means...), td.pendingMeans...)
	weights := append(append(make([]float64, 0, n), td.weights...), td.pendingWeights...)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return means[idx[a]] < means[idx[b]] })

	total := td.total()
	outMeans := make([]float64, 0, n)
	outWeights := make([]float64, 0, n)
	curMean, curWeight := means[idx[0]], weights[idx[0]]
	soFar := 0.0
	for _, j := range idx[1:] {
		m, w := means[j], weights[j]
		if td.scaleK(math.Min(1, (soFar+curWeight+w)/total))-td.scaleK(soFar/total) <= 1 {
			curWeight += w
			curMean += (m - curMean) * w / curWeight
			continue
		}
		soFar += curWeight
		outMeans = append(outMeans, curMean)
		outWeights = append(outWeights, curWeight)
		curMean, curWeight = m, w
	}
	outMeans = append(outMeans, curMean)
	outWeights = append(outWeights, curWeight)

	td.means = outMeans
	td.weights = outWeights
	td.pendingMeans = td.pendingMeans[:0]
	td.pendingWeights = td.pendingWeights[:0]
	td.mergedWeight = total
	td.unmergedWeight = 0
	td.totalCompressions++
}

func tdWeightedAverage(x1, w1, x2, w2 float64) float64 {
	if x1 > x2 {
		x1, w1, x2, w2 = x2, w2, x1, w1
	}
	v := (x1*w1 + x2*w2) / (w1 + w2)
	return math.Max(x1, math.Min(v, x2))
}

func (td *tdigestSketch) quantileLocked(q float64) float64 {
	td.compressLocked()
	n := len(td.means)
	if n == 0 {
		return math.NaN()
	}
	if q <= 0 {
		return td.min
	}
	if q >= 1 {
		return td.max
	}
	if n == 1 {
		return td.means[0]
	}
	total := td.mergedWeight
	index := q * total
	if index < 1 {
		return td.min
	}
	if td.weights[0] > 2 && index < td.weights[0]/2 {
		return td.min + (index-1)/(td.weights[0]/2-1)*(td.means[0]-td.min)
	}
	if index > total-1 {
		return td.max
	}
	last := n - 1
	if td.weights[last] > 2 && total-index <= td.weights[last]/2 {
		return td.max - (total-index-1)/(td.weights[last]/2-1)*(td.max-td.means[last])
	}
	soFar := td.weights[0] / 2
	for i := 0; i < last; i++ {
		dw := (td.weights[i] + td.weights[i+1]) / 2
		if soFar+dw > index {
			leftUnit := 0.0
			if td.weights[i] == 1 {
				if index-soFar < 0.5 {
					return td.means[i]
				}
				leftUnit = 0.5
			}
			rightUnit := 0.0
			if td.weights[i+1] == 1 {
				if soFar+dw-index <= 0.5 {
					return td.means[i+1]
				}
				rightUnit = 0.5
			}
			z1 := index - soFar - leftUnit
			z2 := soFar + dw - index - rightUnit
			return tdWeightedAverage(td.means[i], z2, td.means[i+1], z1)
		}
		soFar += dw
	}
	z1 := index - soFar
	z2 := td.weights[last]/2 - z1
	return tdWeightedAverage(td.means[last], z2, td.max, z1)
}

func (td *tdigestSketch) cdfLocked(x float64) float64 {
	td.compressLocked()
	n := len(td.means)
	if n == 0 {
		return math.NaN()
	}
	if x < td.min {
		return 0
	}
	if x > td.max {
		return 1
	}
	total := td.mergedWeight
	if n == 1 {
		if td.max-td.min == 0 {
			return 0.5
		}
		return (x - td.min) / (td.max - td.min)
	}
	if x < td.means[0] {
		if td.means[0]-td.min > 0 {
			if x == td.min {
				return 0.5 / total
			}
			return (1 + (x-td.min)/(td.means[0]-td.min)*(td.weights[0]/2-1)) / total
		}
		return 0
	}
	last := n - 1
	if x > td.means[last] {
		if td.max-td.means[last] > 0 {
			if x == td.max {
				return 1 - 0.5/total
			}
			dq := (1 + (td.max-x)/(td.max-td.means[last])*(td.weights[last]/2-1)) / total
			return 1 - dq
		}
		return 1
	}
	soFar := 0.0
	for i := 0; i < last; i++ {
		if td.means[i] == x {
			dw := 0.0
			for i < n && td.means[i] == x {
				dw += td.weights[i]
				i++
			}
			return (soFar + dw/2) / total
		}
		if td.means[i] <= x && x < td.means[i+1] {
			if td.means[i+1]-td.means[i] > 0 {
				leftExcluded, rightExcluded := 0.0, 0.0
				if td.weights[i] == 1 {
					if td.weights[i+1] == 1 {
						return (soFar + 1) / total
					}
					leftExcluded = 0.5
				} else if td.weights[i+1] == 1 {
					rightExcluded = 0.5
				}
				dw := (td.weights[i] + td.weights[i+1]) / 2
				base := soFar + td.weights[i]/2 + leftExcluded
				return (base + (dw-leftExcluded-rightExcluded)*(x-td.means[i])/(td.means[i+1]-td.means[i])) / total
			}
			return (soFar + (td.weights[i]+td.weights[i+1])/2) / total
		}
		soFar += td.weights[i]
	}
	return 1 - 0.5/total
}

func (td *tdigestSketch) rankLocked(v float64, reverse bool) int64 {
	total := td.total()
	if total == 0 {
		return -2
	}
	if v < td.min {
		if reverse {
			return int64(total)
		}
		return -1
	}
	if v > td.max {
		if reverse {
			return -1
		}
		return int64(total)
	}
	abs := td.cdfLocked(v) * total
	if reverse {
		return int64(total - math.Round(abs))
	}
	if abs-math.Floor(abs) > 0.5 {
		return int64(math.Ceil(abs))
	}
	return int64(math.Floor(abs))
}

func (td *tdigestSketch) byRankLocked(rank float64, reverse bool) float64 {
	total := td.total()
	if total == 0 {
		return math.NaN()
	}
	if rank >= total {
		if reverse {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	if reverse {
		return td.quantileLocked((total - rank - 1) / total)
	}
	return td.quantileLocked(rank / total)
}

func (td *tdigestSketch) trimmedMeanLocked(low, high float64) float64 {
	td.compressLocked()
	total := td.mergedWeight
	if total == 0 {
		return math.NaN()
	}
	left, right := low*total, high*total
	var sum, weight, cum float64
	for i, m := range td.means {
		start, end := cum, cum+td.weights[i]
		cum = end
		overlap := math.Min(end, right) - math.Max(start, left)
		if overlap > 0 {
			sum += m * overlap
			weight += overlap
		}
	}
	if weight == 0 {
		return math.NaN()
	}
	return sum / weight
}

func (td *tdigestSketch) meanLocked() float64 {
	td.compressLocked()
	if td.mergedWeight == 0 {
		return math.NaN()
	}
	var sum float64
	for i, m := range td.means {
		sum += m * td.weights[i]
	}
	return sum / td.mergedWeight
}

func formatTDigestFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func tdigestDouble(v float64) *resp.Value {
	return resp.BulkString(formatTDigestFloat(v))
}

func parseTDigestCompression(b []byte) (int64, error) {
	c, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil || c <= 0 || c > 1<<20 {
		return 0, errTDigestCompression
	}
	return c, nil
}

func withTDigest(ctx *Context, fn func(td *tdigestSketch) *resp.Value) error {
	tdigestsMu.RLock()
	td, exists := tdigests[ctx.ArgString(0)]
	if !exists {
		tdigestsMu.RUnlock()
		return ctx.WriteError(errTDigestNoKey)
	}
	td.mu.Lock()
	reply := fn(td)
	td.mu.Unlock()
	tdigestsMu.RUnlock()
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
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	compression := int64(tdigestDefaultCompression)
	if ctx.ArgCount() == 3 {
		if !strings.EqualFold(ctx.ArgString(1), "COMPRESSION") {
			return ctx.WriteError(ErrSyntaxError)
		}
		c, err := parseTDigestCompression(ctx.Arg(2))
		if err != nil {
			return ctx.WriteError(err)
		}
		compression = c
	}

	tdigestsMu.Lock()
	if _, exists := tdigests[key]; exists {
		tdigestsMu.Unlock()
		return ctx.WriteError(errTDigestKeyExists)
	}
	tdigests[key] = newTDigestSketch(compression)
	tdigestsMu.Unlock()

	return ctx.WriteOK()
}

func cmdTDIGESTADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	values, err := parseTDigestFloats(ctx, 1, errTDigestValue)
	if err != nil {
		return ctx.WriteError(err)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		for _, v := range values {
			td.addLocked(v, 1)
		}
		return resp.SimpleString("OK")
	})
}

func cmdTDIGESTQUANTILE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	qs, err := parseTDigestFloats(ctx, 1, errTDigestQuantile)
	if err != nil {
		return ctx.WriteError(err)
	}
	for _, q := range qs {
		if q < 0 || q > 1 {
			return ctx.WriteError(errTDigestQuantile)
		}
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		out := make([]*resp.Value, len(qs))
		for i, q := range qs {
			out[i] = tdigestDouble(td.quantileLocked(q))
		}
		return resp.ArrayValue(out)
	})
}

func cmdTDIGESTCDF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	vals, err := parseTDigestFloats(ctx, 1, errTDigestValue)
	if err != nil {
		return ctx.WriteError(err)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		out := make([]*resp.Value, len(vals))
		for i, v := range vals {
			out[i] = tdigestDouble(td.cdfLocked(v))
		}
		return resp.ArrayValue(out)
	})
}

func tdigestRankCmd(ctx *Context, reverse bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	vals, err := parseTDigestFloats(ctx, 1, errTDigestValue)
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
		return ctx.WriteError(ErrWrongArgCount)
	}
	ranks := make([]float64, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		r, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
		if err != nil || r < 0 {
			return ctx.WriteError(errTDigestRank)
		}
		ranks = append(ranks, float64(r))
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		out := make([]*resp.Value, len(ranks))
		for i, r := range ranks {
			out[i] = tdigestDouble(td.byRankLocked(r, reverse))
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
		return ctx.WriteError(ErrWrongArgCount)
	}
	cuts, err := parseTDigestFloats(ctx, 1, errTDigestCut)
	if err != nil {
		return ctx.WriteError(err)
	}
	low, high := cuts[0], cuts[1]
	if low < 0 || low > 1 || high < 0 || high > 1 {
		return ctx.WriteError(errTDigestCut)
	}
	if low >= high {
		return ctx.WriteError(errTDigestCutOrder)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		return tdigestDouble(td.trimmedMeanLocked(low, high))
	})
}

func cmdTDIGESTMEAN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		return tdigestDouble(td.meanLocked())
	})
}

func cmdTDIGESTMIN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		if td.total() == 0 {
			return tdigestDouble(math.NaN())
		}
		return tdigestDouble(td.min)
	})
}

func cmdTDIGESTMAX(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		if td.total() == 0 {
			return tdigestDouble(math.NaN())
		}
		return tdigestDouble(td.max)
	})
}

func cmdTDIGESTINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		capacity := int64(td.capacity())
		return resp.ArrayValue([]*resp.Value{
			resp.BulkString("Compression"), resp.IntegerValue(td.compression),
			resp.BulkString("Capacity"), resp.IntegerValue(capacity),
			resp.BulkString("Merged nodes"), resp.IntegerValue(int64(len(td.means))),
			resp.BulkString("Unmerged nodes"), resp.IntegerValue(int64(len(td.pendingMeans))),
			resp.BulkString("Merged weight"), resp.IntegerValue(int64(td.mergedWeight)),
			resp.BulkString("Unmerged weight"), resp.IntegerValue(int64(td.unmergedWeight)),
			resp.BulkString("Observations"), resp.IntegerValue(int64(td.total())),
			resp.BulkString("Total compressions"), resp.IntegerValue(td.totalCompressions),
			resp.BulkString("Memory usage"), resp.IntegerValue(int64(cap(td.means)+cap(td.weights)+cap(td.pendingMeans)+cap(td.pendingWeights))*8 + 128),
		})
	})
}

func cmdTDIGESTRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return withTDigest(ctx, func(td *tdigestSketch) *resp.Value {
		td.resetLocked()
		return resp.SimpleString("OK")
	})
}

func cmdTDIGESTMERGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	destKey := ctx.ArgString(0)
	numKeys, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil || numKeys <= 0 {
		return ctx.WriteError(errTDigestNumKeys)
	}
	if 2+numKeys > ctx.ArgCount() {
		return ctx.WriteError(ErrWrongArgCount)
	}
	srcKeys := make([]string, numKeys)
	for i := range srcKeys {
		srcKeys[i] = ctx.ArgString(2 + i)
	}
	compression := int64(0)
	override := false
	for i := 2 + numKeys; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "COMPRESSION":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			c, err := parseTDigestCompression(ctx.Arg(i + 1))
			if err != nil {
				return ctx.WriteError(err)
			}
			compression = c
			i++
		case "OVERRIDE":
			override = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	tdigestsMu.Lock()
	sources := make([]*tdigestSketch, 0, numKeys+1)
	seen := make(map[string]bool, numKeys)
	maxCompression := int64(0)
	for _, k := range srcKeys {
		src, exists := tdigests[k]
		if !exists {
			tdigestsMu.Unlock()
			return ctx.WriteError(errTDigestNoKey)
		}
		if src.compression > maxCompression {
			maxCompression = src.compression
		}
		if !seen[k] {
			seen[k] = true
			sources = append(sources, src)
		}
	}
	dest, destExists := tdigests[destKey]
	if destExists && !override && !seen[destKey] {
		sources = append(sources, dest)
	}
	if compression == 0 {
		if destExists && !override {
			compression = dest.compression
		} else {
			compression = maxCompression
		}
	}

	merged := newTDigestSketch(compression)
	for _, src := range sources {
		src.mu.Lock()
		src.compressLocked()
		for i, m := range src.means {
			merged.addLocked(m, src.weights[i])
		}
		if src.total() > 0 {
			merged.min = math.Min(merged.min, src.min)
			merged.max = math.Max(merged.max, src.max)
		}
		src.mu.Unlock()
	}
	merged.compressLocked()
	tdigests[destKey] = merged
	tdigestsMu.Unlock()

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
