package command

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"sync"

	"github.com/cachestorm/cachestorm/internal/store"
)

// tdigestSketch is RedisBloom's t-digest (t-digest-c): centroids kept as
// parallel mean and weight arrays, merged nodes first, compressed when the
// arrays reach their capacity of 6*compression+10.
type tdigestSketch struct {
	mu                sync.Mutex
	compression       int64
	cap               int
	means             []float64
	weights           []int64
	mergedNodes       int
	mergedWeight      int64
	unmergedWeight    int64
	min               float64
	max               float64
	totalCompressions int64
}

var errTDigestOverflow = errors.New("ERR T-Digest: overflow detected")

func newTDigestSketch(compression int64) *tdigestSketch {
	td := &tdigestSketch{compression: compression, cap: int(6*compression + 10)}
	td.resetLocked()
	return td
}

func (td *tdigestSketch) Type() store.DataType { return store.DataTypeTDigest }
func (td *tdigestSketch) String() string       { return "tdigest" }
func (td *tdigestSketch) ExtensionType() byte  { return store.ExtensionTDigest }

func (td *tdigestSketch) SizeOf() int64 {
	td.mu.Lock()
	defer td.mu.Unlock()
	return int64(cap(td.means)+cap(td.weights))*8 + 128
}

func (td *tdigestSketch) Clone() store.Value {
	td.mu.Lock()
	defer td.mu.Unlock()
	return &tdigestSketch{
		compression:       td.compression,
		cap:               td.cap,
		means:             append([]float64(nil), td.means...),
		weights:           append([]int64(nil), td.weights...),
		mergedNodes:       td.mergedNodes,
		mergedWeight:      td.mergedWeight,
		unmergedWeight:    td.unmergedWeight,
		min:               td.min,
		max:               td.max,
		totalCompressions: td.totalCompressions,
	}
}

func (td *tdigestSketch) MarshalBinary() ([]byte, error) {
	td.mu.Lock()
	defer td.mu.Unlock()
	out := binary.LittleEndian.AppendUint64(nil, uint64(td.compression))
	for _, v := range []uint64{uint64(td.mergedNodes), uint64(td.mergedWeight), uint64(td.unmergedWeight),
		uint64(td.totalCompressions), math.Float64bits(td.min), math.Float64bits(td.max), uint64(len(td.means))} {
		out = binary.LittleEndian.AppendUint64(out, v)
	}
	for i, m := range td.means {
		out = binary.LittleEndian.AppendUint64(out, math.Float64bits(m))
		out = binary.LittleEndian.AppendUint64(out, uint64(td.weights[i]))
	}
	return out, nil
}

func decodeTDigest(data []byte) (store.Value, error) {
	if len(data) < 64 {
		return nil, store.ErrExtensionFormat
	}
	u := func(i int) uint64 { return binary.LittleEndian.Uint64(data[8*i:]) }
	compression := int64(u(0))
	if compression <= 0 || compression > 1<<20 {
		return nil, store.ErrExtensionFormat
	}
	td := newTDigestSketch(compression)
	td.mergedNodes, td.mergedWeight, td.unmergedWeight = int(u(1)), int64(u(2)), int64(u(3))
	td.totalCompressions = int64(u(4))
	td.min, td.max = math.Float64frombits(u(5)), math.Float64frombits(u(6))
	n := u(7)
	if n > uint64(td.cap) || uint64(len(data)) != 64+16*n || td.mergedNodes < 0 || uint64(td.mergedNodes) > n {
		return nil, store.ErrExtensionFormat
	}
	for i := uint64(0); i < n; i++ {
		td.means = append(td.means, math.Float64frombits(u(8+2*int(i))))
		td.weights = append(td.weights, int64(u(9+2*int(i))))
	}
	return td, nil
}

func init() { store.RegisterExtension(store.ExtensionTDigest, decodeTDigest) }

func (td *tdigestSketch) capacity() int { return td.cap }

func (td *tdigestSketch) resetLocked() {
	td.means = td.means[:0]
	td.weights = td.weights[:0]
	td.mergedNodes = 0
	td.mergedWeight = 0
	td.unmergedWeight = 0
	td.min = math.MaxFloat64
	td.max = -math.MaxFloat64
	td.totalCompressions = 0
}

func (td *tdigestSketch) unmergedNodes() int { return len(td.means) - td.mergedNodes }

func (td *tdigestSketch) total() int64 { return td.mergedWeight + td.unmergedWeight }

func tdWeightedAverage(x1, w1, x2, w2 float64) float64 {
	if x1 > x2 {
		x1, w1, x2, w2 = x2, w2, x1, w1
	}
	x := (x1*w1 + x2*w2) / (w1 + w2)
	return math.Max(x1, math.Min(x, x2))
}

func tdOverflow(unmerged, total float64) bool {
	return math.IsInf(unmerged, 1) || math.IsInf(total, 1) || math.IsInf(2*math.Pi*total*tdLog(total), 1)
}

// tdQsort is t-digest-c's quicksort on the parallel arrays, central pivot,
// so equal means end up in the same order as in RedisBloom.
func tdQsort(means []float64, weights []int64, start, end int) {
	for start < end {
		if end-start == 1 {
			if means[start] > means[end] {
				means[start], means[end] = means[end], means[start]
				weights[start], weights[end] = weights[end], weights[start]
			}
			return
		}
		pivot := (end + start) / 2
		pivotMean := means[pivot]
		means[pivot], means[end] = means[end], means[pivot]
		weights[pivot], weights[end] = weights[end], weights[pivot]
		i := start - 1
		for j := start; j < end; j++ {
			if means[j] < pivotMean {
				i++
				means[i], means[j] = means[j], means[i]
				weights[i], weights[j] = weights[j], weights[i]
			}
		}
		means[i+1], means[end] = means[end], means[i+1]
		weights[i+1], weights[end] = weights[end], weights[i+1]
		p := i + 1
		if p > start {
			tdQsort(means, weights, start, p-1)
		}
		start = p + 1
	}
}

// addLocked is td_add.
func (td *tdigestSketch) addLocked(mean float64, weight int64) error {
	if len(td.means) >= td.cap-1 {
		if err := td.compressLocked(); err != nil {
			return err
		}
	}
	if len(td.means) >= td.cap {
		return errTDigestOverflow
	}
	newUnmerged := td.unmergedWeight + weight
	newTotal := newUnmerged + td.mergedWeight
	if newUnmerged < td.unmergedWeight || newTotal < newUnmerged || tdOverflow(float64(newUnmerged), float64(newTotal)) {
		return errTDigestOverflow
	}
	td.min = math.Min(td.min, mean)
	td.max = math.Max(td.max, mean)
	td.means = append(td.means, mean)
	td.weights = append(td.weights, weight)
	td.unmergedWeight = newUnmerged
	return nil
}

// compressLocked is td_compress. Like it, a digest of total weight 1 keeps
// its node unmerged.
func (td *tdigestSketch) compressLocked() error {
	if td.unmergedNodes() == 0 {
		return nil
	}
	n := len(td.means)
	m, w := td.means, td.weights
	tdQsort(m, w, 0, n-1)
	total := float64(td.mergedWeight) + float64(td.unmergedWeight)
	if tdOverflow(float64(td.unmergedWeight), total) {
		return errTDigestOverflow
	}
	if total <= 1 {
		return nil
	}
	denom := 2 * math.Pi * total * tdLog(total)
	normalizer := float64(td.compression) / denom
	if math.IsInf(denom, 1) || math.IsInf(normalizer, 1) {
		return errTDigestOverflow
	}
	cur := 0
	soFar := 0.0
	for i := 1; i < n; i++ {
		proposed := float64(w[cur]) + float64(w[i])
		z := proposed * normalizer
		q0 := soFar / total
		q2 := (soFar + proposed) / total
		if z <= q0*(1-q0) && z <= q2*(1-q2) {
			w[cur] += w[i]
			delta := m[i] - m[cur]
			m[cur] += delta * float64(w[i]) / float64(w[cur])
		} else {
			soFar += float64(w[cur])
			cur++
			w[cur] = w[i]
			m[cur] = m[i]
		}
	}
	td.means, td.weights = m[:cur+1], w[:cur+1]
	td.mergedNodes = cur + 1
	td.mergedWeight = int64(total)
	td.unmergedWeight = 0
	td.totalCompressions++
	return nil
}

// compressForRead compresses before a query; like RedisBloom's queries it
// answers from whatever is merged when compression overflows.
func (td *tdigestSketch) compressForRead() {
	if err := td.compressLocked(); err != nil {
		return
	}
}

// mergeFrom is td_merge: from is compressed and its centroids added.
func (td *tdigestSketch) mergeFrom(from *tdigestSketch) error {
	if err := td.compressLocked(); err != nil {
		return err
	}
	if err := from.compressLocked(); err != nil {
		return err
	}
	for i, m := range from.means {
		if err := td.addLocked(m, from.weights[i]); err != nil {
			return err
		}
	}
	return nil
}

// cdfLocked is td_cdf.
func (td *tdigestSketch) cdfLocked(val float64) float64 {
	td.compressForRead()
	n := td.mergedNodes
	if n == 0 {
		return math.NaN()
	}
	if val < td.min {
		return 0
	}
	if val > td.max {
		return 1
	}
	m, w := td.means, td.weights
	if n == 1 {
		width := td.max - td.min
		if val-td.min <= width {
			return 0.5
		}
		return (val - td.min) / width
	}
	total := float64(td.mergedWeight)
	if val < m[0] {
		width := m[0] - td.min
		if width > 0 {
			if val == td.min {
				return 0.5 / total
			}
			return (1 + (val-td.min)/width*(float64(w[0])/2-1)) / total
		}
		return 0
	}
	if val > m[n-1] {
		width := td.max - m[n-1]
		if width > 0 {
			if val == td.max {
				return 1 - 0.5/total
			}
			return 1 - (1+(td.max-val)/width*(float64(w[n-1])/2-1))/total
		}
		return 1
	}
	soFar := 0.0
	for it := 0; it < n-1; it++ {
		switch {
		case m[it] == val:
			dw := 0.0
			for it < n && m[it] == val {
				dw += float64(w[it])
				it++
			}
			return (soFar + dw/2) / total
		case m[it] <= val && val < m[it+1]:
			nw, nwNext := float64(w[it]), float64(w[it+1])
			if m[it+1]-m[it] > 0 {
				leftExcluded, rightExcluded := 0.0, 0.0
				if nw == 1 {
					if nwNext == 1 {
						return (soFar + 1) / total
					}
					leftExcluded = 0.5
				} else if nwNext == 1 {
					rightExcluded = 0.5
				}
				dw := (nw + nwNext) / 2
				dwNoSingleton := dw - leftExcluded - rightExcluded
				base := soFar + nw/2 + leftExcluded
				return (base + dwNoSingleton*(val-m[it])/(m[it+1]-m[it])) / total
			}
			return (soFar + (nw+nwNext)/2) / total
		default:
			soFar += float64(w[it])
		}
	}
	return 1 - 0.5/total
}

// quantileWalk is td_internal_iterate_centroids_to_index; soFar and pos
// carry over between sorted quantiles.
func (td *tdigestSketch) quantileWalk(index, leftWeight float64, soFar *float64, pos *int) float64 {
	m, w, n := td.means, td.weights, td.mergedNodes
	if leftWeight > 1 && index < leftWeight/2 {
		return td.min + (index-1)/(leftWeight/2-1)*(m[0]-td.min)
	}
	if index > float64(td.mergedWeight-1) {
		return td.max
	}
	rightWeight, rightMean := float64(w[n-1]), m[n-1]
	if rightWeight > 1 && float64(td.mergedWeight)-index <= rightWeight/2 {
		return td.max - (float64(td.mergedWeight)-index-1)/(rightWeight/2-1)*(td.max-rightMean)
	}
	for ; *pos < n-1; *pos++ {
		i := *pos
		nw, nwNext := float64(w[i]), float64(w[i+1])
		dw := (nw + nwNext) / 2
		if *soFar+dw > index {
			leftUnit := 0.0
			if nw == 1 {
				if index-*soFar < 0.5 {
					return m[i]
				}
				leftUnit = 0.5
			}
			rightUnit := 0.0
			if nwNext == 1 {
				if *soFar+dw-index <= 0.5 {
					return m[i+1]
				}
				rightUnit = 0.5
			}
			z1 := index - *soFar - leftUnit
			z2 := *soFar + dw - index - rightUnit
			return tdWeightedAverage(m[i], z2, m[i+1], z1)
		}
		*soFar += dw
	}
	z1 := index - float64(td.mergedWeight) - rightWeight/2.0
	z2 := rightWeight/2 - z1
	return tdWeightedAverage(rightMean, z1, td.max, z2)
}

// quantileLocked is td_quantile.
func (td *tdigestSketch) quantileLocked(q float64) float64 {
	td.compressForRead()
	if q < 0 || q > 1 || td.mergedNodes == 0 {
		return math.NaN()
	}
	if td.mergedNodes == 1 {
		return td.means[0]
	}
	index := q * float64(td.mergedWeight)
	if index < 1 {
		return td.min
	}
	left := float64(td.weights[0])
	soFar, pos := left/2, 0
	return td.quantileWalk(index, left, &soFar, &pos)
}

// quantilesLocked is td_quantiles over a non-decreasing run of quantiles.
func (td *tdigestSketch) quantilesLocked(qs []float64, out []float64) {
	td.compressForRead()
	switch td.mergedNodes {
	case 0:
		for i := range qs {
			out[i] = math.NaN()
		}
		return
	case 1:
		for i := range qs {
			out[i] = td.means[0]
		}
		return
	}
	left := float64(td.weights[0])
	soFar, pos := left/2, 0
	for i, q := range qs {
		out[i] = td.quantileWalk(q*float64(td.mergedWeight), left, &soFar, &pos)
	}
}

// trimmedMeanLocked is td_trimmed_mean.
func (td *tdigestSketch) trimmedMeanLocked(low, high float64) float64 {
	td.compressForRead()
	if td.mergedNodes == 0 {
		return math.NaN()
	}
	if td.mergedNodes == 1 {
		return td.means[0]
	}
	left := math.Floor(float64(td.mergedWeight) * low)
	right := math.Ceil(float64(td.mergedWeight) * high)
	var done, sum, count float64
	for i := 0; i < td.mergedNodes; i++ {
		w := float64(td.weights[i])
		add := w - math.Min(math.Max(0, left-done), w)
		add = math.Min(math.Max(0, right-done), add)
		done += w
		sum += td.means[i] * add
		count += add
		if done >= right {
			break
		}
	}
	return sum / count
}

func (td *tdigestSketch) meanLocked() float64 {
	td.compressForRead()
	if td.mergedNodes == 0 {
		return math.NaN()
	}
	var sum float64
	for i := 0; i < td.mergedNodes; i++ {
		sum += td.means[i] * float64(td.weights[i])
	}
	return sum / float64(td.mergedWeight)
}

// tdHalfRoundDown rounds halves toward zero, as TDIGEST.RANK does.
func tdHalfRoundDown(f float64) float64 {
	i, frac := math.Modf(f)
	if math.Abs(frac) <= 0.5 {
		return i
	}
	if i >= 0 {
		return i + 1
	}
	return i - 1
}

func (td *tdigestSketch) rankLocked(v float64, reverse bool) int64 {
	size := float64(td.total())
	switch {
	case size == 0:
		return -2
	case v < td.min:
		if reverse {
			return int64(size)
		}
		return -1
	case v > td.max:
		if reverse {
			return -1
		}
		return int64(size)
	}
	r := td.cdfLocked(v) * size
	if reverse {
		return int64(math.Round(size - math.Round(r)))
	}
	return int64(tdHalfRoundDown(r))
}

func (td *tdigestSketch) byRankLocked(rank float64, reverse bool) float64 {
	size := float64(td.total())
	switch {
	case size == 0:
		return math.NaN()
	case rank == 0 && reverse:
		return td.max
	case rank == 0:
		return td.min
	case rank >= size && reverse:
		return math.Inf(-1)
	case rank >= size:
		return math.Inf(1)
	case reverse:
		return td.quantileLocked((size - rank - 1) / size)
	}
	return td.quantileLocked(rank / size)
}

// tdLog is a correctly rounded natural logarithm, as glibc's log is; Go's
// math.Log is off by one unit in the last place for about 1% of inputs,
// which would shift t-digest centroid boundaries.
func tdLog(x float64) float64 {
	if x <= 0 || math.IsInf(x, 0) || math.IsNaN(x) {
		return math.Log(x)
	}
	frac, exp := math.Frexp(x)
	m := new(big.Float).SetPrec(tdLogPrec).SetFloat64(frac * 2)
	r := tdAtanhLog(m)
	r.Add(r, new(big.Float).SetPrec(tdLogPrec).Mul(tdLn2(), new(big.Float).SetInt64(int64(exp-1))))
	f, _ := r.Float64()
	return f
}

const tdLogPrec = 160

// tdAtanhLog is log(m) = 2*atanh((m-1)/(m+1)) for m in [1, 2).
func tdAtanhLog(m *big.Float) *big.Float {
	one := new(big.Float).SetPrec(tdLogPrec).SetInt64(1)
	z := new(big.Float).SetPrec(tdLogPrec).Quo(new(big.Float).SetPrec(tdLogPrec).Sub(m, one), new(big.Float).SetPrec(tdLogPrec).Add(m, one))
	z2 := new(big.Float).SetPrec(tdLogPrec).Mul(z, z)
	sum := new(big.Float).SetPrec(tdLogPrec)
	term := new(big.Float).SetPrec(tdLogPrec).Set(z)
	for k := int64(1); k < 400; k += 2 {
		t := new(big.Float).SetPrec(tdLogPrec).Quo(term, new(big.Float).SetInt64(k))
		if t.Sign() == 0 || (sum.Sign() != 0 && t.MantExp(nil)-sum.MantExp(nil) < -tdLogPrec) {
			break
		}
		sum.Add(sum, t)
		term.Mul(term, z2)
	}
	return sum.Mul(sum, new(big.Float).SetInt64(2))
}

var tdLn2Value = sync.OnceValue(func() *big.Float {
	return tdAtanhLog(new(big.Float).SetPrec(tdLogPrec).SetInt64(2))
})

func tdLn2() *big.Float { return tdLn2Value() }
