package command

import (
	"slices"
	"testing"
)

// Commands the hand-written key tables do not name take their keys from
// Redis' key specs: the write gate, ACL key patterns and cluster routing all
// see every key, not the first argument.
func TestCommandKeysFromSpecs(t *testing.T) {
	b := func(s ...string) [][]byte {
		out := make([][]byte, len(s))
		for i, x := range s {
			out[i] = []byte(x)
		}
		return out
	}
	for _, tc := range []struct {
		cmd  string
		args [][]byte
		want []string
	}{
		{"MSETEX", b("2", "a", "1", "b", "2", "EX", "10"), []string{"a", "b"}},
		{"LMOVEM", b("src", "dst", "LEFT", "RIGHT", "COUNT", "2", "OBO"), []string{"src", "dst"}},
		{"BLMOVEM", b("src", "dst", "LEFT", "RIGHT", "0"), []string{"src", "dst"}},
		{"SUNIONCARD", b("2", "x", "y", "APPROX"), []string{"x", "y"}},
		{"SDIFFCARD", b("3", "x", "y", "z", "LIMIT", "1"), []string{"x", "y", "z"}},
		{"DELEX", b("k", "IFEQ", "v"), []string{"k"}},
		{"DIGEST", b("k"), []string{"k"}},
	} {
		if got := CommandKeys(tc.cmd, tc.args); !slices.Equal(got, tc.want) {
			t.Errorf("CommandKeys(%s %q) = %q, want %q", tc.cmd, tc.args, got, tc.want)
		}
		_, sole := SoleFirstKey(tc.cmd, tc.args)
		if wantSole := len(tc.want) == 1 && string(tc.args[0]) == tc.want[0]; sole != wantSole {
			t.Errorf("SoleFirstKey(%s) ok = %v, want %v", tc.cmd, sole, wantSole)
		}
	}
}
