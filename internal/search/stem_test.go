package search

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// testdata/snowball_english.txt pairs words with the stems Redis 8.2's
// RediSearch expands them to (FT.EXPLAIN's "+stem" terms).
func TestSnowballEnglishMatchesRediSearch(t *testing.T) {
	f, err := os.Open("testdata/snowball_english.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		word, want, _ := strings.Cut(sc.Text(), " ")
		if got := snowballEnglish(word); got != want {
			t.Errorf("%s: got %s, want %s", word, got, want)
		}
		n++
	}
	if n < 1000 {
		t.Fatalf("read %d words", n)
	}
}
