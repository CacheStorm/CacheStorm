package command

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ldNum is an x87 long double as AROP holds it: a 64-bit mantissa value, or
// an infinity, or NaN once opposite infinities met.
type ldNum struct {
	f   *big.Float
	inf int
	nan bool
}

var ldMaxInt64 = new(big.Float).SetPrec(longDoublePrec).SetInt64(math.MaxInt64)
var ldMinInt64 = new(big.Float).SetPrec(longDoublePrec).SetInt64(math.MinInt64)

func ldFromInt(i int64) ldNum { return ldNum{f: new(big.Float).SetPrec(longDoublePrec).SetInt64(i)} }

func ldFromFloat(d float64) ldNum {
	return ldNum{f: new(big.Float).SetPrec(longDoublePrec).SetFloat64(d)}
}

// arrayNumber is how AROP reads an array value: an integer, a double kept
// inline, or else the string as Redis' string2ll and string2ld read it.
func arrayNumber(s string) (n ldNum, i int64, isInt, ok bool) {
	kind, iv, fv := store.ClassifyArrayValue(s)
	switch kind {
	case store.ArrayInt:
		return ldFromInt(iv), iv, true, true
	case store.ArrayFloat:
		return ldFromFloat(fv), 0, false, true
	}
	if v, ok := parseRedisInt([]byte(s)); ok {
		return ldFromInt(v), v, true, true
	}
	if len(s) >= 5*1024 {
		return ldNum{}, 0, false, false
	}
	f, ok := parseLongDouble(s)
	if !ok {
		return ldNum{}, 0, false, false
	}
	if f.IsInf() {
		if f.Sign() < 0 {
			return ldNum{inf: -1}, 0, false, true
		}
		return ldNum{inf: 1}, 0, false, true
	}
	return ldNum{f: f}, 0, false, true
}

func (a ldNum) add(b ldNum) ldNum {
	switch {
	case a.nan || b.nan:
		return ldNum{nan: true}
	case a.inf != 0 && b.inf != 0:
		if a.inf != b.inf {
			return ldNum{nan: true}
		}
		return a
	case a.inf != 0:
		return a
	case b.inf != 0:
		return b
	}
	sum := new(big.Float).SetPrec(longDoublePrec).SetMode(big.ToNearestEven).Add(a.f, b.f)
	if sum.Sign() != 0 && sum.MantExp(nil) > longDoubleMaxExp {
		return ldNum{inf: sum.Sign()}
	}
	return ldNum{f: sum}
}

// cmp orders a and b; neither is NaN.
func (a ldNum) cmp(b ldNum) int {
	if a.inf != 0 || b.inf != 0 {
		switch {
		case a.inf == b.inf:
			return 0
		case a.inf > b.inf:
			return 1
		}
		return -1
	}
	return a.f.Cmp(b.f)
}

// int64Part is AROP's truncation of a number for AND, OR and XOR; false
// when it does not fit an int64.
func (a ldNum) int64Part() (int64, bool) {
	if a.nan || a.inf != 0 || a.f.Cmp(ldMinInt64) < 0 || a.f.Cmp(ldMaxInt64) > 0 {
		return 0, false
	}
	i, _ := a.f.Int64()
	return i, true
}

// format is ld2string(LD_STR_AUTO): "%.17Lg".
func (a ldNum) format() string {
	switch {
	case a.nan:
		return "nan"
	case a.inf > 0:
		return "inf"
	case a.inf < 0:
		return "-inf"
	}
	return formatLongDoubleG17(a.f)
}

// formatLongDoubleG17 is C's "%.17Lg" of v.
func formatLongDoubleG17(v *big.Float) string {
	e := v.Text('e', 16)
	at := strings.LastIndexByte(e, 'e')
	exp, err := strconv.Atoi(e[at+1:])
	if err != nil {
		return e
	}
	if exp < -4 || exp >= 17 {
		mant := trimFraction(e[:at])
		sign := "+"
		if exp < 0 {
			sign, exp = "-", -exp
		}
		es := strconv.Itoa(exp)
		if len(es) < 2 {
			es = "0" + es
		}
		return mant + "e" + sign + es
	}
	return trimFraction(v.Text('f', 16-exp))
}

func trimFraction(s string) string {
	if strings.IndexByte(s, '.') < 0 {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// asciiLower lowercases ASCII letters only, as ARGREP's NOCASE does.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func asciiContains(haystack, needle string, nocase bool) bool {
	if nocase {
		return strings.Contains(asciiLower(haystack), asciiLower(needle))
	}
	return strings.Contains(haystack, needle)
}

func asciiEqual(a, b string, nocase bool) bool {
	if nocase {
		return len(a) == len(b) && asciiLower(a) == asciiLower(b)
	}
	return a == b
}

var errArgrepBackrefs = errors.New("ERR regular expression backreferences are not supported")

// compileArgrepRegex compiles an ARGREP RE pattern, a POSIX extended
// regular expression Redis runs with TRE, as Go's regexp: anchors match
// only at the ends and '.' matches a newline, as without REG_NEWLINE. The
// errors carry TRE's messages.
func compileArgrepRegex(pattern string, nocase bool) (*regexp.Regexp, error) {
	for i := 0; i+1 < len(pattern); i++ {
		if pattern[i] == '\\' {
			if c := pattern[i+1]; c >= '1' && c <= '9' {
				return nil, errArgrepBackrefs
			}
			i++
		}
	}
	flags := "(?s)"
	if nocase {
		flags = "(?si)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		msg := "Invalid regexp"
		var se *syntax.Error
		if errors.As(err, &se) {
			switch se.Code {
			case syntax.ErrMissingBracket:
				msg = "Missing ']'"
			case syntax.ErrMissingParen, syntax.ErrUnexpectedParen:
				msg = "Missing ')'"
			case syntax.ErrInvalidCharRange:
				msg = "Invalid character range"
			case syntax.ErrTrailingBackslash:
				msg = "Trailing backslash"
			case syntax.ErrInvalidCharClass:
				msg = "Unknown character class name"
			case syntax.ErrMissingRepeatArgument, syntax.ErrInvalidRepeatOp:
				msg = "Invalid use of repetition operators"
			case syntax.ErrInvalidRepeatSize:
				msg = "Invalid contents of {}"
			}
		}
		return nil, errors.New("ERR invalid regular expression: " + msg)
	}
	return re, nil
}
