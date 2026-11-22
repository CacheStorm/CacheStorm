package sparsearray

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

type model map[uint64]string

func (m model) sortedKeys() []uint64 {
	keys := make([]uint64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (m model) length() uint64 {
	var n uint64
	for k := range m {
		if k+1 > n {
			n = k + 1
		}
	}
	return n
}

func checkAccounting(t *testing.T, a *Array) {
	t.Helper()
	var sb, vb int64
	var n uint64
	each := func(s *slice) {
		if s == nil {
			return
		}
		sb += s.footprint()
		vb += s.valueBytes()
		n++
	}
	if a.superdir {
		for i := range a.sdir {
			var c uint32
			for _, s := range a.sdir[i].slots {
				each(s)
				if s != nil {
					c++
				}
			}
			if c != a.sdir[i].count || c == 0 {
				t.Fatalf("block %d count %d, actual %d", a.sdir[i].blockID, a.sdir[i].count, c)
			}
			if i > 0 && a.sdir[i-1].blockID >= a.sdir[i].blockID {
				t.Fatal("superdir not sorted")
			}
		}
	} else {
		for _, s := range a.dir {
			each(s)
		}
	}
	if sb != a.sliceBytes || vb != a.valBytes {
		t.Fatalf("accounting drift: slices %d/%d values %d/%d", a.sliceBytes, sb, a.valBytes, vb)
	}
	if n != a.numSlices {
		t.Fatalf("numSlices %d, actual %d", a.numSlices, n)
	}
}

func checkScan(t *testing.T, a *Array, m model, keys []uint64, start, end uint64, limit int) {
	t.Helper()
	lo, hi := min(start, end), max(start, end)
	var want []uint64
	for _, k := range keys {
		if k >= lo && k <= hi {
			want = append(want, k)
		}
	}
	if start > end {
		slices.Reverse(want)
	}
	if limit > 0 && len(want) > limit {
		want = want[:limit]
	}
	var got []uint64
	a.Scan(start, end, func(idx uint64, v string) bool {
		if mv, ok := m[idx]; !ok || mv != v {
			t.Fatalf("scan yielded %d=%q, model has %q (%v)", idx, v, mv, ok)
		}
		got = append(got, idx)
		return limit <= 0 || len(got) < limit
	})
	if !slices.Equal(got, want) {
		t.Fatalf("scan %d..%d limit %d: got %d items %v, want %d items %v", start, end, limit, len(got), head(got), len(want), head(want))
	}
}

func head(s []uint64) []uint64 {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func checkAll(t *testing.T, a *Array, m model, r *rand.Rand, pick func() uint64) {
	t.Helper()
	if a.Count() != uint64(len(m)) {
		t.Fatalf("count %d, model %d", a.Count(), len(m))
	}
	if a.Len() != m.length() {
		t.Fatalf("len %d, model %d", a.Len(), m.length())
	}
	for k, v := range m {
		got, ok := a.Get(k)
		if !ok || got != v {
			t.Fatalf("get %d = %q,%v want %q", k, got, ok, v)
		}
	}
	for range 50 {
		k := pick()
		got, ok := a.Get(k)
		mv, mok := m[k]
		if ok != mok || got != mv {
			t.Fatalf("get %d = %q,%v want %q,%v", k, got, ok, mv, mok)
		}
	}
	keys := m.sortedKeys()
	checkScan(t, a, m, keys, 0, math.MaxUint64, 0)
	checkScan(t, a, m, keys, math.MaxUint64, 0, 0)
	for range 6 {
		s, e := pick(), pick()
		checkScan(t, a, m, keys, s, e, r.IntN(4)*r.IntN(30))
	}
	checkAccounting(t, a)
}

func randValue(r *rand.Rand) string {
	n := r.IntN(4)
	switch n {
	case 0:
		return ""
	case 1:
		return string([]byte{byte(r.IntN(256)), 0, byte(r.IntN(256))})
	}
	b := make([]byte, r.IntN(40))
	for i := range b {
		b[i] = byte(r.IntN(256))
	}
	return string(b)
}

func TestModelRandom(t *testing.T) {
	type cfgCase struct {
		name       string
		sliceSize  uint32
		kmax, kmin uint32
		changes    bool
	}
	cases := []cfgCase{
		{"default", 4096, 10, 5, false},
		{"slice256", 256, 10, 5, false},
		{"nosparse", 1024, 0, 0, false},
		{"k16", 512, 16, 8, false},
		{"configchanges", 256, 10, 5, true},
	}
	for ci, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := rand.New(rand.NewPCG(uint64(ci)+7, 99))
			kmax, kmin := c.kmax, c.kmin
			a := New(Config{SliceSize: c.sliceSize, Limits: func() (uint32, uint32) { return kmax, kmin }})
			m := model{}
			anchors := []uint64{0, 1 << 20, 1 << 33, math.MaxUint64 - 1, math.MaxUint64 / 3}
			pick := func() uint64 {
				switch r.IntN(6) {
				case 0:
					return r.Uint64N(1 << 12)
				case 1:
					return r.Uint64N(1 << 16)
				case 2:
					return r.Uint64N(1 << 24)
				case 3:
					a := anchors[r.IntN(len(anchors))]
					d := r.Uint64N(1 << 14)
					if a > math.MaxUint64-1-d {
						return a - d
					}
					return a + d
				case 4:
					return math.MaxUint64 - 1 - r.Uint64N(1<<13)
				default:
					if r.IntN(20) == 0 {
						return r.Uint64N(math.MaxUint64)
					}
					return r.Uint64N(1 << 14)
				}
			}
			for step := 0; step < 25000; step++ {
				if len(m) > 12000 {
					a.DeleteRange(0, math.MaxUint64)
					clear(m)
				}
				switch op := r.IntN(100); {
				case op < 50:
					k, v := pick(), randValue(r)
					a.Set(k, v)
					m[k] = v
				case op < 75:
					k := pick()
					_, had := m[k]
					if a.Del(k) != had {
						t.Fatalf("del %d returned %v", k, !had)
					}
					delete(m, k)
				case op < 80:
					lo := pick()
					hi := lo + r.Uint64N(1<<uint(r.IntN(30)))
					if hi < lo {
						hi = math.MaxUint64
					}
					if r.IntN(10) == 0 {
						lo, hi = hi, lo
					}
					var n uint64
					for k := range m {
						if k >= lo && k <= hi {
							delete(m, k)
							n++
						}
					}
					if got := a.DeleteRange(lo, hi); got != n {
						t.Fatalf("deleteRange %d..%d = %d, want %d", lo, hi, got, n)
					}
				case op < 82:
					limit := m.length()
					if r.IntN(2) == 0 {
						limit -= min(limit, r.Uint64N(1<<16))
					} else if limit < math.MaxUint64 {
						limit = r.Uint64N(limit + 1)
					}
					a.Truncate(limit)
					for k := range m {
						if k >= limit {
							delete(m, k)
						}
					}
				case op < 90:
					lo := pick()
					n := 1 + r.Uint64N(uint64(1+r.IntN(1200)))
					if lo > math.MaxUint64-1-n {
						lo = math.MaxUint64 - 1 - n
					}
					if n > 1 {
						a.MayPromoteToDenseForRangeSet(lo, lo+n-1)
					}
					for i := range n {
						v := randValue(r)
						a.Set(lo+i, v)
						m[lo+i] = v
					}
				case op < 91:
					d := a.Dup()
					if d.InsertIdx != a.InsertIdx {
						t.Fatal("dup lost InsertIdx")
					}
					if d.Info(true) != a.Info(true) {
						t.Fatalf("dup info %+v != %+v", d.Info(true), a.Info(true))
					}
					a.Set(pick(), "mutated-original")
					a = d
				case op < 92 && c.changes:
					switch r.IntN(4) {
					case 0:
						kmax, kmin = 0, 0
					case 1:
						kmax, kmin = 10, 5
					case 2:
						kmax, kmin = 3, 1
					default:
						kmax, kmin = 40, 30
					}
				default:
					k := pick()
					got, ok := a.Get(k)
					mv, mok := m[k]
					if ok != mok || got != mv {
						t.Fatalf("get %d = %q,%v want %q,%v", k, got, ok, mv, mok)
					}
				}
				if step%997 == 0 {
					checkAll(t, a, m, r, pick)
				}
			}
			checkAll(t, a, m, r, pick)
			a.DeleteRange(0, math.MaxUint64)
			if a.Count() != 0 || a.Len() != 0 || a.Info(false).NumSlices != 0 {
				t.Fatalf("not empty after full delete: %+v", a.Info(true))
			}
			checkAccounting(t, a)
			if a.valBytes != 0 || a.sliceBytes != 0 {
				t.Fatalf("leftover bytes %d %d", a.valBytes, a.sliceBytes)
			}
		})
	}
}

func TestPromotionAndDemotion(t *testing.T) {
	a := New(DefaultConfig())
	for i := range uint64(10) {
		a.Set(i*2, "x")
	}
	in := a.Info(true)
	if in.SparseSlices != 1 || in.DenseSlices != 0 || in.SparseTotalCap != 10 {
		t.Fatalf("after 10 sets: %+v", in)
	}
	a.Set(100, "x")
	in = a.Info(true)
	if in.SparseSlices != 0 || in.DenseSlices != 1 || in.DenseTotalWinsize != 128 || in.DenseTotalCount != 11 {
		t.Fatalf("after promotion: %+v", in)
	}
	for i := range uint64(6) {
		a.Del(i * 2)
	}
	in = a.Info(true)
	if in.SparseSlices != 1 || in.SparseTotalCap != 5 || in.Count != 5 {
		t.Fatalf("after demotion: %+v", in)
	}
	for i := uint64(6); i < 10; i++ {
		if v, ok := a.Get(i * 2); !ok || v != "x" {
			t.Fatalf("lost element %d after demotion", i*2)
		}
	}
	if a.Len() != 101 {
		t.Fatalf("len %d", a.Len())
	}
}

func TestSmallDenseNeverDemotes(t *testing.T) {
	a := New(DefaultConfig())
	for i := range uint64(11) {
		a.Set(i%8, "v")
	}
	a.MayPromoteToDenseForRangeSet(0, 20)
	for i := range uint64(8) {
		a.Set(i, "v")
	}
	for i := range uint64(7) {
		a.Del(i)
	}
	in := a.Info(true)
	if in.DenseSlices != 1 || in.DenseTotalWinsize != 8 {
		t.Fatalf("winsize-8 dense slice changed: %+v", in)
	}
}

func TestSuperDirTransition(t *testing.T) {
	a := New(Config{SliceSize: 256, KMax: 10, KMin: 5})
	a.Set(5, "a")
	a.Set(2047*256, "b")
	in := a.Info(false)
	if in.DirectorySize != 2048 || in.SuperDirEntries != 0 {
		t.Fatalf("flat: %+v", in)
	}
	a.Set(2048*256, "c")
	in = a.Info(false)
	if in.DirectorySize != 4 || in.SuperDirEntries != 2 || in.NumSlices != 3 {
		t.Fatalf("superdir: %+v", in)
	}
	for i := range uint64(5) {
		a.Set(math.MaxUint64-1-i<<30, "z")
	}
	in = a.Info(false)
	if in.DirectorySize != 8 || in.SuperDirEntries != 7 {
		t.Fatalf("superdir growth: %+v", in)
	}
	if a.Len() != math.MaxUint64 {
		t.Fatalf("len %d", a.Len())
	}
	var got []uint64
	a.Scan(math.MaxUint64, 0, func(idx uint64, _ string) bool {
		got = append(got, idx)
		return len(got) < 3
	})
	if !slices.Equal(got, []uint64{math.MaxUint64 - 1, math.MaxUint64 - 1 - 1<<30, math.MaxUint64 - 1 - 2<<30}) {
		t.Fatalf("reverse scan %v", got)
	}
	if n := a.DeleteRange(1000, math.MaxUint64); n != 7 {
		t.Fatalf("deleted %d", n)
	}
	in = a.Info(false)
	if in.DirectorySize != 8 || in.SuperDirEntries != 1 || in.Len != 6 || in.Count != 1 {
		t.Fatalf("after range delete: %+v", in)
	}
}

func TestFlatDirectoryShrink(t *testing.T) {
	a := New(DefaultConfig())
	a.Set(0, "a")
	a.Set(1000*4096, "b")
	if a.Info(false).DirectorySize != 1024 {
		t.Fatalf("%+v", a.Info(false))
	}
	a.Del(1000 * 4096)
	if in := a.Info(false); in.DirectorySize != 1 || in.Len != 1 {
		t.Fatalf("%+v", in)
	}
}

func TestByteExactValuesAndDup(t *testing.T) {
	a := New(DefaultConfig())
	vals := []string{"", "\x00", "\xff\xfe", "12345678901234567890", "1.0", "-0", "héllo"}
	for i, v := range vals {
		a.Set(uint64(i)*3, v)
	}
	a.InsertIdx = 42
	d := a.Dup()
	a.Set(0, "changed")
	a.Del(3)
	for i, v := range vals {
		got, ok := d.Get(uint64(i) * 3)
		if !ok || got != v {
			t.Fatalf("dup %d: %q,%v want %q", i, got, ok, v)
		}
	}
	if d.InsertIdx != 42 || d.KMax != a.KMax {
		t.Fatal("dup lost fields")
	}
	if v, ok := a.Get(0); !ok || v != "changed" {
		t.Fatal("original not updated")
	}
	if _, ok := d.Get(1); ok {
		t.Fatal("unexpected element")
	}
	if New(DefaultConfig()).InsertIdx != InsertIdxNone {
		t.Fatal("InsertIdx default")
	}
}

func TestSizeOf(t *testing.T) {
	a := New(DefaultConfig())
	base := a.SizeOf()
	for i := range uint64(5000) {
		a.Set(i, "0123456789")
	}
	if a.SizeOf() < base+5000*10 {
		t.Fatalf("size %d too small", a.SizeOf())
	}
	a.DeleteRange(0, 4999)
	a.Set(7, "x")
	a.Del(7)
	if a.sliceBytes != 0 || a.valBytes != 0 {
		t.Fatalf("leftover %d %d", a.sliceBytes, a.valBytes)
	}
}

func TestEmptyAndBounds(t *testing.T) {
	a := New(DefaultConfig())
	if a.Len() != 0 || a.Count() != 0 || a.Del(5) || a.DeleteRange(0, math.MaxUint64) != 0 {
		t.Fatal("empty array misbehaves")
	}
	a.Scan(0, math.MaxUint64, func(uint64, string) bool { t.Fatal("scan on empty"); return false })
	a.Truncate(0)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for UINT64_MAX")
		}
	}()
	a.Set(math.MaxUint64, "x")
}

func TestScanHugeSpanIsCheap(t *testing.T) {
	a := New(DefaultConfig())
	for i := range uint64(64) {
		a.Set(i<<56|i, "v")
	}
	n := 0
	a.Scan(0, math.MaxUint64-1, func(uint64, string) bool { n++; return true })
	if n != 64 {
		t.Fatalf("scanned %d", n)
	}
	if got := a.DeleteRange(1, math.MaxUint64-1); got != 63 {
		t.Fatalf("deleted %d", got)
	}
}
