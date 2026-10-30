package store

import (
	"testing"
	"unsafe"
)

func TestEntryFitsFortyEightBytes(t *testing.T) {
	if n := unsafe.Sizeof(Entry{}); n > 48 {
		t.Fatalf("Entry is %d bytes, want at most 48", n)
	}
}
