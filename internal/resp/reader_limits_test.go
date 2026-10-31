package resp

import (
	"strconv"
	"strings"
	"testing"
)

func TestReadClientCommandLimits(t *testing.T) {
	read := func(wire string, unauth bool) ([][]byte, error) {
		r := NewReader(strings.NewReader(wire))
		r.SetUnauthenticated(unauth)
		_, args, err := r.ReadClientCommand()
		return args, err
	}

	var b strings.Builder
	n := MaxArrayElements + 2
	b.WriteString("*" + strconv.Itoa(n+1) + "\r\n$4\r\nSADD\r\n")
	for i := 0; i < n; i++ {
		b.WriteString("$1\r\nx\r\n")
	}
	if args, err := read(b.String(), false); err != nil || len(args) != n {
		t.Fatalf("a command with more than MaxArrayElements arguments: %d args, %v", len(args), err)
	}
	if _, err := read("*2147483648\r\n", false); err != ErrArrayTooLarge {
		t.Fatalf("a count above INT_MAX: %v", err)
	}

	eleven := "*11\r\n" + strings.Repeat("$1\r\na\r\n", 11)
	if _, err := read(eleven, true); err != ErrUnauthenticatedMultibulk {
		t.Fatalf("11 arguments before AUTH: %v", err)
	}
	if _, err := read(eleven, false); err != nil {
		t.Fatalf("11 arguments after AUTH: %v", err)
	}
	big := "*2\r\n$4\r\nAUTH\r\n$16385\r\n" + strings.Repeat("p", 16385) + "\r\n"
	if _, err := read(big, true); err != ErrUnauthenticatedBulk {
		t.Fatalf("a 16385-byte argument before AUTH: %v", err)
	}

	defer SetQueryBufferLimit(1 << 30)
	SetQueryBufferLimit(1 << 20)
	value := strings.Repeat("v", 600<<10)
	two := "*3\r\n$3\r\nSET\r\n$" + strconv.Itoa(len(value)) + "\r\n" + value + "\r\n$" + strconv.Itoa(len(value)) + "\r\n" + value + "\r\n"
	if _, err := read(two, false); err != ErrQueryBufferLimit {
		t.Fatalf("a command over client-query-buffer-limit: %v", err)
	}
	one := "*2\r\n$3\r\nGET\r\n$" + strconv.Itoa(len(value)) + "\r\n" + value + "\r\n"
	if _, err := read(one, false); err != nil {
		t.Fatalf("a command under client-query-buffer-limit: %v", err)
	}

	defer SetProtoMaxBulkLen(MaxBulkStringSize)
	SetProtoMaxBulkLen(1 << 10)
	if _, err := read("*2\r\n$3\r\nGET\r\n$1025\r\n", false); err != ErrBulkStringTooBig {
		t.Fatalf("an argument over proto-max-bulk-len: %v", err)
	}
}

func TestReadCommandIgnoresClientLimits(t *testing.T) {
	defer SetQueryBufferLimit(1 << 30)
	defer SetProtoMaxBulkLen(MaxBulkStringSize)
	SetQueryBufferLimit(1 << 20)
	SetProtoMaxBulkLen(1 << 20)
	value := strings.Repeat("v", 2<<20)
	wire := "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$" + strconv.Itoa(len(value)) + "\r\n" + value + "\r\n"
	r := NewReader(strings.NewReader(wire))
	r.SetUnauthenticated(true)
	if _, args, err := r.ReadCommand(); err != nil || len(args) != 2 {
		t.Fatalf("an AOF command over the client limits: %v", err)
	}
}
