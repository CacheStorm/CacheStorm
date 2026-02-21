package store

import (
	"math"
	"math/rand"
	"sort"
	"sync"
)

type TDigest struct {
	Means       []float64
	Counts      []float64
	Delta       float64
	K           int
	Compression float64
	mu          sync.RWMutex
}

func NewTDigest(compression float64) *TDigest {
	if compression <= 0 {
		compression = 100
	}
	return &TDigest{
		Means:       make([]float64, 0),
		Counts:      make([]float64, 0),
		Delta:       compression,
		K:           int(compression * 2),
		Compression: compression,
	}
}

func (td *TDigest) Add(value float64, count float64) {
	td.mu.Lock()
	defer td.mu.Unlock()

	td.addUnsafe(value, count)
}

func (td *TDigest) addUnsafe(value float64, count float64) {
	idx := sort.SearchFloat64s(td.Means, value)

	td.Means = append(td.Means, 0)
	td.Counts = append(td.Counts, 0)

	copy(td.Means[idx+1:], td.Means[idx:])
	copy(td.Counts[idx+1:], td.Counts[idx:])

	td.Means[idx] = value
	td.Counts[idx] = count

	if len(td.Means) > td.K {
		td.compressUnsafe()
	}
}

func (td *TDigest) AddBatch(values []float64) {
	td.mu.Lock()
	defer td.mu.Unlock()

	for _, v := range values {
		td.addUnsafe(v, 1)
	}
}

func (td *TDigest) compressUnsafe() {
	if len(td.Means) <= td.K {
		return
	}

	n := td.totalCountUnsafe()
	if n == 0 {
		return
	}

	newMeans := make([]float64, 0, td.K)
	newCounts := make([]float64, 0, td.K)

	q := 0.0
	qLimit := 1.0 / td.Compression
	qMultiplier := 1.0 / n

	currentMean := td.Means[0]
	currentCount := td.Counts[0]

	for i := 1; i < len(td.Means); i++ {
		mean := td.Means[i]
		count := td.Counts[i]

		q += currentCount * qMultiplier

		if q <= qLimit && (len(newMeans)+len(td.Means)-i) > int(td.Compression) {
			mergedMean := (currentMean*currentCount + mean*count) / (currentCount + count)
			currentMean = mergedMean
			currentCount += count
		} else {
			newMeans = append(newMeans, currentMean)
			newCounts = append(newCounts, currentCount)
			currentMean = mean
			currentCount = count
			qLimit = (float64(len(newMeans)) + 0.5) / td.Compression
		}
	}

	newMeans = append(newMeans, currentMean)
	newCounts = append(newCounts, currentCount)

	td.Means = newMeans
	td.Counts = newCounts
}

func (td *TDigest) totalCountUnsafe() float64 {
	var total float64
	for _, c := range td.Counts {
		total += c
	}
	return total
}

func (td *TDigest) Quantile(q float64) float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if len(td.Means) == 0 {
		return 0
	}

	if q <= 0 {
		return td.Means[0]
	}
	if q >= 1 {
		return td.Means[len(td.Means)-1]
	}

	n := td.totalCountUnsafe()
	target := q * n

	var cumSum float64
	for i := 0; i < len(td.Means); i++ {
		cumSum += td.Counts[i]
		if cumSum >= target {
			if i == 0 {
				return td.Means[0]
			}
			prevCumSum := cumSum - td.Counts[i]
			fraction := (target - prevCumSum) / td.Counts[i]
			return td.Means[i-1] + fraction*(td.Means[i]-td.Means[i-1])
		}
	}

	return td.Means[len(td.Means)-1]
}

func (td *TDigest) CDF(value float64) float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if len(td.Means) == 0 {
		return 0
	}

	if value <= td.Means[0] {
		return 0
	}
	if value >= td.Means[len(td.Means)-1] {
		return 1
	}

	n := td.totalCountUnsafe()
	var cumSum float64

	for i := 0; i < len(td.Means); i++ {
		cumSum += td.Counts[i]
		if td.Means[i] >= value {
			if i == 0 {
				return 0.5 * td.Counts[0] / n
			}
			prevCumSum := cumSum - td.Counts[i]
			return (prevCumSum + 0.5*td.Counts[i]) / n
		}
	}

	return 1
}

func (td *TDigest) Mean() float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if len(td.Means) == 0 {
		return 0
	}

	var total, count float64
	for i, m := range td.Means {
		total += m * td.Counts[i]
		count += td.Counts[i]
	}

	if count == 0 {
		return 0
	}
	return total / count
}

func (td *TDigest) Min() float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if len(td.Means) == 0 {
		return 0
	}
	return td.Means[0]
}

func (td *TDigest) Max() float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if len(td.Means) == 0 {
		return 0
	}
	return td.Means[len(td.Means)-1]
}

func (td *TDigest) Count() float64 {
	td.mu.RLock()
	defer td.mu.RUnlock()
	return td.totalCountUnsafe()
}

func (td *TDigest) Size() int {
	td.mu.RLock()
	defer td.mu.RUnlock()
	return len(td.Means)
}

func (td *TDigest) Reset() {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.Means = make([]float64, 0)
	td.Counts = make([]float64, 0)
}

func (td *TDigest) Merge(other *TDigest) {
	td.mu.Lock()
	defer td.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	for i, m := range other.Means {
		td.addUnsafe(m, other.Counts[i])
	}
}

type ReservoirSampler struct {
	Data    []float64
	MaxSize int
	Count   int64
	mu      sync.RWMutex
	rng     *rand.Rand
}

func NewReservoirSampler(size int) *ReservoirSampler {
	return &ReservoirSampler{
		Data:    make([]float64, 0, size),
		MaxSize: size,
		rng:     rand.New(rand.NewSource(42)),
	}
}

func (rs *ReservoirSampler) Add(value float64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.Count++

	if len(rs.Data) < rs.MaxSize {
		rs.Data = append(rs.Data, value)
		return
	}

	j := rs.rng.Intn(int(rs.Count))
	if j < rs.MaxSize {
		rs.Data[j] = value
	}
}

func (rs *ReservoirSampler) AddBatch(values []float64) {
	for _, v := range values {
		rs.Add(v)
	}
}

func (rs *ReservoirSampler) Get() []float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	result := make([]float64, len(rs.Data))
	copy(result, rs.Data)
	return result
}

func (rs *ReservoirSampler) Reset() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.Data = make([]float64, 0, rs.MaxSize)
	rs.Count = 0
}

func (rs *ReservoirSampler) Size() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.Data)
}

func (rs *ReservoirSampler) TotalCount() int64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.Count
}

type Histogram struct {
	Buckets     map[int]int64
	Min         float64
	Max         float64
	BucketWidth float64
	Count       int64
	Sum         float64
	mu          sync.RWMutex
}

func NewHistogram(min, max float64, bucketWidth float64) *Histogram {
	if bucketWidth <= 0 {
		bucketWidth = 1
	}
	return &Histogram{
		Buckets:     make(map[int]int64),
		Min:         min,
		Max:         max,
		BucketWidth: bucketWidth,
	}
}

func (h *Histogram) Add(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Count++
	h.Sum += value

	bucket := h.bucketIndex(value)
	h.Buckets[bucket]++
}

func (h *Histogram) bucketIndex(value float64) int {
	if value < h.Min {
		return -1
	}
	if value >= h.Max {
		return int((h.Max - h.Min) / h.BucketWidth)
	}
	return int((value - h.Min) / h.BucketWidth)
}

func (h *Histogram) bucketRange(idx int) (float64, float64) {
	return h.Min + float64(idx)*h.BucketWidth, h.Min + float64(idx+1)*h.BucketWidth
}

func (h *Histogram) Get() map[string]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[string]int64)
	for idx, count := range h.Buckets {
		start, end := h.bucketRange(idx)
		key := formatBucket(start, end)
		result[key] = count
	}
	return result
}

func (h *Histogram) Mean() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.Count == 0 {
		return 0
	}
	return h.Sum / float64(h.Count)
}

func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Buckets = make(map[int]int64)
	h.Count = 0
	h.Sum = 0
}

func formatBucket(start, end float64) string {
	return "[" + formatFloat(start) + "," + formatFloat(end) + ")"
}

func formatFloat(f float64) string {
	if f == math.Floor(f) {
		return formatInt(int64(f))
	}
	s := ""
	if f < 0 {
		s = "-"
		f = -f
	}

	intPart := int64(f)
	fracPart := f - float64(intPart)

	s += formatInt(intPart)
	if fracPart > 0 {
		fracStr := ""
		for fracPart > 0 && len(fracStr) < 6 {
			fracPart *= 10
			digit := int64(fracPart)
			fracPart -= float64(digit)
			fracStr += string(rune('0' + digit))
		}
		for len(fracStr) > 0 && fracStr[len(fracStr)-1] == '0' {
			fracStr = fracStr[:len(fracStr)-1]
		}
		if fracStr != "" {
			s += "." + fracStr
		}
	}

	return s
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}

	if neg {
		s = "-" + s
	}

	return s
}
