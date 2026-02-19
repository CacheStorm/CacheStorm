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
