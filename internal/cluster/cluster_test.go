package cluster

import (
	"testing"
)

func TestNewCluster(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	if c == nil {
		t.Fatal("cluster should not be nil")
	}

	if c.Self().ID != "node-1" {
		t.Errorf("expected node-1, got %s", c.Self().ID)
	}
}

func TestClusterAddNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}

	c.AddNode(node)

	if c.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", c.NodeCount())
	}

	n := c.GetNode("node-2")
	if n == nil {
		t.Error("node-2 should exist")
	}
}

func TestClusterRemoveNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}

	c.AddNode(node)
	c.RemoveNode("node-2")

	if c.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", c.NodeCount())
	}

	n := c.GetNode("node-2")
	if n != nil {
		t.Error("node-2 should not exist")
	}
}

func TestClusterAssignSlots(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	slots := []SlotRange{
		{Start: 0, End: 8191},
	}

	c.AssignSlots(slots)

	if len(c.Self().Slots) != 1 {
		t.Errorf("expected 1 slot range, got %d", len(c.Self().Slots))
	}

	owner := c.GetSlotOwner(0)
	if owner == nil || owner.ID != "node-1" {
		t.Error("slot 0 should be owned by node-1")
	}
}

func TestClusterGetSlotOwner(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	slots := []SlotRange{
		{Start: 0, End: 8191},
	}
	c.AssignSlots(slots)

	owner := c.GetSlotOwner(5000)
	if owner == nil {
		t.Error("slot 5000 should have an owner")
	}

	owner = c.GetSlotOwner(10000)
	if owner != nil {
		t.Error("slot 10000 should not have an owner")
	}
}

func TestClusterBalanceSlots(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	c.BalanceSlots()

	covered := 0
	for i := 0; i < 16384; i++ {
		if c.GetSlotOwner(uint16(i)) != nil {
			covered++
		}
	}

	if covered != 16384 {
		t.Errorf("expected 16384 slots covered, got %d", covered)
	}
}

func TestClusterGetClusterInfo(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	info := c.GetClusterInfo()

	if info["cluster_slots"] != 16384 {
		t.Errorf("expected 16384 slots, got %v", info["cluster_slots"])
	}
}

func TestClusterGetNodes(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RoleReplica,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	nodes := c.GetNodes()

	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestNodeStateString(t *testing.T) {
	tests := []struct {
		state    NodeState
		expected string
	}{
		{NodeStateJoining, "joining"},
		{NodeStateOnline, "online"},
		{NodeStateFailed, "failed"},
		{NodeStateLeaving, "leaving"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

func TestFailoverManager(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	if fm == nil {
		t.Fatal("failover manager should not be nil")
	}

	if fm.GetState() != FailoverNone {
		t.Errorf("expected FailoverNone state, got %v", fm.GetState())
	}
}

func TestSlotMigrator(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	sm := NewSlotMigrator(c)

	status := sm.GetStatus()
	if status["state"] != "none" {
		t.Errorf("expected none state, got %v", status["state"])
	}

	if sm.IsMigrating() {
		t.Error("should not be migrating initially")
	}
}

func TestClusterCheckClusterHealth(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline

	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	health := c.CheckClusterHealth()

	coveredSlots, ok := health["covered_slots"].(int)
	if !ok {
		t.Errorf("covered_slots should be an int")
		return
	}

	if coveredSlots != 16384 {
		t.Errorf("expected 16384 covered slots, got %d", coveredSlots)
	}
}

func TestClusterGetSlotDistribution(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	dist := c.GetSlotDistribution()

	if dist["node-1"] != 16384 {
		t.Errorf("expected 16384 slots for node-1, got %d", dist["node-1"])
	}
}

func TestClusterRebalance(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	result := c.Rebalance()

	if result["ok"] != true {
		t.Errorf("expected ok rebalance, got %v", result)
	}

	dist := c.GetSlotDistribution()
	if dist["node-2"] == 0 {
		t.Error("node-2 should have some slots after rebalance")
	}
}
