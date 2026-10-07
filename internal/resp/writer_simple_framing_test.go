package resp

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWriteSimpleStringPreservesReplyFraming(t *testing.T) {
	modes := map[string]func(*Writer, string) error{
		"direct": (*Writer).WriteSimpleString,
		"value": func(w *Writer, s string) error {
			return w.WriteValue(SimpleString(s))
		},
		"buffered": func(w *Writer, s string) error {
			return w.WriteValueNoFlush(SimpleString(s))
		},
		"nested": func(w *Writer, s string) error {
			return w.WriteArray([]*Value{ArrayValue([]*Value{SimpleString(s)}), BulkString("a\r\nb")})
		},
	}
	cases := []struct {
		name, message, expected string
	}{
		{"split", "OK\r\n:99", "OK  :99"},
		{"ordinary", "OK", "OK"},
		{"empty", "", ""},
		{"CR", "\r", " "},
		{"LF", "\n", " "},
		{"unicode", "ü\r\n+OK\n", "ü  +OK "},
		{"large", strings.Repeat("x", 8192) + "\r\n:99", strings.Repeat("x", 8192) + "  :99"},
	}
	for mode, write := range modes {
		for _, tt := range cases {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				var output bytes.Buffer
				w := NewWriter(&output)
				if err := write(w, tt.message); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				if mode == "buffered" && len(tt.message) < 4090 && output.Len() != 0 {
					t.Fatal("buffered write forced an early flush")
				}
				if err := w.WriteInteger(7); err != nil {
					t.Fatalf("following reply: %v", err)
				}
				r := NewReader(&output)
				v, err := r.ReadValue()
				if err != nil {
					t.Fatalf("first reply: %v", err)
				}
				if mode == "nested" {
					if v.Type != TypeArray || len(v.Array) != 2 || v.Array[0].Type != TypeArray || len(v.Array[0].Array) != 1 || string(v.Array[1].Bulk) != "a\r\nb" {
						t.Fatalf("nested structure/bulk bytes changed: %+v", v)
					}
					v = v.Array[0].Array[0]
				}
				if v.Type != TypeSimpleString || v.Str != tt.expected {
					t.Fatalf("got value=%v type=%v, want simple string %q", v, v.Type, tt.expected)
				}
				v, err = r.ReadValue()
				if err != nil || v.Type != TypeInteger || v.Int != 7 {
					t.Fatalf("following reply corrupted: value=%v error=%v", v, err)
				}
				if _, err := r.ReadValue(); err != io.EOF {
					t.Fatalf("expected EOF with no extra replies, got %v", err)
				}
			})
		}
		t.Run(mode+"/write failure", func(t *testing.T) {
			w := NewWriter(&flushErrWriter{})
			err := write(w, "OK\r\nfailure")
			if err == nil {
				err = w.Flush()
			}
			if err == nil || !strings.Contains(err.Error(), "write error") {
				t.Fatalf("underlying error lost: %v", err)
			}
		})
	}
}
