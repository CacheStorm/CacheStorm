package store

import (
	"bytes"
	"math/rand"
	"strconv"
	"testing"
)

type modelEntry struct {
	id    StreamID
	pairs []StreamField
}

func sameEntry(t *testing.T, ctx string, got *StreamEntry, want modelEntry) {
	t.Helper()
	if got.ID != want.id.String() || len(got.Pairs) != len(want.pairs) {
		t.Fatalf("%s: got %s %v, want %s %v", ctx, got.ID, got.Pairs, want.id, want.pairs)
	}
	for i, p := range got.Pairs {
		if p.Name != want.pairs[i].Name || !bytes.Equal(p.Value, want.pairs[i].Value) {
			t.Fatalf("%s: field %d is %s=%q, want %s=%q", ctx, i, p.Name, p.Value, want.pairs[i].Name, want.pairs[i].Value)
		}
	}
}

// The packed stream behaves as a slice of entries under adds, deletes and
// trims, and entries read earlier keep their values.
func TestPackedStreamMatchesSliceModel(t *testing.T) {
	for seed := int64(1); seed <= 10; seed++ {
		rng := rand.New(rand.NewSource(seed))
		v := NewStreamValue(0)
		var model []modelEntry
		var held []*StreamEntry
		var heldWant []modelEntry
		ms := uint64(0)
		var top StreamID
		names := []string{"f", "g", "name", "longer-field"}
		for step := 0; step < 4000; step++ {
			switch op := rng.Intn(10); {
			case op < 5:
				ms += uint64(rng.Intn(3))
				id := StreamID{Ms: ms, Seq: uint64(rng.Intn(5))}
				if !top.Less(id) {
					id = StreamID{Ms: top.Ms + 1}
					ms = id.Ms
				}
				top = id
				n := []int{1, 1, 2, 3}[rng.Intn(4)]
				pairs := make([]StreamField, n)
				for i := range pairs {
					val := make([]byte, rng.Intn(200))
					rng.Read(val)
					pairs[i] = StreamField{Name: names[rng.Intn(len(names))], Value: val}
				}
				if !v.LoadEntry(id, pairs) {
					t.Fatalf("seed %d step %d: LoadEntry refused %s", seed, step, id)
				}
				model = append(model, modelEntry{id, pairs})
			case op == 5 && len(model) > 0:
				i := rng.Intn(len(model))
				if got := v.DeleteIDs([]StreamID{model[i].id}); got != 1 {
					t.Fatalf("seed %d step %d: deleted %d", seed, step, got)
				}
				model = append(model[:i], model[i+1:]...)
			case op == 6 && len(model) > 0:
				keep := rng.Intn(len(model) + 1)
				v.TrimStream(StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: int64(keep)})
				model = model[len(model)-keep:]
			case op == 7 && len(model) > 0:
				i := rng.Intn(len(model))
				v.TrimStream(StreamTrimArgs{Strategy: StreamTrimMinID, MinID: model[i].id})
				model = model[i:]
			default:
				if len(model) == 0 {
					continue
				}
				a, b := rng.Intn(len(model)), rng.Intn(len(model))
				if a > b {
					a, b = b, a
				}
				count := int64(rng.Intn(8))
				rev := rng.Intn(2) == 0
				got := v.Range(model[a].id, model[b].id, count, rev)
				want := append([]modelEntry(nil), model[a:b+1]...)
				if rev {
					for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
						want[i], want[j] = want[j], want[i]
					}
				}
				if count > 0 && int64(len(want)) > count {
					want = want[:count]
				}
				if len(got) != len(want) {
					t.Fatalf("seed %d step %d: range got %d entries, want %d", seed, step, len(got), len(want))
				}
				for i := range got {
					sameEntry(t, "seed "+strconv.FormatInt(seed, 10)+" range", got[i], want[i])
					if len(held) < 500 {
						held = append(held, got[i])
						heldWant = append(heldWant, want[i])
					}
				}
			}
			if v.Len() != int64(len(model)) {
				t.Fatalf("seed %d step %d: len %d, want %d", seed, step, v.Len(), len(model))
			}
			if v.SizeOf() != v.RecountSize() {
				t.Fatalf("seed %d step %d: size %d, recounted %d", seed, step, v.SizeOf(), v.RecountSize())
			}
		}
		for i, e := range held {
			sameEntry(t, "held", e, heldWant[i])
		}
	}
}

func TestPackedStreamNilAndEmptyFields(t *testing.T) {
	v := NewStreamValue(0)
	v.LoadEntry(StreamID{Ms: 1}, nil)
	v.LoadEntry(StreamID{Ms: 2}, []StreamField{})
	v.LoadEntry(StreamID{Ms: 3}, nil)
	es := v.Range(StreamID{}, MaxStreamID, 0, false)
	if es[0].Pairs != nil || es[1].Pairs == nil || len(es[1].Pairs) != 0 || es[2].Pairs != nil {
		t.Fatalf("fields %#v %#v %#v", es[0].Pairs, es[1].Pairs, es[2].Pairs)
	}
}
