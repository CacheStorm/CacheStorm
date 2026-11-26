package search

import (
	"errors"
	"strings"
)

// phoneticCode is the index term of a word's sound: "<" and its primary
// Double Metaphone code, which RediSearch indexes for a word of a PHONETIC
// field and adds to a query term; "" when the code is empty.
func phoneticCode(word string) string {
	if p, _ := doubleMetaphone(word); p != "" {
		return "<" + p
	}
	return ""
}

// phoneticIn reports whether mask holds a PHONETIC field.
func (idx *Index) phoneticIn(mask uint64) bool {
	for i := range idx.Fields {
		if idx.Fields[i].Phonetic && mask&fieldBit(i) != 0 {
			return true
		}
	}
	return false
}

// phoneticAttribute reads a $phonetic attribute: true or 1 turns sound
// matching on, false or 0 off.
func phoneticAttribute(v string) (int8, error) {
	switch strings.ToLower(v) {
	case "true", "1":
		return 1, nil
	case "false", "0":
		return 2, nil
	}
	return 0, errors.New("Invalid value (" + v + ") for `phonetic`") //nolint:staticcheck // RediSearch's reply text
}

// doubleMetaphone is Lawrence Philips' Double Metaphone as RediSearch runs
// it (dep/phonetics/double_metaphone.c, Maurice Aubrey's C version): the
// primary and secondary codes of a word, at most four characters each. It
// works on bytes, upper-casing ASCII only, and reads past the end of the
// word into five spaces of padding, as the C code does.
func doubleMetaphone(word string) (primary, secondary string) {
	length := len(word)
	last := length - 1
	b := []byte(word + "     ")
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	s := string(b)
	n := len(s)
	var p, q strings.Builder
	add := func(main, alt string) {
		p.WriteString(main)
		q.WriteString(alt)
	}
	at := func(pos int) byte {
		if pos < 0 || pos >= n {
			return 0
		}
		return s[pos]
	}
	vowel := func(pos int) bool {
		switch at(pos) {
		case 'A', 'E', 'I', 'O', 'U', 'Y':
			return true
		}
		return false
	}
	// stringAt reports whether one of opts (all of length l) starts at start.
	stringAt := func(start, l int, opts ...string) bool {
		if start < 0 || start >= n {
			return false
		}
		for _, o := range opts {
			if start+l <= n && s[start:start+l] == o {
				return true
			}
		}
		return false
	}
	slavoGermanic := strings.Contains(s, "W") || strings.Contains(s, "K") || strings.Contains(s, "CZ") || strings.Contains(s, "WITZ")
	germanic := func() bool { return stringAt(0, 4, "VAN ", "VON ") || stringAt(0, 3, "SCH") }

	current := 0
	if stringAt(0, 2, "GN", "KN", "PN", "WR", "PS") {
		current++
	}
	if at(0) == 'X' {
		add("S", "S")
		current++
	}
	for p.Len() < 4 || q.Len() < 4 {
		if current >= length {
			break
		}
		switch at(current) {
		case 'A', 'E', 'I', 'O', 'U', 'Y':
			if current == 0 {
				add("A", "A")
			}
			current++
		case 'B':
			add("P", "P")
			if at(current+1) == 'B' {
				current += 2
			} else {
				current++
			}
		case 0xC7: // Ç in Latin-1
			add("S", "S")
			current++
		case 'C':
			current = dmC(s, current, at, vowel, stringAt, germanic, add)
		case 'D':
			switch {
			case stringAt(current, 2, "DG"):
				if stringAt(current+2, 1, "I", "E", "Y") {
					add("J", "J")
					current += 3
				} else {
					add("TK", "TK")
					current += 2
				}
			case stringAt(current, 2, "DT", "DD"):
				add("T", "T")
				current += 2
			default:
				add("T", "T")
				current++
			}
		case 'F':
			if at(current+1) == 'F' {
				current += 2
			} else {
				current++
			}
			add("F", "F")
		case 'G':
			current = dmG(s, current, at, vowel, stringAt, germanic, slavoGermanic, add)
		case 'H':
			if (current == 0 || vowel(current-1)) && vowel(current+1) {
				add("H", "H")
				current += 2
			} else {
				current++
			}
		case 'J':
			if stringAt(current, 4, "JOSE") || stringAt(0, 4, "SAN ") {
				if (current == 0 && at(current+4) == ' ') || stringAt(0, 4, "SAN ") {
					add("H", "H")
				} else {
					add("J", "H")
				}
				current++
				break
			}
			if current == 0 && !stringAt(current, 4, "JOSE") {
				add("J", "A")
			} else if vowel(current-1) && !slavoGermanic && (at(current+1) == 'A' || at(current+1) == 'O') {
				add("J", "H")
			} else if current == last {
				add("J", "")
			} else if !stringAt(current+1, 1, "L", "T", "K", "S", "N", "M", "B", "Z") && !stringAt(current-1, 1, "S", "K", "L") {
				add("J", "J")
			}
			if at(current+1) == 'J' {
				current += 2
			} else {
				current++
			}
		case 'K':
			if at(current+1) == 'K' {
				current += 2
			} else {
				current++
			}
			add("K", "K")
		case 'L':
			if at(current+1) == 'L' {
				if (current == length-3 && stringAt(current-1, 4, "ILLO", "ILLA", "ALLE")) ||
					((stringAt(last-1, 2, "AS", "OS") || stringAt(last, 1, "A", "O")) && stringAt(current-1, 4, "ALLE")) {
					add("L", "")
					current += 2
					break
				}
				current += 2
			} else {
				current++
			}
			add("L", "L")
		case 'M':
			if (stringAt(current-1, 3, "UMB") && (current+1 == last || stringAt(current+2, 2, "ER"))) || at(current+1) == 'M' {
				current += 2
			} else {
				current++
			}
			add("M", "M")
		case 'N':
			if at(current+1) == 'N' {
				current += 2
			} else {
				current++
			}
			add("N", "N")
		case 0xD1: // Ñ in Latin-1
			current++
			add("N", "N")
		case 'P':
			if at(current+1) == 'H' {
				add("F", "F")
				current += 2
				break
			}
			if stringAt(current+1, 1, "P", "B") {
				current += 2
			} else {
				current++
			}
			add("P", "P")
		case 'Q':
			if at(current+1) == 'Q' {
				current += 2
			} else {
				current++
			}
			add("K", "K")
		case 'R':
			if current == last && !slavoGermanic && stringAt(current-2, 2, "IE") && !stringAt(current-4, 2, "ME", "MA") {
				add("", "R")
			} else {
				add("R", "R")
			}
			if at(current+1) == 'R' {
				current += 2
			} else {
				current++
			}
		case 'S':
			current = dmS(s, current, last, at, vowel, stringAt, slavoGermanic, add)
		case 'T':
			switch {
			case stringAt(current, 4, "TION"):
				add("X", "X")
				current += 3
			case stringAt(current, 3, "TIA", "TCH"):
				add("X", "X")
				current += 3
			case stringAt(current, 2, "TH") || stringAt(current, 3, "TTH"):
				if stringAt(current+2, 2, "OM", "AM") || germanic() {
					add("T", "T")
				} else {
					add("0", "T")
				}
				current += 2
			default:
				if stringAt(current+1, 1, "T", "D") {
					current += 2
				} else {
					current++
				}
				add("T", "T")
			}
		case 'V':
			if at(current+1) == 'V' {
				current += 2
			} else {
				current++
			}
			add("F", "F")
		case 'W':
			if stringAt(current, 2, "WR") {
				add("R", "R")
				current += 2
				break
			}
			if current == 0 && (vowel(current+1) || stringAt(current, 2, "WH")) {
				if vowel(current + 1) {
					add("A", "F")
				} else {
					add("A", "A")
				}
			}
			if (current == last && vowel(current-1)) || stringAt(current-1, 5, "EWSKI", "EWSKY", "OWSKI", "OWSKY") || stringAt(0, 3, "SCH") {
				add("", "F")
				current++
				break
			}
			if stringAt(current, 4, "WICZ", "WITZ") {
				add("TS", "FX")
				current += 4
				break
			}
			current++
		case 'X':
			if current != last || !stringAt(current-3, 3, "IAU", "EAU") && !stringAt(current-2, 2, "AU", "OU") {
				add("KS", "KS")
			}
			if stringAt(current+1, 1, "C", "X") {
				current += 2
			} else {
				current++
			}
		case 'Z':
			if at(current+1) == 'H' {
				add("J", "J")
				current += 2
				break
			} else if stringAt(current+1, 2, "ZO", "ZI", "ZA") || (slavoGermanic && current > 0 && at(current-1) != 'T') {
				add("S", "TS")
			} else {
				add("S", "S")
			}
			if at(current+1) == 'Z' {
				current += 2
			} else {
				current++
			}
		default:
			current++
		}
	}
	primary, secondary = p.String(), q.String()
	if len(primary) > 4 {
		primary = primary[:4]
	}
	if len(secondary) > 4 {
		secondary = secondary[:4]
	}
	return primary, secondary
}

type dmAt func(int) byte
type dmVowel func(int) bool
type dmStringAt func(int, int, ...string) bool
type dmAdd func(string, string)

func dmC(s string, current int, at dmAt, vowel dmVowel, stringAt dmStringAt, germanic func() bool, add dmAdd) int {
	// various germanic
	if current > 1 && !vowel(current-2) && stringAt(current-1, 3, "ACH") &&
		at(current+2) != 'I' && (at(current+2) != 'E' || stringAt(current-2, 6, "BACHER", "MACHER")) {
		add("K", "K")
		return current + 2
	}
	if current == 0 && stringAt(current, 6, "CAESAR") {
		add("S", "S")
		return current + 2
	}
	if stringAt(current, 4, "CHIA") {
		add("K", "K")
		return current + 2
	}
	if stringAt(current, 2, "CH") {
		if current > 0 && stringAt(current, 4, "CHAE") {
			add("K", "X")
			return current + 2
		}
		if current == 0 && (stringAt(current+1, 5, "HARAC", "HARIS") || stringAt(current+1, 3, "HOR", "HYM", "HIA", "HEM")) && !stringAt(0, 5, "CHORE") {
			add("K", "K")
			return current + 2
		}
		if germanic() || stringAt(current-2, 6, "ORCHES", "ARCHIT", "ORCHID") || stringAt(current+2, 1, "T", "S") ||
			((stringAt(current-1, 1, "A", "O", "U", "E") || current == 0) &&
				stringAt(current+2, 1, "L", "R", "N", "M", "B", "H", "F", "V", "W", " ")) {
			add("K", "K")
		} else if current > 0 {
			if stringAt(0, 2, "MC") {
				add("K", "K")
			} else {
				add("X", "K")
			}
		} else {
			add("X", "X")
		}
		return current + 2
	}
	if stringAt(current, 2, "CZ") && !stringAt(current-2, 4, "WICZ") {
		add("S", "X")
		return current + 2
	}
	if stringAt(current+1, 3, "CIA") {
		add("X", "X")
		return current + 3
	}
	if stringAt(current, 2, "CC") && (current != 1 || at(0) != 'M') {
		if stringAt(current+2, 1, "I", "E", "H") && !stringAt(current+2, 2, "HU") {
			if (current == 1 && at(current-1) == 'A') || stringAt(current-1, 5, "UCCEE", "UCCES") {
				add("KS", "KS")
			} else {
				add("X", "X")
			}
			return current + 3
		}
		add("K", "K")
		return current + 2
	}
	if stringAt(current, 2, "CK", "CG", "CQ") {
		add("K", "K")
		return current + 2
	}
	if stringAt(current, 2, "CI", "CE", "CY") {
		if stringAt(current, 3, "CIO", "CIE", "CIA") {
			add("S", "X")
		} else {
			add("S", "S")
		}
		return current + 2
	}
	add("K", "K")
	switch {
	case stringAt(current+1, 2, " C", " Q", " G"):
		return current + 3
	case stringAt(current+1, 1, "C", "K", "Q") && !stringAt(current+1, 2, "CE", "CI"):
		return current + 2
	}
	return current + 1
}

func dmG(s string, current int, at dmAt, vowel dmVowel, stringAt dmStringAt, germanic func() bool, slavoGermanic bool, add dmAdd) int {
	if at(current+1) == 'H' {
		if current > 0 && !vowel(current-1) {
			add("K", "K")
			return current + 2
		}
		if current < 3 && current == 0 {
			if at(current+2) == 'I' {
				add("J", "J")
			} else {
				add("K", "K")
			}
			return current + 2
		}
		if (current > 1 && stringAt(current-2, 1, "B", "H", "D")) ||
			(current > 2 && stringAt(current-3, 1, "B", "H", "D")) ||
			(current > 3 && stringAt(current-4, 1, "B", "H")) {
			return current + 2
		}
		if current > 2 && at(current-1) == 'U' && stringAt(current-3, 1, "C", "G", "L", "R", "T") {
			add("F", "F")
		} else if current > 0 && at(current-1) != 'I' {
			add("K", "K")
		}
		return current + 2
	}
	if at(current+1) == 'N' {
		if current == 1 && vowel(0) && !slavoGermanic {
			add("KN", "N")
		} else if !stringAt(current+2, 2, "EY") && at(current+1) != 'Y' && !slavoGermanic {
			add("N", "KN")
		} else {
			add("KN", "KN")
		}
		return current + 2
	}
	if stringAt(current+1, 2, "LI") && !slavoGermanic {
		add("KL", "L")
		return current + 2
	}
	if current == 0 && (at(current+1) == 'Y' || stringAt(current+1, 2, "ES", "EP", "EB", "EL", "EY", "IB", "IL", "IN", "IE", "EI", "ER")) {
		add("K", "J")
		return current + 2
	}
	if (stringAt(current+1, 2, "ER") || at(current+1) == 'Y') && !stringAt(0, 6, "DANGER", "RANGER", "MANGER") &&
		!stringAt(current-1, 1, "E", "I") && !stringAt(current-1, 3, "RGY", "OGY") {
		add("K", "J")
		return current + 2
	}
	if stringAt(current+1, 1, "E", "I", "Y") || stringAt(current-1, 4, "AGGI", "OGGI") {
		if germanic() || stringAt(current+1, 2, "ET") {
			add("K", "K")
		} else if stringAt(current+1, 4, "IER ") {
			add("J", "J")
		} else {
			add("J", "K")
		}
		return current + 2
	}
	add("K", "K")
	if at(current+1) == 'G' {
		return current + 2
	}
	return current + 1
}

func dmS(s string, current, last int, at dmAt, vowel dmVowel, stringAt dmStringAt, slavoGermanic bool, add dmAdd) int {
	if stringAt(current-1, 3, "ISL", "YSL") {
		return current + 1
	}
	if current == 0 && stringAt(current, 5, "SUGAR") {
		add("X", "S")
		return current + 1
	}
	if stringAt(current, 2, "SH") {
		if stringAt(current+1, 4, "HEIM", "HOEK", "HOLM", "HOLZ") {
			add("S", "S")
		} else {
			add("X", "X")
		}
		return current + 2
	}
	if stringAt(current, 3, "SIO", "SIA") || stringAt(current, 4, "SIAN") {
		if !slavoGermanic {
			add("S", "X")
		} else {
			add("S", "S")
		}
		return current + 3
	}
	if (current == 0 && stringAt(current+1, 1, "M", "N", "L", "W")) || stringAt(current+1, 1, "Z") {
		add("S", "X")
		if stringAt(current+1, 1, "Z") {
			return current + 2
		}
		return current + 1
	}
	if stringAt(current, 2, "SC") {
		if at(current+2) == 'H' {
			if stringAt(current+3, 2, "OO", "ER", "EN", "UY", "ED", "EM") {
				if stringAt(current+3, 2, "ER", "EN") {
					add("X", "SK")
				} else {
					add("SK", "SK")
				}
				return current + 3
			}
			if current == 0 && !vowel(3) && at(3) != 'W' {
				add("X", "S")
			} else {
				add("X", "X")
			}
			return current + 3
		}
		if stringAt(current+2, 1, "I", "E", "Y") {
			add("S", "S")
			return current + 3
		}
		add("SK", "SK")
		return current + 3
	}
	if current == last && stringAt(current-2, 2, "AI", "OI") {
		add("", "S")
	} else {
		add("S", "S")
	}
	if stringAt(current+1, 1, "S", "Z") {
		return current + 2
	}
	return current + 1
}
