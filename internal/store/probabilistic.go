package store

import (
	"hash/fnv"
	"sync"
)

type BloomFilter struct {
	bits     []bool
	size     uint
	hashFunc uint
	k        uint
	count    uint
	mu       sync.RWMutex
}

func NewBloomFilter(size uint, falsePositiveRate float64) *BloomFilter {
	k := uint(float64(size) * 0.69 / 100)
	if k < 1 {
		k = 1
	}
	if k > 20 {
		k = 20
	}

	return &BloomFilter{
		bits:     make([]bool, size),
		size:     size,
		hashFunc: 0x811c9dc5,
		k:        k,
		count:    0,
	}
}

func (bf *BloomFilter) Add(item []byte) bool {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	hashes := bf.getHashes(item)
	for _, h := range hashes {
		bf.bits[h%bf.size] = true
	}
	bf.count++
	return true
}

func (bf *BloomFilter) Exists(item []byte) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	hashes := bf.getHashes(item)
	for _, h := range hashes {
		if !bf.bits[h%bf.size] {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) getHashes(item []byte) []uint {
	hashes := make([]uint, bf.k)

	h1 := fnv.New32a()
	h1.Write(item)
	hash1 := h1.Sum32()

	h2 := fnv.New32()
	h2.Write(item)
	hash2 := h2.Sum32()

	for i := uint(0); i < bf.k; i++ {
		hashes[i] = uint(hash1 + uint32(i)*hash2)
	}

	return hashes
}

func (bf *BloomFilter) Count() uint {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.count
}

func (bf *BloomFilter) Info() map[string]interface{} {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	bitsSet := 0
	for _, b := range bf.bits {
		if b {
			bitsSet++
		}
	}

	return map[string]interface{}{
		"size":      bf.size,
		"hashes":    bf.k,
		"count":     bf.count,
		"bits_set":  bitsSet,
		"fill_rate": float64(bitsSet) / float64(bf.size),
	}
}

func (bf *BloomFilter) Clear() {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	bf.bits = make([]bool, bf.size)
	bf.count = 0
}

type CountMinSketch struct {
	matrix [][]uint
	depth  uint
	width  uint
	count  uint64
	mu     sync.RWMutex
}

func NewCountMinSketch(depth, width uint) *CountMinSketch {
	matrix := make([][]uint, depth)
	for i := range matrix {
		matrix[i] = make([]uint, width)
	}

	return &CountMinSketch{
		matrix: matrix,
		depth:  depth,
		width:  width,
		count:  0,
	}
}

func (cms *CountMinSketch) Add(item []byte, count uint) uint64 {
	cms.mu.Lock()
	defer cms.mu.Unlock()

	minCount := ^uint64(0)

	for i := uint(0); i < cms.depth; i++ {
		h := cms.hash(item, i)
		cms.matrix[i][h%cms.width] += count
		if uint64(cms.matrix[i][h%cms.width]) < minCount {
			minCount = uint64(cms.matrix[i][h%cms.width])
		}
	}

	cms.count += uint64(count)
	return minCount
}

func (cms *CountMinSketch) Count(item []byte) uint64 {
	cms.mu.RLock()
	defer cms.mu.RUnlock()

	minCount := ^uint64(0)

	for i := uint(0); i < cms.depth; i++ {
		h := cms.hash(item, i)
		c := uint64(cms.matrix[i][h%cms.width])
		if c < minCount {
			minCount = c
		}
	}

	return minCount
}

func (cms *CountMinSketch) hash(item []byte, seed uint) uint {
	h := fnv.New32a()
	h.Write([]byte{byte(seed)})
	h.Write(item)
	return uint(h.Sum32())
}

func (cms *CountMinSketch) Info() map[string]interface{} {
	cms.mu.RLock()
	defer cms.mu.RUnlock()

	return map[string]interface{}{
		"depth": cms.depth,
		"width": cms.width,
		"count": cms.count,
	}
}

func (cms *CountMinSketch) Clear() {
	cms.mu.Lock()
	defer cms.mu.Unlock()

	for i := range cms.matrix {
		for j := range cms.matrix[i] {
			cms.matrix[i][j] = 0
		}
	}
	cms.count = 0
}

type TopK struct {
	items map[string]uint64
	k     int
	count uint64
	mu    sync.RWMutex
}

func NewTopK(k int) *TopK {
	return &TopK{
		items: make(map[string]uint64),
		k:     k,
	}
}

func (tk *TopK) Add(item string, count uint64) uint64 {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	tk.items[item] += count
	tk.count += count
	return tk.items[item]
}

func (tk *TopK) Query(item string) uint64 {
	tk.mu.RLock()
	defer tk.mu.RUnlock()
	return tk.items[item]
}

func (tk *TopK) List() []string {
	tk.mu.RLock()
	defer tk.mu.RUnlock()

	type kv struct {
		key   string
		value uint64
	}

	var sorted []kv
	for k, v := range tk.items {
		sorted = append(sorted, kv{k, v})
	}

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].value > sorted[i].value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([]string, 0, tk.k)
	for i := 0; i < len(sorted) && i < tk.k; i++ {
		result = append(result, sorted[i].key)
	}

	return result
}

func (tk *TopK) ListWithCount() []map[string]interface{} {
	tk.mu.RLock()
	defer tk.mu.RUnlock()

	type kv struct {
		key   string
		value uint64
	}

	var sorted []kv
	for k, v := range tk.items {
		sorted = append(sorted, kv{k, v})
	}

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].value > sorted[i].value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([]map[string]interface{}, 0, tk.k)
	for i := 0; i < len(sorted) && i < tk.k; i++ {
		result = append(result, map[string]interface{}{
			"item":  sorted[i].key,
			"count": sorted[i].value,
		})
	}

	return result
}

func (tk *TopK) Info() map[string]interface{} {
	tk.mu.RLock()
	defer tk.mu.RUnlock()

	return map[string]interface{}{
		"k":     tk.k,
		"items": len(tk.items),
		"total": tk.count,
	}
}

func (tk *TopK) Clear() {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	tk.items = make(map[string]uint64)
	tk.count = 0
}

type CuckooFilter struct {
	buckets    [][]byte
	size       uint
	bucketSize uint
	count      uint
	kicks      uint
	mu         sync.RWMutex
}

func NewCuckooFilter(size, bucketSize uint) *CuckooFilter {
	buckets := make([][]byte, size)
	for i := range buckets {
		buckets[i] = make([]byte, bucketSize)
		for j := range buckets[i] {
			buckets[i][j] = 0
		}
	}

	return &CuckooFilter{
		buckets:    buckets,
		size:       size,
		bucketSize: bucketSize,
		count:      0,
		kicks:      500,
	}
}

func (cf *CuckooFilter) Add(item []byte) bool {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	fp := cf.fingerprint(item)
	i1 := cf.hash1(item)
	i2 := i1 ^ cf.hash2(fp)

	if cf.insert(i1, fp) || cf.insert(i2, fp) {
		cf.count++
		return true
	}

	i := i1
	if cf.buckets[i2][0] == 0 {
		i = i2
	}

	for n := uint(0); n < cf.kicks; n++ {
		fp, cf.buckets[i%cf.size][0] = cf.buckets[i%cf.size][0], fp
		i = i ^ cf.hash2(fp)
		if cf.insert(i, fp) {
			cf.count++
			return true
		}
	}

	return false
}

func (cf *CuckooFilter) Exists(item []byte) bool {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	fp := cf.fingerprint(item)
	i1 := cf.hash1(item)
	i2 := i1 ^ cf.hash2(fp)

	return cf.contains(i1, fp) || cf.contains(i2, fp)
}

func (cf *CuckooFilter) Delete(item []byte) bool {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	fp := cf.fingerprint(item)
	i1 := cf.hash1(item)
	i2 := i1 ^ cf.hash2(fp)

	if cf.remove(i1, fp) || cf.remove(i2, fp) {
		cf.count--
		return true
	}

	return false
}

func (cf *CuckooFilter) fingerprint(item []byte) byte {
	h := fnv.New32a()
	h.Write(item)
	return byte(h.Sum32()%255) + 1
}

func (cf *CuckooFilter) hash1(item []byte) uint {
	h := fnv.New32a()
	h.Write(item)
	return uint(h.Sum32()) % cf.size
}

func (cf *CuckooFilter) hash2(fp byte) uint {
	h := fnv.New32a()
	h.Write([]byte{fp})
	return uint(h.Sum32()) % cf.size
}

func (cf *CuckooFilter) insert(i uint, fp byte) bool {
	for j := uint(0); j < cf.bucketSize; j++ {
		if cf.buckets[i%cf.size][j] == 0 {
			cf.buckets[i%cf.size][j] = fp
			return true
		}
	}
	return false
}

func (cf *CuckooFilter) contains(i uint, fp byte) bool {
	for j := uint(0); j < cf.bucketSize; j++ {
		if cf.buckets[i%cf.size][j] == fp {
			return true
		}
	}
	return false
}

func (cf *CuckooFilter) remove(i uint, fp byte) bool {
	for j := uint(0); j < cf.bucketSize; j++ {
		if cf.buckets[i%cf.size][j] == fp {
			cf.buckets[i%cf.size][j] = 0
			return true
		}
	}
	return false
}

func (cf *CuckooFilter) Count() uint {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	return cf.count
}

func (cf *CuckooFilter) Info() map[string]interface{} {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	return map[string]interface{}{
		"size":        cf.size,
		"bucket_size": cf.bucketSize,
		"count":       cf.count,
		"load_factor": float64(cf.count) / float64(cf.size*cf.bucketSize),
	}
}
