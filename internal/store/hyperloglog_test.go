package store

import (
	"bytes"
	"sync"
	"testing"
)

func TestHLLCodecAndPackedUpdates(t *testing.T) {
	for _, dense := range []bool{false, true} {
		h := &HLLState{Dense: dense}
		for _, s := range []string{"a", "b", "", "\x00binary"} {
			h.Add([]byte(s))
		}
		if h.Count() != 4 {
			t.Fatalf("count=%d", h.Count())
		}
		encoded, err := h.Encode(true)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := DecodeHLL(encoded)
		if err != nil || restored.Registers != h.Registers {
			t.Fatalf("codec err=%v", err)
		}
		if dense {
			next, changed, err := AddDenseHLL(encoded, [][]byte{[]byte("next"), []byte("next")})
			if err != nil || !changed {
				t.Fatalf("dense add=%v %v", changed, err)
			}
			h.Add([]byte("next"))
			restored, err = DecodeHLL(next)
			if err != nil || restored.Registers != h.Registers {
				t.Fatal("packed update differs from register model")
			}
			same, changed, err := AddDenseHLL(next, [][]byte{[]byte("next")})
			if err != nil || changed || !bytes.Equal(next, same) {
				t.Fatal("duplicate dense update changed data")
			}
		}
	}
}

func TestHLLMalformedSparseAndLegacySerialization(t *testing.T) {
	valid, _ := (&HLLState{}).Encode(true)
	for _, data := range [][]byte{valid[:16], append(append([]byte(nil), valid[:16]...), 64), append(append([]byte(nil), valid...), 0)} {
		if _, err := DecodeHLL(data); err == nil {
			t.Fatalf("bad sparse accepted: %x", data)
		}
	}
	legacy := &HyperLogLogValue{}
	legacy.Add([]byte("a"))
	legacy.Add([]byte("b"))
	encoded, err := legacy.RedisStringBytes()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHLL(encoded)
	if err != nil || decoded.Count() != 2 {
		t.Fatal("legacy value serialized diagnostic text")
	}
	legacy.Registers[0] = 64
	if _, err := legacy.RedisStringBytes(); err == nil {
		t.Fatal("unrepresentable register silently truncated")
	}
}

func TestHLLLegacyMethodsRaceFree(t *testing.T) {
	h := &HyperLogLogValue{}
	var wg sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch worker {
				case 0:
					h.Add([]byte{byte(i)})
				case 1:
					h.Count()
					_ = h.String()
				default:
					h.Clone()
					h.Merge(h)
				}
			}
		}(worker)
	}
	wg.Wait()
	if h.Count() <= 0 {
		t.Fatal("lost values")
	}
}

func FuzzDecodeHLL(f *testing.F) {
	sparse, _ := (&HLLState{}).Encode(true)
	dense, _ := (&HLLState{Dense: true}).Encode(true)
	f.Add(sparse)
	f.Add(dense)
	f.Add([]byte("HYLL\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x80\x40"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		h, err := DecodeHLL(data)
		if err != nil {
			return
		}
		h.Count()
		encoded, err := h.Encode(false)
		if err != nil {
			t.Fatal(err)
		}
		again, err := DecodeHLL(encoded)
		if err != nil || again.Registers != h.Registers {
			t.Fatal("valid HLL roundtrip lost registers")
		}
	})
}

func BenchmarkDenseHLLAdd(b *testing.B) {
	data, _ := (&HLLState{Dense: true}).Encode(true)
	element := [][]byte{[]byte("element")}
	duplicate, _, _ := AddDenseHLL(data, element)
	for _, tc := range []struct {
		name string
		data []byte
	}{{"changed", data}, {"duplicate", duplicate}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := AddDenseHLL(tc.data, element); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
