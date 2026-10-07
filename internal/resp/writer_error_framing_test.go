package resp

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWriteErrorPreservesReplyFraming(t *testing.T) {
	modes := map[string]func(*Writer, string) error{
		"WriteError": (*Writer).WriteError,
		"WriteValue": func(w *Writer, s string) error {
			return w.WriteValue(ErrorValue(s))
		},
		"WriteValueNoFlush": func(w *Writer, s string) error {
			return w.WriteValueNoFlush(ErrorValue(s))
		},
		"nested array": func(w *Writer, s string) error {
			return w.WriteArray([]*Value{ArrayValue([]*Value{ErrorValue(s)}), BulkString("a\r\nb")})
		},
	}
	for mode, write := range modes {
		for name, message := range map[string]string{
			"split":    "ERR unknown command 'BAD\r\n+OK'",
			"ordinary": "ERR ordinary",
			"empty":    "",
			"CR":       "\r",
			"LF":       "\n",
			"trailing": "ERR\r\n",
			"unicode":  "ERR ü\r\n:42\r\n+OK",
			"large":    strings.Repeat("x", 8192) + "\r\n+OK",
		} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				var output bytes.Buffer
				w := NewWriter(&output)
				if err := write(w, message); err != nil {
					t.Fatalf("write error: %v", err)
				}
				if mode == "WriteValueNoFlush" && len(message) < 4090 && output.Len() != 0 {
					t.Fatal("buffered error forced an early flush")
				}
				if err := w.WriteInteger(99); err != nil {
					t.Fatalf("following reply: %v", err)
				}
				r := NewReader(&output)
				v, err := r.ReadValue()
				if err != nil {
					t.Fatalf("first response: %v", err)
				}
				if mode == "nested array" {
					if v.Type != TypeArray || len(v.Array) != 2 || v.Array[0].Type != TypeArray || len(v.Array[0].Array) != 1 || string(v.Array[1].Bulk) != "a\r\nb" {
						t.Fatalf("nested response changed: %+v", v)
					}
					v = v.Array[0].Array[0]
				}
				want := strings.ReplaceAll(strings.ReplaceAll(message, "\r", " "), "\n", " ")
				if v.Type != TypeError || v.Err != want {
					t.Fatalf("got error=%q type=%v, want %q", v.Err, v.Type, want)
				}
				v, err = r.ReadValue()
				if err != nil || v.Type != TypeInteger || v.Int != 99 {
					t.Fatalf("following reply corrupted: value=%v error=%v", v, err)
				}
				if _, err := r.ReadValue(); err != io.EOF {
					t.Fatalf("expected no extra reply, got error=%v", err)
				}
			})
		}
		t.Run(mode+"/write failure", func(t *testing.T) {
			w := NewWriter(&flushErrWriter{})
			err := write(w, "ERR\r\nfailure")
			if err == nil {
				err = w.Flush()
			}
			if err == nil || !strings.Contains(err.Error(), "write error") {
				t.Fatalf("expected underlying write error, got %v", err)
			}
			if !errors.Is(w.Flush(), err) {
				t.Fatal("write error was not retained")
			}
		})
	}
}
