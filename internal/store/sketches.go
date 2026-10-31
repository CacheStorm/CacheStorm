package store

import (
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

func newBloomLayer(capacity uint64, errorRate float64) *BloomLayer {
	bpe := -math.Log(errorRate) / (math.Ln2 * math.Ln2)
	nbits := max(uint64(math.Ceil(float64(capacity)*bpe)), 64)
	nbits = (nbits + 63) / 64 * 64
	return &BloomLayer{
		Bits:     make([]uint64, nbits/64),
		NBits:    nbits,
		Hashes:   uint32(max(math.Ceil(math.Ln2*bpe), 1)),
		Capacity: capacity,
	}
}

// NewBloomValue returns an empty filter for capacity items at errorRate.
func NewBloomValue(capacity uint64, errorRate float64, expansion uint32) *BloomValue {
	return &BloomValue{Filters: []*BloomLayer{newBloomLayer(capacity, errorRate)}, ErrorRate: errorRate, Expansion: expansion}
}

func (l *BloomLayer) has(h1, h2 uint64) bool {
	for i := uint64(0); i < uint64(l.Hashes); i++ {
		bit := (h1 + i*h2) % l.NBits
		if l.Bits[bit/64]&(1<<(bit%64)) == 0 {
			return false
		}
	}
	return true
}

func (l *BloomLayer) set(h1, h2 uint64) {
	for i := uint64(0); i < uint64(l.Hashes); i++ {
		bit := (h1 + i*h2) % l.NBits
		l.Bits[bit/64] |= 1 << (bit % 64)
	}
}

// Add adds item and reports whether it was new; an item the filter may
// already hold is not added again.
func (b *BloomValue) Add(item []byte) (bool, error) {
	h1, h2 := itemHashes(item)
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
		// Each new filter is larger and, as in RedisBloom, has half the
		// error rate of the one before, so the total stays bounded.
		rate := b.ErrorRate * math.Pow(0.5, float64(len(b.Filters)))
		last = newBloomLayer(last.Capacity*uint64(b.Expansion), max(rate, math.SmallestNonzeroFloat64))
		b.Filters = append(b.Filters, last)
	}
	last.set(h1, h2)
	last.Count++
	return true, nil
}

// Exists reports whether the filter may hold item.
func (b *BloomValue) Exists(item []byte) bool {
	h1, h2 := itemHashes(item)
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
	info := BloomInfo{Filters: uint64(len(b.Filters)), Expansion: b.Expansion, Size: 96}
	for _, l := range b.Filters {
		info.Capacity += l.Capacity
		info.Items += l.Count
		info.Size += uint64(len(l.Bits))*8 + 48
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

// CuckooBuckets is the bucket count of a filter for capacity items: a power
// of two, as in RedisBloom.
func CuckooBuckets(capacity uint64, bucketSize uint8) uint64 {
	n := max((capacity+uint64(bucketSize)-1)/uint64(bucketSize), 1)
	return 1 << bits.Len64(n-1)
}

func NewCuckooValue(capacity uint64, bucketSize uint8, maxIterations, expansion uint16) *CuckooValue {
	return &CuckooValue{
		Filters:       []*CuckooLayer{newCuckooLayer(CuckooBuckets(capacity, bucketSize), bucketSize)},
		BucketSize:    bucketSize,
		MaxIterations: maxIterations,
		Expansion:     expansion,
	}
}

func cuckooFingerprint(item []byte) (uint8, uint64) {
	h1, h2 := itemHashes(item)
	return uint8(h2%255 + 1), h1
}

func cuckooAlt(i uint64, fp uint8) uint64 {
	return i ^ (uint64(fp) * 0x5bd1e995)
}

func (l *CuckooLayer) bucket(i uint64, size uint8) []byte {
	i &= l.NumBuckets - 1
	return l.Slots[i*uint64(size) : (i+1)*uint64(size)]
}

func (l *CuckooLayer) count(fp uint8, h uint64, size uint8) uint64 {
	var n uint64
	i1 := h & (l.NumBuckets - 1)
	i2 := cuckooAlt(i1, fp) & (l.NumBuckets - 1)
	for _, s := range l.bucket(i1, size) {
		if s == fp {
			n++
		}
	}
	if i2 != i1 {
		for _, s := range l.bucket(i2, size) {
			if s == fp {
				n++
			}
		}
	}
	return n
}

func (l *CuckooLayer) remove(fp uint8, h uint64, size uint8) bool {
	i1 := h & (l.NumBuckets - 1)
	for _, i := range []uint64{i1, cuckooAlt(i1, fp)} {
		b := l.bucket(i, size)
		for j, s := range b {
			if s == fp {
				b[j] = 0
				return true
			}
		}
	}
	return false
}

func placeFree(b []byte, fp uint8) bool {
	for j, s := range b {
		if s == 0 {
			b[j] = fp
			return true
		}
	}
	return false
}

// insert places fp, kicking fingerprints to their other bucket up to
// maxIterations times; when that fails the kicks are undone.
func (l *CuckooLayer) insert(fp uint8, h uint64, size uint8, maxIterations uint16) bool {
	i1 := h & (l.NumBuckets - 1)
	i2 := cuckooAlt(i1, fp) & (l.NumBuckets - 1)
	if placeFree(l.bucket(i1, size), fp) || placeFree(l.bucket(i2, size), fp) {
		return true
	}
	type kick struct {
		bucket uint64
		slot   int
		old    uint8
	}
	var kicks []kick
	i := i1
	if rand.IntN(2) == 1 {
		i = i2
	}
	for n := uint16(0); n < maxIterations; n++ {
		b := l.bucket(i, size)
		slot := rand.IntN(int(size))
		kicks = append(kicks, kick{i, slot, b[slot]})
		fp, b[slot] = b[slot], fp
		i = cuckooAlt(i, fp) & (l.NumBuckets - 1)
		if placeFree(l.bucket(i, size), fp) {
			return true
		}
	}
	for k := len(kicks) - 1; k >= 0; k-- {
		l.bucket(kicks[k].bucket, size)[kicks[k].slot] = kicks[k].old
	}
	return false
}

// Add adds item, a second copy too when it is already present; nx adds it
// only when absent and reports whether it did.
func (c *CuckooValue) Add(item []byte, nx bool) (bool, error) {
	fp, h := cuckooFingerprint(item)
	c.mu.Lock()
	defer c.mu.Unlock()
	if nx {
		for _, l := range c.Filters {
			if l.count(fp, h, c.BucketSize) > 0 {
				return false, nil
			}
		}
	}
	last := c.Filters[len(c.Filters)-1]
	if !last.insert(fp, h, c.BucketSize, c.MaxIterations) {
		if c.Expansion == 0 {
			return false, ErrCuckooFull
		}
		last = newCuckooLayer(last.NumBuckets*uint64(1<<bits.Len16(c.Expansion-1)), c.BucketSize)
		if !last.insert(fp, h, c.BucketSize, c.MaxIterations) {
			return false, ErrCuckooFull
		}
		c.Filters = append(c.Filters, last)
	}
	c.Inserted++
	return true, nil
}

// Count returns how many copies of item the filter may hold.
func (c *CuckooValue) Count(item []byte) uint64 {
	fp, h := cuckooFingerprint(item)
	c.mu.RLock()
	defer c.mu.RUnlock()
	var n uint64
	for _, l := range c.Filters {
		n += l.count(fp, h, c.BucketSize)
	}
	return n
}

// Delete removes one copy of item and reports whether there was one.
func (c *CuckooValue) Delete(item []byte) bool {
	fp, h := cuckooFingerprint(item)
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.Filters) - 1; i >= 0; i-- {
		if c.Filters[i].remove(fp, h, c.BucketSize) {
			c.Inserted--
			c.Deleted++
			return true
		}
	}
	return false
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
		BucketSize: c.BucketSize, Expansion: c.Expansion, MaxIterations: c.MaxIterations, Size: 56}
	for _, l := range c.Filters {
		info.Buckets += l.NumBuckets
		info.Size += uint64(len(l.Slots))
	}
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

// ErrCMSOverflow is reported for an increment past a counter's maximum,
// which leaves the counter at the maximum.
var ErrCMSOverflow = errors.New("CMS: INCRBY overflow")

func NewCMSValue(width, depth uint32) *CMSValue {
	return &CMSValue{Width: width, Depth: depth, Counters: make([]uint32, uint64(width)*uint64(depth))}
}

func (m *CMSValue) cell(row uint32, h1, h2 uint64) int {
	return int(uint64(row)*uint64(m.Width) + (h1+uint64(row)*h2)%uint64(m.Width))
}

// IncrBy adds incr to item and returns its new estimate.
func (m *CMSValue) IncrBy(item []byte, incr uint64) (uint64, error) {
	h1, h2 := itemHashes(item)
	m.mu.Lock()
	defer m.mu.Unlock()
	var err error
	least := uint64(math.MaxUint32)
	for row := uint32(0); row < m.Depth; row++ {
		c := &m.Counters[m.cell(row, h1, h2)]
		sum := uint64(*c) + incr
		if sum > math.MaxUint32 {
			sum, err = math.MaxUint32, ErrCMSOverflow
		}
		*c = uint32(sum)
		least = min(least, sum)
	}
	m.Count += incr
	return least, err
}

// Query returns item's estimated count.
func (m *CMSValue) Query(item []byte) uint64 {
	h1, h2 := itemHashes(item)
	m.mu.RLock()
	defer m.mu.RUnlock()
	least := uint64(math.MaxUint32)
	for row := uint32(0); row < m.Depth; row++ {
		least = min(least, uint64(m.Counters[m.cell(row, h1, h2)]))
	}
	return least
}

// Merge replaces m's counters with the weighted sum of srcs, which must
// have m's dimensions.
func (m *CMSValue) Merge(srcs []*CMSValue, weights []int64) {
	counters := make([]int64, len(m.Counters))
	var count int64
	for i, s := range srcs {
		s.mu.RLock()
		for j, c := range s.Counters {
			counters[j] += int64(c) * weights[i]
		}
		count += int64(s.Count) * weights[i]
		s.mu.RUnlock()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for j, c := range counters {
		m.Counters[j] = uint32(min(max(c, 0), math.MaxUint32))
	}
	m.Count = uint64(max(count, 0))
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
}

func NewTopKValue(k, width, depth uint32, decay float64) *TopKValue {
	return &TopKValue{K: k, Width: width, Depth: depth, Decay: decay, Buckets: make([]TopKBucket, uint64(width)*uint64(depth))}
}

// Add adds incr to item and returns the item it pushed out of the top list,
// if any.
func (t *TopKValue) Add(item []byte, incr uint32) (string, bool) {
	h1, h2 := itemHashes(item)
	fp := uint32(h2>>32) | 1
	t.mu.Lock()
	defer t.mu.Unlock()
	var count uint32
	for row := uint32(0); row < t.Depth; row++ {
		b := &t.Buckets[uint64(row)*uint64(t.Width)+(h1+uint64(row)*h2)%uint64(t.Width)]
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
	name := string(item)
	for i := range t.Top {
		if t.Top[i].Item == name {
			t.Top[i].Count = max(t.Top[i].Count, count)
			return "", false
		}
	}
	if count == 0 {
		return "", false
	}
	if uint32(len(t.Top)) < t.K {
		t.Top = append(t.Top, TopKItem{name, count})
		return "", false
	}
	least := 0
	for i := range t.Top {
		if t.Top[i].Count < t.Top[least].Count {
			least = i
		}
	}
	if count < t.Top[least].Count {
		return "", false
	}
	dropped := t.Top[least].Item
	t.Top[least] = TopKItem{name, count}
	return dropped, true
}

// Query reports whether item is in the top list.
func (t *TopKValue) Query(item []byte) bool {
	name := string(item)
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, it := range t.Top {
		if it.Item == name {
			return true
		}
	}
	return false
}

// Count returns item's estimated count.
func (t *TopKValue) Count(item []byte) uint32 {
	h1, h2 := itemHashes(item)
	fp := uint32(h2>>32) | 1
	t.mu.RLock()
	defer t.mu.RUnlock()
	var count uint32
	for row := uint32(0); row < t.Depth; row++ {
		if b := t.Buckets[uint64(row)*uint64(t.Width)+(h1+uint64(row)*h2)%uint64(t.Width)]; b.Fp == fp {
			count = max(count, b.Count)
		}
	}
	return count
}

// List returns the top items, heaviest first.
func (t *TopKValue) List() []TopKItem {
	t.mu.RLock()
	list := append([]TopKItem(nil), t.Top...)
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
	n := r.count(12)
	if n > uint64(t.K) {
		return nil, ErrExtensionFormat
	}
	for i := uint64(0); i < n && r.err == nil; i++ {
		t.Top = append(t.Top, TopKItem{Item: string(r.bytes()), Count: r.u32()})
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
