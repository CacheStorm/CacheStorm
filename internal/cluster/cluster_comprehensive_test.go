package cluster

import (
	"testing"
)

func TestClusterStartStop(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	err := c.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !c.IsEnabled() {
		t.Error("cluster should be enabled after start")
	}

	c.Stop()
}

func TestClusterIsEnabled(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	if c.IsEnabled() {
		t.Error("cluster should not be enabled initially")
	}

	c.Start()
	if !c.IsEnabled() {
		t.Error("cluster should be enabled after start")
	}
}

func TestClusterGetClusterNodes(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	nodes := c.GetClusterNodes()
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}

	if nodes[0]["id"] != "node-1" {
		t.Errorf("expected node-1, got %v", nodes[0]["id"])
	}

	if nodes[0]["role"] != "master" {
		t.Errorf("expected master role, got %v", nodes[0]["role"])
	}
}

func TestNodeRoleReplica(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RoleReplica,
		State: NodeStateOnline,
	}
	c.AddNode(node)

	nodes := c.GetClusterNodes()
	var found bool
	for _, n := range nodes {
		if n["id"] == "node-2" && n["role"] == "slave" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find node-2 as slave")
	}
}

func TestNodeStateUnknown(t *testing.T) {
	state := NodeState(99)
	if state.String() != "unknown" {
		t.Errorf("expected 'unknown', got '%s'", state.String())
	}
}

func TestFailoverManagerStartFailover(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	err := fm.StartFailover("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
		Slots: []SlotRange{{Start: 0, End: 8191}},
	}
	c.AddNode(node2)

	err = fm.StartFailover("node-2")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSlotMigratorStartMigration(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	sm := NewSlotMigrator(c)

	err := sm.StartMigration("nonexistent", "node-1", []uint16{0})
	if err == nil {
		t.Error("expected error for nonexistent source")
	}

	err = sm.StartMigration("node-1", "nonexistent", []uint16{0})
	if err == nil {
		t.Error("expected error for nonexistent target")
	}

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	err = sm.StartMigration("node-2", "node-1", []uint16{0})
	if err == nil {
		t.Error("expected error for slots not owned by source")
	}

	err = sm.StartMigration("node-1", "node-2", []uint16{0})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = sm.StartMigration("node-1", "node-2", []uint16{1})
	if err == nil {
		t.Error("expected error for migration already in progress")
	}
}

func TestSlotMigratorUpdateProgress(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
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
	sm.StartMigration("node-1", "node-2", []uint16{0})

	sm.UpdateProgress(50, 1024)

	status := sm.GetStatus()
	if status["progress"] != 50 {
		t.Errorf("expected progress 50, got %v", status["progress"])
	}
	if status["bytes_sent"] != int64(1024) {
		t.Errorf("expected bytes_sent 1024, got %v", status["bytes_sent"])
	}
}

func TestSlotMigratorComplete(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
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

	err := sm.Complete()
	if err == nil {
		t.Error("expected error when no migration in progress")
	}

	sm.StartMigration("node-1", "node-2", []uint16{0})

	err = sm.Complete()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	status := sm.GetStatus()
	if status["state"] != "completed" {
		t.Errorf("expected state completed, got %v", status["state"])
	}
	if status["progress"] != 100 {
		t.Errorf("expected progress 100, got %v", status["progress"])
	}
}

func TestSlotMigratorCancel(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
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
	sm.StartMigration("node-1", "node-2", []uint16{0})
	sm.UpdateProgress(50, 1024)

	sm.Cancel()

	status := sm.GetStatus()
	if status["state"] != "cancelled" {
		t.Errorf("expected state cancelled, got %v", status["state"])
	}
	if status["progress"] != 0 {
		t.Errorf("expected progress 0, got %v", status["progress"])
	}
}

func TestCRC16(t *testing.T) {
	result := CRC16([]byte(""))
	if result != 0 {
		t.Errorf("CRC16(empty) = %x, expected 0", result)
	}

	result = CRC16([]byte("test"))
	if result == 0 {
		t.Error("CRC16 should not be 0 for non-empty string")
	}

	result1 := CRC16([]byte("key1"))
	result2 := CRC16([]byte("key2"))
	if result1 == result2 {
		t.Error("CRC16 should produce different results for different inputs")
	}
}

func TestKeySlot(t *testing.T) {
	slot1 := KeySlot("somekey")
	if slot1 >= NumSlots {
		t.Errorf("slot should be less than %d, got %d", NumSlots, slot1)
	}

	slot2 := KeySlot("{user:1}:profile")
	slot3 := KeySlot("{user:1}:data")
	if slot2 != slot3 {
		t.Errorf("keys with same hash tag should map to same slot: %d != %d", slot2, slot3)
	}

	slot4 := KeySlot("{hash}:key1")
	slot5 := KeySlot("{hash}:key2")
	if slot4 != slot5 {
		t.Errorf("keys with same hash tag should map to same slot: %d != %d", slot4, slot5)
	}

	slot6 := KeySlot("{}:key")
	if slot6 >= NumSlots {
		t.Errorf("slot should be less than %d, got %d", NumSlots, slot6)
	}

	slot7 := KeySlot("{key")
	if slot7 >= NumSlots {
		t.Errorf("slot should be less than %d, got %d", NumSlots, slot7)
	}
}

func TestHashSlotRouter(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	router := NewHashSlotRouter(c)

	slot := router.GetSlot("testkey")
	if slot >= NumSlots {
		t.Errorf("slot should be less than %d, got %d", NumSlots, slot)
	}

	node := router.GetNodeForKey("testkey")
	if node == nil {
		t.Error("expected node for key")
	}

	if !router.IsLocal("testkey") {
		t.Error("key should be local")
	}

	slot, addr, port := router.GetMovedError("testkey")
	if slot >= NumSlots {
		t.Errorf("slot should be less than %d, got %d", NumSlots, slot)
	}
	if addr != "127.0.0.1" {
		t.Errorf("expected addr 127.0.0.1, got %s", addr)
	}
	if port != 6380 {
		t.Errorf("expected port 6380, got %d", port)
	}
}

func TestHashSlotRouterNoOwner(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	router := NewHashSlotRouter(c)

	node := router.GetNodeForKey("testkey")
	if node != nil {
		t.Error("expected nil node when no slots assigned")
	}

	if !router.IsLocal("testkey") {
		t.Error("should be local when no owner")
	}

	_, addr, port := router.GetMovedError("testkey")
	if addr != "" || port != 0 {
		t.Errorf("expected empty addr and port, got %s:%d", addr, port)
	}
}

func TestClusterGetClusterStats(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	stats := c.GetClusterStats()

	if stats["health"] == nil {
		t.Error("expected health in stats")
	}
	if stats["slot_distribution"] == nil {
		t.Error("expected slot_distribution in stats")
	}
}

func TestGossipNew(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	if g == nil {
		t.Fatal("expected gossip")
	}
}

func TestGossipGetNodeInfoList(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline

	g := NewGossip(c)
	nodes := g.getNodeInfoList()

	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}

	if nodes[0].ID != "node-1" {
		t.Errorf("expected node-1, got %s", nodes[0].ID)
	}
}

func TestFailoverStateString(t *testing.T) {
	tests := []struct {
		state    FailoverState
		expected int
	}{
		{FailoverNone, 0},
		{FailoverWaiting, 1},
		{FailoverInProgress, 2},
		{FailoverCompleted, 3},
	}

	for _, tt := range tests {
		if int(tt.state) != tt.expected {
			t.Errorf("expected %d, got %d", tt.expected, tt.state)
		}
	}
}
