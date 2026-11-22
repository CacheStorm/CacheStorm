package store

import (
	"math"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/fpconv"
)

// ArrayKind is how Redis 8.8's sparse array holds a value (arEncode): an
// immediate integer, an immediate double, an inline string of up to 7
// bytes, or a heap string. Every kind gives back the bytes it was given;
// the kind decides how AROP reads a number and how RDB files store it.
type ArrayKind uint8

const (
	ArrayString ArrayKind = iota
	ArrayInt
	ArrayFloat
	ArraySmallString
)

// ArraySmallStringMax is the longest string a 64-bit Redis keeps inline.
const ArraySmallStringMax = 7

// ClassifyArrayValue returns the kind Redis gives s, with its integer or
// double when it is one.
func ClassifyArrayValue(s string) (kind ArrayKind, i int64, f float64) {
	if n, ok := arrayInt(s); ok && n >= -(1<<61) && n <= 1<<61-1 {
		return ArrayInt, n, 0
	}
	if d, ok := arrayFloat(s); ok {
		return ArrayFloat, 0, d
	}
	if len(s) <= ArraySmallStringMax {
		return ArraySmallString, 0, 0
	}
	return ArrayString, 0, 0
}

// arrayInt is Redis' string2ll.
func arrayInt(s string) (int64, bool) {
	n := len(s)
	if n == 0 || n > 20 {
		return 0, false
	}
	if s == "0" {
		return 0, true
	}
	i := 0
	if s[0] == '-' {
		i = 1
		if n == 1 {
			return 0, false
		}
	}
	if s[i] < '1' || s[i] > '9' {
		return 0, false
	}
	for j := i + 1; j < n; j++ {
		if s[j] < '0' || s[j] > '9' {
			return 0, false
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}

// arrayFloat is arTryEncodeFloat: s must be a plain decimal with one dot
// that reads back exactly from the double with its two lowest bits
// cleared, written as Redis writes array doubles.
func arrayFloat(s string) (float64, bool) {
	body := strings.TrimPrefix(s, "-")
	if body == "" || strings.Count(body, ".") != 1 {
		return 0, false
	}
	for i := 0; i < len(body); i++ {
		if c := body[i]; c != '.' && (c < '0' || c > '9') {
			return 0, false
		}
	}
	d, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(d, 0) || math.IsNaN(d) {
		return 0, false
	}
	t := math.Float64frombits(math.Float64bits(d) &^ 3)
	out := d2string(t)
	if out == s || out+".0" == s {
		return t, true
	}
	return 0, false
}

// d2string is Redis' d2string.
func d2string(d float64) string {
	switch {
	case math.IsNaN(d):
		return "nan"
	case math.IsInf(d, 1):
		return "inf"
	case math.IsInf(d, -1):
		return "-inf"
	case d == 0:
		if math.Signbit(d) {
			return "-0"
		}
		return "0"
	}
	if d >= -4.611686018427387904e18 && d <= 4.611686018427387904e18 && d == math.Trunc(d) {
		return strconv.FormatInt(int64(d), 10)
	}
	return fpconv.Dtoa(d)
}

// FormatArrayFloat is arFormatFloat: how an array's double reads back,
// "1.0" rather than "1".
func FormatArrayFloat(d float64) string {
	s := d2string(d)
	if !math.IsInf(d, 0) && !math.IsNaN(d) && !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

var arraySliceSize, arraySparseKMax, arraySparseKMin atomic.Int64

func init() {
	arraySliceSize.Store(4096)
	arraySparseKMax.Store(10)
	arraySparseKMin.Store(5)
}

// SetArraySliceSize, SetArraySparseKMax and SetArraySparseKMin are the
// array-slice-size, array-sparse-kmax and array-sparse-kmin configs.
func SetArraySliceSize(v int64)  { arraySliceSize.Store(v) }
func SetArraySparseKMax(v int64) { arraySparseKMax.Store(v) }
func SetArraySparseKMin(v int64) { arraySparseKMin.Store(v) }

// ArrayConfig returns array-slice-size, array-sparse-kmax and
// array-sparse-kmin.
func ArrayConfig() (sliceSize, kmax, kmin int64) {
	return arraySliceSize.Load(), arraySparseKMax.Load(), arraySparseKMin.Load()
}
