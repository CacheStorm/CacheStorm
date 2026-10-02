package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func execBf(s *store.Store, r *Router, cmd string, args ...string) string {
	handler, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered"
	}
	var buf bytes.Buffer
	ctx := NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(&buf))
	if err := handler.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// rawLen reads the actual stored byte length of a bitmap key straight from the
// store, so the assertion never trusts the command's own reply.
func rawLen(t *testing.T, s *store.Store, key string) int {
	t.Helper()
	entry, exists := s.Get(key)
	if !exists {
		return -1
	}
	bm, ok := entry.Value.(*BitmapValue)
	if !ok {
		return -2
	}
	return len(bm.Data)
}

// TestProofBitfieldSetAllocatesExactLength is the round proof.
//
// Contract (Redis BITFIELD): the string is grown to hold exactly the bits the
// field needs. SET u8 at offset 0 needs 1 byte, SET u64 at offset 0 needs 8.
// The stored length is observable through BITOP, which replies with the length
// of the destination string it wrote.
//
// Defect: bitfieldSet sized the buffer as
//     byteOffset + (bits+7)/8 + 1
// The trailing "+ 1" is not part of any correct formula — the true
// requirement is byteOffset + ceil((bitOffset + bits)/8). So every BITFIELD
// SET leaves a phantom zero byte, and BITOP then reports a destination longer
// than the one Redis would write.
//
// Controls (must pass before AND after): SETBIT allocates exactly (no "+ 1"),
// and the VALUE read back through BITFIELD GET is correct.
func TestProofBitfieldSetAllocatesExactLength(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)
	RegisterStringCommands(r) // SET, for the OR control

	// ---- CONTROL 1: SETBIT allocates exactly (its own sizing is correct).
	if got := execBf(s, r, "SETBIT", "sb", "0", "1"); got != ":0\r\n" {
		t.Fatalf("CONTROL 1 broken harness: SETBIT = %q, want \":0\\r\\n\"", got)
	}
	if n := rawLen(t, s, "sb"); n != 1 {
		t.Fatalf("CONTROL 1 broken harness: SETBIT at offset 0 stored %d bytes, want exactly 1", n)
	}
	t.Log("CONTROL 1 ok: SETBIT stores exactly 1 byte")

	// ---- CONTROL 2: the VALUE written by BITFIELD SET is correct.
	if got := execBf(s, r, "BITFIELD", "bf", "SET", "u8", "0", "5"); got != "*1\r\n:0\r\n" {
		t.Fatalf("CONTROL 2 broken harness: BITFIELD SET = %q, want \"*1\\r\\n:0\\r\\n\"", got)
	}
	if got := execBf(s, r, "BITFIELD", "bf", "GET", "u8", "0"); got != "*1\r\n:5\r\n" {
		t.Fatalf("CONTROL 2 broken harness: BITFIELD GET = %q, want the value 5", got)
	}
	t.Log("CONTROL 2 ok: the stored VALUE (5) is correct")

	// ---- THE DEFECT: the stored LENGTH carries a phantom trailing byte.
	// u8 at offset 0 needs exactly 1 byte.
	if n := rawLen(t, s, "bf"); n != 1 {
		t.Fatalf("FAIL: BITFIELD SET u8 0 5 stored %d bytes, want exactly 1 "+
			"(bitfieldSet adds a spurious \"+ 1\" to the buffer size)", n)
	}
	t.Log("PASS: u8 at offset 0 occupies exactly one byte")

	// ---- THE DEFECT, observed through a real client-visible reply:
	// BITOP replies with the length of the destination it wrote.
	orLen := execBf(s, r, "BITOP", "OR", "or:dst", "bf", "bf")
	if strings.TrimSpace(orLen) != ":1" {
		t.Logf("observed BITOP OR length reply: %q", orLen)
	}

	// ---- SECONDARY: a 64-bit field needs 8 bytes at offset 0, not 9.
	if got := execBf(s, r, "BITFIELD", "bf64", "SET", "u64", "0", "1"); got != "*1\r\n:0\r\n" {
		t.Fatalf("SECONDARY setup: BITFIELD SET u64 = %q", got)
	}
	if n := rawLen(t, s, "bf64"); n != 8 {
		t.Fatalf("FAIL: BITFIELD SET u64 0 1 stored %d bytes, want exactly 8", n)
	}
	t.Log("PASS: u64 at offset 0 occupies exactly eight bytes")

	// ---- THIRDARY: an unaligned field genuinely needs 2 bytes — the exact
	// case the spurious "+1" was presumably meant to cover. It must stay 2,
	// not become 3.
	if got := execBf(s, r, "BITFIELD", "bf7", "SET", "u8", "7", "5"); got != "*1\r\n:0\r\n" {
		t.Fatalf("THIRDARY setup: BITFIELD SET u8 7 = %q", got)
	}
	if n := rawLen(t, s, "bf7"); n != 2 {
		t.Fatalf("FAIL: BITFIELD SET u8 7 5 stored %d bytes, want exactly 2 "+
			"(offset 7 + 8 bits spans two bytes)", n)
	}
	t.Log("PASS: unaligned u8 at offset 7 occupies exactly two bytes")

	// ---- FOURTHARY: a field straddling a byte boundary with a larger width
	// is the boundary where an off-by-one is easiest to hide. i16 at offset 4
	// needs ceil((4+16)/8) = 3 bytes.
	if got := execBf(s, r, "BITFIELD", "bf4", "SET", "i16", "4", "7"); got != "*1\r\n:0\r\n" {
		t.Fatalf("FOURTHARY setup: BITFIELD SET i16 4 = %q", got)
	}
	if n := rawLen(t, s, "bf4"); n != 3 {
		t.Fatalf("FAIL: BITFIELD SET i16 4 7 stored %d bytes, want exactly 3 "+
			"(bits 4..19 span three bytes)", n)
	}
	t.Log("PASS: i16 at offset 4 occupies exactly three bytes")
}