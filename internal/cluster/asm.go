package cluster

import "github.com/cachestorm/cachestorm/internal/logger"

// InternalSecret is the secret "AUTH internal connection" checks.
func (c *Cluster) InternalSecret() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.secret
}

// AnySlotOpen reports whether any slot is migrating or importing, the
// legacy slot migration state that atomic slot migration waits out.
func (c *Cluster) AnySlotOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := 0; i < Slots; i++ {
		if c.migrating[i] != nil || c.importing[i] != nil {
			return true
		}
	}
	return false
}

// NodeAddr returns the address of the node id, and whether it is a master.
func (c *Cluster) NodeAddr(id string) (addr Addr, master, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := c.nodes[id]
	if n == nil || n.Flags&FlagHandshake != 0 {
		return Addr{}, false, false
	}
	return n.addr(), !n.replica(), true
}

// TakeOverSlots makes the slots of ranges this node's, bumps its config
// epoch without asking the other masters and tells every node, as Redis'
// atomic slot migration does once the destination has caught up.
func (c *Cluster) TakeOverSlots(ranges []SlotRange) {
	c.mu.Lock()
	for _, r := range ranges {
		for j := r.Start; j <= r.End; j++ {
			c.migrating[j], c.importing[j] = nil, nil
			c.addSlotLocked(c.myself, j)
		}
	}
	c.bumpEpochLocked()
	c.todoSave = true
	logger.Info().Uint64("epoch", c.myself.ConfigEpoch).Msg("cluster: took over the slots of a slot migration")
	c.broadcastPongLocked(false)
	c.updateStateLocked()
	c.unlockAndSave()
}
