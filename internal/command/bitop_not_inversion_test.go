package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execBit runs the real BITOP/BITCOUNT handler against s, returning the reply.
func execBit(s *store.Store, r *Router, cmd string, args ...string) string {
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

// TestProofBITOPNotInverts is the round proof.
//
// Contract (Redis BITOP): "NOT destkey srckey" — NOT takes exactly ONE source
// and stores its bitwise INVERSION (~src) in destkey. AND/OR/XOR combine two or
// more sources.
//
// Defect: the loop's first branch (`if i == 0 { copy; continue }`) runs before
// the inner op switch, so the `case "NOT": if i == 0 { result[j] = ^bm.Data[j] }`
// inside that switch can never execute — i is never 0 there. BITOP NOT
// therefore stores a straight COPY of the source instead of its inversion.
//
// Controls: AND and OR (multi-source combining ops) must be correct before and
// after the fix, so a broken harness cannot masquerade as the defect.
func TestProofBITOPNotInverts(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)

	// Seed source: bit 0 set, bit 1 clear -> data byte 0b00000001.
	if got := execBit(s, r, "SETBIT", "src", "0", "1"); got != ":0\r\n" {
		t.Fatalf("CONTROL: SETBIT seeding = %q, want \":0\\r\\n\"", got)
	}
	if got := execBit(s, r, "BITCOUNT", "src"); got != ":1\r\n" {
		t.Fatalf("CONTROL: source BITCOUNT = %q, want \":1\\r\\n\" (one bit set)", got)
	}

	// ---- CONTROL 1: OR of two sources (multi-source combining op).
	if got := execBit(s, r, "SETBIT", "b", "1", "1"); got != ":0\r\n" {
		t.Fatalf("CONTROL 1 setup: SETBIT b = %q", got)
	}
	if got := execBit(s, r, "BITOP", "OR", "or:dst", "src", "b"); got != ":1\r\n" {
		t.Fatalf("CONTROL 1 broken harness: BITOP OR = %q, want \":1\\r\\n\"", got)
	}
	// src has bit0, b has bit1 -> OR has bits 0 and 1 -> count 2.
	if got := execBit(s, r, "BITCOUNT", "or:dst"); got != ":2\r\n" {
		t.Fatalf("CONTROL 1 broken harness: OR result BITCOUNT = %q, want \":2\\r\\n\"", got)
	}
	if got := execBit(s, r, "GETBIT", "or:dst", "0"); got != ":1\r\n" {
		t.Fatalf("CONTROL 1 broken harness: OR result bit0 = %q, want \":1\\r\\n\"", got)
	}

	// ---- CONTROL 2: AND of two sources.
	if got := execBit(s, r, "SETBIT", "c", "0", "1"); got != ":0\r\n" {
		t.Fatalf("CONTROL 2 setup: SETBIT c = %q", got)
	}
	if got := execBit(s, r, "BITOP", "AND", "and:dst", "src", "c"); got != ":1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: BITOP AND = %q, want \":1\\r\\n\"", got)
	}
	if got := execBit(s, r, "BITCOUNT", "and:dst"); got != ":1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: AND result BITCOUNT = %q, want \":1\\r\\n\"", got)
	}

	// ---- THE DEFECT: BITOP NOT must INVERT its single source.
	// src byte = 0b00000001. Its inversion is 0b11111110: bit0 must become 0
	// and bit1 must become 1.
	if got := execBit(s, r, "BITOP", "NOT", "not:dst", "src"); got != ":1\r\n" {
		t.Fatalf("DEFECT case harness: BITOP NOT = %q, want \":1\\r\\n\" (one byte result)", got)
	}

	bit0 := execBit(s, r, "GETBIT", "not:dst", "0")
	bit1 := execBit(s, r, "GETBIT", "not:dst", "1")
	bit7 := execBit(s, r, "GETBIT", "not:dst", "7")

	if bit0 != ":0\r\n" || bit1 != ":1\r\n" || bit7 != ":1\r\n" {
		t.Fatalf("FAIL: BITOP NOT did not invert. Source src bit0=1 bit1=0 bit7=0; "+
			"NOT result bit0=%s bit1=%s bit7=%s — want bit0=0 bit1=1 bit7=1 "+
			"(~0b00000001 = 0b11111110)",
			strings.TrimSpace(bit0), strings.TrimSpace(bit1), strings.TrimSpace(bit7))
	}
	t.Log("PASS: BITOP NOT produced the inverted bitmap")

	// ---- SECONDARY CHECK: the inverted result must have the flipped popcount.
	// Source has exactly 1 bit set out of 8; its inversion must have 7.
	if got := execBit(s, r, "BITCOUNT", "not:dst"); got != ":7\r\n" {
		t.Fatalf("FAIL: BITOP NOT result BITCOUNT = %q, want \":7\\r\\n\" "+
			"(inverting 1 set bit of 8 yields 7 set bits)", got)
	}
	t.Log("PASS: inverted popcount is 7 as expected")
}
