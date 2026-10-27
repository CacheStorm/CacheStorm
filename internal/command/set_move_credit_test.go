package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetMoveCreditX(t *testing.T) {
	s := store.NewStore()
	s.ConfigureMemory(1024, store.EvictionNoEviction, 70, 90, 5)
	r := NewRouter()
	RegisterSetCommands(r)
	member := strings.Repeat("m", 700)
	if got := runEntryTransfer(t, s, nil, r, "SADD", "s", member); got != ":1\r\n" {
		t.Fatal("CONTROL", got)
	}
	before := s.MemUsage()
	got := runEntryTransfer(t, s, nil, r, "SMOVE", "s", "d", member)
	t.Logf("EXPECTED: :1, moved, net-neutral usage=%d | ACTUAL: %q src=%v dst=%v usage=%d", before, got, s.Exists("s"), s.Exists("d"), s.MemUsage())
	if got != ":1\r\n" || s.Exists("s") || !s.Exists("d") || s.MemUsage() != before {
		t.Error("net-neutral move must not need duplicate capacity")
	}
}
