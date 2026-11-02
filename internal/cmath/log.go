// Package cmath has the math functions whose results must match the C
// library's to the last bit: Go's math.Log can be one unit in the last
// place away from glibc's correctly rounded log.
package cmath

import (
	"math"
	"math/big"
	"sync"
)

const prec = 160

// Log is the natural logarithm, correctly rounded as glibc's log is.
func Log(x float64) float64 {
	if x <= 0 || math.IsInf(x, 0) || math.IsNaN(x) {
		return math.Log(x)
	}
	frac, exp := math.Frexp(x)
	m := new(big.Float).SetPrec(prec).SetFloat64(frac * 2)
	r := atanhLog(m)
	r.Add(r, new(big.Float).SetPrec(prec).Mul(ln2(), new(big.Float).SetInt64(int64(exp-1))))
	f, _ := r.Float64()
	return f
}

// atanhLog is log(m) = 2*atanh((m-1)/(m+1)) for m in [1, 2).
func atanhLog(m *big.Float) *big.Float {
	one := new(big.Float).SetPrec(prec).SetInt64(1)
	z := new(big.Float).SetPrec(prec).Quo(new(big.Float).SetPrec(prec).Sub(m, one), new(big.Float).SetPrec(prec).Add(m, one))
	z2 := new(big.Float).SetPrec(prec).Mul(z, z)
	sum := new(big.Float).SetPrec(prec)
	term := new(big.Float).SetPrec(prec).Set(z)
	for k := int64(1); k < 400; k += 2 {
		t := new(big.Float).SetPrec(prec).Quo(term, new(big.Float).SetInt64(k))
		if t.Sign() == 0 || (sum.Sign() != 0 && t.MantExp(nil)-sum.MantExp(nil) < -prec) {
			break
		}
		sum.Add(sum, t)
		term.Mul(term, z2)
	}
	return sum.Mul(sum, new(big.Float).SetInt64(2))
}

var ln2 = sync.OnceValue(func() *big.Float {
	return atanhLog(new(big.Float).SetPrec(prec).SetInt64(2))
})
