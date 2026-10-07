package resp

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestWriterRejectsNilValueTrees(t *testing.T) {
	modes := map[string]func(*Writer, *Value) error{
		"direct":   (*Writer).WriteValue,
		"buffered": (*Writer).WriteValueNoFlush,
		"array": func(w *Writer, v *Value) error {
			return w.WriteArray([]*Value{v})
		},
	}
	deep := ArrayValue([]*Value{nil})
	for i := 0; i < 8; i++ {
		deep = ArrayValue([]*Value{deep})
	}
	for mode, write := range modes {
		for name, input := range map[string]*Value{
			"top-level nil":  nil,
			"nil first":      ArrayValue([]*Value{nil, IntegerValue(7)}),
			"nil last":       ArrayValue([]*Value{IntegerValue(7), nil}),
			"nested nil":     deep,
			"map child":      MapValue(map[string]*Value{"nil": nil}),
			"map array":      MapValue(map[string]*Value{"nested": ArrayValue([]*Value{nil})}),
			"array map":      ArrayValue([]*Value{MapValue(map[string]*Value{"nil": nil})}),
			"unknown type":   {Type: Type(255)},
			"zero value":     {},
			"unknown child":  ArrayValue([]*Value{{Type: Type(255)}}),
			"large then nil": ArrayValue([]*Value{BulkString(strings.Repeat("x", 8192)), nil}),
		} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				var output bytes.Buffer
				if err := write(NewWriter(&output), input); err != ErrInvalidType {
					t.Fatalf("invalid value: got %v, want ErrInvalidType", err)
				}
			})
		}
		for name, input := range map[string]*Value{
			"null":       NullValue(),
			"null bulk":  NullBulkString(),
			"null array": NullArray(),
			"empty":      ArrayValue([]*Value{}),
			"nested": ArrayValue([]*Value{ArrayValue([]*Value{NullValue(), NullBulkString(), NullArray()}),
				MapValue(map[string]*Value{"k": BulkBytes([]byte{0, '\r', '\n', 255})}), IntegerValue(7)}),
		} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				var output bytes.Buffer
				w := NewWriter(&output)
				if err := write(w, input); err != nil {
					t.Fatal(err)
				}
				if mode == "buffered" && output.Len() != 0 {
					t.Fatal("small buffered value forced early flush")
				}
				if err := w.WriteInteger(99); err != nil {
					t.Fatal(err)
				}
				r := NewReader(&output)
				want := input
				if mode == "array" {
					want = ArrayValue([]*Value{input})
				}
				if got, err := r.ReadValue(); err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("valid control: value=%#v error=%v, want %#v", got, err, want)
				}
				if got, err := r.ReadValue(); err != nil || got.Type != TypeInteger || got.Int != 99 {
					t.Fatalf("following frame: value=%v error=%v", got, err)
				}
				if _, err := r.ReadValue(); err != io.EOF {
					t.Fatalf("extra frame: %v", err)
				}
			})
		}
	}
}

func TestWriterNilRejectionPreservesBufferedReply(t *testing.T) {
	for mode, write := range map[string]func(*Writer, *Value) error{
		"direct": (*Writer).WriteValue, "buffered": (*Writer).WriteValueNoFlush,
	} {
		t.Run(mode, func(t *testing.T) {
			var output bytes.Buffer
			w := NewWriter(&output)
			if err := w.WriteValueNoFlush(IntegerValue(5)); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				if err := write(w, nil); err != ErrInvalidType {
					t.Fatalf("nil write: %v", err)
				}
				if output.Len() != 0 {
					t.Fatal("nil write flushed pending reply")
				}
			}
			if err := w.WriteInteger(7); err != nil {
				t.Fatal(err)
			}
			if output.String() != ":5\r\n:7\r\n" {
				t.Fatalf("nil write changed buffered reply: %q", output.String())
			}
		})
	}
}
