package cluster

import "testing"

// Slot-bounds audit for internal/cluster.
//
// c.slots is [NumSlots]*SlotInfo with NumSlots = 16384, but every slot number in
// this package is a uint16 (0..65535). Any site that indexes c.slots with a
// caller-supplied or state-carried uint16 without a NumSlots check panics with
// "index out of range [N] with length 16384".
//
// AUDIT of every c.slots[] access in the package (cluster.go + failover.go):
//
//	cluster.go:125   RemoveNode            range &c.slots          SAFE (0..16383)
//	cluster.go:159   AssignSlots           bounded in round 33    SAFE
//	cluster.go:167   GetSlotOwner          slot uint16, caller     *** was UNGUARDED ***
//	cluster.go:207   BalanceSlots          counts sum to 16384    SAFE
//	failover.go:204  completeFailover      range f.failedSlots     guarded (defence in depth)
//	failover.go:300  SlotMigrator.Complete range m.slots, read    guarded (defence in depth)
//	failover.go:308  SlotMigrator.Complete range m.slots, write   guarded (defence in depth)
//	failover.go:440  Rebalance             slot += count, sum 16384  SAFE
//	failover.go:462  GetSlotDistribution   for i := 0; i < 16384   SAFE
//	failover.go:495  CheckClusterHealth    for i := 0; i < 16384   SAFE
//
// GetSlotKey does NOT exist anywhere in this repository -- there is no such
// accessor to audit.
//
// GetSlotOwner was the only site reachable with an out-of-range slot: it is
// exported, and hash_slots.go:85, hash_slots.go:98 and failover.go:257 all call
// it with a slot number. StartMigration (failover.go:261) routes its per-slot
// ownership check through GetSlotOwner rather than indexing c.slots directly, so
// bounding GetSlotOwner also makes StartMigration reject an out-of-range slot
// with a normal error instead of panicking.

func newSlotBoundsCluster() *Cluster {
	return New("node1", "127.0.0.1", 7000, 7001, nil)
}

// TestGetSlotOwnerRejectsOutOfRangeSlot is the primary case: the exported
// accessor must return nil rather than indexing past the array.
func TestGetSlotOwnerRejectsOutOfRangeSlot(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetSlotOwner panicked on slot 20000: %v — slot is uint16 "+
				"(0..65535) but c.slots is [NumSlots]*SlotInfo (16384)", r)
		}
	}()

	c := newSlotBoundsCluster()
	for _, slot := range []uint16{NumSlots, NumSlots + 1, 20000, 65535} {
		if owner := c.GetSlotOwner(slot); owner != nil {
			t.Fatalf("GetSlotOwner(%d) = %v, want nil for an out-of-range slot", slot, owner)
		}
	}
}

// TestStartMigrationRejectsOutOfRangeSlot covers the migrated path: StartMigration
// validates every slot through GetSlotOwner (failover.go:261), so an
// out-of-range slot must be refused with an error and must never be accepted
// into m.slots, which Complete() would then index directly.
func TestStartMigrationRejectsOutOfRangeSlot(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("StartMigration panicked on slot 20000: %v", r)
		}
	}()

	c := newSlotBoundsCluster()
	c.AddNode(&Node{ID: "n2", Addr: "127.0.0.2", Role: RolePrimary})

	m := NewSlotMigrator(c)
	if err := m.StartMigration("n1", "n2", []uint16{20000}); err == nil {
		t.Fatal("StartMigration ACCEPTED out-of-range slot 20000; m.slots is read and " +
			"written directly by Complete() at failover.go:300/308")
	}
}

// TestSlotAccessorsControls is the control: in-range slot access is unchanged --
// owned slots still resolve, unowned ones still return nil, and the last valid
// index (NumSlots-1) is still addressable.
func TestSlotAccessorsControls(t *testing.T) {
	c := newSlotBoundsCluster()
	c.AssignSlots([]SlotRange{{Start: 100, End: 200}})

	if c.GetSlotOwner(150) == nil {
		t.Fatal("CONTROL FAIL: slot 150 unowned after {100,200}")
	}
	if c.GetSlotOwner(100) == nil || c.GetSlotOwner(200) == nil {
		t.Fatal("CONTROL FAIL: {100,200} bounds must be inclusive")
	}
	if c.GetSlotOwner(99) != nil {
		t.Fatal("CONTROL FAIL: slot 99 is outside {100,200} but is owned")
	}
	if c.GetSlotOwner(201) != nil {
		t.Fatal("CONTROL FAIL: slot 201 is outside {100,200} but is owned")
	}
	if c.GetSlotOwner(0) != nil {
		t.Fatal("CONTROL FAIL: slot 0 is owned on a fresh cluster")
	}
	if c.GetSlotOwner(NumSlots-1) != nil {
		t.Fatalf("CONTROL FAIL: slot %d is owned though only {100,200} was assigned", NumSlots-1)
	}

	// A full-range assignment must still own every slot, including the boundary.
	c2 := newSlotBoundsCluster()
	c2.AssignSlots([]SlotRange{{Start: 0, End: NumSlots - 1}})
	if c2.GetSlotOwner(0) == nil {
		t.Fatal("CONTROL FAIL: slot 0 unowned after a full-range assignment")
	}
	if c2.GetSlotOwner(NumSlots-1) == nil {
		t.Fatalf("CONTROL FAIL: slot %d unowned after a full-range assignment", NumSlots-1)
	}
}
