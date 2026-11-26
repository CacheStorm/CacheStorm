package search

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// testdata/double_metaphone_redis82.txt holds words and the primary code
// Redis 8.2's RediSearch gives each ("-" for none), read from FT.EXPLAIN
// of a PHONETIC field: a sample of 63,000 dictionary words, which all
// agreed, and names that reach the algorithm's special cases.
func TestDoubleMetaphoneMatchesRediSearch(t *testing.T) {
	f, err := os.Open("testdata/double_metaphone_redis82.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		word, want, _ := strings.Cut(sc.Text(), " ")
		got := strings.TrimPrefix(phoneticCode(word), "<")
		if got == "" {
			got = "-"
		}
		if got != want {
			t.Errorf("%s: %s, RediSearch %s", word, got, want)
		}
		n++
	}
	if n < 1000 {
		t.Fatalf("only %d words", n)
	}
}
