package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// flushErrWriter always fails on Write, to make bufio.Writer.Flush() fail.
type flushErrWriter struct{}

func (w *flushErrWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

// ---- Reader tests ----

// TestNewReaderWithBufioReader verifies the optimization path where
// NewReader receives a *bufio.Reader and reuses it.
func TestNewReaderWithBufioReader(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("+OK\r\n"))
	r := NewReader(br)
	if r == nil {
		t.Fatal("expected reader")
	}
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Str != "OK" {
		t.Errorf("expected 'OK', got '%s'", v.Str)
	}
}

// TestResetWithBufioReader tests Reset when given a *bufio.Reader.
func TestResetWithBufioReader(t *testing.T) {
	r := NewReader(strings.NewReader("+FIRST\r\n"))
	v, _ := r.ReadValue()
	if v.Str != "FIRST" {
		t.Errorf("expected 'FIRST', got '%s'", v.Str)
	}

	br := bufio.NewReader(strings.NewReader("+SECOND\r\n"))
	r.Reset(br)
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Str != "SECOND" {
		t.Errorf("expected 'SECOND', got '%s'", v.Str)
	}
}

// TestResetWithPlainReader tests Reset when given a plain io.Reader.
func TestResetWithPlainReader(t *testing.T) {
	r := NewReader(strings.NewReader("+FIRST\r\n"))
	v, _ := r.ReadValue()
	if v.Str != "FIRST" {
		t.Errorf("expected 'FIRST', got '%s'", v.Str)
	}

	r.Reset(strings.NewReader("+SECOND\r\n"))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Str != "SECOND" {
		t.Errorf("expected 'SECOND', got '%s'", v.Str)
	}
}

// TestReadValueEOF tests ReadValue when the reader is empty.
func TestReadValueEOF(t *testing.T) {
	r := NewReader(strings.NewReader(""))
	_, err := r.ReadValue()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

// TestReadErrorValueEdge reads an error response with truncated input.
func TestReadErrorValueEdge(t *testing.T) {
	input := "-ERR incomplete"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for truncated error value")
	}
}

// TestReadIntegerTruncated tests readInteger with truncated input.
func TestReadIntegerTruncated(t *testing.T) {
	input := ":42"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for truncated integer")
	}
}

// TestReadBulkStringTooBig tests the ErrBulkStringTooBig path.
func TestReadBulkStringTooBig(t *testing.T) {
	input := fmt.Sprintf("$%d\r\n", MaxBulkStringSize+1)
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err != ErrBulkStringTooBig {
		t.Errorf("expected ErrBulkStringTooBig, got %v", err)
	}
}

// TestReadBulkStringReadLineError tests readBulkString when readLine fails.
func TestReadBulkStringReadLineError(t *testing.T) {
	input := "$5"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for truncated bulk string header")
	}
}

// TestReadBulkStringReadFullError tests readBulkString when io.ReadFull fails.
func TestReadBulkStringReadFullError(t *testing.T) {
	input := "$10\r\nhello"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for short bulk string body")
	}
}

// TestReadBulkStringCRLFError tests readBulkString when readCRLF after data fails.
func TestReadBulkStringCRLFError(t *testing.T) {
	input := "$5\r\nhelloXY"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

// TestReadBulkStringEmptyCRLFError tests readBulkString with size 0 but bad CRLF.
func TestReadBulkStringEmptyCRLFError(t *testing.T) {
	input := "$0\r\nXY"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

// TestReadArrayTooLarge tests the ErrArrayTooLarge path.
func TestReadArrayTooLarge(t *testing.T) {
	input := fmt.Sprintf("*%d\r\n", MaxArrayElements+1)
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err != ErrArrayTooLarge {
		t.Errorf("expected ErrArrayTooLarge, got %v", err)
	}
}

// TestReadArrayElementError tests readArray when reading an element fails.
func TestReadArrayElementError(t *testing.T) {
	input := "*2\r\n$3\r\nfoo\r\n"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for incomplete array")
	}
}

// TestReadArrayReadLineError tests readArray when readLine for count fails.
func TestReadArrayReadLineError(t *testing.T) {
	input := "*5"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for truncated array header")
	}
}

// TestReadCRLFFirstByteError tests readCRLF when the first byte read fails.
func TestReadCRLFFirstByteError(t *testing.T) {
	input := "_"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error when CRLF is missing after null type")
	}
}

// TestReadCRLFSecondByteError tests readCRLF when only first byte is available.
func TestReadCRLFSecondByteError(t *testing.T) {
	input := "_\r"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error when second byte of CRLF is missing")
	}
}

// TestReadCRLFBadBytes tests readCRLF when bytes are present but not \r\n.
func TestReadCRLFBadBytes(t *testing.T) {
	input := "_AB"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

// TestReadCommandWithIntegerArg tests ReadCommand with a non-bulk-string arg.
func TestReadCommandWithIntegerArg(t *testing.T) {
	input := "*2\r\n$3\r\nGET\r\n:42\r\n"
	r := NewReader(strings.NewReader(input))
	cmd, args, err := r.ReadCommand()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "GET" {
		t.Errorf("expected 'GET', got '%s'", cmd)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	if args[0] != nil {
		t.Errorf("expected nil arg for integer type, got %v", args[0])
	}
}

// TestReadCommandReadValueError tests ReadCommand when ReadValue fails.
func TestReadCommandReadValueError(t *testing.T) {
	r := NewReader(strings.NewReader(""))
	_, _, err := r.ReadCommand()
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// TestReadNestedArray reads a nested array structure.
func TestReadNestedArray(t *testing.T) {
	input := "*2\r\n*2\r\n$1\r\na\r\n$1\r\nb\r\n$3\r\nfoo\r\n"
	r := NewReader(strings.NewReader(input))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != TypeArray {
		t.Errorf("expected TypeArray, got %v", v.Type)
	}
	if len(v.Array) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(v.Array))
	}
	if v.Array[0].Type != TypeArray {
		t.Errorf("expected first element to be TypeArray, got %v", v.Array[0].Type)
	}
}

// ---- Writer tests ----

// TestWriteValueSimpleString tests WriteValue with a simple string.
func TestWriteValueSimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	v := SimpleString("hello")
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "+hello\r\n" {
		t.Errorf("expected '+hello\\r\\n', got '%s'", buf.String())
	}
}

// TestWriteValueError tests WriteValue with an error value.
func TestWriteValueError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	v := ErrorValue("ERR something")
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "-ERR something\r\n" {
		t.Errorf("expected '-ERR something\\r\\n', got '%s'", buf.String())
	}
}

// TestWriteValueInteger tests WriteValue with an integer.
func TestWriteValueInteger(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	v := IntegerValue(99)
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != ":99\r\n" {
		t.Errorf("expected ':99\\r\\n', got '%s'", buf.String())
	}
}

// TestWriteValueBulkString tests WriteValue with a bulk string.
func TestWriteValueBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	v := BulkBytes([]byte("test"))
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "$4\r\ntest\r\n" {
		t.Errorf("expected '$4\\r\\ntest\\r\\n', got '%s'", buf.String())
	}
}

// TestWriteValueArray tests WriteValue with a non-null array.
func TestWriteValueArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	v := ArrayValue([]*Value{BulkString("a")})
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "*1\r\n$1\r\na\r\n"
	if buf.String() != expected {
		t.Errorf("expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteSimpleStringFlushError tests WriteSimpleString when flush fails.
func TestWriteSimpleStringFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteSimpleString("OK")
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteErrorFlushError tests WriteError when flush fails.
func TestWriteErrorFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteError("ERR test")
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteIntegerFlushError tests WriteInteger when flush fails.
func TestWriteIntegerFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteInteger(42)
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteBulkBytesFlushError tests WriteBulkBytes when flush fails.
func TestWriteBulkBytesFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteBulkBytes([]byte("hello"))
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteNullBulkStringFlushError tests WriteNullBulkString when flush fails.
func TestWriteNullBulkStringFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteNullBulkString()
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteNullFlushError tests WriteNull when flush fails.
func TestWriteNullFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteNull()
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteNullArrayFlushError tests WriteNullArray when flush fails.
func TestWriteNullArrayFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteNullArray()
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteArrayFlushError tests WriteArray when flush fails.
func TestWriteArrayFlushError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	items := []*Value{BulkString("a")}
	err := w.WriteArray(items)
	if err == nil {
		t.Error("expected error from flush")
	}
}

// TestWriteArrayItemError tests WriteArray when writing an item of invalid type fails.
func TestWriteArrayItemError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	items := []*Value{{Type: Type(255)}}
	err := w.WriteArray(items)
	if err == nil {
		t.Error("expected error from invalid array item")
	}
}

// TestWriteValueFlushErrorSimpleString tests that WriteValue routes to
// WriteSimpleString and propagates flush error.
func TestWriteValueFlushErrorSimpleString(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteValue(SimpleString("OK"))
	if err == nil {
		t.Error("expected error")
	}
}

// TestWriteValueFlushErrorError tests that WriteValue routes to WriteError
// and propagates flush error.
func TestWriteValueFlushErrorError(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteValue(ErrorValue("ERR test"))
	if err == nil {
		t.Error("expected error")
	}
}

// TestWriteValueFlushErrorInteger tests that WriteValue routes to WriteInteger
// and propagates flush error.
func TestWriteValueFlushErrorInteger(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	err := w.WriteValue(IntegerValue(42))
	if err == nil {
		t.Error("expected error")
	}
}

// TestReadBulkStringNullExplicit tests reading a bulk string with size -1 (null).
func TestReadBulkStringNullExplicit(t *testing.T) {
	input := "$-1\r\n"
	r := NewReader(strings.NewReader(input))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.IsNull {
		t.Error("expected null bulk string")
	}
}

// TestReadArrayNullExplicit tests reading a null array.
func TestReadArrayNullExplicit(t *testing.T) {
	input := "*-1\r\n"
	r := NewReader(strings.NewReader(input))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.IsNull {
		t.Error("expected null array")
	}
}

// TestReadArrayEmpty tests reading an empty array.
func TestReadArrayEmptyCov(t *testing.T) {
	input := "*0\r\n"
	r := NewReader(strings.NewReader(input))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v.Array) != 0 {
		t.Errorf("expected 0 elements, got %d", len(v.Array))
	}
}

// TestReadBulkStringZeroLength tests reading a zero-length bulk string.
func TestReadBulkStringZeroLengthCov(t *testing.T) {
	input := "$0\r\n\r\n"
	r := NewReader(strings.NewReader(input))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v.Bulk) != 0 {
		t.Errorf("expected empty bulk, got %d bytes", len(v.Bulk))
	}
}

// TestReadNullType tests reading the null type value.
func TestReadNullTypeCov(t *testing.T) {
	input := "_\r\n"
	r := NewReader(strings.NewReader(input))
	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != TypeNull {
		t.Errorf("expected TypeNull, got %v", v.Type)
	}
}

// TestReadNullWithBadCRLF tests the null type with bad trailing bytes.
func TestReadNullWithBadCRLFCov(t *testing.T) {
	input := "_XY"
	r := NewReader(strings.NewReader(input))
	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

// TestRoundTripAllTypes tests serialization and deserialization of all RESP types.
func TestRoundTripAllTypes(t *testing.T) {
	tests := []struct {
		name  string
		value *Value
	}{
		{"SimpleString", SimpleString("hello")},
		{"Error", ErrorValue("ERR test")},
		{"Integer", IntegerValue(42)},
		{"BulkString", BulkBytes([]byte("data"))},
		{"NullBulkString", NullBulkString()},
		{"NullArray", NullArray()},
		{"EmptyArray", ArrayValue([]*Value{})},
		{"Array", ArrayValue([]*Value{BulkString("a"), IntegerValue(1)})},
		{"Null", NullValue()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf)
			if err := w.WriteValue(tt.value); err != nil {
				t.Fatalf("write error: %v", err)
			}

			r := NewReader(&buf)
			v, err := r.ReadValue()
			if err != nil {
				t.Fatalf("read error: %v", err)
			}
			if v.Type != tt.value.Type {
				t.Errorf("type mismatch: expected %v, got %v", tt.value.Type, v.Type)
			}
		})
	}
}

// TestMaxArrayElementsCov verifies the constant value.
func TestMaxArrayElementsCov(t *testing.T) {
	if MaxArrayElements != 1048576 {
		t.Errorf("expected MaxArrayElements = 1048576, got %d", MaxArrayElements)
	}
}

// TestErrVariablesCov ensures all error variables are non-nil.
func TestErrVariablesCov(t *testing.T) {
	errs := []error{ErrInvalidType, ErrInvalidFormat, ErrBulkStringTooBig, ErrArrayTooLarge}
	for _, e := range errs {
		if e == nil {
			t.Error("error variable should not be nil")
		}
	}
}

// TestWriteBulkStringViaValue tests WriteBulkString by going through WriteValue.
func TestWriteBulkStringViaValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	err := w.WriteBulkString("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "$4\r\ntest\r\n" {
		t.Errorf("expected '$4\\r\\ntest\\r\\n', got '%s'", buf.String())
	}
}

// TestFlushErrorDirect tests Flush with a failing writer.
func TestFlushErrorDirect(t *testing.T) {
	w := NewWriter(&flushErrWriter{})
	// Write something to dirty the buffer.
	w.WriteSimpleString("OK")
	err := w.Flush()
	if err == nil {
		t.Error("expected error from flush")
	}
}
