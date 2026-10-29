package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestStringGrowthKeepsPublishedBytes checks that APPEND and SETRANGE, which
// extend a string in its spare capacity, leave every earlier value and copy
// as it was.
func TestStringGrowthKeepsPublishedBytes(t *testing.T) {
	h := newLuaHarness(t)
	RegisterStringCommands(h.router)
	RegisterKeyCommands(h.router)
	h.do("SET", "k", "ab")
	h.do("APPEND", "k", "cd")
	entry, _ := h.s.Get("k")
	before := entry.Value.(*store.StringValue).Data
	h.do("APPEND", "k", "ef")
	h.do("COPY", "k", "c")
	h.do("APPEND", "k", "gh")
	h.do("APPEND", "c", "XY")
	h.do("SETRANGE", "k", "10", "Z")
	if string(before) != "abcd" {
		t.Errorf("earlier value changed to %q", before)
	}
	for key, want := range map[string]string{"k": "abcdefgh\x00\x00Z", "c": "abcdefXY"} {
		if got := h.do("GET", key); !strings.Contains(got, want) {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
	}
	var want strings.Builder
	for range 5000 {
		h.do("APPEND", "big", "0123456789")
		want.WriteString("0123456789")
	}
	if got := h.do("STRLEN", "big"); got != ":50000\r\n" {
		t.Errorf("STRLEN big: %q", got)
	}
	if got := h.do("GET", "big"); !strings.Contains(got, want.String()) {
		t.Error("appended string differs")
	}
}
