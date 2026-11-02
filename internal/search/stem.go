package search

import "strings"

// snowballEnglish is the Snowball English (Porter2) stemmer RediSearch
// uses for english text: it works on runes, and anything outside a-z is a
// consonant.
func snowballEnglish(word string) string {
	w := []rune(word)
	if len(w) < 3 {
		return word
	}
	if s, ok := porterException1[word]; ok {
		return s
	}
	st := &porter{w: w}
	st.prelude()
	st.markRegions()
	st.step1a()
	if _, ok := porterException2[string(st.w)]; !ok {
		st.step1b()
		st.step1c()
		st.step2()
		st.step3()
		st.step4()
		st.step5()
	}
	for i, r := range st.w {
		if r == 'Y' {
			st.w[i] = 'y'
		}
	}
	return string(st.w)
}

var porterException1 = map[string]string{
	"skis": "ski", "skies": "sky", "dying": "die", "lying": "lie", "tying": "tie",
	"idly": "idl", "gently": "gentl", "ugly": "ugli", "early": "earli", "only": "onli", "singly": "singl",
	"sky": "sky", "news": "news", "howe": "howe", "atlas": "atlas", "cosmos": "cosmos", "bias": "bias", "andes": "andes",
}

var porterException2 = map[string]struct{}{
	"inning": {}, "outing": {}, "canning": {}, "herring": {}, "earring": {}, "proceed": {}, "exceed": {}, "succeed": {},
}

type porter struct {
	w      []rune
	p1, p2 int
}

func porterVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return true
	}
	return false
}

func (s *porter) prelude() {
	if s.w[0] == 'y' {
		s.w[0] = 'Y'
	}
	for i := 1; i < len(s.w); i++ {
		if s.w[i] == 'y' && porterVowel(s.w[i-1]) {
			s.w[i] = 'Y'
		}
	}
}

// pastVowelConsonant returns the position after the first non-vowel that
// follows a vowel at or after from, or len when there is none.
func (s *porter) pastVowelConsonant(from int) int {
	i := from
	for i < len(s.w) && !porterVowel(s.w[i]) {
		i++
	}
	if i >= len(s.w) {
		return len(s.w)
	}
	for i < len(s.w) && porterVowel(s.w[i]) {
		i++
	}
	if i >= len(s.w) {
		return len(s.w)
	}
	return i + 1
}

func (s *porter) markRegions() {
	word := string(s.w)
	s.p1 = -1
	for _, pre := range []string{"gener", "commun", "arsen"} {
		if strings.HasPrefix(word, pre) {
			s.p1 = len([]rune(pre))
		}
	}
	if s.p1 < 0 {
		s.p1 = s.pastVowelConsonant(0)
	}
	s.p2 = s.pastVowelConsonant(s.p1)
}

func (s *porter) hasSuffix(suf string) bool {
	n := len(suf)
	if n > len(s.w) {
		return false
	}
	for i := range n {
		if s.w[len(s.w)-n+i] != rune(suf[i]) {
			return false
		}
	}
	return true
}

// longest returns the longest of sufs that the word ends with.
func (s *porter) longest(sufs ...string) string {
	best := ""
	found := false
	for _, suf := range sufs {
		if s.hasSuffix(suf) && (!found || len(suf) > len(best)) {
			best, found = suf, true
		}
	}
	if !found {
		return "\x00"
	}
	return best
}

func (s *porter) replace(suf, with string) {
	s.w = append(s.w[:len(s.w)-len(suf)], []rune(with)...)
}

// shortAt reports whether the word up to end ends in a short syllable.
func (s *porter) shortAt(end int) bool {
	if end >= 3 {
		c := s.w[end-1]
		return !porterVowel(c) && c != 'w' && c != 'x' && c != 'Y' && porterVowel(s.w[end-2]) && !porterVowel(s.w[end-3])
	}
	return end == 2 && !porterVowel(s.w[1]) && porterVowel(s.w[0])
}

func (s *porter) hasVowel(end int) bool {
	for i := range end {
		if porterVowel(s.w[i]) {
			return true
		}
	}
	return false
}

func (s *porter) step1a() {
	switch suf := s.longest("sses", "ied", "ies", "s", "us", "ss"); suf {
	case "sses":
		s.replace(suf, "ss")
	case "ied", "ies":
		if len(s.w)-len(suf) >= 2 {
			s.replace(suf, "i")
		} else {
			s.replace(suf, "ie")
		}
	case "s":
		if s.hasVowel(len(s.w) - 2) {
			s.replace(suf, "")
		}
	}
}

func (s *porter) step1b() {
	switch suf := s.longest("eed", "eedly", "ed", "edly", "ing", "ingly"); suf {
	case "eed", "eedly":
		if len(s.w)-len(suf) >= s.p1 {
			s.replace(suf, "ee")
		}
	case "ed", "edly", "ing", "ingly":
		if !s.hasVowel(len(s.w) - len(suf)) {
			return
		}
		s.replace(suf, "")
		switch end := s.longest("at", "bl", "iz", "bb", "dd", "ff", "gg", "mm", "nn", "pp", "rr", "tt"); end {
		case "at", "bl", "iz":
			s.w = append(s.w, 'e')
		case "bb", "dd", "ff", "gg", "mm", "nn", "pp", "rr", "tt":
			s.w = s.w[:len(s.w)-1]
		default:
			if len(s.w) == s.p1 && s.shortAt(len(s.w)) {
				s.w = append(s.w, 'e')
			}
		}
	}
}

func (s *porter) step1c() {
	n := len(s.w)
	if n > 2 && (s.w[n-1] == 'y' || s.w[n-1] == 'Y') && !porterVowel(s.w[n-2]) {
		s.w[n-1] = 'i'
	}
}

func (s *porter) inR1(suf string) bool { return len(s.w)-len(suf) >= s.p1 }
func (s *porter) inR2(suf string) bool { return len(s.w)-len(suf) >= s.p2 }

func (s *porter) step2() {
	suf := s.longest("tional", "enci", "anci", "abli", "entli", "izer", "ization", "ational", "ation", "ator",
		"alism", "aliti", "alli", "fulness", "ousli", "ousness", "iveness", "iviti", "biliti", "bli", "ogi",
		"fulli", "lessli", "li")
	if suf == "\x00" || !s.inR1(suf) {
		return
	}
	switch suf {
	case "tional":
		s.replace(suf, "tion")
	case "enci":
		s.replace(suf, "ence")
	case "anci":
		s.replace(suf, "ance")
	case "abli":
		s.replace(suf, "able")
	case "entli":
		s.replace(suf, "ent")
	case "izer", "ization":
		s.replace(suf, "ize")
	case "ational", "ation", "ator":
		s.replace(suf, "ate")
	case "alism", "aliti", "alli":
		s.replace(suf, "al")
	case "fulness":
		s.replace(suf, "ful")
	case "ousli", "ousness":
		s.replace(suf, "ous")
	case "iveness", "iviti":
		s.replace(suf, "ive")
	case "biliti", "bli":
		s.replace(suf, "ble")
	case "ogi":
		if len(s.w) > 3 && s.w[len(s.w)-4] == 'l' {
			s.replace(suf, "og")
		}
	case "fulli":
		s.replace(suf, "ful")
	case "lessli":
		s.replace(suf, "less")
	case "li":
		if len(s.w) > 2 && strings.ContainsRune("cdeghkmnrt", s.w[len(s.w)-3]) {
			s.replace(suf, "")
		}
	}
}

func (s *porter) step3() {
	suf := s.longest("tional", "ational", "alize", "icate", "iciti", "ical", "ful", "ness", "ative")
	if suf == "\x00" || !s.inR1(suf) {
		return
	}
	switch suf {
	case "tional":
		s.replace(suf, "tion")
	case "ational":
		s.replace(suf, "ate")
	case "alize":
		s.replace(suf, "al")
	case "icate", "iciti", "ical":
		s.replace(suf, "ic")
	case "ful", "ness":
		s.replace(suf, "")
	case "ative":
		if s.inR2(suf) {
			s.replace(suf, "")
		}
	}
}

func (s *porter) step4() {
	suf := s.longest("al", "ance", "ence", "er", "ic", "able", "ible", "ant", "ement", "ment", "ent", "ism",
		"ate", "iti", "ous", "ive", "ize", "ion")
	if suf == "\x00" || !s.inR2(suf) {
		return
	}
	if suf == "ion" {
		if n := len(s.w) - 4; n >= 0 && (s.w[n] == 's' || s.w[n] == 't') {
			s.replace(suf, "")
		}
		return
	}
	s.replace(suf, "")
}

func (s *porter) step5() {
	n := len(s.w)
	switch {
	case s.hasSuffix("e"):
		if s.inR2("e") || s.inR1("e") && !s.shortAt(n-1) {
			s.w = s.w[:n-1]
		}
	case s.hasSuffix("l"):
		if s.inR2("l") && n > 1 && s.w[n-2] == 'l' {
			s.w = s.w[:n-1]
		}
	}
}
