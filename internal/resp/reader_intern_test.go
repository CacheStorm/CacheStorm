package resp

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// A client command's name is read into a buffer the reader reuses and
// matched against the interned names, so a known name allocates nothing
// and earlier commands keep their names and arguments.
func TestCommandNamesAreInterned(t *testing.T) {
	InternCommandNames([]string{"SET", "GET"})
	defer commandNames.Store(nil)
	input := "*3\r\n$3\r\nset\r\n$1\r\nk\r\n$1\r\nv\r\n" +
		"*2\r\n$3\r\nGET\r\n$1\r\nk\r\n" +
		"*2\r\n$7\r\nUNKNOWN\r\n$1\r\nx\r\n"
	r := NewReader(bufio.NewReader(strings.NewReader(input)))
	type read struct {
		name string
		args [][]byte
	}
	var got []read
	for range 3 {
		name, args, err := r.ReadClientCommand()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, read{name, args})
	}
	want := []read{{"SET", [][]byte{[]byte("k"), []byte("v")}}, {"GET", [][]byte{[]byte("k")}}, {"UNKNOWN", [][]byte{[]byte("x")}}}
	for i, g := range got {
		if g.name != want[i].name || len(g.args) != len(want[i].args) {
			t.Fatalf("command %d: %q %q, want %q %q", i, g.name, g.args, want[i].name, want[i].args)
		}
		for j := range g.args {
			if !bytes.Equal(g.args[j], want[i].args[j]) {
				t.Fatalf("command %d arg %d: %q, want %q", i, j, g.args[j], want[i].args[j])
			}
		}
	}

	cmd := []byte("*2\r\n$3\r\nGET\r\n$1\r\nk\r\n")
	src := bytes.NewReader(nil)
	br := bufio.NewReader(src)
	r = NewReader(br)
	allocs := testing.AllocsPerRun(100, func() {
		src.Reset(cmd)
		br.Reset(src)
		if _, _, err := r.ReadClientCommand(); err != nil {
			t.Fatal(err)
		}
	})
	// The argument slice and the key.
	if allocs > 2 {
		t.Fatalf("GET k allocates %v times, want 2", allocs)
	}
}
