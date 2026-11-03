package store

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func listStrings(v *ListValue) []string {
	out := make([]string, v.Len())
	for i, e := range v.Elements() {
		out[i] = string(e)
	}
	return out
}

func TestListPushFrontPopFrontOrder(t *testing.T) {
	v := &ListValue{}
	v.PushFront([]byte("c"))
	v.PushFront([]byte("a"), []byte("b"))
	v.AppendLoaded([]byte("d"))
	if got := fmt.Sprint(listStrings(v)); got != "[a b c d]" {
		t.Fatalf("after pushes: %s", got)
	}
	if got := v.PopFront(1); string(got[0]) != "a" {
		t.Fatalf("PopFront: %q", got)
	}
	v.PushFront([]byte("z"))
	if got := fmt.Sprint(listStrings(v)); got != "[z b c d]" {
		t.Fatalf("after pop and push: %s", got)
	}
	if got := v.PopBack(2); string(got[0]) != "d" || string(got[1]) != "c" {
		t.Fatalf("PopBack: %q", got)
	}
	v.ReplaceElements([][]byte{[]byte("x"), []byte("y")})
	v.PushFront([]byte("w"))
	if got := fmt.Sprint(listStrings(v)); got != "[w x y]" {
		t.Fatalf("after reassignment: %s", got)
	}
}

// LPUSH used to copy the whole list on every call.
func TestListPushFrontIsAmortisedConstant(t *testing.T) {
	v := &ListValue{}
	start := time.Now()
	for i := 0; i < 200_000; i++ {
		v.PushFront([]byte("x"))
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("200k PushFront took %v", took)
	}
	if v.Len() != 200_000 {
		t.Fatalf("len %d", v.Len())
	}
}

// The chunked list behaves as a slice of elements under every change, and
// an element handed out earlier keeps its bytes whatever happens after.
func TestListChunksMatchSliceModel(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		rng := rand.New(rand.NewSource(seed))
		v := &ListValue{}
		var model [][]byte
		type held struct{ got, want []byte }
		var handed []held
		hand := func(el []byte) {
			if len(handed) < 2000 {
				handed = append(handed, held{el, append([]byte(nil), el...)})
			}
		}
		elem := func() []byte {
			n := rng.Intn(40)
			switch rng.Intn(20) {
			case 0:
				n = 200 + rng.Intn(300)
			case 1:
				n = listChunkBytes + rng.Intn(100)
			}
			b := make([]byte, n)
			rng.Read(b)
			return b
		}
		for step := 0; step < 3000; step++ {
			switch op := rng.Intn(12); {
			case op < 3:
				el := elem()
				v.PushBack(el)
				model = append(model, el)
			case op < 6:
				el := elem()
				v.PushFront(el)
				model = append([][]byte{el}, model...)
			case op == 6 && len(model) > 0:
				n := 1 + rng.Intn(3)
				for _, el := range v.PopFront(n) {
					if !bytes.Equal(el, model[0]) {
						t.Fatalf("seed %d step %d: PopFront mismatch", seed, step)
					}
					hand(el)
					model = model[1:]
				}
			case op == 7 && len(model) > 0:
				for _, el := range v.PopBack(1 + rng.Intn(3)) {
					if !bytes.Equal(el, model[len(model)-1]) {
						t.Fatalf("seed %d step %d: PopBack mismatch", seed, step)
					}
					hand(el)
					model = model[:len(model)-1]
				}
			case op == 8 && len(model) > 0:
				i := rng.Intn(len(model))
				hand(v.At(i))
				el := elem()
				v.SetAt(i, el)
				model[i] = el
			case op == 9 && len(model) > 2:
				start := rng.Intn(len(model) / 2)
				stop := start + rng.Intn(len(model)-start)
				v.Trim(start, stop)
				model = append([][]byte(nil), model[start:stop+1]...)
			case op == 10 && len(model) > 0:
				i := rng.Intn(len(model))
				el := elem()
				after := rng.Intn(2) == 0
				v.InsertAt(i, el, after)
				at := i
				if after {
					at++
				}
				model = append(model[:at], append([][]byte{el}, model[at:]...)...)
			case op == 11 && len(model) > 0:
				var idx []int
				var kept [][]byte
				for i, el := range model {
					if rng.Intn(4) == 0 {
						idx = append(idx, i)
						continue
					}
					kept = append(kept, el)
				}
				v.DeleteAt(idx)
				model = kept
			}
			if v.Len() != len(model) {
				t.Fatalf("seed %d step %d: len %d, want %d", seed, step, v.Len(), len(model))
			}
			if len(model) > 0 {
				i := rng.Intn(len(model))
				if el := v.At(i); !bytes.Equal(el, model[i]) {
					t.Fatalf("seed %d step %d: At(%d) mismatch", seed, step, i)
				}
				hand(v.At(i))
				from, reverse := rng.Intn(len(model)), rng.Intn(2) == 0
				want := from
				v.Range(from, reverse, func(i int, el []byte) bool {
					if i != want || !bytes.Equal(el, model[i]) {
						t.Fatalf("seed %d step %d: Range(%d, %v) at %d", seed, step, from, reverse, i)
					}
					if reverse {
						want--
					} else {
						want++
					}
					return true
				})
				if (reverse && want != -1) || (!reverse && want != len(model)) {
					t.Fatalf("seed %d step %d: Range stopped at %d", seed, step, want)
				}
			}
			if got := v.SizeOf(); got != listBaseSize+sumListElems(model) {
				t.Fatalf("seed %d step %d: SizeOf %d, want %d", seed, step, got, listBaseSize+sumListElems(model))
			}
		}
		for i, h := range handed {
			if !bytes.Equal(h.got, h.want) {
				t.Fatalf("seed %d: element handed out (%d) changed", seed, i)
			}
		}
	}
}
