package resp

import (
	"bytes"
	"testing"
)

func TestReadSimpleString(t *testing.T) {
	input := "+OK\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeSimpleString {
		t.Errorf("expected TypeSimpleString, got %v", v.Type)
	}

	if v.Str != "OK" {
		t.Errorf("expected 'OK', got '%s'", v.Str)
	}
}

func TestReadError(t *testing.T) {
	input := "-ERR unknown command\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeError {
		t.Errorf("expected TypeError, got %v", v.Type)
	}

	if v.Err != "ERR unknown command" {
		t.Errorf("expected 'ERR unknown command', got '%s'", v.Err)
	}
}

func TestReadInteger(t *testing.T) {
	input := ":1000\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeInteger {
		t.Errorf("expected TypeInteger, got %v", v.Type)
	}

	if v.Int != 1000 {
		t.Errorf("expected 1000, got %d", v.Int)
	}
}

func TestReadBulkString(t *testing.T) {
	input := "$6\r\nfoobar\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeBulkString {
		t.Errorf("expected TypeBulkString, got %v", v.Type)
	}

	if string(v.Bulk) != "foobar" {
		t.Errorf("expected 'foobar', got '%s'", string(v.Bulk))
	}
}

func TestReadNullBulkString(t *testing.T) {
	input := "$-1\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !v.IsNull {
		t.Error("expected null bulk string")
	}
}

func TestReadArray(t *testing.T) {
	input := "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Type != TypeArray {
		t.Errorf("expected TypeArray, got %v", v.Type)
	}

	if len(v.Array) != 2 {
		t.Errorf("expected 2 elements, got %d", len(v.Array))
	}

	if string(v.Array[0].Bulk) != "foo" {
		t.Errorf("expected 'foo', got '%s'", string(v.Array[0].Bulk))
	}
}

func TestReadCommand(t *testing.T) {
	input := "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	cmd, args, err := r.ReadCommand()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd != "SET" {
		t.Errorf("expected 'SET', got '%s'", cmd)
	}

	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestWriteSimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteSimpleString("OK"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "+OK\r\n" {
		t.Errorf("expected '+OK\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteBulkString("hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "$5\r\nhello\r\n" {
		t.Errorf("expected '$5\\r\\nhello\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	items := []*Value{
		BulkString("foo"),
		BulkString("bar"),
	}

	if err := w.WriteArray(items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	if buf.String() != expected {
		t.Errorf("expected '%s', got '%s'", expected, buf.String())
	}
}

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	original := BulkString("test-value")
	if err := w.WriteValue(original); err != nil {
		t.Fatalf("write error: %v", err)
	}

	r := NewReader(&buf)
	read, err := r.ReadValue()
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	if string(read.Bulk) != string(original.Bulk) {
		t.Errorf("expected '%s', got '%s'", string(original.Bulk), string(read.Bulk))
	}
}

func TestWriteInteger(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteInteger(42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ":42\r\n" {
		t.Errorf("expected ':42\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteError("ERR test error"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "-ERR test error\r\n" {
		t.Errorf("expected '-ERR test error\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteNull(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteNull(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "_\r\n" {
		t.Errorf("expected '_\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteNullBulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteNullBulkString(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "$-1\r\n" {
		t.Errorf("expected '$-1\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteOK(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteOK(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "+OK\r\n" {
		t.Errorf("expected '+OK\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteBulkBytes(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteBulkBytes([]byte("hello")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "$5\r\nhello\r\n" {
		t.Errorf("expected '$5\\r\\nhello\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteArray([]*Value{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "*0\r\n" {
		t.Errorf("expected '*0\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteNestedArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	items := []*Value{
		{Type: TypeArray, Array: []*Value{
			BulkString("nested1"),
			BulkString("nested2"),
		}},
		BulkString("outer"),
	}

	if err := w.WriteArray(items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "*2\r\n*2\r\n$7\r\nnested1\r\n$7\r\nnested2\r\n$5\r\nouter\r\n" {
		t.Errorf("unexpected output: '%s'", buf.String())
	}
}

func TestReadNegativeInteger(t *testing.T) {
	input := ":-1000\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Int != -1000 {
		t.Errorf("expected -1000, got %d", v.Int)
	}
}

func TestReadNullArray(t *testing.T) {
	input := "*-1\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !v.IsNull {
		t.Error("expected null array")
	}
}

func TestValueMethods(t *testing.T) {
	v := BulkString("test")
	if v.String() != "test" {
		t.Errorf("expected 'test', got '%s'", v.String())
	}

	v2 := SimpleString("ok")
	if v2.String() != "ok" {
		t.Errorf("expected 'ok', got '%s'", v2.String())
	}

	v3 := IntegerValue(42)
	if v3.Int != 42 {
		t.Errorf("expected 42, got %d", v3.Int)
	}

	v4 := ErrorValue("ERR test")
	if v4.Err != "ERR test" {
		t.Errorf("expected 'ERR test', got '%s'", v4.Err)
	}

	v5 := NullValue()
	if !v5.IsNull {
		t.Error("expected null value")
	}
}

func TestReadMultipleValues(t *testing.T) {
	input := "+OK\r\n+PONG\r\n"
	r := NewReader(bytes.NewReader([]byte(input)))

	v1, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v1.Str != "OK" {
		t.Errorf("expected 'OK', got '%s'", v1.Str)
	}

	v2, err := r.ReadValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v2.Str != "PONG" {
		t.Errorf("expected 'PONG', got '%s'", v2.Str)
	}
}

func TestWriteZero(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteInteger(0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ":0\r\n" {
		t.Errorf("expected ':0\\r\\n', got '%s'", buf.String())
	}
}

func TestWriteNegativeInteger(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteInteger(-100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ":-100\r\n" {
		t.Errorf("expected ':-100\\r\\n', got '%s'", buf.String())
	}
}
