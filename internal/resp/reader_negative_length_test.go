package resp

import (
	"strings"
	"testing"
)

// readValueNoPanic runs the production reader and converts a panic into a test
// failure, so a failure names the protocol defect instead of crashing the run.
func readValueNoPanic(t *testing.T, wire string) (v *Value, panicked interface{}, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = r
		}
	}()
	v, err = NewReader(strings.NewReader(wire)).ReadValue()
	return v, panicked, err
}

// A bulk-string length is a non-negative byte count, with -1 reserved for the
// null bulk string. Any other negative value is a protocol violation and must
// be reported as an error rather than reaching make() as a negative size.
func TestReadBulkStringRejectsNegativeLength(t *testing.T) {
	for _, wire := range []string{"$-2\r\n", "$-42\r\n", "$-9223372036854775808\r\n"} {
		_, panicked, err := readValueNoPanic(t, wire)
		if panicked != nil {
			t.Fatalf("ReadValue(%q) panicked: %v", wire, panicked)
		}
		if err == nil {
			t.Fatalf("ReadValue(%q): want a protocol error, got none", wire)
		}
	}
}

// Same contract for array element counts: -1 is null, anything below is invalid.
func TestReadArrayRejectsNegativeCount(t *testing.T) {
	for _, wire := range []string{"*-2\r\n", "*-42\r\n", "*-9223372036854775808\r\n"} {
		_, panicked, err := readValueNoPanic(t, wire)
		if panicked != nil {
			t.Fatalf("ReadValue(%q) panicked: %v", wire, panicked)
		}
		if err == nil {
			t.Fatalf("ReadValue(%q): want a protocol error, got none", wire)
		}
	}
}

// The negative count is rejected before any element is read, so a rejected
// frame must not consume the bytes that follow it.
func TestReadArrayNegativeCountDoesNotConsumeElements(t *testing.T) {
	r := NewReader(strings.NewReader("*-2\r\n$3\r\nfoo\r\n"))

	if _, err := r.ReadValue(); err == nil {
		t.Fatal("ReadValue: want a protocol error for negative count, got none")
	}

	// The reader is positioned at the payload; it must not have treated
	// "$-2" as an element or silently accepted a partial array.
	next, err := r.ReadValue()
	if err == nil && next != nil && next.Type == TypeArray {
		t.Fatalf("negative count was accepted as a %v array", next.Type)
	}
}

// Control: -1 is the one legitimate negative and must keep meaning null.
func TestReadValueNullBulkControl(t *testing.T) {
	v, panicked, err := readValueNoPanic(t, "$-1\r\n")
	if panicked != nil {
		t.Fatalf("null bulk panicked: %v", panicked)
	}
	if err != nil {
		t.Fatalf("null bulk returned error: %v", err)
	}
	if !v.IsNull {
		t.Fatalf("expected null bulk string, got %+v", v)
	}
}

func TestReadValueNullArrayControl(t *testing.T) {
	v, panicked, err := readValueNoPanic(t, "*-1\r\n")
	if panicked != nil {
		t.Fatalf("null array panicked: %v", panicked)
	}
	if err != nil {
		t.Fatalf("null array returned error: %v", err)
	}
	if !v.IsNull {
		t.Fatalf("expected null array, got %+v", v)
	}
}

// Control: ordinary payloads on both fixed paths still parse.
func TestReadValueNormalBulkControl(t *testing.T) {
	v, panicked, err := readValueNoPanic(t, "$3\r\nfoo\r\n")
	if panicked != nil {
		t.Fatalf("normal bulk panicked: %v", panicked)
	}
	if err != nil {
		t.Fatalf("normal bulk returned error: %v", err)
	}
	if string(v.Bulk) != "foo" {
		t.Fatalf("got %q, want %q", v.Bulk, "foo")
	}
}

func TestReadValueNormalArrayControl(t *testing.T) {
	v, panicked, err := readValueNoPanic(t, "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n")
	if panicked != nil {
		t.Fatalf("normal array panicked: %v", panicked)
	}
	if err != nil {
		t.Fatalf("normal array returned error: %v", err)
	}
	if len(v.Array) != 2 || string(v.Array[0].Bulk) != "foo" || string(v.Array[1].Bulk) != "bar" {
		t.Fatalf("got %+v, want [foo bar]", v.Array)
	}
}

// Control: the boundary values adjacent to the new guard stay valid.
func TestReadValueBoundaryCountsControl(t *testing.T) {
	for _, wire := range []string{"$0\r\n\r\n", "*0\r\n"} {
		v, panicked, err := readValueNoPanic(t, wire)
		if panicked != nil {
			t.Fatalf("ReadValue(%q) panicked: %v", wire, panicked)
		}
		if err != nil {
			t.Fatalf("ReadValue(%q) returned error: %v", wire, err)
		}
		if v.Type == TypeArray && len(v.Array) != 0 {
			t.Fatalf("ReadValue(%q): expected empty array, got %+v", wire, v.Array)
		}
	}
}
