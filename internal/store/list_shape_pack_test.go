package store

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// Folding a listpack's shape into the list must not change any encoding
// decision: random writes give the same OBJECT ENCODING after every step
// with and without the folding.
func TestListShapePackingKeepsEncoding(t *testing.T) {
	defer func() { packListShapes = true }()
	seen := map[string]bool{}
	for _, fill := range []int64{-2, -1, 4, 128} {
		SetListMaxListpackSize(fill)
		for seed := int64(1); seed <= 20; seed++ {
			var lists [2]*ListValue
			var encs [2][]string
			for k := range lists {
				packListShapes = k == 0
				rng := rand.New(rand.NewSource(seed))
				v := &ListValue{}
				lists[k] = v
				for step := 0; step < 400; step++ {
					el := []byte(strconv.Itoa(rng.Intn(1000)))
					if rng.Intn(4) == 0 {
						el = []byte(strings.Repeat("x", rng.Intn(3000)))
					}
					switch op := rng.Intn(9); {
					case op < 3:
						v.PushBack(el, el)
					case op < 4:
						v.PushFront(el)
					case op < 5:
						v.PopFront(rng.Intn(3) + 1)
					case op < 6:
						v.PopBack(rng.Intn(3) + 1)
					case op < 7 && v.Len() > 0:
						v.TryGrow(el)
						v.SetAt(rng.Intn(v.Len()), el)
					case op < 8 && v.Len() > 0:
						v.TryGrow(el)
						v.InsertAt(rng.Intn(v.Len()), el, rng.Intn(2) == 0)
					case v.Len() > 2:
						v.Trim(1, v.Len()-2)
					}
					encs[k] = append(encs[k], v.Encoding())
				}
			}
			for i := range encs[0] {
				seen[encs[0][i]] = true
				if encs[0][i] != encs[1][i] {
					t.Fatalf("fill %d seed %d step %d: %s packed, %s unpacked", fill, seed, i, encs[0][i], encs[1][i])
				}
			}
		}
	}
	SetListMaxListpackSize(-2)
	if !seen["listpack"] || !seen["quicklist"] {
		t.Fatalf("encodings seen: %v, want both", seen)
	}
}
