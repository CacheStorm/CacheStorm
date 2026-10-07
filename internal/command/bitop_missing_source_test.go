package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execB runs the real router handler for cmd and returns the raw RESP reply.
func execB(s *store.Store, r *Router, cmd string, args ...string) string {
	handler, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered"
	}
	raw := make([][]byte, 0, len(args))
	for _, a := range args {
		raw = append(raw, []byte(a))
	}
	var buf bytes.Buffer
	ctx := NewContext(cmd, raw, s, resp.NewWriter(&buf))
	if err := handler.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// destBytes returns the destination key's stored bytes, or "<missing>".
func destBytes(t *testing.T, s *store.Store, key string) string {
	t.Helper()
	entry, exists := s.Get(key)
	if !exists {
		return "<missing>"
	}
	bm, ok := entry.Value.(*BitmapValue)
	if !ok {
		return "<notbitmap>"
	}
	if len(bm.Data) == 0 {
		return "<empty>"
	}
	return string(bm.Data)
}

// TestProofBitopMissingSourceAndAnnihilates is the round proof.
//
// Contract (Redis BITOP): a source key that does not exist "counts as an empty
// string". An empty string is all zero bits, which is the IDENTITY element for
// OR and XOR — but the ANNIHILATOR for AND. So ANDing with a missing source
// must contribute an all-zero operand and clear every bit of the result.
//
// The probe showed CacheStorm does something else entirely for
// `BITOP AND dest k1 missing`: it stored a byte-for-byte COPY of k1 (0xFF),
// which is what OR would produce. That is wrong on two independent grounds:
//
//  1. COMMUTATIVITY (no Redis reference needed): AND is commutative, so
//     `BITOP AND dest k1 missing` and `BITOP AND dest missing k1` must yield
//     identical bytes. CacheStorm returns 0xFF for the first and 0x00 for the
//     second. An operation whose result depends on the order of its operands
//     is self-evidently broken.
//
//  2. THE MISSING-KEY CONTRACT: a missing key is an empty string, so AND must
//     zero the result rather than copy the other operand.
//
// Root cause: the seeding branch was keyed on the LOOP INDEX (`if i == 0`)
// rather than on whether the accumulator had been seeded yet, and a missing
// source was skipped outright via `continue`. So when source 0 was missing the
// next present source was seeded verbatim, with no AND applied at all.
//
// Controls (must pass before AND after): OR and XOR treat a missing source as
// the identity and must still copy the other operand; AND with both sources
// present must still genuinely AND them; BITOP NOT is unaffected.
func TestProofBitopMissingSourceAndAnnihilates(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)

	// k1 = 0xFF (all eight bits set). k2 = 0x80 (only bit 0 set).
	for i := 0; i < 8; i++ {
		execB(s, r, "SETBIT", "k1", string(rune('0'+i)), "1")
	}
	execB(s, r, "SETBIT", "k2", "0", "1")

	// ---- CONTROL 1: OR with a missing source is the identity — the other
	// operand must survive intact.
	if got := execB(s, r, "BITOP", "OR", "o1", "k1", "missing"); got != ":1\r\n" {
		t.Fatalf("CONTROL 1 broken harness: BITOP OR reply = %q, want \":1\\r\\n\"", got)
	}
	if got := destBytes(t, s, "o1"); got != "\xff" {
		t.Fatalf("CONTROL 1 broken harness: BITOP OR k1 missing stored %q, want the copy 0xff", got)
	}
	t.Log("CONTROL 1 ok: OR keeps the other operand (missing is the identity)")

	// ---- CONTROL 2: XOR likewise.
	if got := execB(s, r, "BITOP", "XOR", "x1", "k1", "missing"); got != ":1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: BITOP XOR reply = %q, want \":1\\r\\n\"", got)
	}
	if got := destBytes(t, s, "x1"); got != "\xff" {
		t.Fatalf("CONTROL 2 broken harness: BITOP XOR k1 missing stored %q, want the copy 0xff", got)
	}
	t.Log("CONTROL 2 ok: XOR keeps the other operand")

	// ---- CONTROL 3: AND with BOTH sources present must genuinely AND them.
	if got := execB(s, r, "BITOP", "AND", "b1", "k1", "k2"); got != ":1\r\n" {
		t.Fatalf("CONTROL 3 broken harness: BITOP AND reply = %q, want \":1\\r\\n\"", got)
	}
	if got := destBytes(t, s, "b1"); got != "\x80" {
		t.Fatalf("CONTROL 3 broken harness: BITOP AND k1 k2 stored %q, want 0x80 (0xff AND 0x80)", got)
	}
	t.Log("CONTROL 3 ok: AND with both sources present really ANDs")

	// ---- CONTROL 4: NOT is unaffected by this defect.
	if got := execB(s, r, "BITOP", "NOT", "n1", "k1"); got != ":1\r\n" {
		t.Fatalf("CONTROL 4 broken harness: BITOP NOT reply = %q, want \":1\\r\\n\"", got)
	}
	if got := destBytes(t, s, "n1"); got != "\x00" {
		t.Fatalf("CONTROL 4 broken harness: BITOP NOT k1 stored %q, want the inversion 0x00", got)
	}
	t.Log("CONTROL 4 ok: NOT still inverts")

	// ---- THE DEFECT, ground 1: AND is not commutative in CacheStorm.
	execB(s, r, "BITOP", "AND", "d1", "k1", "missing")
	execB(s, r, "BITOP", "AND", "d2", "missing", "k1")

	forward := destBytes(t, s, "d1")
	reverse := destBytes(t, s, "d2")

	if forward != reverse {
		t.Fatalf("FAIL: BITOP AND is not commutative. `BITOP AND d1 k1 missing` stored %q but "+
			"`BITOP AND d2 missing k1` stored %q. AND is commutative, so the operand order "+
			"must not change the result; a missing source is an empty (all-zero) operand, "+
			"not a reason to copy the other one.",
			forward, reverse)
	}
	t.Log("PASS: BITOP AND is commutative with a missing source")

	// ---- THE DEFECT, ground 2: the missing operand must CLEAR the bits, not
	// copy them. Both orders must land on the all-zero byte.
	if forward != "\x00" {
		t.Fatalf("FAIL: `BITOP AND dest k1 missing` stored %q, want %q — a missing source is an "+
			"empty string of zero bits, and ANDing with it can only clear every bit", forward, "\x00")
	}
	t.Log("PASS: a missing source annihilates the AND result")
}
