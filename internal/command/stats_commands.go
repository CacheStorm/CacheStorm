package command

import (
	"fmt"
	"strconv"
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
	tdigests     = make(map[string]*store.TDigest)
	tdigestsMu   sync.RWMutex
	samplers     = make(map[string]*store.ReservoirSampler)
	samplersMu   sync.RWMutex
	histograms   = make(map[string]*store.Histogram)
	histogramsMu sync.RWMutex
)

func cmdTDIGESTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	compression := 100.0

	if ctx.ArgCount() >= 2 {
		var err error
		compression, err = parseFloat(ctx.Arg(1))
		if err != nil || compression <= 0 {
			compression = 100
		}
	}

	tdigestsMu.Lock()
	tdigests[key] = store.NewTDigest(compression)
	tdigestsMu.Unlock()

	return ctx.WriteOK()
}

func cmdTDIGESTADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		tdigestsMu.Lock()
		td = store.NewTDigest(100)
		tdigests[key] = td
		tdigestsMu.Unlock()
	}

	values := make([]float64, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		v, err := parseFloat(ctx.Arg(i))
		if err != nil {
			continue
		}
		values = append(values, v)
	}

	td.AddBatch(values)

	return ctx.WriteInteger(int64(len(values)))
}

func cmdTDIGESTQUANTILE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	results := make([]*resp.Value, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		q, err := parseFloat(ctx.Arg(i))
		if err != nil {
			continue
		}
		results = append(results, resp.BulkString(strconv.FormatFloat(td.Quantile(q), 'f', -1, 64)))
	}

	if len(results) == 1 {
		return ctx.WriteValue(results[0])
	}

	return ctx.WriteArray(results)
}

func cmdTDIGESTCDF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	results := make([]*resp.Value, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		v, err := parseFloat(ctx.Arg(i))
		if err != nil {
			continue
		}
		cdf := td.CDF(v)
		results = append(results, resp.BulkString(strconv.FormatFloat(cdf, 'f', 6, 64)))
	}

	if len(results) == 1 {
		return ctx.WriteValue(results[0])
	}

	return ctx.WriteArray(results)
}

func cmdTDIGESTMEAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	return ctx.WriteBulkString(strconv.FormatFloat(td.Mean(), 'f', -1, 64))
}

func cmdTDIGESTMIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	return ctx.WriteBulkString(strconv.FormatFloat(td.Min(), 'f', -1, 64))
}

func cmdTDIGESTMAX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	return ctx.WriteBulkString(strconv.FormatFloat(td.Max(), 'f', -1, 64))
}

func cmdTDIGESTINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("compression"),
		resp.BulkString(strconv.FormatFloat(td.Compression, 'f', -1, 64)),
		resp.BulkString("capacity"),
		resp.IntegerValue(int64(td.Size())),
		resp.BulkString("merged_nodes"),
		resp.IntegerValue(int64(td.Size())),
		resp.BulkString("unmerged_nodes"),
		resp.IntegerValue(0),
		resp.BulkString("total_weight"),
		resp.BulkString(strconv.FormatFloat(td.Count(), 'f', -1, 64)),
		resp.BulkString("min"),
		resp.BulkString(strconv.FormatFloat(td.Min(), 'f', -1, 64)),
		resp.BulkString("max"),
		resp.BulkString(strconv.FormatFloat(td.Max(), 'f', -1, 64)),
		resp.BulkString("mean"),
		resp.BulkString(strconv.FormatFloat(td.Mean(), 'f', -1, 64)),
	})
}

func cmdTDIGESTRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	tdigestsMu.RLock()
	td, exists := tdigests[key]
	tdigestsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tdigest not found"))
	}

	td.Reset()

	return ctx.WriteOK()
}

func cmdTDIGESTMERGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	destKey := ctx.ArgString(0)

	tdigestsMu.Lock()
	defer tdigestsMu.Unlock()

	destTD, exists := tdigests[destKey]
	if !exists {
		destTD = store.NewTDigest(100)
		tdigests[destKey] = destTD
	}

	for i := 1; i < ctx.ArgCount(); i++ {
		srcKey := ctx.ArgString(i)
		srcTD, exists := tdigests[srcKey]
		if !exists {
			continue
		}
		destTD.Merge(srcTD)
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
	min, _ := parseFloat(ctx.Arg(1))
	max, _ := parseFloat(ctx.Arg(2))
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

	histogramsMu.RLock()
	h, exists := histograms[key]
	histogramsMu.RUnlock()

	if !exists {
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

	histogramsMu.RLock()
	h, exists := histograms[key]
	histogramsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR histogram not found"))
	}

	h.Reset()

	return ctx.WriteOK()
}

func cmdHISTOGRAMINFO(ctx *Context) error {
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

	return ctx.WriteArray([]*resp.Value{
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
	})
}
