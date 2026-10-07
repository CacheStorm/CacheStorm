package resp

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

var errSizeHeaderIO = errors.New("injected size header I/O failure")

type sizeHeaderErrorReader struct{}

func (sizeHeaderErrorReader) Read([]byte) (int, error) { return 0, errSizeHeaderIO }

func TestReadSizeHeaderRejectsSigns(t *testing.T) {
	for _, prefix := range []string{"$", "*"} {
		for _, number := range []string{"+0", "+1", "+3", "-0", "-00", "-01", "-001", "-2", "--1", "+-1", "-+1"} {
			header := prefix + number + "\r\n"
			for _, fragmented := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/fragmented=%v", header, fragmented), func(t *testing.T) {
					var input io.Reader = strings.NewReader(header + ":99\r\n")
					if fragmented {
						input = iotest.OneByteReader(input)
					}
					r := NewReader(input)
					if v, err := r.ReadValue(); v != nil || err != ErrInvalidFormat {
						t.Fatalf("header accepted: value=%v error=%v", v, err)
					}
					if v, err := r.ReadValue(); err != nil || v.Type != TypeInteger || v.Int != 99 {
						t.Fatalf("invalid header consumed payload: value=%v error=%v", v, err)
					}
				})
			}
		}
	}
	for _, wire := range []string{"*+2\r\n$3\r\nGET\r\n$1\r\nk\r\n", "*2\r\n$+3\r\nGET\r\n$1\r\nk\r\n", "*2\r\n$3\r\nGET\r\n$-0\r\n\r\n", "*1\r\n*+0\r\n", "%1\r\n+k\r\n$+0\r\n\r\n"} {
		if cmd, args, err := NewReader(strings.NewReader(wire)).ReadCommand(); cmd != "" || args != nil || err != ErrInvalidFormat {
			t.Errorf("ReadCommand(%q): command=%q args=%q error=%v", wire, cmd, args, err)
		}
	}
}

func TestReadSizeHeaderValidControls(t *testing.T) {
	for _, tt := range []struct {
		wire string
		want *Value
	}{
		{"$-1\r\n", NullBulkString()},
		{"*-1\r\n", NullArray()},
		{"$0\r\n\r\n", BulkString("")},
		{"*0\r\n", ArrayValue([]*Value{})},
		{"$01\r\nx\r\n", BulkString("x")},
		{"*01\r\n:+7\r\n", ArrayValue([]*Value{IntegerValue(7)})},
		{":-0\r\n", IntegerValue(0)},
		{":-01\r\n", IntegerValue(-1)},
		{":+9223372036854775807\r\n", IntegerValue(9223372036854775807)},
		{":-9223372036854775808\r\n", IntegerValue(-9223372036854775808)},
		{"*2\r\n$3\r\nGET\r\n$1\r\nk\r\n", ArrayValue([]*Value{BulkString("GET"), BulkString("k")})},
	} {
		r := NewReader(iotest.OneByteReader(strings.NewReader(tt.wire + ":99\r\n")))
		if v, err := r.ReadValue(); err != nil || !reflect.DeepEqual(v, tt.want) {
			t.Errorf("ReadValue(%q): value=%#v error=%v, want %#v", tt.wire, v, err, tt.want)
		}
		if v, err := r.ReadValue(); err != nil || v.Type != TypeInteger || v.Int != 99 {
			t.Errorf("following frame for %q: value=%v error=%v", tt.wire, v, err)
		}
	}
}

func TestReadSizeHeaderLimitsAndReadFailures(t *testing.T) {
	for _, tt := range []struct {
		wire string
		want error
	}{
		{fmt.Sprintf("$%d\r\n", MaxBulkStringSize+1), ErrBulkStringTooBig},
		{fmt.Sprintf("*%d\r\n", MaxArrayElements+1), ErrArrayTooLarge},
		{"$9223372036854775808\r\n", ErrInvalidFormat},
		{"*9223372036854775808\r\n", ErrInvalidFormat},
		{"$\r\n", ErrInvalidFormat},
		{"*\r\n", ErrInvalidFormat},
		{"$1x\r\n", ErrInvalidFormat},
		{"*1x\r\n", ErrInvalidFormat},
		{"$1\r\n", io.EOF},
		{"*1\r\n", io.EOF},
		{"$0\r\n\r", io.EOF},
		{"$2\r\nx", io.ErrUnexpectedEOF},
	} {
		if v, err := NewReader(strings.NewReader(tt.wire)).ReadValue(); v != nil || !errors.Is(err, tt.want) {
			t.Errorf("ReadValue(%q): value=%v error=%v, want %v", tt.wire, v, err, tt.want)
		}
	}
	for _, prefix := range []string{"$+", "*-", "$1\r", "*1\r", "$1\r\n", "*1\r\n", "$0\r\n\r", "*1\r\n$1\r\n"} {
		input := io.MultiReader(strings.NewReader(prefix), sizeHeaderErrorReader{})
		if v, err := NewReader(input).ReadValue(); v != nil || !errors.Is(err, errSizeHeaderIO) {
			t.Errorf("read failure for %q: value=%v error=%v", prefix, v, err)
		}
	}
}
