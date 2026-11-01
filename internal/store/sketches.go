package store

import (
	"encoding/binary"
	"errors"
	"math"
	"math/bits"
	"math/rand/v2"
	"sort"
	"sync"
)

// BloomValue is a scalable Bloom filter, RedisBloom's MBbloom-- type: when
// the newest filter holds its capacity, a larger one with a tighter error
// rate is added, unless the filter is non-scaling (Expansion 0).
type BloomValue struct {
	mu        sync.RWMutex
	Filters   []*BloomLayer
	ErrorRate float64
	Expansion uint32
}

// BloomLayer is one fixed-size filter of a BloomValue.
type BloomLayer struct {
	Bits     []uint64
	NBits    uint64
	Hashes   uint32
	Capacity uint64
	Count    uint64
}

// ErrBloomFull is returned when a non-scaling filter holds its capacity.
var ErrBloomFull = errors.New("ERR non scaling filter is full")

// newBloomLayer sizes a filter as RedisBloom's bloom_init does with
// BLOOM_OPT_NOROUND: entries*bpe bits rounded up to whole 64-bit words.
func newBloomLayer(capacity uint64, errorRate float64) *BloomLayer {
	bpe := math.Abs(math.Log(errorRate) / 0.480453013918201)
	nbits := max(uint64(float64(capacity)*bpe), 1)
	if nbits%64 != 0 {
		nbits = (nbits/64 + 1) * 64
	}
	return &BloomLayer{
		Bits:     make([]uint64, nbits/64),
		NBits:    nbits,
		Hashes:   uint32(math.Ceil(0.693147180559945 * bpe)),
		Capacity: capacity,
	}
}

// bloomTightening is RedisBloom's ERROR_TIGHTENING_RATIO: a scaling
// filter's first layer has half the requested error rate and each later
// one half of the one before.
const bloomTightening = 0.5

// layerErrorRate is the error rate of layer i of a filter created with
// errorRate.
func layerErrorRate(errorRate float64, i int, scaling bool) float64 {
	if scaling {
		i++
	}
	return errorRate * math.Pow(bloomTightening, float64(i))
}

// bloomErrorRateCap is RedisBloom's BF_ERROR_RATE_CAP: a higher requested
// error rate is lowered to it.
const bloomErrorRateCap = 0.25

// NewBloomValue returns an empty filter for capacity items at errorRate.
func NewBloomValue(capacity uint64, errorRate float64, expansion uint32) *BloomValue {
	errorRate = min(errorRate, bloomErrorRateCap)
	first := newBloomLayer(capacity, layerErrorRate(errorRate, 0, expansion > 0))
	return &BloomValue{Filters: []*BloomLayer{first}, ErrorRate: errorRate, Expansion: expansion}
}

// murmurHash64A is MurmurHash64A as RedisBloom's 64-bit filters use it.
func murmurHash64A(data []byte, seed uint64) uint64 {
	const m = 0xc6a4a7935bd1e995
	h := seed ^ uint64(len(data))*m
	for ; len(data) >= 8; data = data[8:] {
		k := binary.LittleEndian.Uint64(data)
		k *= m
		k ^= k >> 47
		k *= m
		h ^= k
		h *= m
	}
	if len(data) > 0 {
		for i := len(data) - 1; i >= 0; i-- {
			h ^= uint64(data[i]) << (8 * i)
		}
		h *= m
	}
	h ^= h >> 47
	h *= m
	h ^= h >> 47
	return h
}

func bloomHashes(item []byte) (uint64, uint64) {
	a := murmurHash64A(item, 0xc6a4a7935bd1e995)
	return a, murmurHash64A(item, a)
}

func (l *BloomLayer) has(a, b uint64) bool {
	for i := uint64(0); i < uint64(l.Hashes); i++ {
		bit := (a + i*b) % l.NBits
		if l.Bits[bit/64]&(1<<(bit%64)) == 0 {
			return false
		}
	}
	return true
}

func (l *BloomLayer) set(a, b uint64) {
	for i := uint64(0); i < uint64(l.Hashes); i++ {
		bit := (a + i*b) % l.NBits
		l.Bits[bit/64] |= 1 << (bit % 64)
	}
}

// Add adds item and reports whether it was new; an item the filter may
// already hold is not added again.
func (b *BloomValue) Add(item []byte) (bool, error) {
	h1, h2 := bloomHashes(item)
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, l := range b.Filters {
		if l.has(h1, h2) {
			return false, nil
		}
	}
	last := b.Filters[len(b.Filters)-1]
	if last.Count >= last.Capacity {
		if b.Expansion == 0 {
			return false, ErrBloomFull
		}
		rate := layerErrorRate(b.ErrorRate, len(b.Filters), true)
		last = newBloomLayer(last.Capacity*uint64(b.Expansion), max(rate, math.SmallestNonzeroFloat64))
		b.Filters = append(b.Filters, last)
	}
	last.set(h1, h2)
	last.Count++
	return true, nil
}

// Exists reports whether the filter may hold item.
func (b *BloomValue) Exists(item []byte) bool {
	h1, h2 := bloomHashes(item)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, l := range b.Filters {
		if l.has(h1, h2) {
			return true
		}
	}
	return false
}

// BloomInfo is what BF.INFO reports.
type BloomInfo struct {
	Capacity, Size, Filters, Items uint64
	Expansion                      uint32
}

func (b *BloomValue) Info() BloomInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	info := BloomInfo{Filters: uint64(len(b.Filters)), Expansion: b.Expansion, Size: 32}
	for _, l := range b.Filters {
		info.Capacity += l.Capacity
		info.Items += l.Count
		info.Size += uint64(len(l.Bits))*8 + 64
	}
	return info
}

func (b *BloomValue) Type() DataType { return DataTypeBloom }
func (b *BloomValue) String() string { return "bloom" }
func (b *BloomValue) SizeOf() int64  { return int64(b.Info().Size) }
func (b *BloomValue) ExtensionType() byte {
	return ExtensionBloom
}

func (b *BloomValue) Clone() Value {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c := &BloomValue{ErrorRate: b.ErrorRate, Expansion: b.Expansion}
	for _, l := range b.Filters {
		cl := *l
		cl.Bits = append([]uint64(nil), l.Bits...)
		c.Filters = append(c.Filters, &cl)
	}
	return c
}

func (b *BloomValue) MarshalBinary() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	w := &binWriter{}
	w.f64(b.ErrorRate)
	w.u32(b.Expansion)
	w.u64(uint64(len(b.Filters)))
	for _, l := range b.Filters {
		w.u64(l.NBits)
		w.u32(l.Hashes)
		w.u64(l.Capacity)
		w.u64(l.Count)
		for _, word := range l.Bits {
			w.u64(word)
		}
	}
	return w.b, nil
}

func decodeBloom(data []byte) (Value, error) {
	r := &binReader{b: data}
	b := &BloomValue{ErrorRate: r.f64(), Expansion: r.u32()}
	n := r.count(28)
	for i := uint64(0); i < n && r.err == nil; i++ {
		l := &BloomLayer{NBits: r.u64(), Hashes: r.u32(), Capacity: r.u64(), Count: r.u64()}
		if l.NBits == 0 || l.NBits%64 != 0 || l.NBits/8 > uint64(len(r.b)) || l.Hashes == 0 || l.Capacity == 0 {
			return nil, ErrExtensionFormat
		}
		l.Bits = make([]uint64, l.NBits/64)
		for j := range l.Bits {
			l.Bits[j] = r.u64()
		}
		b.Filters = append(b.Filters, l)
	}
	if err := r.done(); err != nil || len(b.Filters) == 0 || !(b.ErrorRate > 0 && b.ErrorRate < 1) {
		return nil, ErrExtensionFormat
	}
	return b, nil
}

// CuckooValue is a cuckoo filter, RedisBloom's MBbloomCF type: one-byte
// fingerprints in buckets of BucketSize slots, with filters added as the
// newest fills when Expansion is above zero.
type CuckooValue struct {
	mu            sync.RWMutex
	Filters       []*CuckooLayer
	BucketSize    uint8
	MaxIterations uint16
	Expansion     uint16
	Inserted      uint64
	Deleted       uint64
}

// CuckooLayer is one filter of a CuckooValue: NumBuckets (a power of two)
// buckets of fingerprints, 0 marking a free slot.
type CuckooLayer struct {
	NumBuckets uint64
	Slots      []byte
}

// ErrCuckooFull is returned when an item finds no place in a filter that
// cannot expand.
var ErrCuckooFull = errors.New("Filter is full") //nolint:staticcheck // RedisBloom's reply text

func newCuckooLayer(numBuckets uint64, bucketSize uint8) *CuckooLayer {
	return &CuckooLayer{NumBuckets: numBuckets, Slots: make([]byte, numBuckets*uint64(bucketSize))}
}

// nextPow2 is RedisBloom's getNextN2; 0 stays 0.
func nextPow2(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	return 1 << bits.Len64(n-1)
}

// CuckooBuckets is the bucket count of a filter for capacity items, as
// RedisBloom's CuckooFilter_Init computes it.
func CuckooBuckets(capacity uint64, bucketSize uint8) uint64 {
	return max(nextPow2(capacity/uint64(bucketSize)), 1)
}

// NewCuckooValue returns an empty filter; expansion is rounded up to a power
// of two as RedisBloom does.
func NewCuckooValue(capacity uint64, bucketSize uint8, maxIterations, expansion uint16) *CuckooValue {
	return &CuckooValue{
		Filters:       []*CuckooLayer{newCuckooLayer(CuckooBuckets(capacity, bucketSize), bucketSize)},
		BucketSize:    bucketSize,
		MaxIterations: maxIterations,
		Expansion:     uint16(nextPow2(uint64(expansion))),
	}
}

// cuckooLookup is RedisBloom's LookupParams for item: its fingerprint and
// its two bucket hashes.
type cuckooLookup struct {
	fp     uint8
	h1, h2 uint64
}

func cuckooAlt(fp uint8, i uint64) uint64 {
	return i ^ uint64(fp)*0x5bd1e995
}

func newCuckooLookup(item []byte) cuckooLookup {
	h := murmurHash64A(item, 0)
	fp := uint8(h%255 + 1)
	return cuckooLookup{fp: fp, h1: h, h2: cuckooAlt(fp, h)}
}

func (l *CuckooLayer) bucket(h uint64, size uint8) []byte {
	i := h % l.NumBuckets
	return l.Slots[i*uint64(size) : (i+1)*uint64(size)]
}

func bucketCount(b []byte, fp uint8) uint64 {
	var n uint64
	for _, s := range b {
		if s == fp {
			n++
		}
	}
	return n
}

func bucketFree(b []byte) int {
	for j, s := range b {
		if s == 0 {
			return j
		}
	}
	return -1
}

func bucketRemove(b []byte, fp uint8) bool {
	for j, s := range b {
		if s == fp {
			b[j] = 0
			return true
		}
	}
	return false
}

func (l *CuckooLayer) find(k cuckooLookup, size uint8) bool {
	return bucketCount(l.bucket(k.h1, size), k.fp) > 0 || bucketCount(l.bucket(k.h2, size), k.fp) > 0
}

// place puts the fingerprint in a free slot of either bucket.
func (l *CuckooLayer) place(k cuckooLookup, size uint8) bool {
	for _, h := range []uint64{k.h1, k.h2} {
		if b := l.bucket(h, size); bucketFree(b) >= 0 {
			b[bucketFree(b)] = k.fp
			return true
		}
	}
	return false
}

// kickInsert is RedisBloom's Filter_KOInsert: it evicts fingerprints along
// one path and, when that does not free a slot, walks the path back.
func (l *CuckooLayer) kickInsert(k cuckooLookup, size uint8, maxIterations uint16) bool {
	fp, victim, i := k.fp, 0, k.h1%l.NumBuckets
	for n := uint16(0); n < maxIterations; n++ {
		b := l.bucket(i, size)
		fp, b[victim] = b[victim], fp
		i = cuckooAlt(fp, i) % l.NumBuckets
		if nb := l.bucket(i, size); bucketFree(nb) >= 0 {
			nb[bucketFree(nb)] = fp
			return true
		}
		victim = (victim + 1) % int(size)
	}
	for n := uint16(0); n < maxIterations; n++ {
		victim = (victim + int(size) - 1) % int(size)
		i = cuckooAlt(fp, i) % l.NumBuckets
		b := l.bucket(i, size)
		fp, b[victim] = b[victim], fp
	}
	return false
}

func (c *CuckooValue) grow() {
	growth := uint64(math.Pow(float64(c.Expansion), float64(len(c.Filters))))
	c.Filters = append(c.Filters, newCuckooLayer(c.Filters[0].NumBuckets*growth, c.BucketSize))
}

func (c *CuckooValue) insert(k cuckooLookup) bool {
	for {
		for i := len(c.Filters) - 1; i >= 0; i-- {
			if c.Filters[i].place(k, c.BucketSize) {
				c.Inserted++
				return true
			}
		}
		if c.Filters[len(c.Filters)-1].kickInsert(k, c.BucketSize, c.MaxIterations) {
			c.Inserted++
			return true
		}
		if c.Expansion == 0 || len(c.Filters) >= math.MaxUint16 {
			return false
		}
		c.grow()
	}
}

// Add adds item, a second copy too when it is already present; nx adds it
// only when absent and reports whether it did. A full filter that cannot
// expand returns ErrCuckooFull.
func (c *CuckooValue) Add(item []byte, nx bool) (bool, error) {
	k := newCuckooLookup(item)
	c.mu.Lock()
	defer c.mu.Unlock()
	if nx {
		for _, l := range c.Filters {
			if l.find(k, c.BucketSize) {
				return false, nil
			}
		}
	}
	if !c.insert(k) {
		return false, ErrCuckooFull
	}
	return true, nil
}

// NumFilters returns how many filters the cuckoo filter has.
func (c *CuckooValue) NumFilters() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Filters)
}

// Count returns how many copies of item the filter may hold, counting both
// buckets even when they are the same one, as RedisBloom does.
func (c *CuckooValue) Count(item []byte) uint64 {
	k := newCuckooLookup(item)
	c.mu.RLock()
	defer c.mu.RUnlock()
	var n uint64
	for _, l := range c.Filters {
		n += bucketCount(l.bucket(k.h1, c.BucketSize), k.fp) + bucketCount(l.bucket(k.h2, c.BucketSize), k.fp)
	}
	return n
}

// Exists reports whether the filter may hold item.
func (c *CuckooValue) Exists(item []byte) bool {
	k := newCuckooLookup(item)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, l := range c.Filters {
		if l.find(k, c.BucketSize) {
			return true
		}
	}
	return false
}

// Delete removes one copy of item and reports whether there was one; once
// deletions pass a tenth of the items, later filters are compacted into
// earlier ones.
func (c *CuckooValue) Delete(item []byte) bool {
	k := newCuckooLookup(item)
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.Filters) - 1; i >= 0; i-- {
		l := c.Filters[i]
		if bucketRemove(l.bucket(k.h1, c.BucketSize), k.fp) || bucketRemove(l.bucket(k.h2, c.BucketSize), k.fp) {
			c.Inserted--
			c.Deleted++
			if len(c.Filters) > 1 && float64(c.Deleted) > float64(c.Inserted)*0.10 {
				c.compact()
			}
			return true
		}
	}
	return false
}

// compact is RedisBloom's CuckooFilter_Compact without cont: it moves
// fingerprints of the newest filters into older ones, dropping a newest
// filter it empties, until one cannot be emptied.
func (c *CuckooValue) compact() {
	for fi := len(c.Filters) - 1; fi > 0; fi-- {
		l := c.Filters[fi]
		ok := true
		for bi := uint64(0); bi < l.NumBuckets; bi++ {
			b := l.Slots[bi*uint64(c.BucketSize) : (bi+1)*uint64(c.BucketSize)]
			for si, fp := range b {
				if fp == 0 {
					continue
				}
				k := cuckooLookup{fp: fp, h1: bi, h2: cuckooAlt(fp, bi)}
				moved := false
				for _, older := range c.Filters[:fi] {
					if older.place(k, c.BucketSize) {
						b[si] = 0
						moved = true
						break
					}
				}
				if !moved {
					ok = false
				}
			}
		}
		if !ok {
			break
		}
		if fi == len(c.Filters)-1 {
			c.Filters = c.Filters[:fi]
		}
	}
	c.Deleted = 0
}

// CuckooInfo is what CF.INFO reports.
type CuckooInfo struct {
	Size, Buckets, Filters, Inserted, Deleted uint64
	BucketSize                                uint8
	Expansion, MaxIterations                  uint16
}

func (c *CuckooValue) Info() CuckooInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info := CuckooInfo{Filters: uint64(len(c.Filters)), Inserted: c.Inserted, Deleted: c.Deleted,
		BucketSize: c.BucketSize, Expansion: c.Expansion, MaxIterations: c.MaxIterations, Size: 40}
	for _, l := range c.Filters {
		info.Size += 16 + uint64(len(l.Slots))
	}
	info.Buckets = c.Filters[0].NumBuckets
	return info
}

func (c *CuckooValue) Type() DataType { return DataTypeCuckoo }
func (c *CuckooValue) String() string { return "cuckoo" }
func (c *CuckooValue) SizeOf() int64  { return int64(c.Info().Size) }
func (c *CuckooValue) ExtensionType() byte {
	return ExtensionCuckoo
}

func (c *CuckooValue) Clone() Value {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := &CuckooValue{BucketSize: c.BucketSize, MaxIterations: c.MaxIterations, Expansion: c.Expansion, Inserted: c.Inserted, Deleted: c.Deleted}
	for _, l := range c.Filters {
		n.Filters = append(n.Filters, &CuckooLayer{NumBuckets: l.NumBuckets, Slots: append([]byte(nil), l.Slots...)})
	}
	return n
}

func (c *CuckooValue) MarshalBinary() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w := &binWriter{}
	w.u8(c.BucketSize)
	w.u32(uint32(c.MaxIterations))
	w.u32(uint32(c.Expansion))
	w.u64(c.Inserted)
	w.u64(c.Deleted)
	w.u64(uint64(len(c.Filters)))
	for _, l := range c.Filters {
		w.u64(l.NumBuckets)
		w.bytes(l.Slots)
	}
	return w.b, nil
}

func decodeCuckoo(data []byte) (Value, error) {
	r := &binReader{b: data}
	c := &CuckooValue{BucketSize: r.u8()}
	iter, exp := r.u32(), r.u32()
	c.Inserted, c.Deleted = r.u64(), r.u64()
	if iter == 0 || iter > math.MaxUint16 || exp > math.MaxUint16 || c.BucketSize == 0 {
		return nil, ErrExtensionFormat
	}
	c.MaxIterations, c.Expansion = uint16(iter), uint16(exp)
	n := r.count(16)
	for i := uint64(0); i < n && r.err == nil; i++ {
		l := &CuckooLayer{NumBuckets: r.u64(), Slots: r.bytes()}
		if l.NumBuckets == 0 || l.NumBuckets&(l.NumBuckets-1) != 0 || uint64(len(l.Slots)) != l.NumBuckets*uint64(c.BucketSize) {
			return nil, ErrExtensionFormat
		}
		c.Filters = append(c.Filters, l)
	}
	if err := r.done(); err != nil || len(c.Filters) == 0 {
		return nil, ErrExtensionFormat
	}
	return c, nil
}

// CMSValue is a count-min sketch, RedisBloom's CMSk-TYPE: Depth rows of
// Width 32-bit counters.
type CMSValue struct {
	mu       sync.RWMutex
	Width    uint32
	Depth    uint32
	Counters []uint32
	Count    uint64
}

// ErrCMSOverflow is reported when an increment leaves an item's estimate at
// the counters' maximum.
var ErrCMSOverflow = errors.New("CMS: INCRBY overflow")

func NewCMSValue(width, depth uint32) *CMSValue {
	return &CMSValue{Width: width, Depth: depth, Counters: make([]uint32, uint64(width)*uint64(depth))}
}

func (m *CMSValue) cell(row uint32, item []byte) int {
	return int(uint64(row)*uint64(m.Width) + uint64(murmurHash2(item, row)%m.Width))
}

// IncrBy adds incr to item, saturating counters, and returns its new
// estimate; an estimate at the counters' maximum is ErrCMSOverflow, as in
// RedisBloom.
func (m *CMSValue) IncrBy(item []byte, incr uint64) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	least := uint64(math.MaxUint64)
	for row := uint32(0); row < m.Depth; row++ {
		c := &m.Counters[m.cell(row, item)]
		*c = uint32(min(uint64(*c)+incr, math.MaxUint32))
		least = min(least, uint64(*c))
	}
	m.Count += incr
	if least == math.MaxUint32 {
		return least, ErrCMSOverflow
	}
	return least, nil
}

// Query returns item's estimated count.
func (m *CMSValue) Query(item []byte) uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	least := uint64(math.MaxUint64)
	for row := uint32(0); row < m.Depth; row++ {
		least = min(least, uint64(m.Counters[m.cell(row, item)]))
	}
	return least
}

// ErrCMSMergeOverflow is a merge whose weighted sums leave a counter's range.
var ErrCMSMergeOverflow = errors.New("CMS: MERGE overflow")

// Merge replaces m's counters with the weighted sum of srcs, which must
// have m's dimensions; a sum below zero or past a counter's maximum fails
// the whole merge, as in RedisBloom.
func (m *CMSValue) Merge(srcs []*CMSValue, weights []int64) error {
	counters := make([]int64, len(m.Counters))
	var count int64
	overflow := false
	for i, s := range srcs {
		s.mu.RLock()
		for j, c := range s.Counters {
			mul, ok := mulInt64(int64(c), weights[i])
			if !ok {
				overflow = true
			} else if counters[j], ok = addInt64(counters[j], mul); !ok {
				overflow = true
			}
		}
		mul, ok := mulInt64(int64(s.Count), weights[i])
		if !ok {
			overflow = true
		} else if count, ok = addInt64(count, mul); !ok {
			overflow = true
		}
		s.mu.RUnlock()
	}
	for _, c := range counters {
		if c < 0 || c > math.MaxUint32 {
			overflow = true
		}
	}
	if overflow || count < 0 {
		return ErrCMSMergeOverflow
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for j, c := range counters {
		m.Counters[j] = uint32(c)
	}
	m.Count = uint64(count)
	return nil
}

func mulInt64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		return 0, false
	}
	c := a * b
	return c, c/b == a
}

func addInt64(a, b int64) (int64, bool) {
	c := a + b
	return c, (c > a) == (b > 0)
}

// Total returns the sum of every increment, CMS.INFO's count.
func (m *CMSValue) Total() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Count
}

func (m *CMSValue) Type() DataType { return DataTypeCMS }
func (m *CMSValue) String() string { return "cms" }
func (m *CMSValue) SizeOf() int64  { return int64(len(m.Counters))*4 + 48 }
func (m *CMSValue) ExtensionType() byte {
	return ExtensionCMS
}

func (m *CMSValue) Clone() Value {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &CMSValue{Width: m.Width, Depth: m.Depth, Counters: append([]uint32(nil), m.Counters...), Count: m.Count}
}

func (m *CMSValue) MarshalBinary() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w := &binWriter{b: make([]byte, 0, 24+4*len(m.Counters))}
	w.u32(m.Width)
	w.u32(m.Depth)
	w.u64(m.Count)
	for _, c := range m.Counters {
		w.u32(c)
	}
	return w.b, nil
}

func decodeCMS(data []byte) (Value, error) {
	r := &binReader{b: data}
	m := &CMSValue{Width: r.u32(), Depth: r.u32(), Count: r.u64()}
	cells := uint64(m.Width) * uint64(m.Depth)
	if r.err != nil || cells == 0 || cells*4 != uint64(len(r.b)) {
		return nil, ErrExtensionFormat
	}
	m.Counters = make([]uint32, cells)
	for i := range m.Counters {
		m.Counters[i] = r.u32()
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	return m, nil
}

// TopKValue tracks the K heaviest items with HeavyKeeper, RedisBloom's
// TopK-TYPE: Depth rows of Width fingerprinted counters that decay, and a
// list of the current top items.
type TopKValue struct {
	mu      sync.RWMutex
	K       uint32
	Width   uint32
	Depth   uint32
	Decay   float64
	Buckets []TopKBucket
	Top     []TopKItem
}

type TopKBucket struct {
	Fp    uint32
	Count uint32
}

type TopKItem struct {
	Item  string
	Count uint32
	Used  bool
}

func NewTopKValue(k, width, depth uint32, decay float64) *TopKValue {
	return &TopKValue{K: k, Width: width, Depth: depth, Decay: decay, Buckets: make([]TopKBucket, uint64(width)*uint64(depth))}
}

// murmurHash2 is Austin Appleby's 32-bit MurmurHash2, which RedisBloom's
// TopK hashes items with.
func murmurHash2(data []byte, seed uint32) uint32 {
	const m = 0x5bd1e995
	h := seed ^ uint32(len(data))
	for ; len(data) >= 4; data = data[4:] {
		k := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
		k *= m
		k ^= k >> 24
		k *= m
		h = h*m ^ k
	}
	switch len(data) {
	case 3:
		h ^= uint32(data[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(data[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(data[0])
		h *= m
	}
	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}

const topkFingerprintSeed = 1919

// heap returns the top list as RedisBloom keeps it: a min-heap of K slots
// by count; an unused slot has count 0, and so can a used one.
func (t *TopKValue) heap() []TopKItem {
	if uint32(len(t.Top)) < t.K {
		t.Top = append(t.Top, make([]TopKItem, int(t.K)-len(t.Top))...)
		for i := len(t.Top)/2 - 1; i >= 0; i-- {
			topkSiftDown(t.Top, i)
		}
	}
	return t.Top
}

// topkSiftDown is RedisBloom's heapifyDown: ties move the start down.
func topkSiftDown(h []TopKItem, start int) {
	n := len(h)
	if n < 2 || (n-2)/2 < start {
		return
	}
	child := 2*start + 1
	if child+1 < n && h[child].Count > h[child+1].Count {
		child++
	}
	if h[child].Count > h[start].Count {
		return
	}
	top := h[start]
	for {
		h[start] = h[child]
		start = child
		if (n-2)/2 < child {
			break
		}
		child = 2*child + 1
		if child+1 < n && h[child].Count > h[child+1].Count {
			child++
		}
		if h[child].Count >= top.Count {
			break
		}
	}
	h[start] = top
}

func (t *TopKValue) heapIndex(name string) int {
	for i := len(t.Top) - 1; i >= 0; i-- {
		if t.Top[i].Used && t.Top[i].Item == name {
			return i
		}
	}
	return -1
}

// Add adds incr to item and returns the item it pushed out of the top list,
// if any.
func (t *TopKValue) Add(item []byte, incr uint32) (string, bool) {
	fp := murmurHash2(item, topkFingerprintSeed)
	t.mu.Lock()
	defer t.mu.Unlock()
	h := t.heap()
	heapMin := h[0].Count
	var count uint32
	for row := uint32(0); row < t.Depth; row++ {
		b := &t.Buckets[uint64(row)*uint64(t.Width)+uint64(murmurHash2(item, row)%t.Width)]
		switch {
		case b.Count == 0:
			b.Fp, b.Count = fp, incr
		case b.Fp == fp:
			b.Count = uint32(min(uint64(b.Count)+uint64(incr), math.MaxUint32))
		default:
			// Each unit of the increment decays the foreign counter with
			// probability decay^count; an emptied bucket is taken over.
			for left := incr; left > 0; left-- {
				if rand.Float64() < math.Pow(t.Decay, float64(b.Count)) {
					b.Count--
					if b.Count == 0 {
						b.Fp, b.Count = fp, left
						break
					}
				}
			}
		}
		if b.Fp == fp {
			count = max(count, b.Count)
		}
	}
	if count < heapMin {
		return "", false
	}
	name := string(item)
	if i := t.heapIndex(name); i >= 0 {
		h[i].Count = count
		topkSiftDown(h, i)
		return "", false
	}
	dropped, had := h[0].Item, h[0].Used
	h[0] = TopKItem{name, count, true}
	topkSiftDown(h, 0)
	return dropped, had
}

// Query reports whether item is in the top list.
func (t *TopKValue) Query(item []byte) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.heapIndex(string(item)) >= 0
}

// Count returns item's estimated count as RedisBloom does: for an item in
// the top list only counters at or above the list's minimum count.
func (t *TopKValue) Count(item []byte) uint32 {
	fp := murmurHash2(item, topkFingerprintSeed)
	t.mu.RLock()
	defer t.mu.RUnlock()
	var heapMin uint32
	inHeap := t.heapIndex(string(item)) >= 0
	if len(t.Top) > 0 && uint32(len(t.Top)) == t.K {
		heapMin = t.Top[0].Count
	}
	var count uint32
	for row := uint32(0); row < t.Depth; row++ {
		b := t.Buckets[uint64(row)*uint64(t.Width)+uint64(murmurHash2(item, row)%t.Width)]
		if b.Fp == fp && (!inHeap || b.Count >= heapMin) {
			count = max(count, b.Count)
		}
	}
	return count
}

// List returns the top items, heaviest first and ties in heap order, as
// RedisBloom sorts its heap.
func (t *TopKValue) List() []TopKItem {
	t.mu.RLock()
	list := make([]TopKItem, 0, len(t.Top))
	for _, it := range t.Top {
		if it.Count > 0 {
			list = append(list, it)
		}
	}
	t.mu.RUnlock()
	sort.SliceStable(list, func(i, j int) bool { return list[i].Count > list[j].Count })
	return list
}

func (t *TopKValue) Type() DataType { return DataTypeTopK }
func (t *TopKValue) String() string { return "topk" }
func (t *TopKValue) ExtensionType() byte {
	return ExtensionTopK
}

func (t *TopKValue) SizeOf() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := int64(len(t.Buckets))*8 + 64
	for _, it := range t.Top {
		n += int64(len(it.Item)) + 24
	}
	return n
}

func (t *TopKValue) Clone() Value {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return &TopKValue{K: t.K, Width: t.Width, Depth: t.Depth, Decay: t.Decay,
		Buckets: append([]TopKBucket(nil), t.Buckets...), Top: append([]TopKItem(nil), t.Top...)}
}

func (t *TopKValue) MarshalBinary() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	w := &binWriter{b: make([]byte, 0, 32+8*len(t.Buckets))}
	w.u32(t.K)
	w.u32(t.Width)
	w.u32(t.Depth)
	w.f64(t.Decay)
	for _, b := range t.Buckets {
		w.u32(b.Fp)
		w.u32(b.Count)
	}
	w.u64(uint64(len(t.Top)))
	for _, it := range t.Top {
		w.bytes([]byte(it.Item))
		w.u32(it.Count)
		w.u8(boolByte(it.Used))
	}
	return w.b, nil
}

func decodeTopK(data []byte) (Value, error) {
	r := &binReader{b: data}
	t := &TopKValue{K: r.u32(), Width: r.u32(), Depth: r.u32(), Decay: r.f64()}
	cells := uint64(t.Width) * uint64(t.Depth)
	if r.err != nil || t.K == 0 || cells == 0 || cells*8 > uint64(len(r.b)) || !(t.Decay > 0 && t.Decay <= 1) {
		return nil, ErrExtensionFormat
	}
	t.Buckets = make([]TopKBucket, cells)
	for i := range t.Buckets {
		t.Buckets[i] = TopKBucket{Fp: r.u32(), Count: r.u32()}
	}
	n := r.count(13)
	if n > uint64(t.K) {
		return nil, ErrExtensionFormat
	}
	for i := uint64(0); i < n && r.err == nil; i++ {
		t.Top = append(t.Top, TopKItem{Item: string(r.bytes()), Count: r.u32(), Used: r.u8() != 0})
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	return t, nil
}

func init() {
	RegisterExtension(ExtensionBloom, decodeBloom)
	RegisterExtension(ExtensionCuckoo, decodeCuckoo)
	RegisterExtension(ExtensionCMS, decodeCMS)
	RegisterExtension(ExtensionTopK, decodeTopK)
}
