package resp

import (
	"bytes"
	"strings"
	"testing"
)

func TestWritePONG(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WritePONG(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "+PONG\r\n" {
		t.Errorf("expected '+PONG\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteQueued(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteQueued(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "+QUEUED\r\n" {
		t.Errorf("expected '+QUEUED\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteOne(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteOne(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ":1\r\n" {
		t.Errorf("expected ':1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteZeroMethod(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteZero(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ":0\r\n" {
		t.Errorf("expected ':0\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteNullArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteNullArray(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "*-1\r\n" {
		t.Errorf("expected '*-1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNullBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := NullBulkString()
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "$-1\r\n" {
		t.Errorf("expected '$-1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNullArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := NullArray()
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "*-1\r\n" {
		t.Errorf("expected '*-1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNull(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := NullValue()
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "_\r\n" {
		t.Errorf("expected '_\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueInvalidType(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := &Value{Type: Type(255)}
	err := w.WriteValue(v)
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestWriteValueNoFlushInvalidType(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := &Value{Type: Type(255)}
	err := w.WriteValueNoFlush(v)
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestReadInvalidType(t *testing.T) {
	input := "!invalid\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestReadNull(t *testing.T) {
	input := "_\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeNull {
		t.Errorf("expected TypeNull, got %v", v.Type)
	}

	if !v.IsNull {
		t.Error("expected IsNull to be true")
	}
}

func TestReadEmptyArray(t *testing.T) {
	input := "*0\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeArray {
		t.Errorf("expected TypeArray, got %v", v.Type)
	}

	if len(v.Array) != 0 {
		t.Errorf("expected 0 elements, got %d", len(v.Array))
	}
}

func TestReadEmptyBulkString(t *testing.T) {
	input := "$0\r\n\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeBulkString {
		t.Errorf("expected TypeBulkString, got %v", v.Type)
	}

	if len(v.Bulk) != 0 {
		t.Errorf("expected 0 length, got %d", len(v.Bulk))
	}
}

func TestReadIntegerInvalid(t *testing.T) {
	input := ":notanumber\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadBulkStringInvalidSize(t *testing.T) {
	input := "$notanumber\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadArrayInvalidCount(t *testing.T) {
	input := "*notanumber\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadCommandNullArray(t *testing.T) {
	input := "*-1\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, _, err := r.ReadCommand()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadCommandNotArray(t *testing.T) {
	input := "+OK\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, _, err := r.ReadCommand()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadCommandEmptyArray(t *testing.T) {
	input := "*0\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, _, err := r.ReadCommand()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadCommandWithNilArg(t *testing.T) {
	input := "*2\r\n$3\r\nGET\r\n$-1\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	cmd, args, err := r.ReadCommand()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd != "GET" {
		t.Errorf("expected 'GET', got '%s'", cmd)
	}

	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}

	if args[0] != nil {
		t.Errorf("expected nil arg, got %v", args[0])
	}
}

func TestBulkBytesNil(t *testing.T) {
	v := BulkBytes(nil)
	if !v.IsNull {
		t.Error("expected IsNull to be true for nil bytes")
	}
}

func TestBulkStringEmpty(t *testing.T) {
	v := BulkString("")
	if v.Type != TypeBulkString {
		t.Errorf("expected TypeBulkString, got %v", v.Type)
	}
	if len(v.Bulk) != 0 {
		t.Errorf("expected empty bulk, got %d", len(v.Bulk))
	}
}

func TestMapValue(t *testing.T) {
	m := map[string]*Value{
		"key1": SimpleString("val1"),
		"key2": IntegerValue(42),
	}
	v := MapValue(m)

	if v.Type != TypeMap {
		t.Errorf("expected TypeMap, got %v", v.Type)
	}

	if len(v.Map) != 2 {
		t.Errorf("expected 2 map entries, got %d", len(v.Map))
	}
}

func TestOK(t *testing.T) {
	v := OK()
	if v.Str != "OK" {
		t.Errorf("expected 'OK', got '%s'", v.Str)
	}
}

func TestPONG(t *testing.T) {
	v := PONG()
	if v.Str != "PONG" {
		t.Errorf("expected 'PONG', got '%s'", v.Str)
	}
}

func TestQueued(t *testing.T) {
	v := Queued()
	if v.Str != "QUEUED" {
		t.Errorf("expected 'QUEUED', got '%s'", v.Str)
	}
}

func TestValueStringMethods(t *testing.T) {
	tests := []struct {
		value    *Value
		expected string
	}{
		{SimpleString("hello"), "hello"},
		{ErrorValue("ERR test"), "ERR test"},
		{IntegerValue(65), "A"},
		{BulkBytes([]byte("data")), "data"},
		{NullBulkString(), "(nil)"},
		{NullArray(), "(nil)"},
		{NullValue(), "(nil)"},
		{ArrayValue([]*Value{}), "(array)"},
		{&Value{Type: Type(255)}, "(unknown)"},
	}

	for _, tt := range tests {
		result := tt.value.String()
		if result != tt.expected {
			t.Errorf("String() = %q, expected %q", result, tt.expected)
		}
	}
}

func TestWriterFlush(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.Flush(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteValueNoFlushSimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := SimpleString("test")
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "+test\r\n" {
		t.Errorf("expected '+test\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNoFlushError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := ErrorValue("ERR test")
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "-ERR test\r\n" {
		t.Errorf("expected '-ERR test\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNoFlushInteger(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := IntegerValue(123)
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != ":123\r\n" {
		t.Errorf("expected ':123\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNoFlushBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := BulkBytes([]byte("hello"))
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "$5\r\nhello\r\n" {
		t.Errorf("expected '$5\\r\\nhello\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNoFlushNullBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := NullBulkString()
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "$-1\r\n" {
		t.Errorf("expected '$-1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNoFlushArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := ArrayValue([]*Value{BulkString("a"), BulkString("b")})
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "*2\r\n$1\r\na\r\n$1\r\nb\r\n" {
		t.Errorf("unexpected output: '%s'", buf.String())
	}
}

func TestWriteValueNoFlushNullArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := NullArray()
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "*-1\r\n" {
		t.Errorf("expected '*-1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteValueNoFlushNull(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	v := NullValue()
	if err := w.WriteValueNoFlush(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if buf.String() != "_\r\n" {
		t.Errorf("expected '_\\r\\n', got '%s'", buf.String())
	}
}

func TestReadUnexpectedEOF(t *testing.T) {
	input := "+OK"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for incomplete input")
	}
}

func TestReadInvalidCRLF(t *testing.T) {
	input := "+OK\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for invalid line ending")
	}
}

func TestReadBulkStringInvalidCRLF(t *testing.T) {
	input := "$5\r\nhelloXXextra"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadNullInvalidCRLF(t *testing.T) {
	input := "_XX"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadBulkStringTruncated(t *testing.T) {
	input := "$10\r\nhello"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for truncated bulk string")
	}
}

func TestNewReader(t *testing.T) {
	r := NewReader(strings.NewReader("+OK\r\n"))
	if r == nil {
		t.Fatal("expected reader")
	}
}

func TestNewWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if w == nil {
		t.Fatal("expected writer")
	}
}

func TestTypeConstants(t *testing.T) {
	if TypeSimpleString != '+' {
		t.Errorf("expected TypeSimpleString = '+', got %c", TypeSimpleString)
	}
	if TypeError != '-' {
		t.Errorf("expected TypeError = '-', got %c", TypeError)
	}
	if TypeInteger != ':' {
		t.Errorf("expected TypeInteger = ':', got %c", TypeInteger)
	}
	if TypeBulkString != '$' {
		t.Errorf("expected TypeBulkString = '$', got %c", TypeBulkString)
	}
	if TypeArray != '*' {
		t.Errorf("expected TypeArray = '*', got %c", TypeArray)
	}
	if TypeMap != '%' {
		t.Errorf("expected TypeMap = '%%', got %c", TypeMap)
	}
	if TypeNull != '_' {
		t.Errorf("expected TypeNull = '_', got %c", TypeNull)
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrInvalidType == nil {
		t.Error("ErrInvalidType should not be nil")
	}
	if ErrInvalidFormat == nil {
		t.Error("ErrInvalidFormat should not be nil")
	}
	if ErrBulkStringTooBig == nil {
		t.Error("ErrBulkStringTooBig should not be nil")
	}
}

func TestMaxBulkStringSize(t *testing.T) {
	if MaxBulkStringSize != 512*1024*1024 {
		t.Errorf("expected MaxBulkStringSize = 536870912, got %d", MaxBulkStringSize)
	}
}

func TestReadLineWithEmbeddedCR(t *testing.T) {
	input := "+hello\rworld\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Str != "hello\rworld" {
		t.Errorf("expected 'hello\\rworld', got '%s'", v.Str)
	}
}

func TestReadLineUnexpectedEOF(t *testing.T) {
	input := "+hello\r"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for incomplete line")
	}
}

func TestReadLineUnexpectedEOFAfterCR(t *testing.T) {
	input := "+hello\r"
	r := NewReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadValue()
	if err == nil {
		t.Error("expected error for incomplete line")
	}
}
