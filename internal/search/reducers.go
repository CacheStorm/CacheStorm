package search

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
)

// reduced is a reducer's result and how RediSearch holds it.
type reduced struct {
	v     string
	f     float64
	num   bool
	null  bool
	isArr bool
	arr   []cell
}

func numReduced(f float64) reduced { return reduced{v: FormatNumber(f), f: f, num: true} }

// cell is a row's value of a property as RediSearch's lookup holds it.
type cell struct {
	v       string
	f       float64
	present bool
	num     bool
	null    bool
	lit     bool
	isArr   bool
	arr     []cell
}

func (r *Row) cell(field string) cell {
	name := strings.TrimPrefix(field, "@")
	v, ok := r.Get(name)
	if !ok {
		return cell{}
	}
	if r.IsNull(name) {
		return cell{present: true, null: true}
	}
	if r.lists[name] {
		return cell{v: v, present: true, isArr: true, arr: r.arrays[name]}
	}
	f, num := r.Number(name)
	return cell{v: v, f: f, present: true, num: num, lit: r.lit[name]}
}

// number is RSValue_ToNumber: a number, or a string strtod reads whole.
func (c cell) number() (float64, bool) {
	if !c.present || c.null || c.isArr {
		return 0, false
	}
	if c.num {
		return c.f, true
	}
	return parseCNumber(c.v)
}

func (c cell) hash(seed uint64) uint64 {
	if c.isArr {
		for _, item := range c.arr {
			seed = item.hash(seed)
		}
		return seed
	}
	return groupHash(seed, c.v, c.f, c.num, c.null)
}

// items are the numbers STDDEV and QUANTILE read: a value's, or each of a
// list's.
func (c cell) items() []cell {
	if c.isArr {
		return c.arr
	}
	return []cell{c}
}

func reduce(red Reducer, rows []*Row) (reduced, error) {
	arg := func(i int) string {
		if i < len(red.Args) {
			return red.Args[i]
		}
		return ""
	}
	numbers := func() []float64 {
		var out []float64
		for _, r := range rows {
			if f, ok := r.cell(arg(0)).number(); ok {
				out = append(out, f)
			}
		}
		return out
	}
	itemNumbers := func() []float64 {
		var out []float64
		for _, r := range rows {
			for _, c := range r.cell(arg(0)).items() {
				if f, ok := c.number(); ok {
					out = append(out, f)
				}
			}
		}
		return out
	}
	switch strings.ToUpper(red.Fn) {
	case "COUNT":
		return numReduced(float64(len(rows))), nil
	case "COUNT_DISTINCT":
		seen := make(map[uint64]bool)
		for _, r := range rows {
			if c := r.cell(arg(0)); c.present && !c.null {
				seen[c.hash(0)] = true
			}
		}
		return numReduced(float64(len(seen))), nil
	case "COUNT_DISTINCTISH":
		var h hll
		for _, r := range rows {
			if c := r.cell(arg(0)); c.present && !c.null {
				x := c.hash(0x5f61767a)
				h.add(uint32(x) ^ uint32(x>>32))
			}
		}
		return numReduced(float64(h.count())), nil
	case "SUM", "AVG":
		vs := numbers()
		if len(vs) == 0 {
			return numReduced(math.NaN()), nil
		}
		sum := 0.0
		for _, v := range vs {
			sum += v
		}
		if strings.EqualFold(red.Fn, "AVG") {
			sum /= float64(len(vs))
		}
		return numReduced(sum), nil
	case "MIN":
		best := math.Inf(1)
		for _, v := range numbers() {
			if v < best {
				best = v
			}
		}
		return numReduced(best), nil
	case "MAX":
		best := math.Inf(-1)
		for _, v := range numbers() {
			if v > best {
				best = v
			}
		}
		return numReduced(best), nil
	case "STDDEV":
		// Welford's running variance, as RediSearch computes it.
		n, m, s := 0, 0.0, 0.0
		for _, d := range itemNumbers() {
			n++
			if n == 1 {
				m, s = d, 0
				continue
			}
			nm := m + (d-m)/float64(n)
			s += (d - m) * (d - nm)
			m = nm
		}
		variance := 0.0
		if n > 1 {
			variance = s / float64(n-1)
		}
		return numReduced(math.Sqrt(variance)), nil
	case "QUANTILE":
		q, err := strconv.ParseFloat(arg(1), 64)
		if err != nil {
			return numReduced(math.NaN()), nil
		}
		resolution := 500
		if n, err := strconv.Atoi(arg(2)); err == nil && n > 0 {
			resolution = n
		}
		qs := newQuantileStream(resolution)
		for _, v := range itemNumbers() {
			qs.insert(v)
		}
		return numReduced(qs.query(q)), nil
	case "FIRST_VALUE":
		return firstValue(red, rows), nil
	case "TOLIST":
		var d rsDict
		for _, r := range rows {
			if c := r.cell(arg(0)); c.present {
				for _, item := range c.items() {
					d.add(item)
				}
			}
		}
		return reduced{isArr: true, arr: d.order()}, nil
	case "RANDOM_SAMPLE":
		size, err := strconv.Atoi(arg(1))
		if err != nil {
			size = 0
		}
		var sample []cell
		seen := 0
		for _, r := range rows {
			c := r.cell(arg(0))
			if !c.present {
				continue
			}
			seen++
			if len(sample) < size {
				sample = append(sample, c)
			} else if i := rand.IntN(seen); i < size {
				sample[i] = c
			}
		}
		return reduced{isArr: true, arr: sample}, nil
	}
	return reduced{}, errors.New("Bad arguments for REDUCE: unknown reducer " + red.Fn) //nolint:staticcheck // RediSearch's reply text
}

// firstValue is RediSearch's FIRST_VALUE: the first row's value, or with
// BY the value of the row whose sort property is best (lowest unless
// DESC), rows lacking it losing to any that has it.
func firstValue(red Reducer, rows []*Row) reduced {
	if len(rows) == 0 {
		return reduced{null: true}
	}
	field := red.Args[0]
	by, desc := "", false
	if len(red.Args) >= 3 && strings.EqualFold(red.Args[1], "BY") {
		by = red.Args[2]
		desc = len(red.Args) >= 4 && strings.EqualFold(red.Args[3], "DESC")
	}
	pick := rows[0]
	if by != "" {
		best := rows[0].cell(by)
		for _, r := range rows[1:] {
			c := r.cell(by)
			switch {
			case !c.present || c.null:
			case !best.present || best.null:
				best, pick = c, r
			default:
				rc := compareCells(c, best)
				if !desc && rc < 0 || desc && rc > 0 {
					best, pick = c, r
				}
			}
		}
	}
	c := pick.cell(field)
	if !c.present || c.null {
		return reduced{null: true}
	}
	return reduced{v: c.v, f: c.f, num: c.num, isArr: c.isArr, arr: c.arr}
}

// compareCells is RSValue_Cmp without an error: a string against a number
// compares as a number when it reads as one, otherwise as strings.
func compareCells(a, b cell) int {
	if a.num || b.num {
		x, okA := a.number()
		y, okB := b.number()
		if okA && okB {
			switch {
			case x > y:
				return 1
			case x < y:
				return -1
			}
			return 0
		}
	}
	return strings.Compare(a.v, b.v)
}

// hll is RediSearch's HyperLogLog with 8 bits of precision.
type hll struct {
	registers [256]uint8
}

func (h *hll) add(hash uint32) {
	index := hash >> 24
	rank := uint8(32)
	if hash != 0 {
		rank = 0
		for hash&1 == 0 {
			hash >>= 1
			rank++
		}
	}
	rank = min(rank, 24) + 1
	if rank > h.registers[index] {
		h.registers[index] = rank
	}
}

func (h *hll) count() uint64 {
	size := float64(len(h.registers))
	alpha := 0.7213 / (1.0 + 1.079/size) * size * size
	sum := 0.0
	zeros := 0
	for _, r := range h.registers {
		sum += 1.0 / float64(int(1)<<r)
		if r == 0 {
			zeros++
		}
	}
	if zeros == len(h.registers) {
		return 0
	}
	estimate := alpha / sum
	if estimate <= 5.0/2.0*size {
		if zeros > 0 {
			estimate = size * math.Log(size/float64(zeros))
		}
	} else if estimate > (1.0/30.0)*4294967296.0 {
		estimate = -4294967296.0 * math.Log(1.0-estimate/4294967296.0)
	}
	return uint64(estimate)
}

// quantileStream is RediSearch's QuantStream: a buffer of resolution
// values merged into a compressed list of samples, with error 0.01.
type quantileStream struct {
	buffer  []float64
	cap     int
	samples []qsSample
	n       float64
}

type qsSample struct {
	v    float64
	g, d float32
}

const quantEpsilon = 0.01

func newQuantileStream(resolution int) *quantileStream {
	return &quantileStream{cap: resolution}
}

func (qs *quantileStream) maxVal(r float64) float64 { return quantEpsilon * 2.0 * r }

func (qs *quantileStream) insert(v float64) {
	qs.buffer = append(qs.buffer, v)
	if len(qs.buffer) == qs.cap {
		qs.flush()
		qs.compress()
	}
}

func (qs *quantileStream) flush() {
	sort.Float64s(qs.buffer)
	r := 0.0
	pos := 0
	for _, v := range qs.buffer {
		s := qsSample{v: v, g: 1}
		inserted := false
		for pos < len(qs.samples) {
			if qs.samples[pos].v > v {
				s.d = float32(math.Floor(qs.maxVal(r)) - 1)
				qs.samples = append(qs.samples, qsSample{})
				copy(qs.samples[pos+1:], qs.samples[pos:])
				qs.samples[pos] = s
				pos++
				inserted = true
				break
			}
			r += float64(qs.samples[pos].g)
			pos++
		}
		if !inserted {
			qs.samples = append(qs.samples, s)
			pos = len(qs.samples)
		}
		qs.n++
	}
	qs.buffer = qs.buffer[:0]
}

func (qs *quantileStream) compress() {
	if len(qs.samples) < 2 {
		return
	}
	last := len(qs.samples) - 1
	r := qs.n - 1 - float64(qs.samples[last].g)
	for i := last - 1; i >= 0; i-- {
		g := qs.samples[i].g
		parent := &qs.samples[i+1]
		if float64(g+parent.g+parent.d) <= qs.maxVal(r) {
			parent.g += g
			qs.samples = append(qs.samples[:i], qs.samples[i+1:]...)
		}
		r -= float64(g)
	}
}

func (qs *quantileStream) query(q float64) float64 {
	if len(qs.buffer) > 0 {
		qs.flush()
	}
	if len(qs.samples) == 0 {
		return math.NaN()
	}
	t := math.Ceil(q * qs.n)
	t += math.Floor(qs.maxVal(t) / 2.0)
	prev := qs.samples[0]
	r := 0.0
	for _, cur := range qs.samples[1:] {
		if r+float64(cur.g)+float64(cur.d) >= t {
			break
		}
		r += float64(cur.g)
		prev = cur
	}
	return prev.v
}

// rsDict is the order RediSearch's dict (Redis' incrementally rehashed
// chained table) iterates the distinct values TOLIST adds to it.
type rsDict struct {
	tables    [2][][]cell
	used      [2]int
	rehashIdx int
	started   bool
}

func (d *rsDict) rehashing() bool { return d.started && d.rehashIdx >= 0 }

func (d *rsDict) rehashStep() {
	// One bucket moves, after at most ten empty ones.
	if d.used[0] != 0 {
		empty := 10
		for len(d.tables[0][d.rehashIdx]) == 0 {
			d.rehashIdx++
			empty--
			if empty == 0 {
				return
			}
		}
		mask := uint64(len(d.tables[1]) - 1)
		for _, c := range d.tables[0][d.rehashIdx] {
			h := c.hash(0) & mask
			d.tables[1][h] = append([]cell{c}, d.tables[1][h]...)
			d.used[0]--
			d.used[1]++
		}
		d.tables[0][d.rehashIdx] = nil
		d.rehashIdx++
	}
	if d.used[0] == 0 {
		d.tables[0], d.used[0] = d.tables[1], d.used[1]
		d.tables[1], d.used[1] = nil, 0
		d.rehashIdx = -1
	}
}

func (d *rsDict) expand(size int) {
	real := 4
	for real < size {
		real *= 2
	}
	if !d.started {
		d.tables[0] = make([][]cell, real)
		d.started, d.rehashIdx = true, -1
		return
	}
	if real == len(d.tables[0]) {
		return
	}
	d.tables[1] = make([][]cell, real)
	d.rehashIdx = 0
}

// add puts a value the dict lacks at the head of its bucket.
func (d *rsDict) add(c cell) {
	if d.rehashing() {
		d.rehashStep()
	}
	if !d.started {
		d.expand(4)
	} else if !d.rehashing() && d.used[0] >= len(d.tables[0]) {
		d.expand(d.used[0] * 2)
	}
	h := c.hash(0)
	for t := 0; t <= 1; t++ {
		if t == 1 && !d.rehashing() {
			break
		}
		for _, e := range d.tables[t][h&uint64(len(d.tables[t])-1)] {
			if cellsEqual(e, c) {
				return
			}
		}
	}
	t := 0
	if d.rehashing() {
		t = 1
	}
	i := h & uint64(len(d.tables[t])-1)
	d.tables[t][i] = append([]cell{c}, d.tables[t][i]...)
	d.used[t]++
}

func cellsEqual(a, b cell) bool {
	if a.isArr || b.isArr {
		return a.isArr && b.isArr && a.hash(0) == b.hash(0)
	}
	if a.null || b.null {
		return a.null && b.null
	}
	if a.num != b.num {
		return compareCells(a, b) == 0
	}
	if a.num {
		return a.f == b.f
	}
	return a.v == b.v
}

func (d *rsDict) order() []cell {
	var out []cell
	for t := 0; t <= 1; t++ {
		if t == 1 && !d.rehashing() {
			break
		}
		for _, chain := range d.tables[t] {
			out = append(out, chain...)
		}
	}
	return out
}
