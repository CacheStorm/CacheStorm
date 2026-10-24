package server

import (
	"strings"
	"testing"
)

func TestUnknownCommandErrorMatchesRedis(t *testing.T) {
	cases := []struct {
		raw  string
		args []string
		want string
	}{
		{"NOTACOMMAND", []string{"x"}, "ERR unknown command 'NOTACOMMAND', with args beginning with: 'x' "},
		{"notacommand", []string{"a", "b"}, "ERR unknown command 'notacommand', with args beginning with: 'a' 'b' "},
		{"BOGUS", nil, "ERR unknown command 'BOGUS', with args beginning with: "},
		{"X", []string{"a\r\nb"}, "ERR unknown command 'X', with args beginning with: 'a  b' "},
	}
	for _, c := range cases {
		args := make([][]byte, len(c.args))
		for i, a := range c.args {
			args[i] = []byte(a)
		}
		if got := unknownCommandError(c.raw, strings.ToUpper(c.raw), args); got != c.want {
			t.Errorf("%s %v: got %q want %q", c.raw, c.args, got, c.want)
		}
	}
	long := strings.Repeat("y", 300)
	got := unknownCommandError("X", "X", [][]byte{[]byte(long), []byte("z")})
	if strings.Contains(got, "'z'") || !strings.HasSuffix(got, "' ") {
		t.Fatalf("arguments must be cut at 128 bytes: %q", got)
	}
}
