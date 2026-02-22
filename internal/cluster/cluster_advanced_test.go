package cluster

import (
	"testing"
)

func TestClusterNodeOperations(t *testing.T) {
	t.Run("Node Creation", func(t *testing.T) {
		node := &Node{
			ID:         "node-1",
			Addr:       "127.0.0.1",
			Port:       6379,
			GossipPort: 7946,
			Role:       RolePrimary,
			State:      NodeStateOnline,
		}
		if node.ID != "node-1" {
			t.Error("Node ID mismatch")
		}
	})

	t.Run("Node State String", func(t *testing.T) {
		states := map[NodeState]string{
			NodeStateJoining: "joining",
			NodeStateOnline:  "online",
			NodeStateFailed:  "failed",
			NodeStateLeaving: "leaving",
		}
		for state, expected := range states {
			if state.String() != expected {
				t.Errorf("Expected %s, got %s", expected, state.String())
			}
		}
	})
}

func TestClusterSlotOperations(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6379, 7946, nil)

	t.Run("Assign Slots", func(t *testing.T) {
		c.Self().State = NodeStateOnline
		c.AssignSlots([]SlotRange{{Start: 0, End: 100}})

		info := c.GetClusterInfo()
		if info == nil {
			t.Error("GetClusterInfo returned nil")
		}
	})

	t.Run("Get Slot Owner", func(t *testing.T) {
		c.Self().State = NodeStateOnline
		c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

		node := c.GetSlotOwner(0)
		if node == nil {
			t.Error("GetSlotOwner returned nil")
		}
	})
}

func TestClusterFailoverManager(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6379, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	t.Run("Failover Manager Creation", func(t *testing.T) {
		if fm == nil {
			t.Fatal("NewFailoverManager returned nil")
		}
	})

	t.Run("Start Failover Nonexistent", func(t *testing.T) {
		err := fm.StartFailover("nonexistent")
		if err == nil {
			t.Error("Should error for nonexistent node")
		}
	})
}

func TestClusterGossip(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6379, 7946, nil)
	g := NewGossip(c)

	t.Run("Gossip Creation", func(t *testing.T) {
		if g == nil {
			t.Fatal("NewGossip returned nil")
		}
	})

	t.Run("Get Node Info", func(t *testing.T) {
		c.Self().State = NodeStateOnline
		nodes := g.getNodeInfoList()
		if len(nodes) == 0 {
			t.Error("Should have at least one node")
		}
	})
}

func TestClusterSlotMigrator(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6379, 7946, nil)
	sm := NewSlotMigrator(c)

	t.Run("Slot Migrator Creation", func(t *testing.T) {
		if sm == nil {
			t.Fatal("NewSlotMigrator returned nil")
		}
	})

	t.Run("Get Status", func(t *testing.T) {
		status := sm.GetStatus()
		if status == nil {
			t.Error("GetStatus returned nil")
		}
	})
}

func TestHashSlotFunctions(t *testing.T) {
	t.Run("CRC16", func(t *testing.T) {
		result := CRC16([]byte("test"))
		if result == 0 {
			t.Error("CRC16 should not return 0 for non-empty string")
		}
	})

	t.Run("Key Slot", func(t *testing.T) {
		slot := KeySlot("testkey")
		if slot >= NumSlots {
			t.Errorf("Slot %d should be less than %d", slot, NumSlots)
		}
	})

	t.Run("Key Slot Hash Tag", func(t *testing.T) {
		slot1 := KeySlot("{user:1}:profile")
		slot2 := KeySlot("{user:1}:settings")
		if slot1 != slot2 {
			t.Error("Keys with same hash tag should map to same slot")
		}
	})
}

func TestClusterRebalanceOperations(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6379, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	t.Run("Rebalance Single Node", func(t *testing.T) {
		result := c.Rebalance()
		if result == nil {
			t.Error("Rebalance returned nil")
		}
	})
}
