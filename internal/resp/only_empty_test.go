package resp

import "testing"

// Only complete empty commands count as nothing to come; a partial command,
// or any real one, is still to come.
func TestOnlyEmptyCommands(t *testing.T) {
	for _, c := range []struct {
		buf  string
		want bool
	}{
		{"", true},
		{"\r\n", true},
		{"*0\r\n", true},
		{"*-1\r\n\r\n  \r\n*0\r\n", true},
		{"*0\r\n*1\r\n$4\r\nPING\r\n", false},
		{"*1\r\n$4\r\nPI", false},
		{"*", false},
		{"PING\r\n", false},
		{"PIN", false},
		{"\r\n*2", false},
	} {
		if got := OnlyEmptyCommands([]byte(c.buf)); got != c.want {
			t.Errorf("OnlyEmptyCommands(%q) = %v", c.buf, got)
		}
	}
}
