package sparsearray

import (
	"bufio"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// genScripts produces deterministic operation scripts. testdata/*.golden hold
// the output of replaying them through a C harness linked against Redis
// 8.10.3's sparsearray.c; SPARSEARRAY_GEN_OPS=<dir> writes the scripts there.

func runOps(t *testing.T, script string) []string {
	t.Helper()
	var out []string
	var ar *Array
	var kmax, kmin uint32
	sc := bufio.NewScanner(strings.NewReader(script))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var a, b, c uint64
		switch line[0] {
		case 'C':
			fmt.Sscan(line[1:], &a, &b, &c)
			kmax, kmin = uint32(b), uint32(c)
			ar = New(Config{SliceSize: uint32(a), Limits: func() (uint32, uint32) { return kmax, kmin }})
		case 'K':
			fmt.Sscan(line[1:], &a, &b)
			kmax, kmin = uint32(a), uint32(b)
		case 'S':
			fmt.Sscan(line[1:], &a)
			ar.Set(a, "v")
		case 'D':
			fmt.Sscan(line[1:], &a)
			ar.Del(a)
		case 'R':
			fmt.Sscan(line[1:], &a, &b)
			out = append(out, fmt.Sprintf("r %d", ar.DeleteRange(a, b)))
		case 'P':
			fmt.Sscan(line[1:], &a, &b)
			ar.MayPromoteToDenseForRangeSet(a, b)
		case 'T':
			fmt.Sscan(line[1:], &a)
			ar.Truncate(a)
		case 'U':
			ar = ar.Dup()
		case 'I':
			in := ar.Info(true)
			out = append(out, fmt.Sprintf("i %d %d %d %d %d %d %d %d %d %d %d",
				in.Count, in.Len, in.NumSlices, in.DirectorySize, in.SuperDirEntries, in.SliceSize,
				in.DenseSlices, in.SparseSlices, in.DenseTotalWinsize, in.DenseTotalCount, in.SparseTotalCap))
		default:
			t.Fatalf("bad op %q", line)
		}
	}
	return out
}

func TestGoldenAgainstC(t *testing.T) {
	scripts := genScripts()
	if dir := os.Getenv("SPARSEARRAY_GEN_OPS"); dir != "" {
		for name, script := range scripts {
			if err := os.WriteFile(filepath.Join(dir, name+".ops"), []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", name+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			got := runOps(t, scripts[name])
			wantLines := strings.Split(strings.TrimSpace(string(want)), "\n")
			if len(got) != len(wantLines) {
				t.Fatalf("got %d lines, want %d", len(got), len(wantLines))
			}
			for i := range got {
				if got[i] != wantLines[i] {
					t.Fatalf("line %d: got %q, want %q", i, got[i], wantLines[i])
				}
			}
		})
	}
}

func genScripts() map[string]string {
	scripts := map[string]string{}
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	take := func(name string) { scripts[name] = b.String(); b.Reset() }
	r := rand.New(rand.NewPCG(1, 2))

	w("C 4096 10 5")
	for i := uint64(0); i < 20000; i++ {
		w("S %d", i)
		if i%250 == 0 || i < 40 {
			w("I")
		}
	}
	for i := uint64(0); i < 20000; i += 3 {
		w("D %d", i)
		if i%600 == 0 {
			w("I")
		}
	}
	for i := 19999; i > 0; i -= 2 {
		w("D %d", i)
		if i%401 == 0 {
			w("I")
		}
	}
	w("I")
	take("sequential")

	w("C 4096 10 5")
	for i := uint64(30000); i > 0; i-- {
		w("S %d", i-1)
		if i%300 == 0 || i > 29960 {
			w("I")
		}
	}
	take("reverse")

	w("C 4096 10 5")
	for i := 0; i < 6000; i++ {
		idx := r.Uint64N(1 << 20)
		if r.IntN(4) == 0 {
			w("D %d", idx)
		} else {
			w("S %d", idx)
		}
		if i%50 == 0 {
			w("I")
		}
	}
	for i := 0; i < 40; i++ {
		lo := r.Uint64N(1 << 20)
		w("R %d %d", lo, lo+r.Uint64N(40000))
		w("I")
	}
	w("T 300000")
	w("I")
	take("scattered")

	w("C 4096 10 5")
	for blk := uint64(0); blk < 12; blk++ {
		base := blk * 4096
		n := 20 + blk*37
		for i := uint64(0); i < n; i++ {
			w("S %d", base+(i*7919)%4096)
		}
		w("I")
		for i := uint64(0); i < n; i++ {
			if i%(blk+2) != 0 {
				w("D %d", base+(i*7919)%4096)
				w("I")
			}
		}
	}
	for blk := uint64(0); blk < 12; blk++ {
		base := blk * 4096
		for i := uint64(0); i < 200; i++ {
			w("S %d", base+1000+i)
		}
		w("R %d %d", base+1003, base+1195)
		w("I")
	}
	take("demotion")

	w("C 4096 10 5")
	hi := uint64(math.MaxUint64 - 1)
	for i := uint64(0); i < 300; i++ {
		w("S %d", hi-i*5)
		if i%20 == 0 {
			w("I")
		}
	}
	for i := 0; i < 250; i++ {
		w("S %d", r.Uint64N(math.MaxUint64))
		if i%10 == 0 {
			w("I")
		}
	}
	for i := 0; i < 400; i++ {
		w("S %d", r.Uint64N(1<<16))
	}
	w("I")
	for i := 0; i < 400; i++ {
		w("S %d", (uint64(5)<<40)+r.Uint64N(1<<22))
	}
	w("I")
	w("D %d", hi)
	w("I")
	w("R %d %d", uint64(1)<<62, uint64(3)<<62)
	w("I")
	w("R 0 70000")
	w("I")
	w("R %d %d", hi-700, hi-3)
	w("I")
	w("U")
	w("I")
	w("T %d", uint64(5)<<40+(1<<21))
	w("I")
	w("R %d %d", uint64(5)<<40, uint64(math.MaxUint64))
	w("I")
	for i := uint64(0); i < 30; i++ {
		w("S %d", i*4096*100)
	}
	w("I")
	take("huge")

	w("C 256 10 5")
	for i := 0; i < 4000; i++ {
		idx := r.Uint64N(1 << 17)
		switch r.IntN(10) {
		case 0, 1:
			w("D %d", idx)
		case 2:
			w("R %d %d", idx, idx+r.Uint64N(2000))
		default:
			w("S %d", idx)
		}
		if i == 1500 {
			w("K 20 3")
		}
		if i == 2500 {
			w("K 0 0")
		}
		if i == 3200 {
			w("K 6 2")
		}
		if i%40 == 0 {
			w("I")
		}
	}
	take("slice256_config")

	w("C 1024 16 8")
	for i := 0; i < 300; i++ {
		lo := r.Uint64N(1 << 22)
		if r.IntN(8) == 0 {
			lo = r.Uint64N(math.MaxUint64 - 5000)
		}
		n := 1 + r.Uint64N(uint64(1+r.IntN(3000)))
		if n > 1 {
			w("P %d %d", lo, lo+n-1)
		}
		for j := uint64(0); j < n; j++ {
			w("S %d", lo+j)
		}
		w("I")
		if r.IntN(3) == 0 {
			x := r.Uint64N(1 << 22)
			w("R %d %d", x, x+r.Uint64N(5000))
			w("I")
		}
	}
	take("rangeset")

	w("C 4096 10 5")
	for i := 0; i < 20; i++ {
		w("S %d", 100+i)
		w("I")
	}
	for i := 0; i < 20; i++ {
		w("D %d", 100+i*3%20)
		w("I")
	}
	for k := uint64(1); k <= 12; k++ {
		base := k * 8192
		for i := uint64(0); i < k; i++ {
			w("S %d", base+i*3)
		}
		w("I")
		w("P %d %d", base+4000, base+4000+(10-k))
		w("I")
		w("P %d %d", base+4096, base+4096+k+8)
		w("I")
		w("P %d %d", base+4096+500, base+4096+500+k+9)
		w("I")
		w("P %d %d", base+2, base+2+(10-k))
		w("I")
	}
	w("R %d %d", 12*8192, 12*8192+8191)
	w("I")
	w("R %d %d", 11*8192+4096, 11*8192+8191)
	w("I")
	w("R %d %d", 11*8192, 11*8192+4095)
	w("I")
	w("D %d", 100)
	w("D %d", 101)
	w("I")
	for i := uint64(0); i < 64; i++ {
		w("S %d", 900000+i)
	}
	for i := uint64(0); i < 60; i++ {
		w("D %d", 900000+i)
		w("I")
	}
	w("S %d", uint64(1)<<40)
	w("S %d", uint64(1)<<40+5000)
	w("I")
	w("R %d %d", uint64(1)<<40+4096, uint64(1)<<40+8191)
	w("I")
	w("R %d %d", uint64(1)<<40, uint64(1)<<40+10)
	w("I")
	w("K 64 40")
	for i := uint64(0); i < 300; i++ {
		w("S %d", 2000000+i*2)
	}
	w("I")
	for i := uint64(0); i < 300; i++ {
		w("D %d", 2000000+i*2)
		if i > 240 {
			w("I")
		}
	}
	w("C 4096 10 5")
	for k := uint64(0); k < 10; k++ {
		base := (k + 1) * 8192
		for i := uint64(0); i < k; i++ {
			w("S %d", base+i*3)
		}
		w("P %d %d", base+4000, base+4000+(9-k))
		w("I")
		w("S %d", base+4096+500+k)
		w("P %d %d", base+4096+500, base+4096+509)
		w("I")
	}
	w("C 4096 10 5")
	w("S 50000")
	w("D 50000")
	w("S 3")
	w("I")
	w("S 70000")
	w("R 70000 70000")
	w("S 4")
	w("I")
	take("edges")

	w("C 4096 0 0")
	for i := 0; i < 2000; i++ {
		w("S %d", r.Uint64N(1<<25))
		if i%40 == 0 {
			w("I")
		}
	}
	w("K 10 5")
	for i := 0; i < 2000; i++ {
		w("D %d", r.Uint64N(1<<25))
		w("S %d", r.Uint64N(1<<25))
		if i%40 == 0 {
			w("I")
		}
	}
	take("nosparse")
	return scripts
}
