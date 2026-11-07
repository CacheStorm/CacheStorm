package command

import (
	"strconv"
	"testing"
)

// cFloat makes Go's float parsers read exactly what C's strtod reads: hex
// floats with or without an exponent, but no digit separators and no binary
// exponent on a decimal, which Go would otherwise accept.
func TestCFloatMatchesStrtod(t *testing.T) {
	for in, want := range map[string]float64{
		"1.5": 1.5, "0x10": 16, "0X1.8": 1.5, "-0x1P-2": -0.25, "0x1p1": 2, "1e1": 10, "+.5": 0.5,
	} {
		if got, err := strconv.ParseFloat(cFloat(in), 64); err != nil || got != want {
			t.Errorf("%q: %v %v, want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"1_0", "0x1_0", "1p1", "1P1", "_1", "1.5_", "0x", "1e"} {
		if got, err := strconv.ParseFloat(cFloat(in), 64); err == nil {
			t.Errorf("%q parsed as %v; strtod refuses it", in, got)
		}
		if _, ok := parseIncrByFloatArg([]byte(in)); ok {
			t.Errorf("INCRBYFLOAT accepted %q", in)
		}
		if _, ok := parseLongDouble(in); ok {
			t.Errorf("HINCRBYFLOAT accepted %q", in)
		}
	}
}
