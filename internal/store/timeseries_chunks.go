package store

import (
	"errors"
	"math"
	"math/bits"
)

// TSGorillaChunk is a RedisTimeSeries compressed chunk: samples as
// delta-of-delta timestamps and XORed values in a bit buffer of Size bytes,
// with the state appending the next sample needs. Without Data it only
// measures the bits, to lay samples out in chunks as RedisTimeSeries does.
type TSGorillaChunk struct {
	Size, Count, Idx          uint64
	BaseValue, BaseTimestamp  uint64
	PrevTimestamp             uint64
	PrevDelta                 int64
	PrevValue                 uint64
	PrevLeading, PrevTrailing uint8
	Data                      []uint64
}

// NewTSGorillaChunk returns an empty chunk of size bytes; data is false to
// only measure.
func NewTSGorillaChunk(size uint64, data bool) *TSGorillaChunk {
	c := &TSGorillaChunk{Size: size, PrevLeading: 32, PrevTrailing: 32}
	if data {
		c.Data = make([]uint64, (size+7)/8)
	}
	return c
}

func (c *TSGorillaChunk) appendBits(data uint64, n uint) {
	if c.Data != nil {
		word, lbit := c.Idx>>6, uint(c.Idx&63)
		if n < 64 {
			data &= 1<<n - 1
		}
		c.Data[word] |= data << lbit
		if avail := 64 - lbit; avail < n {
			c.Data[word+1] |= data >> avail
		}
	}
	c.Idx += uint64(n)
}

func (c *TSGorillaChunk) space(n uint64) bool { return n <= c.Size*8-c.Idx }

func tsBinInRange(x int64, n uint) bool {
	return x >= -(1<<(n-1)) && x <= 1<<(n-1)-1
}

// Append is Compressed_Append: false when the sample does not fit, leaving
// the chunk as it was.
func (c *TSGorillaChunk) Append(ts uint64, value float64) bool {
	vbits := math.Float64bits(value)
	if c.Count == 0 {
		c.BaseValue, c.PrevValue = vbits, vbits
		c.BaseTimestamp, c.PrevTimestamp = ts, ts
		c.PrevDelta = 0
		c.Count++
		return true
	}
	idx := c.Idx
	if !c.appendInteger(ts) || !c.appendFloat(vbits) {
		if c.Data != nil {
			for b := idx; b < c.Idx && b < c.Size*8; b++ {
				c.Data[b>>6] &^= 1 << (b & 63)
			}
		}
		c.Idx = idx
		return false
	}
	c.Count++
	return true
}

var tsIntSteps = [...]uint{5, 8, 11, 15, 32}

func (c *TSGorillaChunk) appendInteger(ts uint64) bool {
	delta := ts - c.PrevTimestamp
	dd := int64(delta) - c.PrevDelta
	switch dd {
	case 0:
		if !c.space(2) {
			return false
		}
		c.appendBits(0, 1)
	default:
		done := false
		for i, n := range tsIntSteps {
			if tsBinInRange(dd, n) {
				ctl := uint(i + 2)
				if !c.space(uint64(ctl + n + 1)) {
					return false
				}
				c.appendBits(1<<(ctl-1)-1, ctl)
				c.appendBits(uint64(dd), n)
				done = true
				break
			}
		}
		if !done {
			if !c.space(6 + 64 + 1) {
				return false
			}
			c.appendBits(0x3f, 6)
			c.appendBits(uint64(dd), 64)
		}
	}
	c.PrevDelta = int64(delta)
	c.PrevTimestamp = ts
	return true
}

func (c *TSGorillaChunk) appendFloat(vbits uint64) bool {
	xor := vbits ^ c.PrevValue
	if xor == 0 {
		c.appendBits(0, 1)
		return true
	}
	c.appendBits(1, 1)
	leading := uint(min(bits.LeadingZeros64(xor), 31))
	trailing := uint(bits.TrailingZeros64(xor))
	block := 64 - leading - trailing
	expected := 5 + 6 + block
	prevBlock := 64 - uint(c.PrevLeading) - uint(c.PrevTrailing)
	if leading >= uint(c.PrevLeading) && trailing >= uint(c.PrevTrailing) && expected > prevBlock {
		if !c.space(uint64(prevBlock) + 1) {
			return false
		}
		c.appendBits(0, 1)
		c.appendBits(xor>>c.PrevTrailing, prevBlock)
	} else {
		if !c.space(uint64(expected) + 1) {
			return false
		}
		c.appendBits(1, 1)
		c.appendBits(uint64(leading), 5)
		c.appendBits(uint64(block-1), 6)
		c.appendBits(xor>>trailing, block)
		c.PrevLeading, c.PrevTrailing = uint8(leading), uint8(trailing)
	}
	c.PrevValue = vbits
	return true
}

// ErrTSChunk is a compressed chunk whose bits run past its buffer.
var ErrTSChunk = errors.New("time series chunk is corrupt")

type tsBitReader struct {
	data []uint64
	idx  uint64
	end  uint64
	err  bool
}

func (r *tsBitReader) read(n uint) uint64 {
	if r.err || r.idx+uint64(n) > r.end {
		r.err = true
		return 0
	}
	word, lbit := r.idx>>6, uint(r.idx&63)
	v := r.data[word] >> lbit
	if avail := 64 - lbit; avail < n {
		v |= r.data[word+1] << avail
	}
	if n < 64 {
		v &= 1<<n - 1
	}
	r.idx += uint64(n)
	return v
}

func (r *tsBitReader) bit() bool { return r.read(1) == 1 }

// Samples decodes the chunk's samples as Compressed_ChunkIteratorGetNext
// does, refusing bits beyond the buffer.
func (c *TSGorillaChunk) Samples() ([]TimeSeriesSample, error) {
	if c.Count == 0 {
		return nil, nil
	}
	if c.Count-1 > c.Size*4 {
		return nil, ErrTSChunk
	}
	out := make([]TimeSeriesSample, 0, c.Count)
	out = append(out, TimeSeriesSample{Timestamp: int64(c.BaseTimestamp), Value: math.Float64frombits(c.BaseValue)})
	r := &tsBitReader{data: c.Data, end: uint64(len(c.Data)) * 64}
	prevTS, prevDelta, prevValue := c.BaseTimestamp, int64(0), c.BaseValue
	trailing, block := uint(32), uint(0)
	for i := uint64(1); i < c.Count; i++ {
		if r.bit() {
			n := uint(64)
			for _, step := range tsIntSteps {
				if !r.bit() {
					n = step
					break
				}
			}
			raw := r.read(n)
			if n < 64 && raw&(1<<(n-1)) != 0 {
				raw -= 1 << n
			}
			prevDelta += int64(raw)
		}
		prevTS += uint64(prevDelta)
		if r.bit() {
			var xor uint64
			if !r.bit() {
				xor = r.read(block) << trailing
			} else {
				leading := uint(r.read(5))
				block = uint(r.read(6)) + 1
				if leading+block > 64 {
					return nil, ErrTSChunk
				}
				trailing = 64 - leading - block
				xor = r.read(block) << trailing
			}
			prevValue ^= xor
		}
		if r.err {
			return nil, ErrTSChunk
		}
		out = append(out, TimeSeriesSample{Timestamp: int64(prevTS), Value: math.Float64frombits(prevValue)})
	}
	return out, nil
}

// tsSampleBytes is an uncompressed sample's size: a timestamp and a value.
const tsSampleBytes = 16

// tsLayout is how RedisTimeSeries would lay the series' samples out in
// chunks had they been appended in order: the samples each chunk holds,
// and the last compressed chunk's state, to extend it as samples append.
type tsLayout struct {
	valid        bool
	n            int
	chunkSize    int64
	uncompressed bool
	counts       []int64
	sizes        []int64
	tail         *TSGorillaChunk
	// keys are the chunks' keys in the series' chunk dictionary: each
	// chunk's first timestamp, but 0 for a series' first chunk, made
	// before it had a sample, unless the series was loaded; nodes is the
	// number of nodes the dictionary's radix tree has, -1 until counted.
	keys     []uint64
	keyFirst bool
	nodes    int
}

func (v *TimeSeriesValue) layoutLocked() []int64 {
	l := &v.layout
	if l.valid && l.n == len(v.Samples) && l.chunkSize == v.chunkSizeLocked() && l.uncompressed == v.Uncompressed {
		return l.counts
	}
	*l = tsLayout{valid: true, chunkSize: v.chunkSizeLocked(), uncompressed: v.Uncompressed, counts: []int64{0},
		sizes: []int64{v.chunkSizeLocked()}, keys: []uint64{0}, keyFirst: v.loadedChunks, nodes: -1}
	for _, s := range v.Samples {
		l.extend(s)
	}
	l.n = len(v.Samples)
	return l.counts
}

func (l *tsLayout) extend(s TimeSeriesSample) {
	last := len(l.counts) - 1
	if last == 0 && l.counts[0] == 0 && l.keyFirst {
		l.keys[0], l.nodes = uint64(s.Timestamp), -1
	}
	if l.uncompressed {
		if l.counts[last] >= max(l.sizes[last]/tsSampleBytes, 1) {
			l.counts, l.sizes = append(l.counts, 0), append(l.sizes, l.chunkSize)
			l.keys, l.nodes = append(l.keys, uint64(s.Timestamp)), -1
			last++
		}
		l.counts[last]++
		return
	}
	if l.tail == nil {
		l.tail = NewTSGorillaChunk(uint64(l.chunkSize), false)
	}
	if !l.tail.Append(uint64(s.Timestamp), s.Value) {
		l.tail = NewTSGorillaChunk(uint64(l.chunkSize), false)
		l.tail.Append(uint64(s.Timestamp), s.Value)
		l.counts, l.sizes = append(l.counts, 0), append(l.sizes, l.chunkSize)
		l.keys, l.nodes = append(l.keys, uint64(s.Timestamp)), -1
		last++
	}
	l.counts[last]++
}

// appended extends a valid layout by the sample just appended last.
func (v *TimeSeriesValue) appended() {
	l := &v.layout
	if !l.valid || l.n != len(v.Samples)-1 || l.chunkSize != v.chunkSizeLocked() || l.uncompressed != v.Uncompressed {
		l.valid = false
		return
	}
	l.extend(v.Samples[len(v.Samples)-1])
	l.n++
}

// ChunkLayout returns how many samples each of the series' chunks holds
// and each chunk's size in bytes: as the series was loaded, extended by
// the samples appended since as RedisTimeSeries appends them, or else as
// it lays samples appended in order out. The caller holds the lock.
func (v *TimeSeriesValue) ChunkLayout() (counts, sizes []int64) {
	v.layoutLocked()
	return append([]int64(nil), v.layout.counts...), append([]int64(nil), v.layout.sizes...)
}

// SetLoadedChunks records the chunks a loaded series came in: each one's
// sample count and size, and the state of the compressed last chunk, so
// TS.INFO reports them as RedisTimeSeries does and appends extend them.
// The counts must add up to the series' samples.
func (v *TimeSeriesValue) SetLoadedChunks(counts, sizes []int64, tail *TSGorillaChunk) {
	l := tsLayout{valid: true, n: len(v.Samples), chunkSize: v.chunkSizeLocked(), uncompressed: v.Uncompressed,
		counts: counts, sizes: sizes, keyFirst: true, nodes: -1}
	if tail != nil {
		t := *tail
		t.Data = nil
		l.tail = &t
	}
	i := 0
	for _, n := range counts {
		var key uint64
		if n > 0 && i < len(v.Samples) {
			key = uint64(v.Samples[i].Timestamp)
		}
		l.keys = append(l.keys, key)
		i += int(n)
	}
	if i != len(v.Samples) || len(counts) == 0 || len(counts) != len(sizes) {
		return
	}
	v.layout = l
}

// raxNodes counts the nodes of a Redis radix tree holding the 8-byte
// big-endian keys, sorted, from byte depth on: a node per run of bytes the
// keys share, per branch, and per key.
func raxNodes(keys []uint64, depth int) int {
	if depth == 8 {
		return 1
	}
	at := func(k uint64, d int) byte { return byte(k >> (56 - 8*d)) }
	d := depth
	for d < 8 && at(keys[0], d) == at(keys[len(keys)-1], d) {
		d++
	}
	if d > depth {
		return 1 + raxNodes(keys, d)
	}
	n := 1
	for i := 0; i < len(keys); {
		j := i + 1
		for j < len(keys) && at(keys[j], depth) == at(keys[i], depth) {
			j++
		}
		n += raxNodes(keys[i:j], depth+1)
		i = j
	}
	return n
}

// jemallocSize is the size class jemalloc, as Redis builds it, serves a
// request of n bytes from.
func jemallocSize(n int64) int64 {
	if n <= 64 {
		return max((n+7)&^7, 8)
	}
	k := 63 - bits.LeadingZeros64(uint64(n-1))
	step := int64(1) << (k - 2)
	return (n + step - 1) &^ (step - 1)
}

// redisStringSize is RedisModule_MallocSizeString of a string created from
// a command argument: an embedded string up to 44 bytes, else an object
// and its sds.
func redisStringSize(n int) int64 {
	if n <= 44 {
		return jemallocSize(int64(16 + 3 + n + 1))
	}
	hdr := 3
	switch {
	case n >= 1<<32:
		hdr = 17
	case n >= 1<<16:
		hdr = 9
	case n >= 1<<8:
		hdr = 5
	}
	return 16 + jemallocSize(int64(hdr+n+1))
}

// RedisStringEnc is how an RDB saved a string.
type RedisStringEnc int

const (
	RedisStringPlain RedisStringEnc = iota
	RedisStringInt
	RedisStringLZF
)

// RedisLoadedStringSize is RedisModule_MallocSizeString of a string of n
// bytes Redis loaded from an RDB: one saved as an integer or LZF-compressed
// is loaded into an object and an sds of its own, whose header for a
// decompressed string under 32 bytes is 1 byte.
func RedisLoadedStringSize(n int, enc RedisStringEnc) int64 {
	if enc == RedisStringPlain {
		return redisStringSize(n)
	}
	hdr := 3
	switch {
	case n >= 1<<32:
		hdr = 17
	case n >= 1<<16:
		hdr = 9
	case n >= 1<<8:
		hdr = 5
	case n < 32 && enc == RedisStringLZF:
		hdr = 1
	}
	return 16 + jemallocSize(int64(hdr+n+1))
}

// SetLoadedLabelSizes records what the labels' strings take as Redis
// loaded them, until the labels change.
func (v *TimeSeriesValue) SetLoadedLabelSizes(size int64) {
	v.labelMem, v.labelMemSet = size, true
}

// tsContextSizes are the sizes of the aggregations' contexts.
var tsContextSizes = map[string]int64{"sum": 8, "count": 8, "last": 8, "first": 16, "min": 16, "max": 16, "range": 16,
	"avg": 24, "std.p": 24, "std.s": 24, "var.p": 24, "var.s": 24, "twa": 72}

// memoryUsageLocked is TS.INFO's memoryUsage as RedisTimeSeries' SeriesMemUsage
// counts it under jemalloc: the series, its rules and their contexts, its
// labels, its chunks, and its chunk dictionary, which Redis counts as 244
// bytes per radix tree node.
func (v *TimeSeriesValue) memoryUsageLocked() int64 {
	v.layoutLocked()
	l := &v.layout
	if l.nodes < 0 {
		l.nodes = raxNodes(l.keys, 0)
	}
	size := 192 + 244*int64(l.nodes)
	header := int64(80)
	if v.Uncompressed {
		header = 32
	}
	for _, n := range l.sizes {
		size += header + jemallocSize(n)
	}
	if n := len(v.Labels); n > 0 {
		size += jemallocSize(int64(16 * n))
		if v.labelMemSet {
			size += v.labelMem
		} else {
			for k, val := range v.Labels {
				size += redisStringSize(len(k)) + redisStringSize(len(val))
			}
		}
	}
	for i := range v.Rules {
		size += 64 + jemallocSize(tsContextSizes[v.Rules[i].Aggregation])
	}
	return size
}

// MarkLoaded records that the series was loaded from a save: RedisTimeSeries
// then keys its first chunk by its first timestamp.
func (v *TimeSeriesValue) MarkLoaded() {
	v.loadedChunks = true
	v.layout.valid = false
}
