package persistence

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// RedisTimeSeries' series type. Redis 8.2 saves encoding version 8; every
// version it loads (0 to 8) is read.
var rdbModuleTimeSeries = rdbModuleID("TSDB-TYPE", tsEncVer)

// The encoding versions that changed the layout, as RedisTimeSeries names
// them.
const (
	tsEncUncompressed  = 1 // options saved
	tsEncSize          = 2 // chunks saved whole, chunk size in bytes
	tsEncResetDupPol   = 3 // duplicate policy, first's isResetted
	tsEncOverflow      = 4 // avg's overflow flag
	tsEncAlignment     = 6 // rule alignment
	tsEncLastAggEmpty  = 7 // single-value contexts lose isResetted
	tsEncVer           = 8 // ignore settings
	tsOptUncompressed  = 1
	tsOptCompressedGor = 2
)

// tsAggTypes are RedisTimeSeries' aggregation types by their enum value.
var tsAggTypes = []string{"", "min", "max", "sum", "avg", "count", "first", "last", "range", "std.p", "std.s", "var.p", "var.s", "twa"}

// tsDupPolicies are the duplicate policies by their enum value; 0 is none,
// a series that follows the module's default.
var tsDupPolicies = []string{store.TSNoPolicy, "BLOCK", "LAST", "FIRST", "MIN", "MAX", "SUM"}

func tsAggType(agg string) uint64 {
	for i, a := range tsAggTypes {
		if a == agg && i > 0 {
			return uint64(i)
		}
	}
	return 0
}

// maxTSRetentionMillis is the longest retention a series keeps, as
// TS.CREATE clamps it: what a duration holds.
const maxTSRetentionMillis = math.MaxInt64 / int64(time.Millisecond)

// appendRedisTimeSeries writes a series as series_rdb_save does at encoding
// version 8. crossRefs saves its source key and rules, which RedisTimeSeries
// saves only while persisting (an RDB, a full sync, an AOF rewrite), not in
// a DUMP payload. key is the series' own name, saved in the value; RESTORE
// replaces it with the key restored to.
func appendRedisTimeSeries(w *moduleWriter, key string, v *store.TimeSeriesValue, crossRefs bool) {
	v.RLock()
	defer v.RUnlock()
	w.rdbString([]byte(key))
	w.uint(uint64(v.Retention.Milliseconds()))
	chunkSize := v.ChunkSize
	if chunkSize <= 0 {
		chunkSize = store.TSDefaultChunkSize
	}
	w.uint(uint64(chunkSize))
	options := uint64(tsOptCompressedGor)
	if v.Uncompressed {
		options = tsOptUncompressed
	}
	w.uint(options)
	var lastTS uint64
	var lastValue float64
	if last, ok := v.Last(); ok {
		lastTS, lastValue = uint64(last.Timestamp), last.Value
	}
	w.uint(lastTS)
	w.double(lastValue)
	w.uint(uint64(v.TotalSamples()))
	policy := uint64(1)
	for i, p := range tsDupPolicies {
		if p == v.DuplicatePolicy {
			policy = uint64(i)
		}
	}
	w.uint(policy)
	if crossRefs && v.SourceKey != "" {
		w.uint(1)
		w.rdbString([]byte(v.SourceKey))
	} else {
		w.uint(0)
	}
	w.sint(v.IgnoreMaxTimeDiff)
	w.double(v.IgnoreMaxValDiff)
	labels := v.LabelList()
	w.uint(uint64(len(labels)))
	for _, l := range labels {
		w.rdbString([]byte(l[0]))
		w.rdbString([]byte(l[1]))
	}
	if !crossRefs {
		w.uint(0)
	} else {
		w.uint(uint64(len(v.Rules)))
		for i := range v.Rules {
			r := &v.Rules[i]
			w.rdbString([]byte(r.Dest))
			w.sint(r.Bucket)
			w.sint(r.Align)
			w.uint(tsAggType(r.Aggregation))
			start := uint64(math.MaxUint64)
			if r.Started {
				start = r.Start
			}
			w.uint(start)
			appendTSContext(w, r.Aggregation, &r.Ctx)
		}
	}
	appendTSChunks(w, v)
}

func tsBool(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// appendTSContext writes a rule's context as its aggregation's
// writeContext does.
func appendTSContext(w *moduleWriter, agg string, ctx *store.TSAggContext) {
	c := ctx
	if !c.Ready {
		c = &store.TSAggContext{}
		c.Init()
	}
	switch agg {
	case "min", "max", "range":
		w.double(c.Max)
		w.double(c.Min)
	case "sum", "count", "last":
		w.double(c.Value)
	case "first":
		w.double(c.Value)
		w.uint(tsBool(c.Reset))
	case "avg":
		w.double(c.Value)
		w.double(c.Count)
		w.uint(tsBool(c.Overflow))
	case "std.p", "std.s", "var.p", "var.s":
		w.double(c.Sum)
		w.double(c.Sum2)
		w.uint(c.N)
	case "twa":
		w.double(c.Res)
		w.uint(c.PrevTS)
		w.double(c.PrevValue)
		w.uint(c.BucketStart)
		w.uint(c.BucketEnd)
		w.uint(c.FirstTS)
		w.uint(c.LastTS)
		w.uint(tsBool(c.FirstBucket))
		w.uint(uint64(c.Iteration))
		w.uint(tsBool(c.Reverse))
	}
}

// appendTSChunks writes the samples in chunks of chunkSize bytes, filled
// as RedisTimeSeries fills them appending in order; an empty series has one
// empty chunk, as a new series does.
func appendTSChunks(w *moduleWriter, v *store.TimeSeriesValue) {
	counts, sizes := v.ChunkLayout()
	w.uint(uint64(len(counts)))
	samples := v.Samples
	for i, n := range counts {
		chunk := samples[:n]
		samples = samples[n:]
		size := uint64(sizes[i])
		if v.Uncompressed {
			size = max(size, uint64(n)*16)
			var base uint64
			if len(chunk) > 0 {
				base = uint64(chunk[0].Timestamp)
			}
			w.uint(base)
			w.uint(uint64(len(chunk)))
			w.uint(size)
			data := make([]byte, size)
			for j, s := range chunk {
				binary.LittleEndian.PutUint64(data[j*16:], uint64(s.Timestamp))
				binary.LittleEndian.PutUint64(data[j*16+8:], math.Float64bits(s.Value))
			}
			w.rdbString(data)
			continue
		}
		c := store.NewTSGorillaChunk(size, true)
		for _, s := range chunk {
			// As ensureAddSample grows a chunk an upsert overfills.
			for !c.Append(uint64(s.Timestamp), s.Value) {
				c.Size += 32
				c.Data = append(c.Data, 0, 0, 0, 0)
			}
		}
		w.uint(c.Size)
		w.uint(c.Count)
		w.uint(c.Idx)
		w.uint(c.BaseValue)
		w.uint(c.BaseTimestamp)
		w.uint(c.PrevTimestamp)
		w.sint(c.PrevDelta)
		w.uint(c.PrevValue)
		w.uint(uint64(c.PrevLeading))
		w.uint(uint64(c.PrevTrailing))
		data := make([]byte, 0, len(c.Data)*8)
		for _, word := range c.Data {
			data = binary.LittleEndian.AppendUint64(data, word)
		}
		w.rdbString(data[:c.Size])
	}
}

// rdbString writes a string item as RedisModule_SaveStringBuffer does,
// with rdbSaveRawString's integer and LZF encodings.
func (w *moduleWriter) rdbString(b []byte) {
	w.out = appendRDBRawString(appendDumpLength(w.out, rdbModuleOpcodeStr), b)
}

// moduleStringEnc reads a module string item into a string of its own,
// and how it was saved: plainly, as an integer or LZF-compressed.
func (d *dumpDecoder) moduleStringEnc() (string, store.RedisStringEnc) {
	if op, _ := d.length(); op != rdbModuleOpcodeStr {
		d.err = ErrDumpFormat
		return "", store.RedisStringPlain
	}
	enc := store.RedisStringPlain
	if d.err == nil && d.pos < len(d.data) && d.data[d.pos]>>6 == 3 {
		enc = store.RedisStringInt
		if d.data[d.pos]&63 == 3 {
			enc = store.RedisStringLZF
		}
	}
	b, _ := d.raw()
	return string(b), enc
}

// moduleString reads a module string item into a string of its own.
func (d *dumpDecoder) moduleString() string { return string(d.moduleBuffer()) }

// timeSeries reads a series as series_rdb_load does at encoding version
// encver, refusing what it refuses. Its rules are loaded in reverse, as
// RedisTimeSeries prepends each rule it loads.
func (d *dumpDecoder) timeSeries(encver uint64) store.Value {
	d.moduleString()
	retention := d.moduleUint()
	chunkSize := d.moduleUint()
	if encver < tsEncSize {
		chunkSize *= 16
	}
	options := uint64(tsOptUncompressed)
	if encver >= tsEncUncompressed {
		options = d.moduleUint()
	}
	if d.err != nil || chunkSize == 0 || chunkSize%8 != 0 || chunkSize > math.MaxInt32 {
		d.err = ErrDumpFormat
		return nil
	}
	policy := uint64(0)
	hasSrc := false
	var last store.TimeSeriesSample
	var total uint64
	if encver >= tsEncSize {
		lastTS := d.moduleUint()
		last = store.TimeSeriesSample{Timestamp: int64(lastTS), Value: d.moduleDouble()}
		total = d.moduleUint()
		if lastTS > math.MaxInt64 || total > math.MaxInt64 {
			d.err = ErrDumpFormat
		}
	}
	if encver >= tsEncResetDupPol {
		policy = d.moduleUint()
	}
	if encver >= tsEncSize {
		hasSrc = d.moduleUint() != 0
	}
	v := store.NewTimeSeriesValue(0)
	if hasSrc {
		v.SourceKey = d.moduleString()
	}
	if encver >= tsEncVer {
		v.IgnoreMaxTimeDiff = d.moduleSint()
		v.IgnoreMaxValDiff = d.moduleDouble()
	}
	if d.err != nil || policy >= uint64(len(tsDupPolicies)) {
		d.err = ErrDumpFormat
		return nil
	}
	v.DuplicatePolicy = tsDupPolicies[policy]
	v.ChunkSize = int64(chunkSize)
	v.Uncompressed = options&tsOptUncompressed != 0
	nlabels := d.moduleUint()
	if d.err != nil || nlabels > uint64(len(d.data)-d.pos) {
		d.err = ErrDumpFormat
		return nil
	}
	keys := make([]string, 0, nlabels)
	values := make([]string, 0, nlabels)
	var labelMem int64
	for i := uint64(0); i < nlabels && d.err == nil; i++ {
		k, kenc := d.moduleStringEnc()
		val, venc := d.moduleStringEnc()
		keys, values = append(keys, k), append(values, val)
		labelMem += store.RedisLoadedStringSize(len(k), kenc) + store.RedisLoadedStringSize(len(val), venc)
	}
	v.SetLabels(keys, values)
	if len(v.LabelKeys) == len(keys) {
		v.SetLoadedLabelSizes(labelMem)
	}
	nrules := d.moduleUint()
	if d.err != nil || nrules > uint64(len(d.data)-d.pos) {
		d.err = ErrDumpFormat
		return nil
	}
	for i := uint64(0); i < nrules && d.err == nil; i++ {
		r := store.TSRule{Dest: d.moduleString()}
		bucket := d.moduleUint()
		var align uint64
		if encver >= tsEncAlignment {
			align = d.moduleUint()
		}
		agg := d.moduleUint()
		start := d.moduleUint()
		if d.err != nil || bucket == 0 || bucket > math.MaxInt64 || agg == 0 || agg >= uint64(len(tsAggTypes)) {
			d.err = ErrDumpFormat
			return nil
		}
		r.Bucket, r.Align, r.Aggregation = int64(bucket), int64(align), tsAggTypes[agg]
		r.Started, r.Start = start != math.MaxUint64, start
		r.Ctx = d.tsContext(r.Aggregation, encver)
		v.Rules = append([]store.TSRule{r}, v.Rules...)
	}
	if d.err != nil {
		return nil
	}
	var samples []store.TimeSeriesSample
	var counts, sizes []int64
	var tail *store.TSGorillaChunk
	if encver < tsEncSize {
		n := d.moduleUint()
		if d.err != nil || n > uint64(len(d.data)-d.pos) {
			d.err = ErrDumpFormat
			return nil
		}
		for i := uint64(0); i < n && d.err == nil; i++ {
			ts := d.moduleUint()
			value := d.moduleDouble()
			if ts > math.MaxInt64 {
				d.err = ErrDumpFormat
			}
			if _, err := v.Upsert(int64(ts), value, "LAST"); err != nil {
				d.err = ErrDumpFormat
			}
		}
		samples = v.Samples
	} else {
		n := d.moduleUint()
		if d.err != nil || n == 0 || n > uint64(len(d.data)-d.pos) {
			d.err = ErrDumpFormat
			return nil
		}
		for i := uint64(0); i < n && d.err == nil; i++ {
			before := len(samples)
			var size uint64
			if v.Uncompressed {
				samples, size = d.tsUncompressedChunk(samples)
			} else {
				samples, tail = d.tsCompressedChunk(samples)
				if tail != nil {
					size = tail.Size
				}
			}
			counts, sizes = append(counts, int64(len(samples)-before)), append(sizes, int64(size))
		}
	}
	if d.err != nil {
		return nil
	}
	for i := 1; i < len(samples); i++ {
		if samples[i].Timestamp <= samples[i-1].Timestamp {
			d.err = ErrDumpFormat
			return nil
		}
	}
	v.Samples = samples
	if encver >= tsEncSize {
		v.SetLast(last)
		v.SetTotalSamples(int64(total))
	}
	v.MarkLoaded()
	if counts != nil {
		v.SetLoadedChunks(counts, sizes, tail)
	}
	v.SetRetention(time.Duration(min(retention, uint64(maxTSRetentionMillis))) * time.Millisecond)
	return v
}

func tsSample(ts uint64, value uint64) (store.TimeSeriesSample, bool) {
	return store.TimeSeriesSample{Timestamp: int64(ts), Value: math.Float64frombits(value)}, ts <= math.MaxInt64
}

// tsUncompressedChunk reads an uncompressed chunk as
// Uncompressed_LoadFromRDB does and appends its samples.
func (d *dumpDecoder) tsUncompressedChunk(out []store.TimeSeriesSample) ([]store.TimeSeriesSample, uint64) {
	d.moduleUint()
	n, size := d.moduleUint(), d.moduleUint()
	data := d.moduleBuffer()
	if d.err != nil || n > size/16 || size != uint64(len(data)) {
		d.err = ErrDumpFormat
		return nil, 0
	}
	for i := uint64(0); i < n; i++ {
		s, ok := tsSample(binary.LittleEndian.Uint64(data[i*16:]), binary.LittleEndian.Uint64(data[i*16+8:]))
		if !ok {
			d.err = ErrDumpFormat
			return nil, 0
		}
		out = append(out, s)
	}
	return out, size
}

// tsCompressedChunk reads a compressed chunk as Compressed_LoadFromRDB does,
// refusing what it refuses and bits past the buffer, and appends its
// samples.
func (d *dumpDecoder) tsCompressedChunk(out []store.TimeSeriesSample) ([]store.TimeSeriesSample, *store.TSGorillaChunk) {
	c := &store.TSGorillaChunk{Size: d.moduleUint(), Count: d.moduleUint(), Idx: d.moduleUint()}
	c.BaseValue, c.BaseTimestamp = d.moduleUint(), d.moduleUint()
	c.PrevTimestamp, c.PrevDelta, c.PrevValue = d.moduleUint(), int64(d.moduleUint()), d.moduleUint()
	c.PrevLeading, c.PrevTrailing = uint8(d.moduleUint()), uint8(d.moduleUint())
	data := d.moduleBuffer()
	n := uint64(len(data))
	if d.err != nil || n == 0 || c.Idx > n*8 || c.Size != n || n%8 != 0 || (c.Count > 0 && c.Idx/2 < c.Count-1) {
		d.err = ErrDumpFormat
		return nil, nil
	}
	c.Data = make([]uint64, n/8)
	for i := range c.Data {
		c.Data[i] = binary.LittleEndian.Uint64(data[i*8:])
	}
	samples, err := c.Samples()
	if err != nil {
		d.err = ErrDumpFormat
		return nil, nil
	}
	for _, s := range samples {
		if s.Timestamp < 0 {
			d.err = ErrDumpFormat
			return nil, nil
		}
	}
	return append(out, samples...), c
}

// tsContext reads a rule's context as its aggregation's readContext does
// at encoding version encver.
func (d *dumpDecoder) tsContext(agg string, encver uint64) store.TSAggContext {
	var c store.TSAggContext
	c.Init()
	switch agg {
	case "min", "max", "range":
		c.Max, c.Min = d.moduleDouble(), d.moduleDouble()
		if encver < tsEncAlignment {
			d.moduleBuffer()
		}
	case "sum", "count", "last":
		c.Value = d.moduleDouble()
		if encver >= tsEncResetDupPol && encver < tsEncLastAggEmpty {
			d.moduleUint()
		}
	case "first":
		c.Value = d.moduleDouble()
		if encver >= tsEncResetDupPol {
			c.Reset = d.moduleUint() != 0
		}
	case "avg":
		c.Value, c.Count = d.moduleDouble(), d.moduleDouble()
		if encver >= tsEncOverflow {
			c.Overflow = d.moduleUint() != 0
		}
	case "std.p", "std.s", "var.p", "var.s":
		c.Sum, c.Sum2, c.N = d.moduleDouble(), d.moduleDouble(), d.moduleUint()
	case "twa":
		c.Res = d.moduleDouble()
		c.PrevTS = d.moduleUint()
		c.PrevValue = d.moduleDouble()
		c.BucketStart, c.BucketEnd = d.moduleUint(), d.moduleUint()
		c.FirstTS, c.LastTS = d.moduleUint(), d.moduleUint()
		c.FirstBucket = d.moduleUint() != 0
		c.Iteration = int64(d.moduleUint())
		c.Reverse = d.moduleUint() != 0
	}
	return c
}

// appendTimeSeriesValue appends a series as Redis 8 saves a TSDB-TYPE
// value: the module value's type, id, data and EOF opcode.
func appendTimeSeriesValue(out []byte, key string, v *store.TimeSeriesValue, crossRefs bool) []byte {
	w := &moduleWriter{out: append(out, rdbTypeModule2)}
	w.out = appendDumpLength(w.out, rdbModuleTimeSeries)
	appendRedisTimeSeries(w, key, v, crossRefs)
	return appendDumpLength(w.out, rdbModuleOpcodeEOF)
}

// encodeTimeSeriesRestore is a RESTORE payload of a series that keeps its
// source key and rules, as RedisTimeSeries' payloads do while persisting:
// the AOF rewrite restores series from it.
func encodeTimeSeriesRestore(key string, v *store.TimeSeriesValue) []byte {
	out := appendTimeSeriesValue(nil, key, v, true)
	out = binary.LittleEndian.AppendUint16(out, 11)
	return binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
}

// legacyTimeSeries is a series as older CacheStorm versions saved it (gob,
// module type CStorm-TS): rules kept their open bucket's samples, and the
// last sample of the bucket before it.
type legacyTimeSeries struct {
	Samples           []store.TimeSeriesSample
	Labels            map[string]string
	LabelKeys         []string
	Retention         time.Duration
	DuplicatePolicy   string
	ChunkSize         int64
	Uncompressed      bool
	IgnoreMaxTimeDiff int64
	IgnoreMaxValDiff  float64
	Rules             []struct {
		Dest        string
		Aggregation string
		Bucket      int64
		Align       int64
		Open        []store.TimeSeriesSample
		Prev        *store.TimeSeriesSample
	}
	SourceKey string
}

// value converts the series, aggregating each rule's open samples into the
// context RedisTimeSeries would hold for them.
func (l *legacyTimeSeries) value() *store.TimeSeriesValue {
	v := store.NewTimeSeriesValue(l.Retention)
	v.Samples = l.Samples
	if v.Samples == nil {
		v.Samples = []store.TimeSeriesSample{}
	}
	if l.Labels != nil {
		v.Labels = l.Labels
	}
	v.LabelKeys = l.LabelKeys
	v.DuplicatePolicy, v.ChunkSize, v.Uncompressed = l.DuplicatePolicy, l.ChunkSize, l.Uncompressed
	v.IgnoreMaxTimeDiff, v.IgnoreMaxValDiff, v.SourceKey = l.IgnoreMaxTimeDiff, l.IgnoreMaxValDiff, l.SourceKey
	for _, lr := range l.Rules {
		r := store.TSRule{Dest: lr.Dest, Aggregation: lr.Aggregation, Bucket: lr.Bucket, Align: lr.Align}
		r.Ctx.Init()
		if len(lr.Open) > 0 && r.Bucket > 0 {
			first := uint64(lr.Open[0].Timestamp)
			cur := store.TSCalcBucketStart(first, uint64(r.Bucket), uint64(r.Align))
			r.Started, r.Start = true, store.TSNormalizeBucket(cur)
			if r.Aggregation == "twa" {
				r.Ctx.TwaBucket(r.Start, cur+uint64(r.Bucket))
				if lr.Prev != nil {
					r.Ctx.TwaPrev(lr.Prev.Value, uint64(lr.Prev.Timestamp))
				}
			}
			for _, s := range lr.Open {
				r.Ctx.Add(r.Aggregation, s.Value, uint64(s.Timestamp))
			}
		}
		v.Rules = append(v.Rules, r)
	}
	return v
}
