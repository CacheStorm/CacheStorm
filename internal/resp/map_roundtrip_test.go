package resp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

var errMapIO = errors.New("injected map I/O failure")

type mapErrorReader struct{}

func (mapErrorReader) Read([]byte) (int, error) { return 0, errMapIO }

type mapErrorWriter struct{}

func (mapErrorWriter) Write([]byte) (int, error) { return 0, errMapIO }

func TestMapValueRoundTrip(t *testing.T) {
	ordinary := MapValue(map[string]*Value{"key": IntegerValue(7)})
	binary := MapValue(map[string]*Value{"a\r\n\x00": BulkBytes([]byte{0, '\r', '\n', 255}), "": BulkString("")})
	nested := MapValue(map[string]*Value{"inner": MapValue(map[string]*Value{"null": NullValue(), "bulk": NullBulkString(), "array": NullArray(), "list": ArrayValue([]*Value{IntegerValue(1), MapValue(map[string]*Value{})})}), "ok": SimpleString("OK"), "err": ErrorValue("ERR value")})
	large := MapValue(map[string]*Value{"large": BulkString(strings.Repeat("x", 8192))})
	cases := []struct {
		name        string
		input, want *Value
	}{
		{"ordinary", ordinary, ordinary},
		{"empty", MapValue(map[string]*Value{}), MapValue(map[string]*Value{})},
		{"nil map", MapValue(nil), MapValue(map[string]*Value{})},
		{"binary keys", binary, binary},
		{"nested", nested, nested},
		{"large", large, large},
	}
	modes := map[string]func(*Writer, *Value) error{
		"direct":   (*Writer).WriteValue,
		"buffered": (*Writer).WriteValueNoFlush,
		"array": func(w *Writer, v *Value) error {
			return w.WriteArray([]*Value{v, IntegerValue(5)})
		},
	}
	for mode, write := range modes {
		for _, tt := range cases {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				var output bytes.Buffer
				w := NewWriter(&output)
				if err := write(w, tt.input); err != nil {
					t.Fatalf("write: %v", err)
				}
				if mode == "buffered" && tt.name != "large" && output.Len() != 0 {
					t.Fatal("small buffered map forced an early flush")
				}
				if err := w.WriteInteger(99); err != nil {
					t.Fatalf("following reply: %v", err)
				}
				r := NewReader(iotest.OneByteReader(&output))
				v, err := r.ReadValue()
				if err != nil {
					t.Fatalf("read: %v", err)
				}
				if mode == "array" {
					if v.Type != TypeArray || len(v.Array) != 2 || v.Array[1].Int != 5 {
						t.Fatalf("outer array changed: %+v", v)
					}
					v = v.Array[0]
				}
				if !reflect.DeepEqual(v, tt.want) {
					t.Fatalf("roundtrip mismatch: got %#v, want %#v", v, tt.want)
				}
				v, err = r.ReadValue()
				if err != nil || v.Type != TypeInteger || v.Int != 99 {
					t.Fatalf("following reply changed: value=%v error=%v", v, err)
				}
				if _, err := r.ReadValue(); err != io.EOF {
					t.Fatalf("map consumed incorrect pair count: %v", err)
				}
			})
		}
		for _, input := range []*Value{ordinary, large} {
			w := NewWriter(mapErrorWriter{})
			err := write(w, input)
			if err == nil {
				err = w.Flush()
			}
			if !errors.Is(err, errMapIO) {
				t.Fatalf("%s lost write failure: %v", mode, err)
			}
		}
		for _, child := range []*Value{nil, {Type: Type(255)}} {
			var output bytes.Buffer
			if err := write(NewWriter(&output), MapValue(map[string]*Value{"bad": child})); err != ErrInvalidType {
				t.Fatalf("%s invalid child: expected ErrInvalidType, got %v", mode, err)
			}
		}
	}
}

func TestReadMapWireFormat(t *testing.T) {
	r := NewReader(strings.NewReader("%2\r\n+first\r\n:1\r\n+second\r\n:2\r\n:99\r\n"))
	v, err := r.ReadValue()
	if err != nil || v.Type != TypeMap || len(v.Map) != 2 || v.Map["first"].Int != 1 || v.Map["second"].Int != 2 {
		t.Fatalf("simple-key map: value=%#v error=%v", v, err)
	}
	v, err = r.ReadValue()
	if err != nil || v.Type != TypeInteger || v.Int != 99 {
		t.Fatalf("following frame: value=%v error=%v", v, err)
	}
	var output bytes.Buffer
	if err := NewWriter(&output).WriteValue(MapValue(map[string]*Value{"key": IntegerValue(7)})); err != nil {
		t.Fatal(err)
	}
	if output.String() != "%1\r\n$3\r\nkey\r\n:7\r\n" {
		t.Fatalf("map wire format: %q", output.String())
	}
	cmd, args, err := NewReader(strings.NewReader("%0\r\n")).ReadCommand()
	if err != ErrInvalidFormat || cmd != "" || args != nil {
		t.Fatalf("map accepted as command: command=%q args=%q error=%v", cmd, args, err)
	}
}

func TestReadMapRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		wire string
		want error
	}{
		{"%\r\n", ErrInvalidFormat},
		{"%bad\r\n", ErrInvalidFormat},
		{"%-1\r\n", ErrInvalidFormat},
		{"%-2\r\n", ErrInvalidFormat},
		{"%+1\r\n", ErrInvalidFormat},
		{"%18446744073709551616\r\n", ErrInvalidFormat},
		{fmt.Sprintf("%%%d\r\n", MaxArrayElements+1), ErrArrayTooLarge},
		{fmt.Sprintf("%%%d\r\n", MaxArrayElements), io.EOF},
		{"%1\r\n", io.EOF},
		{"%1\r\n+k\r\n", io.EOF},
		{"%1\r\n+k\r\n$3\r\nab", io.ErrUnexpectedEOF},
		{"%1\r\n:1\r\n+v\r\n", ErrInvalidFormat},
		{"%1\r\n$-1\r\n+v\r\n", ErrInvalidFormat},
		{"%1\r\n_\r\n+v\r\n", ErrInvalidFormat},
		{"%1\r\n*0\r\n+v\r\n", ErrInvalidFormat},
		{"%1\r\n%0\r\n+v\r\n", ErrInvalidFormat},
		{"%1\r\n+k\r\n!", ErrInvalidType},
	} {
		v, err := NewReader(strings.NewReader(tt.wire)).ReadValue()
		if v != nil || !errors.Is(err, tt.want) {
			t.Errorf("ReadValue(%q): value=%v error=%v, want %v", tt.wire, v, err, tt.want)
		}
	}
}

func TestReadMapPropagatesReadFailure(t *testing.T) {
	for _, prefix := range []string{"%1\r", "%1\r\n$3\r\nke", "%1\r\n+k\r\n$3\r\nab"} {
		v, err := NewReader(io.MultiReader(strings.NewReader(prefix), mapErrorReader{})).ReadValue()
		if v != nil || !errors.Is(err, errMapIO) {
			t.Fatalf("read failure for %q: value=%v error=%v", prefix, v, err)
		}
	}
}
