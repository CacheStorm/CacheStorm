package vectorset

import (
	"math"
	"math/bits"
)

// The explicit float32 conversions around products keep the Go compiler
// from fusing them into FMA instructions, so every result is rounded as
// Redis's x86-64 build (plain SSE, no FMA) rounds it.

func hnswDistanceFloat(x, y []float32) float32 {
	y = y[:len(x)]
	var dot0, dot1 float32
	for len(x) >= 8 && len(y) >= 8 {
		xs := (*[8]float32)(x)
		ys := (*[8]float32)(y)
		s0 := float32(xs[0]*ys[0]) + float32(xs[1]*ys[1])
		s0 += float32(xs[2] * ys[2])
		s0 += float32(xs[3] * ys[3])
		dot0 += s0
		s1 := float32(xs[4]*ys[4]) + float32(xs[5]*ys[5])
		s1 += float32(xs[6] * ys[6])
		s1 += float32(xs[7] * ys[7])
		dot1 += s1
		x = x[8:]
		y = y[8:]
	}
	for i := range x {
		dot0 += float32(x[i] * y[i])
	}
	return 1.0 - (dot0 + dot1)
}

func hnswDistanceQ8(x, y []int8, rangeA, rangeB float32) float32 {
	if rangeA == 0 || rangeB == 0 {
		return 1.0
	}
	scale := float32(rangeA/127) * float32(rangeB/127)
	y = y[:len(x)]
	var dot0, dot1 int32
	for len(x) >= 8 && len(y) >= 8 {
		xs := (*[8]int8)(x)
		ys := (*[8]int8)(y)
		dot0 += int32(xs[0])*int32(ys[0]) + int32(xs[1])*int32(ys[1]) +
			int32(xs[2])*int32(ys[2]) + int32(xs[3])*int32(ys[3])
		dot1 += int32(xs[4])*int32(ys[4]) + int32(xs[5])*int32(ys[5]) +
			int32(xs[6])*int32(ys[6]) + int32(xs[7])*int32(ys[7])
		x = x[8:]
		y = y[8:]
	}
	for i := range x {
		dot0 += int32(x[i]) * int32(y[i])
	}
	dotf := float32(float32(dot0+dot1) * scale)
	distance := 1.0 - dotf
	if distance < 0 {
		distance = 0
	} else if distance > 2 {
		distance = 2
	}
	return distance
}

func hnswDistanceBin(x, y []uint64, dim uint32) float32 {
	y = y[:len(x)]
	var opposite uint32
	for j := range x {
		opposite += uint32(bits.OnesCount64(x[j] ^ y[j]))
	}
	return float32(opposite) * 2 / float32(dim)
}

// hnswNormalize L2-normalizes x in place and returns the norm. A zero vector
// is left untouched and reported with ok=false (Redis then keeps l2 = 1).
func hnswNormalize(x []float32) (l2 float32, ok bool) {
	dim := len(x)
	i := 0
	for ; i+3 < dim; i += 4 {
		xs := x[i : i+4 : i+4]
		s := float32(xs[0]*xs[0]) + float32(xs[1]*xs[1])
		s += float32(xs[2] * xs[2])
		s += float32(xs[3] * xs[3])
		l2 += s
	}
	for ; i < dim; i++ {
		l2 += float32(x[i] * x[i])
	}
	if l2 == 0 {
		return 0, false
	}
	l2 = float32(math.Sqrt(float64(l2)))
	for j := range x {
		x[j] /= l2
	}
	return l2, true
}

// hnswQuantizeQ8 mirrors quantize_to_q8: per-vector symmetric int8 with
// roundf (half away from zero) and the max absolute value as the range.
func hnswQuantizeQ8(src []float32, dst []int8) float32 {
	var maxAbs float32
	for _, v := range src {
		if v > maxAbs {
			maxAbs = v
		}
		if -v > maxAbs {
			maxAbs = -v
		}
	}
	if maxAbs == 0 {
		clear(dst)
		return 0
	}
	scale := 127.0 / maxAbs
	for j, v := range src {
		dst[j] = int8(int32(math.Round(float64(float32(v * scale)))))
	}
	return maxAbs
}

func hnswQuantizeBin(src []float32, dst []uint64) {
	clear(dst)
	for j, v := range src {
		if v > 0 {
			dst[j/64] |= 1 << uint(j&63)
		}
	}
}

// hnswPairMixer128 is secure_pair_mixer_128 from Redis's mixer.h.
func hnswPairMixer128(salt0, salt1, id1, id2, level uint64) (uint64, uint64) {
	const (
		p1 = 0xFF51AFD7ED558CCD
		p2 = 0xC4CEB9FE1A85EC53
		p3 = 0x9E3779B97F4A7C15
		p4 = 0xBF58476D1CE4E5B9
		p5 = 0x94D049BB133111EB
		p6 = 0x2B7E151628AED2A7
	)
	idA, idB := id1, id2
	if id2 < id1 {
		idA, idB = id2, id1
	}
	h1 := salt0 ^ 0xDEADBEEFDEADBEEF
	h2 := salt1 ^ 0xCAFEBABECAFEBABE

	lm := level * p5
	lm ^= lm >> 32
	lm *= p6
	h1 ^= lm
	h2 ^= bits.RotateLeft64(lm, 31)

	h1 ^= idA
	h1 *= p1
	h1 = bits.RotateLeft64(h1, 23)
	h1 *= p2

	h2 ^= idB
	h2 *= p3
	h2 = bits.RotateLeft64(h2, 29)
	h2 *= p4

	for range 3 {
		tmp := h1
		h1 += h2
		h2 += tmp
		h1 ^= bits.RotateLeft64(h1, 31)
		h1 *= p1
		h1 ^= salt0
		h2 ^= bits.RotateLeft64(h2, 37)
		h2 *= p2
		h2 ^= salt1
	}

	h1 ^= h1 >> 33
	h1 *= p3
	h1 ^= h1 >> 29
	h1 *= p4
	h1 ^= h1 >> 32

	h2 ^= h2 >> 33
	h2 *= p5
	h2 ^= h2 >> 29
	h2 *= p6
	h2 ^= h2 >> 32
	return h1, h2
}
