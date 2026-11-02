package search

import (
	"encoding/binary"
	"math"
)

// groupHash is RediSearch's RSValue_Hash of a GROUPBY key: FNV-1a over a
// string's bytes or a number's IEEE bytes, one more for a null, chained
// from the previous property's hash.
func groupHash(h uint64, v string, f float64, isNum, null bool) uint64 {
	switch {
	case null:
		return h + 1
	case isNum:
		bits := math.Float64bits(f)
		if f != f {
			bits = cNaN
		}
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], bits)
		return fnv64a(b[:], h)
	}
	return fnv64a([]byte(v), h)
}

// cNaN is C's NAN, which Go's math.NaN() is not.
const cNaN = 0x7ff8000000000000

func fnv64a(b []byte, h uint64) uint64 {
	for _, c := range b {
		h ^= uint64(c)
		h *= 0x100000001b3
	}
	return h
}

// khashOrder is the order klib's khash (KHASH_MAP_INIT_INT64), into which
// RediSearch's grouper puts each new group's hash, iterates its keys.
type khashOrder struct {
	keys     []uint64
	used     []bool
	size     int
	occupied int
	upper    int
}

func khashBucket(k uint64) uint32 { return uint32(k>>33 ^ k ^ k<<11) }

func khashUpper(n int) int { return int(float64(n)*0.77 + 0.5) }

func (h *khashOrder) resize(n int) {
	x := uint32(n - 1)
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	n = max(int(x+1), 4)
	if h.size >= khashUpper(n) {
		return
	}
	old := len(h.keys)
	filled := make([]bool, n)
	if old < n {
		h.keys = append(h.keys, make([]uint64, n-old)...)
		h.used = append(h.used, make([]bool, n-old)...)
	}
	mask := uint32(n - 1)
	for j := 0; j < old; j++ {
		if !h.used[j] {
			continue
		}
		key := h.keys[j]
		h.used[j] = false
		for {
			i, step := khashBucket(key)&mask, uint32(0)
			for filled[i] {
				step++
				i = (i + step) & mask
			}
			filled[i] = true
			if int(i) < old && h.used[i] {
				h.keys[i], key = key, h.keys[i]
				h.used[i] = false
				continue
			}
			h.keys[i] = key
			break
		}
	}
	h.keys, h.used = h.keys[:n], filled
	h.occupied, h.upper = h.size, khashUpper(n)
}

// put adds a key the table does not hold.
func (h *khashOrder) put(key uint64) {
	if h.occupied >= h.upper {
		if len(h.keys) > h.size<<1 {
			h.resize(len(h.keys) - 1)
		} else {
			h.resize(len(h.keys) + 1)
		}
	}
	mask := uint32(len(h.keys) - 1)
	i, step := khashBucket(key)&mask, uint32(0)
	for h.used[i] {
		step++
		i = (i + step) & mask
	}
	h.keys[i], h.used[i] = key, true
	h.size++
	h.occupied++
}

func (h *khashOrder) order() []uint64 {
	out := make([]uint64, 0, h.size)
	for i, u := range h.used {
		if u {
			out = append(out, h.keys[i])
		}
	}
	return out
}
