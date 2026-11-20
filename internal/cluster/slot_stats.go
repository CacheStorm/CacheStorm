package cluster

import "sync/atomic"

// slotUsage is the usage CLUSTER SLOT-STATS reports for one slot besides its
// key count: the CPU time and network bytes of the commands that ran on it.
type slotUsage struct {
	cpuUsec, bytesIn, bytesOut atomic.Uint64
}

func (u *slotUsage) reset() {
	u.cpuUsec.Store(0)
	u.bytesIn.Store(0)
	u.bytesOut.Store(0)
}

// SlotUsage is one slot's counters.
type SlotUsage struct {
	CPUUsec, BytesIn, BytesOut uint64
}

// AddSlotUsage adds the CPU time and the bytes of a command to slot.
func (c *Cluster) AddSlotUsage(slot int, cpuUsec, bytesIn, bytesOut uint64) {
	if slot < 0 || slot >= Slots {
		return
	}
	u := &c.usage[slot]
	if cpuUsec != 0 {
		u.cpuUsec.Add(cpuUsec)
	}
	if bytesIn != 0 {
		u.bytesIn.Add(bytesIn)
	}
	if bytesOut != 0 {
		u.bytesOut.Add(bytesOut)
	}
}

// SlotUsageOf returns slot's counters.
func (c *Cluster) SlotUsageOf(slot int) SlotUsage {
	u := &c.usage[slot]
	return SlotUsage{CPUUsec: u.cpuUsec.Load(), BytesIn: u.bytesIn.Load(), BytesOut: u.bytesOut.Load()}
}

// ResetSlotUsage zeroes every slot's counters, as CONFIG RESETSTAT does.
func (c *Cluster) ResetSlotUsage() {
	for i := range c.usage {
		c.usage[i].reset()
	}
}

// ShardSlots returns, in ascending order, the slots served by this node's
// shard: its own, or its master's on a replica.
func (c *Cluster) ShardSlots() []int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	primary := c.myself
	if m := c.nodes[c.myself.MasterID]; c.myself.replica() && m != nil {
		primary = m
	}
	slots := make([]int, 0, primary.numSlots)
	for s := 0; s < Slots; s++ {
		if testBit(primary.slots[:], s) {
			slots = append(slots, s)
		}
	}
	return slots
}
