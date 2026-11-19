package vectorset

import (
	"encoding/hex"
	"math"
	"math/rand/v2"
	"sync"
	"testing"
	"time"
)

// hnswVembGolden holds VADD FP32 inputs with the VEMB RAW (vector bytes, l2,
// Q8 range) and VEMB outputs Redis 8.2.11 returned for them.
var hnswVembGolden = []struct {
	quant Quant
	in    []uint32
	raw   string
	l2    uint32
	rng   uint32
	emb   []uint32
}{
	{0, []uint32{0xc0600000}, "000080bf", 0x40600000, 0, []uint32{0xc0600000}},
	{0, []uint32{0xc44189a6, 0x44c7cbc1, 0x430f01bb, 0xc4cf9460, 0xc3a9f0f4, 0xc3ec9b29, 0xc48b6512}, "bb9590be9a42153f90ab553d38131bbfe6e9fdbd7cc230be0146d0be", 0x452b566c, 0, []uint32{0xc44189a6, 0x44c7cbc0, 0x430f01bb, 0xc4cf9461, 0xc3a9f0f4, 0xc3ec9b29, 0xc48b6512}},
	{0, []uint32{0x3f337302, 0x3f10c1b5, 0x3f82a93e, 0xbd0eff47, 0x3f2c49e4, 0xbf15dd10, 0xbfb91c4f, 0x3d5b9328, 0x3fed9ce5}, "5ed07a3e16534a3e949fb63e87dd47bc3ece703e5c7651be055d01bfcf72993ce30d263f", 0x403728e3, 0, []uint32{0x3f337302, 0x3f10c1b5, 0x3f82a93e, 0xbd0eff47, 0x3f2c49e4, 0xbf15dd10, 0xbfb91c50, 0x3d5b9328, 0x3fed9ce5}},
	{0, []uint32{0x3f7c2727, 0xbf8da359, 0x3edcd540, 0x3fd13a19, 0xbeff29b6, 0x3f8c72bb, 0x3cb0dfa3, 0xbf098a93, 0xbf0b3e95, 0x3f5916be, 0x3f03c9a8, 0x3ee65acb, 0xbfb96262}, "16939c3e8ae6afbe5d20093e6deb013f91711ebe3c6cae3ed9a8db3b10d02abe8bed2cbe2dcd863ed8aa233ef5090f3ea33ae6be", 0x404e22b1, 0, []uint32{0x3f7c2727, 0xbf8da359, 0x3edcd540, 0x3fd13a1a, 0xbeff29b7, 0x3f8c72bb, 0x3cb0dfa3, 0xbf098a93, 0xbf0b3e95, 0x3f5916bd, 0x3f03c9a8, 0x3ee65acb, 0xbfb96262}},
	{0, []uint32{0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000}, "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", 0x3f800000, 0, []uint32{0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000}},
	{1, []uint32{0xc0600000}, "81", 0x40600000, 0x3f800000, []uint32{0xc0600000}},
	{1, []uint32{0x42c50206, 0xc4a5e7dc, 0xc3ca1e7c, 0x4312541c, 0xc326d5a0, 0x4484492a, 0x443ea7ca}, "0981d90ef06549", 0x44eff880, 0x3f30fcd9, []uint32{0x42bc1d12, 0xc4a5e7dc, 0xc3cbca28, 0x43124f80, 0xc3273648, 0x4483f0d5, 0x443eb9ec}},
	{1, []uint32{0x3e3a8ff2, 0xbf4bb4dc, 0xbe0f5e30, 0xc021cabb, 0x3fe17575, 0xbfc21abb, 0x3f411e7f, 0xbffc1114, 0xbe698801}, "09d8f98158b4269df5", 0x4083da9a, 0x3f1d101e, []uint32{0x3e3772f9, 0xbf4bd515, 0xbe0eaec1, 0xc021cabb, 0x3fe0372f, 0xbfc1a406, 0x3f41a406, 0xbffc3e15, 0xbe60372f}},
	{1, []uint32{0xbf9696d4, 0x3f648b38, 0x3f763980, 0xbf7ffb79, 0xbf24cca8, 0x3f2a19ea, 0x3f69230a, 0xbfdd65f2, 0xbdfac7cc, 0xbf8258e7, 0x3d7387fa, 0x3fd7b8d1, 0x3efb612f}, "aa4247b7d1314381f7b5047c24", 0x40660b5d, 0x3ef660d8, []uint32{0xbf95ec57, 0x3f661d5d, 0x3f778c30, 0xbf7e8552, 0xbf23de9b, 0x3f2ad7bc, 0x3f6999ec, 0xbfdd65f2, 0xbdfb08c3, 0xbf82bf3a, 0x3d5f243a, 0x3fd82b18, 0x3efb08c3}},
	{1, []uint32{0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000}, "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", 0x3f800000, 0x00000000, []uint32{0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000, 0x00000000}},
	{2, []uint32{0xc0600000}, "0000000000000000", 0x40600000, 0, []uint32{0xbf800000}},
	{2, []uint32{0xc459b636, 0xc4e24e0f, 0xc3a2e166, 0x44d4baee, 0xc2c6611c, 0x44d11cb6, 0xc4bd2683}, "2800000000000000", 0x4559bc55, 0, []uint32{0xbf800000, 0xbf800000, 0xbf800000, 0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000}},
	{2, []uint32{0x3f10c4b5, 0xbf511fec, 0x3fa09822, 0xbfc1dd12, 0x3ed6424f, 0x3fa02cc7, 0x3e8fc36c, 0x3fcd59f1, 0x3f833ca7}, "f501000000000000", 0x404d663c, 0, []uint32{0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000}},
	{2, []uint32{0xbf13810a, 0xbf97d1a9, 0x3f9fef14, 0x3eabf9d9, 0x401e9ce5, 0x3fcc5444, 0x3e2dfddc, 0xbf554ff3, 0x3e712706, 0x3ebaf5dc, 0xbf956d60, 0x3eac269c, 0xbd8e22a0}, "7c0b000000000000", 0x4073ab75, 0, []uint32{0xbf800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000}},
	{2, []uint32{0xc01d8384, 0x3f45c914, 0xbf52ac76, 0x3f8638ff, 0xbe905718, 0x3e566968, 0x3f4ec94e, 0xbe300b26, 0x3fdbd304, 0x3f823cca, 0xbf3198d1, 0x3ec1eb25, 0x400311f3, 0xbebe8843, 0x3f1b4a28, 0x3f947582, 0x3f7a5c39, 0x3ed14f04, 0xbf402502, 0xbf5a26f6, 0x3f899b54, 0xbe10ffed, 0xbfc4c51f, 0x3ed9d8c0, 0x3e6aa672, 0x3d9b55cb, 0x3f3c478e, 0xbf001013, 0x3e7cf1f5, 0x3fb50d76, 0x3f3f570d, 0xbd30aa11, 0x3f648a06, 0x4008b62e, 0x3f7b4647, 0xbec2c5fc, 0xbe1bfd9d, 0xbeeb6beb, 0x3f0e034e, 0xbf6895be, 0x3fe51c3c, 0x3f7e9ab0, 0xbec1429b, 0x3eccfe29, 0xbf2ed7a3, 0xbea9df37, 0xbcd42a97, 0x3fc886fd, 0x3f3fa974, 0x3e6fe057, 0x3f8b4869, 0xbf8cebd3, 0xbe2d03be, 0x3f8dc08b, 0x3fde38c6, 0x3f7014cb, 0xbf3b9624, 0x3fc15eb8, 0xbf25eb4c, 0x3f8ac21c, 0xbf5d8621, 0xbf8bd240, 0xbe096a42, 0x3f65e006, 0xbee935df, 0xbfcced3e, 0x3f0369ff, 0xbb38a23b, 0xbfa75160, 0xbf827bda}, "6adb9377478be78a0400000000000000", 0x4101f79b, 0, []uint32{0xbf800000, 0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0xbf800000, 0x3f800000, 0xbf800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0xbf800000, 0xbf800000, 0x3f800000, 0xbf800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000, 0xbf800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0xbf800000, 0x3f800000, 0x3f800000, 0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000, 0x3f800000, 0xbf800000, 0xbf800000, 0xbf800000, 0x3f800000, 0xbf800000, 0xbf800000, 0x3f800000, 0xbf800000, 0xbf800000, 0xbf800000}},
}

func hnswBits(v []uint32) []float32 {
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = math.Float32frombits(x)
	}
	return out
}

func TestHNSWVembGolden(t *testing.T) {
	for i, g := range hnswVembGolden {
		in := hnswBits(g.in)
		ix := New(uint32(len(in)), g.quant, 0)
		n := ix.Insert(in, nil, 0)
		if got := hex.EncodeToString(n.Raw()); got != g.raw {
			t.Fatalf("case %d: raw %s, want %s", i, got, g.raw)
		}
		if math.Float32bits(n.L2()) != g.l2 {
			t.Fatalf("case %d: l2 %x, want %x", i, math.Float32bits(n.L2()), g.l2)
		}
		if g.quant == QuantQ8 && math.Float32bits(n.QRange()) != g.rng {
			t.Fatalf("case %d: range %x, want %x", i, math.Float32bits(n.QRange()), g.rng)
		}
		for j, x := range ix.NodeVector(n) {
			if math.Float32bits(x) != g.emb[j] {
				t.Fatalf("case %d: emb[%d] %x, want %x", i, j, math.Float32bits(x), g.emb[j])
			}
		}
	}
}

func TestHNSWQuantizeAndNormalize(t *testing.T) {
	q := make([]int8, 5)
	if r := hnswQuantizeQ8([]float32{0.5, -1, 0.25, -0.5, 0}, q); r != 1 {
		t.Fatalf("range %v", r)
	}
	// roundf rounds halves away from zero: 63.5 -> 64, -63.5 -> -64.
	if want := []int8{64, -127, 32, -64, 0}; string(hnswI8Bytes(q)) != string(hnswI8Bytes(want)) {
		t.Fatalf("q8 %v, want %v", q, want)
	}
	if r := hnswQuantizeQ8([]float32{0, 0}, q[:2]); r != 0 || q[0] != 0 || q[1] != 0 {
		t.Fatalf("zero q8 %v %v", r, q)
	}

	b := make([]uint64, 2)
	v := make([]float32, 70)
	v[0], v[3], v[64], v[69], v[5] = 1, 0.1, 2, 3, -1
	hnswQuantizeBin(v, b)
	if b[0] != 0b1001 || b[1] != 1|1<<5 {
		t.Fatalf("bin %b %b", b[0], b[1])
	}

	x := []float32{3, 4}
	l2, ok := hnswNormalize(x)
	if !ok || l2 != 5 || x[0] != 0.6 || x[1] != 0.8 {
		t.Fatalf("normalize %v %v %v", l2, ok, x)
	}
	z := []float32{0, 0, 0}
	if _, ok := hnswNormalize(z); ok {
		t.Fatal("zero vector normalized")
	}

	ix := New(3, QuantQ8, 0)
	n := ix.Insert([]float32{0, 0, 0}, nil, 0)
	if n.L2() != 1 || n.QRange() != 0 {
		t.Fatalf("zero vector l2=%v range=%v", n.L2(), n.QRange())
	}
	m := ix.Insert([]float32{1, 2, 3}, nil, 0)
	if d := ix.Distance(n, m); d != 1 {
		t.Fatalf("distance to zero vector %v", d)
	}
}

func hnswI8Bytes(v []int8) []byte {
	b := make([]byte, len(v))
	for i, x := range v {
		b[i] = byte(x)
	}
	return b
}

func TestHNSWDistances(t *testing.T) {
	f := New(9, QuantNone, 0)
	a := f.Insert([]float32{1, 0, 0, 0, 0, 0, 0, 0, 0}, nil, 0)
	b := f.Insert([]float32{0, 1, 0, 0, 0, 0, 0, 0, 0}, nil, 0)
	c := f.Insert([]float32{-2, 0, 0, 0, 0, 0, 0, 0, 0}, nil, 0)
	if f.Distance(a, a) != 0 || f.Distance(a, b) != 1 || f.Distance(a, c) != 2 {
		t.Fatalf("float distances %v %v %v", f.Distance(a, a), f.Distance(a, b), f.Distance(a, c))
	}
	// Left to right float32 accumulation: (1e8 + 1) - 1e8 is 0, so the
	// dot product is 1 (not 2) and the distance 0 (not -1).
	x := []float32{1e4, 1, 1e4, 0, 0, 0, 0, 0, 1}
	y := []float32{1e4, 1, -1e4, 0, 0, 0, 0, 0, 1}
	if d := hnswDistanceFloat(x, y); d != 0 {
		t.Fatalf("accumulation order: %v", d)
	}

	q := New(3, QuantQ8, 0)
	qa := q.Insert([]float32{1, 2, 3}, nil, 0)
	qb := q.Insert([]float32{-1, -2, -3}, nil, 0)
	if q.Distance(qa, qb) != 2 || q.Distance(qa, qa) > 1e-6 {
		t.Fatalf("q8 distances %v %v", q.Distance(qa, qb), q.Distance(qa, qa))
	}

	bi := New(4, QuantBin, 0)
	ba := bi.Insert([]float32{1, 1, 1, 1}, nil, 0)
	bb := bi.Insert([]float32{1, -1, 1, 1}, nil, 0)
	if bi.Distance(ba, bb) != 0.5 {
		t.Fatalf("bin distance %v", bi.Distance(ba, bb))
	}
	if v := bi.NodeVector(bb); v[0] != 1 || v[1] != -1 {
		t.Fatalf("bin vector %v", v)
	}
}

func TestHNSWPQTies(t *testing.T) {
	n1, n2, n3, n4 := &Node{id: 1}, &Node{id: 2}, &Node{id: 3}, &Node{id: 4}
	pq := hnswPQ{items: make([]hnswPQItem, 3)}
	pq.push(n1, 0.5)
	pq.push(n2, 0.5)
	pq.push(n3, 0.7)
	// Not full: a new equal distance goes nearer to the best end.
	if pq.node(0) != n2 || pq.node(1) != n1 || pq.node(2) != n3 {
		t.Fatalf("not full order %d %d %d", pq.node(0).id, pq.node(1).id, pq.node(2).id)
	}
	// Full: an equal distance goes after the existing equal ones.
	pq.push(n4, 0.5)
	if pq.node(0) != n2 || pq.node(1) != n1 || pq.node(2) != n4 {
		t.Fatalf("full order %d %d %d", pq.node(0).id, pq.node(1).id, pq.node(2).id)
	}
	pq.push(n3, 0.5)
	if pq.node(2) != n4 {
		t.Fatal("equal to the worst must be rejected when full")
	}
	if n, d := pq.pop(); n != n2 || d != 0.5 {
		t.Fatal("pop")
	}
}

func hnswRandVecs(r *rand.Rand, n, dim int) [][]float32 {
	out := make([][]float32, n)
	for i := range out {
		v := make([]float32, dim)
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		out[i] = v
	}
	return out
}

func TestHNSWRecall(t *testing.T) {
	for _, q := range []Quant{QuantNone, QuantQ8, QuantBin} {
		r := rand.New(rand.NewPCG(1, uint64(q)))
		ix := New(32, q, 16)
		ix.SeedLevels(1, 2)
		for i, v := range hnswRandVecs(r, 3000, 32) {
			ix.Insert(v, i, 200)
		}
		hits, total := 0, 0
		for _, qv := range hnswRandVecs(r, 100, 32) {
			got, _ := ix.Search(qv, 100, nil, 0)
			want, wd := ix.GroundTruth(qv, 10, nil)
			// Ties at the boundary make any of the equally distant
			// elements a correct answer.
			limit := wd[len(wd)-1]
			for i := 0; i < 10 && i < len(got); i++ {
				d := ix.Distance(ix.queryNode(qv), got[i])
				if d <= limit {
					hits++
				}
			}
			total += len(want)
		}
		recall := float64(hits) / float64(total)
		min := 0.95
		if q == QuantBin {
			min = 0.85
		}
		if recall < min {
			t.Fatalf("quant %d: recall %.3f", q, recall)
		}
		t.Logf("quant %d recall@10 %.3f", q, recall)
	}
}

func hnswCheckValid(t *testing.T, ix *Index) {
	t.Helper()
	conn, rec := ix.ValidateGraph()
	if !rec {
		t.Fatal("non reciprocal links")
	}
	n := uint64(0)
	maxLevel := uint32(0)
	for x := ix.Head(); x != nil; x = x.Next() {
		n++
		maxLevel = max(maxLevel, x.Level())
		for l := uint32(0); l <= x.Level(); l++ {
			links := x.Links(l)
			seen := map[*Node]bool{}
			for _, nb := range links {
				if nb == x || seen[nb] || nb.Level() < l {
					t.Fatalf("bad link at node %d layer %d", x.ID(), l)
				}
				seen[nb] = true
			}
			if uint32(len(links)) > x.layers[l].maxLinks {
				t.Fatal("too many links")
			}
		}
	}
	if n != ix.Len() {
		t.Fatalf("list has %d nodes, Len %d", n, ix.Len())
	}
	if n > 0 && (ix.EntryPoint() == nil || ix.MaxLevel() != ix.EntryPoint().Level()) {
		t.Fatal("bad entry point")
	}
	if conn*100 < n*99 {
		t.Fatalf("only %d of %d nodes reachable", conn, n)
	}
}

func TestHNSWChurn(t *testing.T) {
	for _, q := range []Quant{QuantNone, QuantQ8, QuantBin} {
		for _, m := range []uint32{4, 16} {
			r := rand.New(rand.NewPCG(uint64(m), uint64(q)))
			ix := New(8, q, m)
			ix.SeedLevels(uint64(m), 9)
			var nodes []*Node
			lastID := uint64(0)
			for op := range 3000 {
				if len(nodes) > 0 && r.IntN(5) < 2 {
					k := r.IntN(len(nodes))
					if op%17 == 0 {
						for i, n := range nodes {
							if n == ix.EntryPoint() {
								k = i
							}
						}
					}
					ix.Delete(nodes[k])
					nodes[k] = nodes[len(nodes)-1]
					nodes = nodes[:len(nodes)-1]
				} else {
					v := hnswRandVecs(r, 1, 8)[0]
					if r.IntN(10) == 0 && len(nodes) > 0 {
						v = ix.NodeVector(nodes[r.IntN(len(nodes))])
					}
					n := ix.Insert(v, op, 50)
					if n.ID() != lastID+1 {
						t.Fatalf("id %d after %d", n.ID(), lastID)
					}
					lastID = n.ID()
					nodes = append(nodes, n)
				}
				if op%500 == 0 {
					hnswCheckValid(t, ix)
				}
			}
			hnswCheckValid(t, ix)
			for len(nodes) > 0 {
				ix.Delete(nodes[len(nodes)-1])
				nodes = nodes[:len(nodes)-1]
				if len(nodes)%200 == 0 {
					hnswCheckValid(t, ix)
				}
			}
			if ix.Len() != 0 || ix.Head() != nil || ix.EntryPoint() != nil {
				t.Fatal("index not empty")
			}
			if res, _ := ix.Search(make([]float32, 8), 5, nil, 0); res != nil {
				t.Fatal("search on empty index")
			}
			if ix.RandomNode(nil) != nil {
				t.Fatal("random node on empty index")
			}
		}
	}
}

func hnswRoundTrip(t *testing.T, ix *Index, stripWorst bool) *Index {
	t.Helper()
	lx := New(ix.Dim(), ix.Quant(), ix.M())
	for n := ix.Head(); n != nil; n = n.Next() {
		vec, params := ix.SerializeNode(n)
		if stripWorst {
			var p []uint64
			p = append(p, params[0], params[1]&0xff)
			idx := 2
			for l := uint32(0); l <= n.Level(); l++ {
				num := int(params[idx])
				p = append(p, params[idx:idx+2+num]...)
				idx += 3 + num
			}
			params = append(p, params[idx])
		}
		if _, err := lx.InsertSerialized(vec, params, n.Value); err != nil {
			t.Fatal(err)
		}
	}
	if err := lx.Deserialize(rand.Uint64(), rand.Uint64()); err != nil {
		t.Fatal(err)
	}
	return lx
}

func TestHNSWSerializationRoundTrip(t *testing.T) {
	for _, q := range []Quant{QuantNone, QuantQ8, QuantBin} {
		r := rand.New(rand.NewPCG(3, uint64(q)))
		ix := New(10, q, 8)
		var nodes []*Node
		for i, v := range hnswRandVecs(r, 600, 10) {
			nodes = append(nodes, ix.Insert(v, i, 100))
		}
		for _, k := range r.Perm(600)[:150] {
			ix.Delete(nodes[k])
		}
		for _, strip := range []bool{false, true} {
			lx := hnswRoundTrip(t, ix, strip)
			hnswCheckValid(t, lx)
			// Like Redis, the reloaded last id is the highest id still present.
			maxID := uint64(0)
			for n := ix.Head(); n != nil; n = n.Next() {
				maxID = max(maxID, n.ID())
			}
			if lx.Len() != ix.Len() || lx.LastID() != maxID || lx.MaxLevel() != ix.MaxLevel() {
				t.Fatalf("len %d/%d lastid %d/%d maxlevel %d/%d", lx.Len(), ix.Len(), lx.LastID(), ix.LastID(), lx.MaxLevel(), ix.MaxLevel())
			}
			byID := map[uint64]*Node{}
			for n := lx.Head(); n != nil; n = n.Next() {
				byID[n.ID()] = n
			}
			for n := ix.Head(); n != nil; n = n.Next() {
				m := byID[n.ID()]
				if m == nil || m.Level() != n.Level() || m.L2() != n.L2() || string(m.Raw()) != string(n.Raw()) || m.Value != n.Value {
					t.Fatalf("node %d differs after reload", n.ID())
				}
				for l := uint32(0); l <= n.Level(); l++ {
					a, b := n.layers[l], m.layers[l]
					if len(a.links) != len(b.links) || a.maxLinks != b.maxLinks {
						t.Fatalf("node %d layer %d links differ", n.ID(), l)
					}
					set := map[uint64]bool{}
					for _, x := range a.links {
						set[x.ID()] = true
					}
					for j, x := range b.links {
						if !set[x.ID()] || (j > 0 && b.links[j-1].ID() >= x.ID()) {
							t.Fatalf("node %d layer %d: links not the same set sorted by id", n.ID(), l)
						}
					}
					if strip {
						var want float32
						for _, x := range b.links {
							want = max(want, lx.Distance(m, x))
						}
						if b.worstDist != want {
							t.Fatalf("worst distance not recomputed: %v vs %v", b.worstDist, want)
						}
					} else if a.worstDist != b.worstDist || a.worstIdx != b.worstIdx {
						t.Fatalf("worst link info not preserved")
					}
				}
			}
			qv := hnswRandVecs(r, 1, 10)[0]
			g1, d1 := ix.GroundTruth(qv, 20, nil)
			g2, d2 := lx.GroundTruth(qv, 20, nil)
			for i := range g1 {
				if d1[i] != d2[i] || (g1[i].ID() != g2[i].ID() && d1[i] != d1[min(i+1, len(d1)-1)] && (i == 0 || d1[i] != d1[i-1])) {
					t.Fatalf("ground truth differs at %d", i)
				}
			}
			if res, _ := lx.Search(qv, 10, nil, 0); len(res) != 10 {
				t.Fatal("search after reload")
			}
			n := lx.Insert(qv, "new", 100)
			if n.ID() != maxID+1 {
				t.Fatalf("id after reload %d", n.ID())
			}
			lx.Delete(lx.EntryPoint())
			hnswCheckValid(t, lx)
		}
	}
}

func TestHNSWDeserializeRejects(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 5))
	ix := New(4, QuantQ8, 4)
	for i, v := range hnswRandVecs(r, 40, 4) {
		ix.Insert(v, i, 50)
	}
	type ser struct {
		vec    []byte
		params []uint64
	}
	var base []ser
	for n := ix.Head(); n != nil; n = n.Next() {
		v, p := ix.SerializeNode(n)
		base = append(base, ser{v, p})
	}
	load := func(nodes []ser) (lx *Index, err error) {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("panic: %v", p)
			}
		}()
		lx = New(4, QuantQ8, 4)
		for _, s := range nodes {
			if _, err := lx.InsertSerialized(s.vec, s.params, nil); err != nil {
				return nil, err
			}
		}
		return lx, lx.Deserialize(11, 22)
	}
	if _, err := load(base); err != nil {
		t.Fatal(err)
	}
	clone := func() []ser {
		out := make([]ser, len(base))
		for i, s := range base {
			out[i] = ser{append([]byte(nil), s.vec...), append([]uint64(nil), s.params...)}
		}
		return out
	}

	// Targeted corruptions that Redis rejects.
	cases := map[string]func(s []ser){
		"short vector":  func(s []ser) { s[0].vec = s[0].vec[:3] },
		"no params":     func(s []ser) { s[0].params = s[0].params[:1] },
		"bad version":   func(s []ser) { s[0].params[1] |= 2 << 24 },
		"level too big": func(s []ser) { s[0].params[1] = s[0].params[1]&^0xff | 16 },
		"truncated":     func(s []ser) { s[0].params = s[0].params[:len(s[0].params)-1] },
		"num > max":     func(s []ser) { s[0].params[3] = s[0].params[2] - 1 },
		"max too big":   func(s []ser) { s[0].params[3] = MaxM*4 + 1 },
		"worst idx":     func(s []ser) { s[0].params[4+s[0].params[2]] |= 0xffff },
		"self link":     func(s []ser) { s[0].params[4] = s[0].params[0] },
		"dangling link": func(s []ser) { s[0].params[4] = 1 << 40 },
		"duplicate link": func(s []ser) {
			s[0].params[5] = s[0].params[4]
		},
		"one-way link": func(s []ser) {
			for i := range s {
				p := s[i].params
				if p[2] > 0 {
					// Drop the first layer-0 link, keeping the layout valid.
					np := append([]uint64{p[0], p[1], p[2] - 1, p[3]}, p[5:]...)
					np[4+p[2]-1] = 0
					s[i].params = np
					return
				}
			}
		},
	}
	for name, corrupt := range cases {
		s := clone()
		corrupt(s)
		if _, err := load(s); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}

	// Random corruptions must never panic, and an accepted graph must stay
	// usable.
	for iter := range 3000 {
		s := clone()
		for range 1 + r.IntN(3) {
			k := r.IntN(len(s))
			p := s[k].params
			if len(p) == 0 {
				continue
			}
			switch r.IntN(6) {
			case 0:
				p[r.IntN(len(p))] ^= 1 << r.IntN(64)
			case 1:
				p[r.IntN(len(p))] = uint64(r.IntN(50))
			case 2:
				s[k].params = p[:r.IntN(len(p))]
			case 3:
				s[k].params = append(p, uint64(r.IntN(50)))
			case 4:
				p[r.IntN(len(p))] = r.Uint64()
			case 5:
				s[k].vec[r.IntN(len(s[k].vec))] ^= byte(r.IntN(256))
			}
		}
		lx, err := load(s)
		if err != nil {
			continue
		}
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("iteration %d: panic using an accepted graph: %v", iter, p)
				}
			}()
			qv := hnswRandVecs(r, 1, 4)[0]
			lx.Search(qv, 5, nil, 0)
			lx.Insert(qv, nil, 20)
			lx.RandomNode(r)
			if ep := lx.EntryPoint(); ep != nil {
				lx.Delete(ep)
			}
			lx.Search(qv, 5, func(any) bool { return true }, 10)
		}()
	}
}

func TestHNSWConcurrentSearch(t *testing.T) {
	r := rand.New(rand.NewPCG(8, 8))
	ix := New(16, QuantQ8, 16)
	for i, v := range hnswRandVecs(r, 2000, 16) {
		ix.Insert(v, i, 100)
	}
	queries := hnswRandVecs(r, 64, 16)
	even := func(v any) bool { return v.(int)%2 == 0 }
	type result struct {
		ids   []uint64
		dists []float32
	}
	run := func(q []float32, i int) result {
		var nodes []*Node
		var dists []float32
		switch i % 3 {
		case 0:
			nodes, dists = ix.Search(q, 20, nil, 0)
		case 1:
			nodes, dists = ix.Search(q, 20, even, 500)
		default:
			nodes, dists = ix.GroundTruth(q, 20, even)
		}
		res := result{dists: dists}
		for _, n := range nodes {
			res.ids = append(res.ids, n.ID())
		}
		return res
	}
	want := make([]result, len(queries))
	for i, q := range queries {
		want[i] = run(q, i)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 16)
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := rand.New(rand.NewPCG(uint64(g), 1))
			for it := range 200 {
				i := (it*7 + g) % len(queries)
				got := run(queries[i], i)
				if len(got.ids) != len(want[i].ids) {
					errs <- "length differs"
					return
				}
				for j := range got.ids {
					if got.ids[j] != want[i].ids[j] || got.dists[j] != want[i].dists[j] {
						errs <- "result differs"
						return
					}
				}
				if n := ix.RandomNode(rr); n == nil {
					errs <- "nil random node"
					return
				} else {
					ix.NodeVector(n)
					ix.Distance(n, ix.EntryPoint())
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func TestHNSWRandomNode(t *testing.T) {
	ix := New(2, QuantNone, 4)
	a := ix.Insert([]float32{1, 0}, "a", 10)
	b := ix.Insert([]float32{0, 1}, "b", 10)
	r := rand.New(rand.NewPCG(1, 1))
	seen := map[*Node]int{}
	for range 200 {
		seen[ix.RandomNode(r)]++
	}
	if seen[a] < 50 || seen[b] < 50 {
		t.Fatalf("random node distribution %v", seen)
	}
	if ix.Head() != b || b.Next() != a || a.Next() != nil {
		t.Fatal("list must be newest first")
	}
	if ix.Insert([]float32{1}, nil, 10) != nil {
		t.Fatal("wrong dimension accepted")
	}
	if New(2, QuantNone, 0).M() != DefaultM || New(2, QuantNone, 1<<20).M() != MaxM {
		t.Fatal("M clamping")
	}
	if New(130, QuantBin, 0).QuantBytes() != 24 || New(3, QuantQ8, 0).QuantBytes() != 3 || New(3, QuantNone, 0).QuantBytes() != 12 {
		t.Fatal("quant bytes")
	}
	if ix.MemUsage() <= 0 {
		t.Fatal("mem usage")
	}
}

var (
	hnswBenchOnce  sync.Once
	hnswBenchIndex *Index
	hnswBenchQs    [][]float32
)

func benchIndex(b *testing.B) (*Index, [][]float32) {
	hnswBenchOnce.Do(func() {
		r := rand.New(rand.NewPCG(1, 1))
		ix := New(128, QuantQ8, 16)
		start := time.Now()
		for i, v := range hnswRandVecs(r, 10000, 128) {
			ix.Insert(v, i, 200)
		}
		b.Logf("built 10000x128 Q8 in %v", time.Since(start))
		hnswBenchIndex, hnswBenchQs = ix, hnswRandVecs(r, 256, 128)
	})
	return hnswBenchIndex, hnswBenchQs
}

func BenchmarkHNSWInsertQ8x128(b *testing.B) {
	ix, vecs := benchIndex(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := ix.Insert(vecs[i%len(vecs)], i, 200)
		b.StopTimer()
		ix.Delete(n)
		b.StartTimer()
	}
}

func BenchmarkHNSWDeleteQ8x128(b *testing.B) {
	ix, vecs := benchIndex(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		n := ix.Insert(vecs[i%len(vecs)], i, 200)
		b.StartTimer()
		ix.Delete(n)
	}
}

func BenchmarkHNSWSearchQ8x128(b *testing.B) {
	ix, qs := benchIndex(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ix.Search(qs[i%len(qs)], 100, nil, 0)
	}
}

func BenchmarkHNSWSearchQ8x128Parallel(b *testing.B) {
	ix, qs := benchIndex(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ix.Search(qs[i%len(qs)], 100, nil, 0)
			i++
		}
	})
}
