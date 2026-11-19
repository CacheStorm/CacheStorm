package cmath

import "math"

// Pow is a port of glibc's pow (sysdeps/ieee754/dbl-64/e_pow.c), which is
// what Redis's ** operator calls. Go's math.Pow differs from it in the last
// bit for about half of all non-integer inputs, which changes the outcome of
// comparisons such as `.x ** 0.5 == 1.4142135623730951`. glibc selects the
// FMA build of this code on x86-64 CPUs that have FMA; that is the variant
// ported here. Explicit float64 conversions keep the Go compiler from fusing
// operations the C code rounds separately.
func Pow(x, y float64) float64 {
	var signBias uint64
	ix := math.Float64bits(x)
	iy := math.Float64bits(y)
	topx := uint32(ix >> 52)
	topy := uint32(iy >> 52)
	if topx-0x001 >= 0x7ff-0x001 || (topy&0x7ff)-0x3be >= 0x43e-0x3be {
		if zeroInfNaN(iy) {
			if 2*iy == 0 {
				return 1
			}
			if ix == oneBits {
				return 1
			}
			if 2*ix > 2*infBits || 2*iy > 2*infBits {
				return x + y
			}
			if 2*ix == 2*oneBits {
				return 1
			}
			if (2*ix < 2*oneBits) == (iy>>63 == 0) {
				return 0
			}
			return y * y
		}
		if zeroInfNaN(ix) {
			x2 := x * x
			if ix>>63 != 0 && checkInt(iy) == 1 {
				x2 = -x2
			}
			if iy>>63 != 0 {
				return 1 / x2
			}
			return x2
		}
		if ix>>63 != 0 {
			switch checkInt(iy) {
			case 0:
				return math.NaN()
			case 1:
				signBias = expSignBias
			}
			ix &= 0x7fffffffffffffff
			topx &= 0x7ff
		}
		if (topy&0x7ff)-0x3be >= 0x43e-0x3be {
			if ix == oneBits {
				return 1
			}
			if topy&0x7ff < 0x3be {
				if ix > oneBits {
					return 1.0 + y
				}
				return 1.0 - y
			}
			if (ix > oneBits) == (topy < 0x800) {
				return math.Inf(1)
			}
			return 0
		}
		if topx == 0 {
			ix = math.Float64bits(x*0x1p52) & 0x7fffffffffffffff
			ix -= 52 << 52
		}
	}
	hi, lo := powLog(ix)
	ehi := float64(y * hi)
	elo := math.FMA(y, lo, math.FMA(y, hi, -ehi))
	return powExp(ehi, elo, signBias)
}

const (
	oneBits     = 0x3ff0000000000000
	infBits     = 0x7ff0000000000000
	expSignBias = 0x800 << 7

	powLogOff = 0x3fe6955500000000
	ln2hi     = 0x1.62e42fefa3800p-1
	ln2lo     = 0x1.ef35793c76730p-45
	logA0     = -0x1p-1
	logA1     = 0x1.555555555556p-2 * -2
	logA2     = -0x1.0000000000006p-2 * -2
	logA3     = 0x1.999999959554ep-3 * 4
	logA4     = -0x1.555555529a47ap-3 * 4
	logA5     = 0x1.2495b9b4845e9p-3 * -8
	logA6     = -0x1.0002b8b263fc3p-3 * -8

	expInvLn2N   = 0x1.71547652b82fep0 * 128
	expNegLn2hiN = -0x1.62e42fefa0000p-8
	expNegLn2loN = -0x1.cf79abc9e3b3ap-47
	expShift     = 0x1.8p52
	expC2        = 0x1.ffffffffffdbdp-2
	expC3        = 0x1.555555555543cp-3
	expC4        = 0x1.55555cf172b91p-5
	expC5        = 0x1.1111167a4d017p-7
)

func zeroInfNaN(i uint64) bool { return 2*i-1 >= 2*infBits-1 }

// checkInt returns 0 if iy is not an integer, 1 if odd, 2 if even.
func checkInt(iy uint64) int {
	e := int(iy >> 52 & 0x7ff)
	if e < 0x3ff {
		return 0
	}
	if e > 0x3ff+52 {
		return 2
	}
	if iy&(1<<(0x3ff+52-e)-1) != 0 {
		return 0
	}
	if iy&(1<<(0x3ff+52-e)) != 0 {
		return 1
	}
	return 2
}

func powLog(ix uint64) (float64, float64) {
	tmp := ix - powLogOff
	i := (tmp >> (52 - 7)) % 128
	k := int64(tmp) >> 52
	iz := ix - (tmp & (0xfff << 52))
	z := math.Float64frombits(iz)
	kd := float64(k)
	invc, logc, logctail := powLogTab[i][0], powLogTab[i][1], powLogTab[i][2]
	r := math.FMA(z, invc, -1.0)
	t1 := math.FMA(kd, ln2hi, logc)
	t2 := t1 + r
	lo1 := math.FMA(kd, ln2lo, logctail)
	lo2 := t1 - t2 + r
	ar := float64(logA0 * r)
	ar2 := float64(r * ar)
	ar3 := float64(r * ar2)
	hi := t2 + ar2
	lo3 := math.FMA(ar, r, -ar2)
	lo4 := t2 - hi + ar2
	inner := math.FMA(r, logA6, logA5)
	mid := math.FMA(ar2, inner, math.FMA(r, logA4, logA3))
	poly := math.FMA(ar2, mid, math.FMA(r, logA2, logA1))
	lo := math.FMA(ar3, poly, lo1+lo2+lo3+lo4)
	y := hi + lo
	return y, hi - y + lo
}

func powExp(x, xtail float64, signBias uint64) float64 {
	abstop := uint32(math.Float64bits(x)>>52) & 0x7ff
	if abstop-0x3c9 >= 0x408-0x3c9 {
		if abstop-0x3c9 >= 0x80000000 {
			one := 1.0 + x
			if signBias != 0 {
				return -one
			}
			return one
		}
		if abstop >= 0x409 {
			neg := signBias != 0
			if math.Float64bits(x)>>63 != 0 {
				if neg {
					return math.Copysign(0, -1)
				}
				return 0
			}
			if neg {
				return math.Inf(-1)
			}
			return math.Inf(1)
		}
		abstop = 0
	}
	kd := math.FMA(expInvLn2N, x, expShift)
	ki := math.Float64bits(kd)
	kd -= expShift
	r := math.FMA(kd, expNegLn2loN, math.FMA(kd, expNegLn2hiN, x))
	r += xtail
	idx := 2 * (ki % 128)
	top := (ki + signBias) << (52 - 7)
	tail := math.Float64frombits(expTab[idx])
	sbits := expTab[idx+1] + top
	r2 := float64(r * r)
	tmp := math.FMA(float64(r2*r2), math.FMA(r, expC5, expC4), math.FMA(r2, math.FMA(r, expC3, expC2), tail+r))
	if abstop == 0 {
		return powExpSpecial(tmp, sbits, ki)
	}
	scale := math.Float64frombits(sbits)
	return math.FMA(scale, tmp, scale)
}

func powExpSpecial(tmp float64, sbits, ki uint64) float64 {
	if ki&0x80000000 == 0 {
		sbits -= 1009 << 52
		scale := math.Float64frombits(sbits)
		return 0x1p1009 * math.FMA(scale, tmp, scale)
	}
	sbits += 1022 << 52
	scale := math.Float64frombits(sbits)
	st := float64(scale * tmp)
	y := scale + st
	if math.Abs(y) < 1.0 {
		one := 1.0
		if y < 0.0 {
			one = -1.0
		}
		lo := scale - y + st
		hi := one + y
		lo = one - hi + y + lo
		y = (hi + lo) - one
		if y == 0 {
			y = math.Float64frombits(sbits & 0x8000000000000000)
		}
	}
	return 0x1p-1022 * y
}
