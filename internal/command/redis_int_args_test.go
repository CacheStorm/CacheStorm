package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func intArgsRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	RegisterSortedSetCommands(router)
	RegisterHashCommands(router)
	RegisterBitmapCommands(router)
	return router
}

// Integer arguments are read with string2ll's syntax, as Redis reads them:
// "+1", "01", "00" and "-0" are not integers, "-1" and "0" are.
func TestIntegerArgumentsUseStringToLongLongSyntax(t *testing.T) {
	s := store.NewStore()
	router := intArgsRouter()
	runCmd(t, s, router, "RPUSH", "l", "a", "b", "c")
	runCmd(t, s, router, "ZADD", "z", "1", "a")
	runCmd(t, s, router, "SET", "k", "hello")
	for _, bad := range []string{"+1", "01", "00", "-0", " 1", "1_0"} {
		for _, cmd := range [][]string{{"LRANGE", "l", bad, "-1"}, {"ZRANGE", "z", bad, "-1"}, {"HINCRBY", "h", "f", bad}, {"BITCOUNT", "k", bad, "-1"}, {"SINTERCARD", bad, "s"}} {
			if got := runCmd(t, s, router, cmd[0], cmd[1:]...); !strings.HasPrefix(got, "-ERR") {
				t.Errorf("%v: %q, want an error", cmd, got)
			}
		}
	}
	if got := runCmd(t, s, router, "LRANGE", "l", "-1", "-1"); got != "*1\r\n$1\r\nc\r\n" {
		t.Errorf("LRANGE -1 -1: %q", got)
	}
}

// A key count near the int64 limit is refused, not added to an index where
// it would overflow into a negative slice bound.
func TestHugeKeyCountsAreRefused(t *testing.T) {
	s := store.NewStore()
	router := intArgsRouter()
	for _, cmd := range [][]string{
		{"SINTERCARD", "9223372036854775807", "s"},
		{"ZINTERCARD", "9223372036854775807", "z"},
		{"LMPOP", "9223372036854775807", "l", "LEFT"},
		{"ZMPOP", "9223372036854775807", "z", "MIN"},
		{"ZUNIONSTORE", "d", "9223372036854775807", "z"},
	} {
		if got := runCmd(t, s, router, cmd[0], cmd[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("%v: %q, want an error", cmd, got)
		}
		args := make([][]byte, len(cmd)-1)
		for i, a := range cmd[1:] {
			args[i] = []byte(a)
		}
		_ = CommandKeys(cmd[0], args)
	}
	if keys := CommandKeys("FCALL", [][]byte{[]byte("f"), []byte("9223372036854775807"), []byte("k")}); keys != nil {
		t.Errorf("FCALL with a huge key count: keys %q", keys)
	}
}

// SCAN cursors are read as string2ull reads them.
func TestParseScanCursorMatchesStringToUnsignedLongLong(t *testing.T) {
	for in, want := range map[string]uint64{
		"0": 0, "17": 17, "+1": 1, "01": 1, "-0": 0, " 5": 5, "\t5": 5,
		"18446744073709551615": 18446744073709551615, "-1": 0,
	} {
		got, ok := parseScanCursor(in)
		if in == "-1" {
			if ok {
				t.Errorf("%q accepted", in)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("%q: %d %v, want %d", in, got, ok, want)
		}
	}
	if got, ok := parseScanCursor("-01"); !ok || got != 18446744073709551615 {
		t.Errorf("-01: %d %v, want strtoull's wrap to 2^64-1", got, ok)
	}
	for _, in := range []string{"", " ", "+", "1 ", "1_0", "0x10", "18446744073709551616", "abc"} {
		if _, ok := parseScanCursor(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
}
