package resp

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"
)

func TestReadClientCommandInline(t *testing.T) {
	for _, tc := range []struct {
		wire string
		want []string
	}{
		{"PING\r\n", []string{"PING"}},
		{"set k v\n", []string{"set", "k", "v"}},
		{"  SET   k   v  \r\n", []string{"SET", "k", "v"}},
		{"SET k \"a b\\n\\x41\"\r\n", []string{"SET", "k", "a b\nA"}},
		{"SET k 'it\\'s'\r\n", []string{"SET", "k", "it's"}},
		{"SET k \"\"\r\n", []string{"SET", "k", ""}},
	} {
		r := NewReader(strings.NewReader(tc.wire))
		cmd, args, err := r.ReadClientCommand()
		if err != nil {
			t.Fatalf("%q: %v", tc.wire, err)
		}
		got := []string{cmd}
		for _, a := range args {
			got = append(got, string(a))
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Fatalf("%q parsed as %q, want %q", tc.wire, got, tc.want)
		}
	}
}

func TestReadClientCommandSkipsEmptyLinesAndMixesForms(t *testing.T) {
	r := NewReader(strings.NewReader("\r\n\nPING\r\n*1\r\n$4\r\nECHO\r\n"))
	for _, want := range []string{"PING", "ECHO"} {
		cmd, _, err := r.ReadClientCommand()
		if err != nil || cmd != want {
			t.Fatalf("got %q %v, want %s", cmd, err, want)
		}
	}
}

func TestReadClientCommandRejectsUnbalancedQuotes(t *testing.T) {
	for _, wire := range []string{"SET k \"abc\r\n", "SET k 'abc\r\n", "SET k \"a\"b\r\n"} {
		if _, _, err := NewReader(strings.NewReader(wire)).ReadClientCommand(); err != ErrUnbalancedQuote {
			t.Fatalf("%q: err %v, want ErrUnbalancedQuote", wire, err)
		}
	}
}

func TestReadCommandIsNotInline(t *testing.T) {
	if _, _, err := NewReader(strings.NewReader("PING\r\n")).ReadCommand(); err != ErrInvalidFormat {
		t.Fatalf("ReadCommand accepted an inline command: %v", err)
	}
}

// A nested array used to recurse once per level, so a few megabytes of
// "*1\r\n" overflowed the goroutine stack, a fatal error that no recover can
// catch: any client could crash the server before authenticating.
func TestReadCommandRejectsNestedArrayWithoutRecursing(t *testing.T) {
	payload := strings.Repeat("*1\r\n", 5_000_000)
	_, _, err := NewReader(strings.NewReader(payload)).ReadClientCommand()
	if err != ErrInvalidFormat {
		t.Fatalf("nested arrays: err %v, want ErrInvalidFormat", err)
	}
}

func TestReadCommandBoundsLineLength(t *testing.T) {
	for _, payload := range []string{
		strings.Repeat("A", MaxInlineSize+1),
		"*" + strings.Repeat("1", MaxInlineSize+1),
	} {
		_, _, err := NewReader(strings.NewReader(payload)).ReadClientCommand()
		if err != ErrLineTooLong {
			t.Fatalf("over-long line: err %v, want ErrLineTooLong", err)
		}
	}
}

// A header announcing a 512 MB argument must not allocate it before the
// bytes arrive.
func TestReadCommandDoesNotPreallocateAnnouncedBulk(t *testing.T) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	wire := "*2\r\n$3\r\nSET\r\n$536870912\r\nonly-a-few-bytes"
	_, _, err := NewReader(strings.NewReader(wire)).ReadClientCommand()
	runtime.ReadMemStats(&after)
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("truncated bulk: err %v, want io.ErrUnexpectedEOF", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 16<<20 {
		t.Fatalf("reading a 16-byte body announced as 512 MB allocated %d bytes", grew)
	}
}

func TestReadCommandReadsLargeBulkInChunks(t *testing.T) {
	body := bytes.Repeat([]byte{0xAB}, 3<<20+7)
	wire := append([]byte("*2\r\n$3\r\nSET\r\n$3145735\r\n"), body...)
	wire = append(wire, '\r', '\n')
	cmd, args, err := NewReader(bytes.NewReader(wire)).ReadCommand()
	if err != nil || cmd != "SET" || !bytes.Equal(args[0], body) {
		t.Fatalf("large bulk: cmd %q err %v len %d", cmd, err, len(args))
	}
}

func TestReadValueBoundsNesting(t *testing.T) {
	deep := strings.Repeat("*1\r\n", MaxNestingDepth+1) + ":1\r\n"
	if _, err := NewReader(strings.NewReader(deep)).ReadValue(); err != ErrInvalidFormat {
		t.Fatalf("nesting past the limit: err %v, want ErrInvalidFormat", err)
	}
	ok := strings.Repeat("*1\r\n", MaxNestingDepth-1) + ":1\r\n"
	if _, err := NewReader(strings.NewReader(ok)).ReadValue(); err != nil {
		t.Fatalf("nesting within the limit: %v", err)
	}
	maps := strings.Repeat("%1\r\n+k\r\n", MaxNestingDepth+1) + ":1\r\n"
	if _, err := NewReader(strings.NewReader(maps)).ReadValue(); err != ErrInvalidFormat {
		t.Fatalf("map nesting past the limit: err %v, want ErrInvalidFormat", err)
	}
}
