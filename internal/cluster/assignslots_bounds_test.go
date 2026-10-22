package cluster

import "testing"

// Regression: AssignSlots indexed c.slots with unvalidated SlotRange bounds.
//
//	internal/cluster/cluster.go:140 (before)
//	  c.self.Slots = slots
//	  for _, sr := range slots {
//	  	for i := sr.Start; i <= sr.End; i++ {
//	  		c.slots[i] = &SlotInfo{Primary: c.self}
//
// SlotRange.Start/End are uint16 (0..65535) but c.slots is [NumSlots]*SlotInfo,
// NumSlots = 16384. Two consequences:
//
//  1. Any bound >= 16384 indexes past the array and panics
//     ("index out of range [16384] with length 16384").
//  2. The loop counter `i` is itself uint16, so a range whose End is MaxUint16
//     would wrap it back to 0 and spin forever instead of even panicking.
//     The array bound fires first today, so this is latent rather than observed,
//     but the same bound check closes both.
//
// Reachable from a client: internal/command/cluster_commands.go:185 calls
// globalCluster.AssignSlots(slots) from CLUSTER ADDSLOTS, where Start/End are
// parsed straight off the wire.

func newBoundsCluster() *Cluster {
	return New("node1", "127.0.0.1", 7000, 7001, nil)
}

// TestAssignSlotsOverWideRangeDoesNotPanic is the primary case: a deliberately
// over-wide range (Start..MaxUint16) must be rejected, not indexed.
func TestAssignSlotsOverWideRangeDoesNotBePanicked(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AssignSlots panicked on {0, 65535}: %v — Start/End are uint16 "+
				"(0..65535) but c.slots is [NumSlots]*SlotInfo, so the range is indexed "+
				"past the end of the array", r)
		}
	}()

	c := newBoundsCluster()
	c.AssignSlots([]SlotRange{{Start: 0, End: 65535}})

	// The over-wide range must not have been recorded as an assignment.
	if len(c.self.Slots) != 0 {
		t.Fatalf("self.Slots kept %d unusable ranges, want 0: %+v", len(c.self.Slots), c.self.Slots)
	}
	// And nothing should have been assigned.
	if c.GetSlotOwner(0) != nil {
		t.Fatal("slot 0 has an owner after only an over-wide range was assigned")
	}
}

// TestAssignSlotsOverlappingOutOfRange covers a range that starts in bounds and
// runs past the end — the shape a malformed CLUSTER ADDSLOTS would produce.
func TestAssignSlotsOverlappingOutOfRange(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AssignSlots panicked on {16000, 20000}: %v", r)
		}
	}()

	c := newBoundsCluster()
	c.AssignSlots([]SlotRange{{Start: 16000, End: 20000}})
	if len(c.self.Slots) != 0 {
		t.Fatalf("self.Slots kept an unusable range: %+v", c.self.Slots)
	}
}

// TestAssignSlotsSkipsUnusableRangesButKeepsValidOnes checks the mixed case: a
// valid range alongside an invalid one is applied, and only the invalid one is
// dropped. The surviving assignment must be exact at both ends.
func TestAssignSlotsSkipsUnusableRangesButKeepsValidOnes(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AssignSlots panicked on a mixed range set: %v", r)
		}
	}()

	c := newBoundsCluster()
	c.AssignSlots([]SlotRange{
		{Start: 0, End: 65535},                // unusable: End >= NumSlots
		{Start: 100, End: 200},                // valid
		{Start: NumSlots, End: NumSlots + 10}, // unusable: Start >= NumSlots
	})

	if len(c.self.Slots) != 1 {
		t.Fatalf("self.Slots has %d ranges, want only the valid one: %+v", len(c.self.Slots), c.self.Slots)
	}
	if c.self.Slots[0].Start != 100 || c.self.Slots[0].End != 200 {
		t.Fatalf("surviving range = %+v, want {100 200}", c.self.Slots[0])
	}
	if c.GetSlotOwner(100) == nil {
		t.Fatal("slot 100 should be owned by the valid range")
	}
	if c.GetSlotOwner(200) == nil {
		t.Fatal("slot 200 should be owned by the valid range (bounds are inclusive)")
	}
	if c.GetSlotOwner(99) != nil {
		t.Fatal("slot 99 is outside {100,200} but has an owner")
	}
	if c.GetSlotOwner(201) != nil {
		t.Fatal("slot 201 is outside {100,200} but has an owner")
	}
}

// TestAssignSlotsFullValidRangeIsControl is the control: a full-range and a
// partial in-range assignment still take effect exactly as before the fix.
func TestAssignSlotsFullValidRangeIsControl(t *testing.T) {
	c := newBoundsCluster()
	c.AssignSlots([]SlotRange{{Start: 0, End: NumSlots - 1}})
	if c.GetSlotOwner(0) == nil {
		t.Fatal("CONTROL FAIL: slot 0 unowned after a full-range assignment")
	}
	if c.GetSlotOwner(NumSlots-1) == nil {
		t.Fatalf("CONTROL FAIL: slot %d unowned after a full-range assignment", NumSlots-1)
	}

	c2 := newBoundsCluster()
	c2.AssignSlots([]SlotRange{{Start: 10, End: 20}})
	if c2.GetSlotOwner(15) == nil {
		t.Fatal("CONTROL FAIL: slot 15 unowned after {10,20}")
	}
	if c2.GetSlotOwner(21) != nil {
		t.Fatal("CONTROL FAIL: slot 21 owned though never assigned")
	}
}
