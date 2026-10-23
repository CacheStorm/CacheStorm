package replication

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

// TestPropagateCommandBacklogIsByteIndexed pins the backlog's ring
// semantics: the backlog is a byte-indexed ring, so a command that lands
// across the end of the buffer is written whole, with the part that does not
// fit continuing at the start.
//
// The write used a COMMAND counter as a BYTE offset, so the bytes of a
// command were placed at the wrong position and a command straddling the
// end of the buffer was silently truncated.
func TestPropagateCommandBacklogIsByteIndexed(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())
	size := len(m.replBacklog)

	// Fill the backlog so that the next write lands at the very end of the
	// ring: the remaining space is exactly the length of the command below.
	m.PropagateCommand(bytes.Repeat([]byte{'x'}, size-8))

	cmd := []byte("ABCDEFGH")
	m.PropagateCommand(cmd)

	ring := m.backlogRing()
	got := ring[len(ring)-len(cmd):]
	if !bytes.Equal(got, cmd) {
		t.Fatalf("the command was truncated at the ring's end: got %q, want %q", got, cmd)
	}
}

// TestPropagateCommandBacklogKeepsTheTailOfAnOversizedCommand pins the other
// half of the contract: a command larger than the whole backlog cannot be
// stored, and the ring keeps the most recent bytes of it.
func TestPropagateCommandBacklogKeepsTheTailOfAnOversizedCommand(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())
	size := len(m.replBacklog)

	cmd := append(bytes.Repeat([]byte{'y'}, size), bytes.Repeat([]byte{'z'}, 100)...)
	m.PropagateCommand(cmd)

	ring := m.backlogRing()
	got := ring[len(ring)-100:]
	if !bytes.Equal(got, bytes.Repeat([]byte{'z'}, 100)) {
		t.Fatalf("the ring did not keep the tail of the oversized command: got %q", got[:min(20, len(got))])
	}

	// The oldest byte still in the ring is the one after the bytes that
	// were overwritten.
	if first := m.replBacklogFirstByteOffset(); first != 100 {
		t.Fatalf("the backlog's first byte offset = %d, want 100", first)
	}
}
