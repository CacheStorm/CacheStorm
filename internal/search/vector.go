package search

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/cpu"

	"github.com/cachestorm/cachestorm/internal/store"
)

// VectorSpec is a VECTOR field's definition.
type VectorSpec struct {
	Algorithm string
	Type      string
	Dim       int
	Metric    string
	// Params are the other algorithm parameters, in order.
	Params [][2]string
}

var vectorTypeSizes = map[string]int{"FLOAT32": 4, "FLOAT64": 8, "FLOAT16": 2, "BFLOAT16": 2, "INT8": 1, "UINT8": 1}

var vectorAlgorithmParams = map[string]map[string]bool{
	"FLAT": {"TYPE": true, "DIM": true, "DISTANCE_METRIC": true, "INITIAL_CAP": true, "BLOCK_SIZE": true},
	"HNSW": {"TYPE": true, "DIM": true, "DISTANCE_METRIC": true, "INITIAL_CAP": true, "M": true, "EF_CONSTRUCTION": true, "EF_RUNTIME": true, "EPSILON": true},
}

// parseVectorSpec reads "algorithm count name value ..." of a VECTOR field
// at args[i]; it returns the index after them.
func parseVectorSpec(args []string, i int) (*VectorSpec, int, error) {
	if i >= len(args) {
		return nil, i, errors.New("Bad arguments for vector similarity algorithm: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
	}
	algo := strings.ToUpper(args[i])
	if algo != "FLAT" && algo != "HNSW" && algo != "SVS-VAMANA" {
		return nil, i, errors.New("Bad arguments for vector similarity algorithm: Unknown argument") //nolint:staticcheck // RediSearch's reply text
	}
	if i+1 >= len(args) {
		return nil, i, errors.New("Bad arguments for vector similarity number of parameters: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
	}
	n, err := strconv.Atoi(args[i+1])
	if err != nil || n < 0 {
		return nil, i, errors.New("Bad arguments for vector similarity number of parameters: Could not convert argument to expected type") //nolint:staticcheck // RediSearch's reply text
	}
	if n%2 != 0 {
		return nil, i, fmt.Errorf("Bad number of arguments for vector similarity index: got %d but expected even number as algorithm parameters (should be submitted as named arguments)", n) //nolint:staticcheck // RediSearch's reply text
	}
	if i+2+n > len(args) {
		return nil, i, fmt.Errorf("Expected %d parameters but got %d", n, len(args)-i-2) //nolint:staticcheck // RediSearch's reply text
	}
	spec := &VectorSpec{Algorithm: algo}
	allowed := vectorAlgorithmParams[algo]
	for j := i + 2; j < i+2+n; j += 2 {
		name, value := strings.ToUpper(args[j]), args[j+1]
		bad := func(what string) error {
			return fmt.Errorf("Bad arguments for vector similarity %s index `%s`: %s", algo, name, what) //nolint:staticcheck // RediSearch's reply text
		}
		switch name {
		case "TYPE":
			t := strings.ToUpper(value)
			if _, ok := vectorTypeSizes[t]; !ok {
				return nil, i, bad("Unknown argument")
			}
			spec.Type = t
		case "DIM":
			d, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, i, bad("Could not convert argument to expected type")
			}
			if d <= 0 || d > math.MaxInt32 {
				return nil, i, bad("Value is outside acceptable bounds")
			}
			spec.Dim = int(d)
		case "DISTANCE_METRIC":
			m := strings.ToUpper(value)
			if m != "L2" && m != "IP" && m != "COSINE" {
				return nil, i, bad("Unknown argument")
			}
			spec.Metric = m
		default:
			if allowed != nil && !allowed[name] {
				return nil, i, fmt.Errorf("Bad arguments for algorithm %s: %s", algo, args[j]) //nolint:staticcheck // RediSearch's reply text
			}
			spec.Params = append(spec.Params, [2]string{name, value})
		}
	}
	for _, m := range []struct {
		name string
		ok   bool
	}{{"TYPE", spec.Type != ""}, {"DIM", spec.Dim > 0}, {"DISTANCE_METRIC", spec.Metric != ""}} {
		if !m.ok {
			return nil, i, fmt.Errorf("Missing mandatory parameter: cannot create %s index without specifying %s argument", algo, m.name) //nolint:staticcheck // RediSearch's reply text
		}
	}
	return spec, i + 2 + n, nil
}

// blobSize is the size of a vector blob of the field.
func (v *VectorSpec) blobSize() int { return v.Dim * vectorTypeSizes[v.Type] }

func float16ToFloat32(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32(h>>10) & 0x1f
	frac := uint32(h) & 0x3ff
	switch {
	case exp == 0 && frac == 0:
		return math.Float32frombits(sign)
	case exp == 0:
		for frac&0x400 == 0 {
			frac <<= 1
			exp--
		}
		exp++
		frac &= 0x3ff
	case exp == 0x1f:
		return math.Float32frombits(sign | 0x7f800000 | frac<<13)
	}
	return math.Float32frombits(sign | (exp+112)<<23 | frac<<13)
}

// decode reads a blob into the field's values, as single or double
// precision numbers; ok is false when its size is wrong.
func (v *VectorSpec) decode(b []byte) ([]float64, bool) {
	if len(b) != v.blobSize() {
		return nil, false
	}
	out := make([]float64, v.Dim)
	for i := range out {
		switch v.Type {
		case "FLOAT32":
			out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:])))
		case "FLOAT64":
			out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[8*i:]))
		case "FLOAT16":
			out[i] = float64(float16ToFloat32(binary.LittleEndian.Uint16(b[2*i:])))
		case "BFLOAT16":
			out[i] = float64(math.Float32frombits(uint32(binary.LittleEndian.Uint16(b[2*i:])) << 16))
		case "INT8":
			out[i] = float64(int8(b[i]))
		case "UINT8":
			out[i] = float64(b[i])
		}
	}
	if v.Metric == "COSINE" {
		v.normalize(out)
	}
	return out, true
}

// jsonVector reads a JSON array of numbers as a vector.
func (v *VectorSpec) jsonVector(n *store.JNode) ([]float64, bool) {
	if n.Kind != store.JArray || len(n.Arr) != v.Dim {
		return nil, false
	}
	out := make([]float64, v.Dim)
	for i, e := range n.Arr {
		f, err := strconv.ParseFloat(string(store.AppendJSON(nil, e)), 64)
		if err != nil {
			return nil, false
		}
		if v.Type != "FLOAT64" {
			f = float64(float32(f))
		}
		out[i] = f
	}
	if v.Metric == "COSINE" {
		v.normalize(out)
	}
	return out, true
}

// normalize scales a vector to unit length as the vector library stores
// vectors for cosine distance: the squared length summed in double
// precision, the division in the field's precision.
func (v *VectorSpec) normalize(x []float64) {
	var sum float64
	for _, f := range x {
		sum += f * f
	}
	if v.Type == "FLOAT64" {
		norm := math.Sqrt(sum)
		for i := range x {
			x[i] /= norm
		}
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range x {
		x[i] = float64(float32(x[i]) / norm)
	}
}

// The vector kernels the vector library picks for this CPU.
var (
	vecAVX512 = cpu.X86.HasAVX512F
	vecAVX    = cpu.X86.HasAVX
)

// distance is the field's metric between two of its vectors, computed as
// RediSearch's vector library computes it on this CPU: squared L2, or one
// minus the inner product (of unit vectors, for cosine).
func (v *VectorSpec) distance(a, b []float64) float64 {
	l2 := v.Metric == "L2"
	var sum float64
	if v.Type == "FLOAT64" {
		sum = kernel64(a, b, l2)
	} else {
		sum = float64(kernel32(a, b, l2))
	}
	if l2 {
		return sum
	}
	if v.Type == "FLOAT64" {
		return 1 - sum
	}
	return float64(1 - float32(sum))
}

func term32(a, b float64, l2 bool) float32 {
	if l2 {
		d := float32(a) - float32(b)
		return d * d
	}
	return float32(a) * float32(b)
}

func fma32(x, y, z float32) float32 { return float32(math.FMA(float64(x), float64(y), float64(z))) }

// kernel32 sums a single-precision inner product or squared distance: in
// order below 16 dimensions, else in 8 AVX or 16 AVX-512 lanes the way the
// library's SIMD kernels do.
func kernel32(a, b []float64, l2 bool) float32 {
	dim := len(a)
	if dim < 16 || !vecAVX && !vecAVX512 {
		var sum float32
		for i := range a {
			sum += term32(a[i], b[i], l2)
		}
		return sum
	}
	if vecAVX512 {
		var lanes [16]float32
		r := dim % 16
		for i := range r {
			lanes[i] = term32(a[i], b[i], l2)
		}
		for i := r; i < dim; i++ {
			x, y := float32(a[i]), float32(b[i])
			if l2 {
				x -= y
				y = x
			}
			lanes[(i-r)%16] = fma32(x, y, lanes[(i-r)%16])
		}
		var t [8]float32
		for i := range t {
			t[i] = lanes[i+8] + lanes[i]
		}
		var u [4]float32
		for i := range u {
			u[i] = t[i+4] + t[i]
		}
		return (u[0] + u[2]) + (u[1] + u[3])
	}
	var lanes [8]float32
	r := dim % 16 % 8
	for i := range r {
		lanes[i] = term32(a[i], b[i], l2)
	}
	for i := r; i < dim; i++ {
		lanes[(i-r)%8] += term32(a[i], b[i], l2)
	}
	sum := lanes[0]
	for _, l := range lanes[1:] {
		sum += l
	}
	return sum
}

func term64(a, b float64, l2 bool) float64 {
	if l2 {
		d := a - b
		return d * d
	}
	return a * b
}

// kernel64 is kernel32's double-precision counterpart: in order below 8
// dimensions, else in 4 AVX or 8 AVX-512 lanes.
func kernel64(a, b []float64, l2 bool) float64 {
	dim := len(a)
	if dim < 8 || !vecAVX && !vecAVX512 {
		var sum float64
		for i := range a {
			sum += term64(a[i], b[i], l2)
		}
		return sum
	}
	if vecAVX512 {
		var lanes [8]float64
		r := dim % 8
		for i := range r {
			lanes[i] = term64(a[i], b[i], l2)
		}
		for i := r; i < dim; i++ {
			x, y := a[i], b[i]
			if l2 {
				x -= y
				y = x
			}
			lanes[(i-r)%8] = math.FMA(x, y, lanes[(i-r)%8])
		}
		var t [4]float64
		for i := range t {
			t[i] = lanes[i+4] + lanes[i]
		}
		return (t[0] + t[2]) + (t[1] + t[3])
	}
	var lanes [4]float64
	r := dim % 8 % 4
	for i := range r {
		lanes[i] = term64(a[i], b[i], l2)
	}
	for i := r; i < dim; i++ {
		lanes[(i-r)%4] += term64(a[i], b[i], l2)
	}
	return ((lanes[0] + lanes[1]) + lanes[2]) + lanes[3]
}

// FormatDistance prints a distance as RediSearch replies with it.
func FormatDistance(d float64) string {
	if math.IsNaN(d) {
		if math.Signbit(d) {
			return "-nan"
		}
		return "nan"
	}
	return FormatNumber(d)
}

//nolint:staticcheck // RediSearch's reply texts
var errVectorOption = errors.New("Error parsing vector similarity parameters: Invalid option")

// vectorQuery checks a vector node against its field and returns the query
// vector.
func (idx *Index) vectorQuery(n *qnode) (*VectorSpec, []float64, error) {
	f := idx.Field(n.field)
	if f == nil || f.Vector == nil {
		return nil, nil, errors.New("Error parsing vector similarity query: field is not a vector field") //nolint:staticcheck // RediSearch's reply text
	}
	spec := f.Vector
	if err := checkVectorAttrs(spec, n); err != nil {
		return nil, nil, err
	}
	if !n.knn && n.radius < 0 {
		return nil, nil, errors.New("Error parsing vector similarity query: negative radius") //nolint:staticcheck // RediSearch's reply text
	}
	q, ok := spec.decode([]byte(n.blob))
	if !ok {
		return nil, nil, fmt.Errorf("Error parsing vector similarity query: query vector blob size (%d) does not match index's expected size (%d).", len(n.blob), spec.blobSize()) //nolint:staticcheck // RediSearch's reply text
	}
	return spec, q, nil
}

// checkVectorAttrs checks a vector query's options against its field's
// algorithm.
func checkVectorAttrs(spec *VectorSpec, n *qnode) error {
	for _, a := range n.vecAttrs {
		name := strings.ToUpper(a[0])
		switch {
		case name == "EF_RUNTIME" && spec.Algorithm == "HNSW" && n.knn:
		case name == "EPSILON" && spec.Algorithm == "HNSW" && !n.knn:
		case name == "BATCH_SIZE" || name == "HYBRID_POLICY" || name == "SHARD_K_RATIO":
		default:
			return errVectorOption
		}
	}
	return nil
}

// checkVectors validates the vector nodes of a query and fills in their
// query vectors.
func (idx *Index) checkVectors(n *qnode, out map[*qnode][]float64) error {
	if n == nil {
		return nil
	}
	if n.typ == qVector {
		_, q, err := idx.vectorQuery(n)
		if err != nil {
			return err
		}
		out[n] = q
	}
	for _, c := range n.children {
		if err := idx.checkVectors(c, out); err != nil {
			return err
		}
	}
	return nil
}

// knn picks the k documents among hits whose field vectors are nearest to
// q, nearest first; each gets its distance as the node's yield.
func (idx *Index) knn(n *qnode, q []float64, hits []Hit) []Hit {
	f := idx.Field(n.field)
	var out []Hit
	for _, h := range hits {
		vec := h.Doc.fields[f.pos].vec
		if vec == nil {
			continue
		}
		h.Distance = f.Vector.distance(vec, q)
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Distance, out[j].Distance
		if a != b && !(math.IsNaN(a) && math.IsNaN(b)) {
			return a < b || math.IsNaN(b)
		}
		return out[i].Doc.id < out[j].Doc.id
	})
	if len(out) > n.k {
		out = out[:n.k]
	}
	for i := range out {
		out[i].Yields = append(out[i].Yields, [2]string{n.yield, FormatDistance(out[i].Distance)})
	}
	return out
}

// rangeYields adds the distances that vector ranges in n yield for doc.
func (idx *Index) rangeYields(n *qnode, doc *Document, qs map[*qnode][]float64, out [][2]string) [][2]string {
	if n == nil || n.typ == qNot {
		return out
	}
	if n.typ == qVector && !n.knn && n.yield != "" {
		f := idx.Field(n.field)
		if vec := doc.fields[f.pos].vec; vec != nil {
			if d := f.Vector.distance(vec, qs[n]); d <= n.radius {
				out = append(out, [2]string{n.yield, FormatDistance(d)})
			}
		}
	}
	for _, c := range n.children {
		out = idx.rangeYields(c, doc, qs, out)
	}
	return out
}

// SetVectorKernels makes distances sum as the library's kernels for a CPU
// with the given features do, and returns a function that restores the
// detected ones; tests use it to reproduce distances recorded elsewhere.
func SetVectorKernels(avx, avx512 bool) (restore func()) {
	oldAVX, oldAVX512 := vecAVX, vecAVX512
	vecAVX, vecAVX512 = avx, avx512
	return func() { vecAVX, vecAVX512 = oldAVX, oldAVX512 }
}
