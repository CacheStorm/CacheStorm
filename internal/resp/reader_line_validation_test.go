package resp

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

type lineErrorReader struct{ err error }

func (r lineErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadLineRejectsEmbeddedCR(t *testing.T) {
	for _, prefix := range []string{"+", "-"} {
		for _, payload := range []string{"hello\rworld", "\r", "\r\r", "a\rb\rc"} {
			t.Run(prefix+payload, func(t *testing.T) {
				r := NewReader(iotest.OneByteReader(strings.NewReader(prefix + payload + "\r\n:99\r\n")))
				v, err := r.ReadValue()
				if err != ErrInvalidFormat || v != nil {
					t.Fatalf("expected nil value and ErrInvalidFormat, got value=%v error=%v", v, err)
				}
				v, err = r.ReadValue()
				if err != nil || v.Type != TypeInteger || v.Int != 99 {
					t.Fatalf("following frame changed: value=%v error=%v", v, err)
				}
			})
		}
	}
}

func TestReadLineValidControls(t *testing.T) {
	for _, prefix := range []string{"+", "-"} {
		for _, payload := range []string{"", "OK", "ü", "a\x00b"} {
			v, err := NewReader(iotest.OneByteReader(strings.NewReader(prefix + payload + "\r\n"))).ReadValue()
			if err != nil || v.String() != payload {
				t.Fatalf("valid line %q: value=%v error=%v", prefix+payload, v, err)
			}
		}
	}
	v, err := NewReader(strings.NewReader("$4\r\na\r\nb\r\n")).ReadValue()
	if err != nil || v.Type != TypeBulkString || string(v.Bulk) != "a\r\nb" {
		t.Fatalf("binary bulk changed: value=%v error=%v", v, err)
	}
}

func TestReadLinePropagatesReadFailure(t *testing.T) {
	injected := errors.New("injected read failure")
	v, err := NewReader(io.MultiReader(strings.NewReader("+partial\r"), lineErrorReader{injected})).ReadValue()
	if !errors.Is(err, injected) || v != nil {
		t.Fatalf("expected nil value and injected error, got value=%v error=%v", v, err)
	}
}
