package vectorset

import (
	"errors"
	"math"
	"strconv"
)

func isSpaceC(c byte) bool {
	return c == ' ' || (c >= '\t' && c <= '\r')
}

func isDigitC(c byte) bool { return c >= '0' && c <= '9' }

func isAlphaC(c byte) bool { return (c|0x20) >= 'a' && (c|0x20) <= 'z' }

func isHexDigitC(c byte) bool {
	return isDigitC(c) || ((c|0x20) >= 'a' && (c|0x20) <= 'f')
}

func hasPrefixFold(s []byte, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if s[i]|0x20 != prefix[i] {
			return false
		}
	}
	return true
}

// strtod mirrors glibc strtod in the C locale: it returns the value of the
// longest valid prefix of s and how many bytes that prefix spans (0 when no
// conversion is possible). Inputs never contain NUL here.
func strtod(s []byte) (float64, int) {
	i := 0
	for i < len(s) && isSpaceC(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	sign := 1.0
	if neg {
		sign = -1
	}
	rest := s[i:]
	switch {
	case hasPrefixFold(rest, "infinity"):
		return math.Inf(int(sign)), i + 8
	case hasPrefixFold(rest, "inf"):
		return math.Inf(int(sign)), i + 3
	case hasPrefixFold(rest, "nan"):
		n := i + 3
		if n < len(s) && s[n] == '(' {
			j := n + 1
			for j < len(s) && (isDigitC(s[j]) || isAlphaC(s[j]) || s[j] == '_') {
				j++
			}
			if j < len(s) && s[j] == ')' {
				n = j + 1
			}
		}
		return math.NaN(), n
	}
	if len(rest) >= 3 && rest[0] == '0' && rest[1]|0x20 == 'x' &&
		(isHexDigitC(rest[2]) || (rest[2] == '.' && len(rest) >= 4 && isHexDigitC(rest[3]))) {
		return strtodHex(s, i+2, sign)
	}
	j := i
	digits := 0
	for j < len(s) && isDigitC(s[j]) {
		j++
		digits++
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && isDigitC(s[j]) {
			j++
			digits++
		}
	}
	if digits == 0 {
		return 0, 0
	}
	if j < len(s) && s[j]|0x20 == 'e' {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigitC(s[k]) {
			for k < len(s) && isDigitC(s[k]) {
				k++
			}
			j = k
		}
	}
	return sign * parseFloatC(string(s[i:j])), j
}

func strtodHex(s []byte, start int, sign float64) (float64, int) {
	j := start
	for j < len(s) && isHexDigitC(s[j]) {
		j++
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && isHexDigitC(s[j]) {
			j++
		}
	}
	mant := string(s[start:j])
	exp := "0"
	if j < len(s) && s[j]|0x20 == 'p' {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigitC(s[k]) {
			for k < len(s) && isDigitC(s[k]) {
				k++
			}
			exp = string(s[j+1 : k])
			j = k
		}
	}
	return sign * parseFloatC("0x"+mant+"p"+exp), j
}

// parseFloatC converts an already validated literal. Out-of-range values come
// back as ±Inf or 0 with ErrRange, which is what strtod returns too.
func parseFloatC(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0
	}
	return v
}
