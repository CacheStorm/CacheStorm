package resp

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// A command whose elements are not all bulk strings is a protocol error. The
// connection is closed after one, as in Redis, so the reader does not try to
// resynchronise the stream behind it.
func TestReadCommandRejectsNonBulkElements(t *testing.T) {
	for name, element := range map[string]string{
		"integer":     ":42\r\n",
		"simple":      "+SET\r\n",
		"error":       "-ERR value\r\n",
		"null bulk":   "$-1\r\n",
		"null array":  "*-1\r\n",
		"empty array": "*0\r\n",
		"nested":      "*1\r\n$1\r\nx\r\n",
		"null":        "_\r\n",
	} {
		for position, wire := range map[string]string{
			"command":   "*1\r\n" + element,
			"first arg": "*3\r\n$3\r\nSET\r\n" + element + "$1\r\nx\r\n",
			"last arg":  "*3\r\n$3\r\nSET\r\n$1\r\nx\r\n" + element,
		} {
			t.Run(name+"/"+position, func(t *testing.T) {
				r := NewReader(strings.NewReader(wire + "*1\r\n$4\r\nPING\r\n"))
				cmd, args, err := r.ReadCommand()
				var want error = ExpectedBulkError(element[0])
				if name == "null bulk" {
					want = ErrBulkStringTooBig
				}
				if err != want || cmd != "" || args != nil {
					t.Fatalf("ReadCommand(%q): got command=%q args=%q error=%v, want empty command, nil args, %v", wire, cmd, args, err, want)
				}
			})
		}
	}
}

func TestReadCommandPreservesBulkBytes(t *testing.T) {
	wire := "*3\r\n$4\r\nA\r\nB\r\n$0\r\n\r\n$4\r\na\x00\nb\r\n"
	r := NewReader(iotest.OneByteReader(strings.NewReader(wire)))
	cmd, args, err := r.ReadCommand()
	if err != nil || cmd != "A\r\nB" || len(args) != 2 {
		t.Fatalf("ReadCommand: got command=%q args=%q error=%v", cmd, args, err)
	}
	if args[0] == nil || len(args[0]) != 0 || string(args[1]) != "a\x00\nb" {
		t.Fatalf("bulk data changed: got args=%q", args)
	}
	if _, _, err := r.ReadCommand(); err != io.EOF {
		t.Fatalf("expected EOF after command, got %v", err)
	}
}
