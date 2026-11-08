package store

import (
	"bytes"
	"strconv"
	"testing"
	"time"
)

// The slow log keeps what Redis keeps: at most 32 words of a command, the
// last replaced by a count of the rest, and 128 bytes of each argument.
func TestSlowLogTrimsArguments(t *testing.T) {
	sl := NewSlowLog(10)
	big := bytes.Repeat([]byte("x"), 1<<20)
	var many [][]byte
	for i := 0; i < 40; i++ {
		many = append(many, []byte("k"+strconv.Itoa(i)))
	}
	sl.AddIfSlow(time.Second, "SET", [][]byte{[]byte("k"), big}, "", "")
	sl.AddIfSlow(time.Second, "DEL", many, "", "")
	got := sl.Get(2)
	set, del := got[0], got[1]
	if set.Command != "SET" {
		set, del = del, set
	}
	if want := string(big[:128]) + "... (1048448 more bytes)"; string(set.Args[1]) != want {
		t.Errorf("SET value kept as %d bytes, want 128 plus a note", len(set.Args[1]))
	}
	if len(del.Args) != 31 || string(del.Args[29]) != "k29" || string(del.Args[30]) != "... (10 more arguments)" {
		t.Errorf("DEL args %q", del.Args)
	}
}
