package store

import "math"

// Redis computes geo distances with the C library's sin, cos and asin, which
// glibc rounds correctly almost always where Go's math package is often one
// unit in the last place off, and that unit shows in STOREDIST scores.
// geoSin, geoCos and geoAsin evaluate in double-double precision and round
// once.

type dd struct{ hi, lo float64 }

func twoSum(a, b float64) dd {
	s := a + b
	bb := s - a
	return dd{s, (a - (s - bb)) + (b - bb)}
}

func quickTwoSum(a, b float64) dd {
	s := a + b
	return dd{s, b - (s - a)}
}

func twoProd(a, b float64) dd {
	p := a * b
	return dd{p, math.FMA(a, b, -p)}
}

func ddAdd(a, b dd) dd {
	s := twoSum(a.hi, b.hi)
	t := twoSum(a.lo, b.lo)
	s.lo += t.hi
	s = quickTwoSum(s.hi, s.lo)
	s.lo += t.lo
	return quickTwoSum(s.hi, s.lo)
}

func ddMul(a, b dd) dd {
	p := twoProd(a.hi, b.hi)
	p.lo += a.hi*b.lo + a.lo*b.hi
	return quickTwoSum(p.hi, p.lo)
}

func ddMulF(a dd, b float64) dd {
	p := twoProd(a.hi, b)
	p.lo += a.lo * b
	return quickTwoSum(p.hi, p.lo)
}

func ddNeg(a dd) dd { return dd{-a.hi, -a.lo} }

// ddPio2 is pi/2 to about 160 bits, in three parts.
var ddPio2 = [3]float64{1.5707963267948966, 6.123233995736766e-17, -1.4973849048591698e-33}

// reduceQuadrant returns x - k*pi/2 and k mod 4, for the k that leaves the
// remainder within pi/4.
func reduceQuadrant(x float64) (dd, int) {
	k := math.RoundToEven(x / ddPio2[0])
	r := dd{x, 0}
	for _, p := range ddPio2 {
		r = ddAdd(r, ddNeg(twoProd(k, p)))
	}
	return r, int(math.Mod(math.Mod(k, 4)+4, 4))
}

// The Taylor coefficients of sin and cos: the first four in double-double,
// the rest, whose terms are too small to need it, in double.
var (
	sinHead = [4]dd{{1, 0}, {-0.16666666666666666, -9.25185853854297e-18}, {0.008333333333333333, 1.1564823173178714e-19}, {-0.0001984126984126984, -1.7209558293420705e-22}}
	sinTail = [10]float64{2.7557319223985893e-06, -2.505210838544172e-08, 1.6059043836821613e-10, -7.647163731819816e-13, 2.8114572543455206e-15, -8.22063524662433e-18, 1.9572941063391263e-20, -3.868170170630684e-23, 6.446950284384474e-26, -9.183689863795546e-29}
	cosHead = [4]dd{{1, 0}, {-0.5, 0}, {0.041666666666666664, 2.3129646346357427e-18}, {-0.001388888888888889, 5.300543954373577e-20}}
	cosTail = [10]float64{2.48015873015873e-05, -2.755731922398589e-07, 2.08767569878681e-09, -1.1470745597729725e-11, 4.779477332387385e-14, -1.5619206968586225e-16, 4.110317623312165e-19, -8.896791392450574e-22, 1.6117375710961184e-24, -2.4795962632247976e-27}
)

// series evaluates head[0] + head[1]*x + ... + x^4*tail(x) by Horner's rule.
func series(head *[4]dd, tail *[10]float64, x dd) dd {
	t := tail[9]
	for i := 8; i >= 0; i-- {
		t = t*x.hi + tail[i]
	}
	acc := ddAdd(head[3], ddMulF(x, t))
	for i := 2; i >= 0; i-- {
		acc = ddAdd(head[i], ddMul(acc, x))
	}
	return acc
}

// sinCosTaylor returns sin(r) and cos(r) for |r| <= pi/4.
func sinCosTaylor(r dd) (dd, dd) {
	r2 := ddMul(r, r)
	return ddMul(r, series(&sinHead, &sinTail, r2)), series(&cosHead, &cosTail, r2)
}

func ddSinCos(x float64) (dd, dd) {
	r, q := reduceQuadrant(x)
	s, c := sinCosTaylor(r)
	switch q {
	case 1:
		return c, ddNeg(s)
	case 2:
		return ddNeg(s), ddNeg(c)
	case 3:
		return ddNeg(c), s
	}
	return s, c
}

func geoSin(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e6 {
		return math.Sin(x)
	}
	s, _ := ddSinCos(x)
	return s.hi + s.lo
}

func geoCos(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e6 {
		return math.Cos(x)
	}
	_, c := ddSinCos(x)
	return c.hi + c.lo
}

// ddAsinSmall refines asin of a in [0, 0.71] by a Newton step on sin.
func ddAsinSmall(a dd) dd {
	y := math.Asin(a.hi + a.lo)
	s, c := ddSinCos(y)
	diff := ddAdd(s, ddNeg(a))
	return ddAdd(dd{y, 0}, dd{-(diff.hi + diff.lo) / (c.hi + c.lo), 0})
}

func geoAsin(x float64) float64 {
	if !(x >= 0 && x <= 1) {
		return math.Asin(x)
	}
	if x <= 0.5 {
		r := ddAsinSmall(dd{x, 0})
		return r.hi + r.lo
	}
	// asin(x) = pi/2 - 2*asin(sqrt((1-x)/2)); 1-x and the halving are exact.
	t := (1 - x) / 2
	if t == 0 {
		return ddPio2[0]
	}
	s := math.Sqrt(t)
	sq := twoProd(s, s)
	root := quickTwoSum(s, ((t-sq.hi)-sq.lo)/(2*s))
	r := ddMulF(ddAsinSmall(root), -2)
	r = ddAdd(r, dd{ddPio2[0], ddPio2[1]})
	return r.hi + r.lo
}
