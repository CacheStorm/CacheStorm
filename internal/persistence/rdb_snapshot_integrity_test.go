package persistence

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRDBChecksumVector(t *testing.T) {
	if got := rdbChecksum(0, []byte("123456789")); got != 0xe9c6d914c4b8d9ca {
		t.Fatalf("CRC64 = %016x", got)
	}
	first := rdbChecksum(0, []byte("1234"))
	if got := rdbChecksum(first, []byte("56789")); got != 0xe9c6d914c4b8d9ca {
		t.Fatalf("incremental CRC64 = %016x", got)
	}
}

func TestRDBAtomicFailurePreservesPreviousSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.rdb")
	previous := []byte("previous successful snapshot")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected serialization failure")
	if err := writeRDBAtomic(path, func(w io.Writer) error {
		if _, err := w.Write([]byte("partial replacement")); err != nil {
			return err
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatalf("writeRDBAtomic = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatalf("previous snapshot overwritten: %q, %v", got, err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".cachestorm-rdb-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary snapshots leaked: %v, %v", files, err)
	}
}
