package xxh3

import "math/bits"

// Uint128 is a 128-bit hash, as XXH128_hash_t holds it.
type Uint128 struct{ Lo, Hi uint64 }

func mul128(a, b uint64) Uint128 {
	hi, lo := bits.Mul64(a, b)
	return Uint128{lo, hi}
}

func mix32(acc Uint128, b []byte, i1, i2, s int) Uint128 {
	acc.Lo += mix16(b, i1, s)
	acc.Lo ^= r64(b, i2) + r64(b, i2+8)
	acc.Hi += mix16(b, i2, s+16)
	acc.Hi ^= r64(b, i1) + r64(b, i1+8)
	return acc
}

func finish128(acc Uint128, n int) Uint128 {
	lo := acc.Lo + acc.Hi
	hi := acc.Lo*prime64_1 + acc.Hi*prime64_4 + uint64(n)*prime64_2
	return Uint128{avalanche(lo), -avalanche(hi)}
}

// Hash128 is XXH3_128bits(b).
func Hash128(b []byte) Uint128 {
	n := len(b)
	sec := secret[:]
	switch {
	case n == 0:
		return Uint128{avalanche64(r64(sec, 64) ^ r64(sec, 72)), avalanche64(r64(sec, 80) ^ r64(sec, 88))}
	case n <= 3:
		cl := uint32(b[0])<<16 | uint32(b[n>>1])<<24 | uint32(b[n-1]) | uint32(n)<<8
		ch := bits.RotateLeft32(bits.ReverseBytes32(cl), 13)
		return Uint128{
			avalanche64(uint64(cl) ^ uint64(r32(sec, 0)^r32(sec, 4))),
			avalanche64(uint64(ch) ^ uint64(r32(sec, 8)^r32(sec, 12))),
		}
	case n <= 8:
		in := uint64(r32(b, 0)) + uint64(r32(b, n-4))<<32
		m := mul128(in^(r64(sec, 16)^r64(sec, 24)), prime64_1+uint64(n)<<2)
		m.Hi += m.Lo << 1
		m.Lo ^= m.Hi >> 3
		m.Lo ^= m.Lo >> 35
		m.Lo *= primeMX2
		m.Lo ^= m.Lo >> 28
		m.Hi = avalanche(m.Hi)
		return m
	case n <= 16:
		flipL := r64(sec, 32) ^ r64(sec, 40)
		flipH := r64(sec, 48) ^ r64(sec, 56)
		lo, hi := r64(b, 0), r64(b, n-8)
		m := mul128(lo^hi^flipL, prime64_1)
		m.Lo += uint64(n-1) << 54
		hi ^= flipH
		m.Hi += hi + uint64(uint32(hi))*(prime32_2-1)
		m.Lo ^= bits.ReverseBytes64(m.Hi)
		h := mul128(m.Lo, prime64_2)
		h.Hi += m.Hi * prime64_2
		return Uint128{avalanche(h.Lo), avalanche(h.Hi)}
	case n <= 128:
		acc := Uint128{uint64(n) * prime64_1, 0}
		if n > 32 {
			if n > 64 {
				if n > 96 {
					acc = mix32(acc, b, 48, n-64, 96)
				}
				acc = mix32(acc, b, 32, n-48, 64)
			}
			acc = mix32(acc, b, 16, n-32, 32)
		}
		acc = mix32(acc, b, 0, n-16, 0)
		return finish128(acc, n)
	case n <= 240:
		acc := Uint128{uint64(n) * prime64_1, 0}
		for i := 32; i < 160; i += 32 {
			acc = mix32(acc, b, i-32, i-16, i-32)
		}
		acc.Lo, acc.Hi = avalanche(acc.Lo), avalanche(acc.Hi)
		for i := 160; i <= n; i += 32 {
			acc = mix32(acc, b, i-32, i-16, 3+i-160)
		}
		acc = mix32(acc, b, n-16, n-32, 136-17-16)
		return finish128(acc, n)
	}
	acc := longAccs(b)
	return Uint128{
		mergeAccs(&acc, 11, uint64(n)*prime64_1),
		mergeAccs(&acc, len(secret)-64-11, ^(uint64(n) * prime64_2)),
	}
}
