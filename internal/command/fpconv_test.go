package command

import (
	"strconv"
	"testing"
)

func TestFormatZScoreMatchesRedisGrisu(t *testing.T) {
	// Replies of Redis 7.4's ZSCORE for these scores.
	for in, want := range map[string]string{
		"0.04780121730387921":      "0.047801217303879207",
		"0.1455056976925133":       "0.14550569769251331",
		"5e-324":                   "5e-324",
		"1.7976931348623157e308":   "1.7976931348623157e+308",
		"-1e-7":                    "-1e-7",
		"123456789012345678901234": "123456789012345690000000",
		"1e21":                     "1e+21",
		"-2.5e-10":                 "-2.5e-10",
		"0.1":                      "0.1",
		"-0.000001":                "-0.000001",
		"2.2250738585072014e-308":  "2.2250738585072014e-308",
		"-1.2345678901234567e-123": "-1.2345678901234567e-123",
	} {
		f, _ := strconv.ParseFloat(in, 64)
		if got := formatZScore(f); got != want {
			t.Errorf("formatZScore(%s) = %s, want %s", in, got, want)
		}
	}
}
