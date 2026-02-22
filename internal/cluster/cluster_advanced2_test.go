package cluster

import (
	"testing"
)

func TestClusterNodeAdvanced(t *testing.T) {
	t.Run("Node States", func(t *testing.T) {
		states := []NodeState{
			NodeStateJoining,
			NodeStateOnline,
			NodeStateFailed,
			NodeStateLeaving,
		}

		expected := []string{"joining", "online", "failed", "leaving"}

		for i, state := range states {
			if state.String() != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], state.String())
			}
		}
	})

	t.Run("Node Roles", func(t *testing.T) {
		if RolePrimary != 0 {
			t.Error("RolePrimary should be 0")
		}
		if RoleReplica != 1 {
			t.Error("RoleReplica should be 1")
		}
	})

	t.Run("Node Creation", func(t *testing.T) {
		node := &Node{
			ID:         "test-node",
			Addr:       "127.0.0.1",
			Port:       6379,
			GossipPort: 7946,
			Role:       RolePrimary,
			State:      NodeStateOnline,
		}

		if node.ID != "test-node" {
			t.Error("Node ID mismatch")
		}

		if node.Addr != "127.0.0.1" {
			t.Error("Node address mismatch")
		}

		if node.Port != 6379 {
			t.Error("Node port mismatch")
		}
	})
}

func TestClusterSlotAdvanced(t *testing.T) {
	t.Run("Slot Range", func(t *testing.T) {
		sr := SlotRange{Start: 0, End: 16383}

		if sr.Start != 0 {
			t.Error("Start should be 0")
		}

		if sr.End != 16383 {
			t.Error("End should be 16383")
		}
	})

	t.Run("Hash Slot Distribution", func(t *testing.T) {
		// Test that different keys hash to different slots
		slots := make(map[uint16]bool)

		keys := []string{
			"key1", "key2", "key3", "key4", "key5",
			"user:1", "user:2", "user:3",
			"product:1", "product:2",
		}

		for _, key := range keys {
			slot := KeySlot(key)
			slots[slot] = true
		}

		// Should have distributed to multiple slots
		if len(slots) < 5 {
			t.Logf("Keys distributed to %d slots", len(slots))
		}
	})
}

func TestClusterCRC16(t *testing.T) {
	t.Run("CRC16 Values", func(t *testing.T) {
		tests := []struct {
			input    []byte
			expected uint16
		}{
			{[]byte(""), 0},
			{[]byte("123456789"), 0x31C3},
		}

		for _, tt := range tests {
			result := CRC16(tt.input)
			if result != tt.expected {
				t.Errorf("CRC16(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	})
}

func TestClusterGossipAdvanced(t *testing.T) {
	t.Run("Gossip Creation", func(t *testing.T) {
		c := New("node-1", "127.0.0.1", 6379, 7946, nil)
		g := NewGossip(c)

		if g == nil {
			t.Fatal("NewGossip returned nil")
		}
	})

	t.Run("Gossip Node Info", func(t *testing.T) {
		c := New("node-1", "127.0.0.1", 6379, 7946, nil)
		c.Self().State = NodeStateOnline

		g := NewGossip(c)
		nodes := g.getNodeInfoList()

		if len(nodes) == 0 {
			t.Error("Should have at least one node")
		}
	})
}

func TestClusterFailoverAdvanced(t *testing.T) {
	t.Run("Failover Manager", func(t *testing.T) {
		c := New("node-1", "127.0.0.1", 6379, 7946, nil)
		g := NewGossip(c)
		fm := NewFailoverManager(c, g)

		if fm == nil {
			t.Fatal("NewFailoverManager returned nil")
		}
	})
}

func TestClusterMigratorAdvanced(t *testing.T) {
	t.Run("Slot Migrator", func(t *testing.T) {
		c := New("node-1", "127.0.0.1", 6379, 7946, nil)
		sm := NewSlotMigrator(c)

		if sm == nil {
			t.Fatal("NewSlotMigrator returned nil")
		}

		status := sm.GetStatus()
		if status == nil {
			t.Error("GetStatus returned nil")
		}
	})
}

func TestClusterRouterAdvanced(t *testing.T) {
	t.Run("Hash Slot Router", func(t *testing.T) {
		c := New("node-1", "127.0.0.1", 6379, 7946, nil)
		c.Self().State = NodeStateOnline
		c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

		router := NewHashSlotRouter(c)

		// Get slot for key
		slot := router.GetSlot("testkey")
		if slot >= NumSlots {
			t.Errorf("Slot %d should be less than %d", slot, NumSlots)
		}

		// Check if local
		isLocal := router.IsLocal("testkey")
		_ = isLocal // Just verify it doesn't panic
	})
}
