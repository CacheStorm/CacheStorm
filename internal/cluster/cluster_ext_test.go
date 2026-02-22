package cluster

import (
	"testing"
	"time"
)

func TestNodeRoleConstants(t *testing.T) {
	if RolePrimary != 0 {
		t.Errorf("expected RolePrimary = 0, got %d", RolePrimary)
	}
	if RoleReplica != 1 {
		t.Errorf("expected RoleReplica = 1, got %d", RoleReplica)
	}
}

func TestNodeStateConstants(t *testing.T) {
	if NodeStateJoining != 0 {
		t.Errorf("expected NodeStateJoining = 0, got %d", NodeStateJoining)
	}
	if NodeStateOnline != 1 {
		t.Errorf("expected NodeStateOnline = 1, got %d", NodeStateOnline)
	}
	if NodeStateFailed != 2 {
		t.Errorf("expected NodeStateFailed = 2, got %d", NodeStateFailed)
	}
	if NodeStateLeaving != 3 {
		t.Errorf("expected NodeStateLeaving = 3, got %d", NodeStateLeaving)
	}
}

func TestNodeStateStringUnknown(t *testing.T) {
	state := NodeState(99)
	if state.String() != "unknown" {
		t.Errorf("expected 'unknown', got '%s'", state.String())
	}
}

func TestClusterIsEnabledFalse(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	if c.IsEnabled() {
		t.Error("expected cluster to be disabled initially")
	}
}

func TestClusterWithSeeds(t *testing.T) {
	seeds := []string{"127.0.0.1:7946", "127.0.0.1:7947"}
	c := New("node-1", "127.0.0.1", 6380, 7946, seeds)

	if c == nil {
		t.Fatal("expected cluster")
	}

	if len(c.seeds) != 2 {
		t.Errorf("expected 2 seeds, got %d", len(c.seeds))
	}
}

func TestClusterGetClusterInfoFail(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateFailed,
	}
	c.AddNode(node2)

	info := c.GetClusterInfo()

	if info["cluster_state"] != "fail" {
		t.Errorf("expected 'fail', got '%v'", info["cluster_state"])
	}
}

func TestClusterGetClusterInfoOK(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline

	info := c.GetClusterInfo()

	if info["cluster_state"] != "ok" {
		t.Errorf("expected 'ok', got '%v'", info["cluster_state"])
	}
}

func TestClusterBalanceSlotsEmpty(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.RemoveNode("node-1")

	c.BalanceSlots()
}

func TestSlotRange(t *testing.T) {
	sr := SlotRange{Start: 0, End: 8191}

	if sr.Start != 0 {
		t.Errorf("expected Start 0, got %d", sr.Start)
	}
	if sr.End != 8191 {
		t.Errorf("expected End 8191, got %d", sr.End)
	}
}

func TestNodeStruct(t *testing.T) {
	n := &Node{
		ID:         "node-1",
		Addr:       "127.0.0.1",
		Port:       6379,
		GossipPort: 7946,
		Role:       RolePrimary,
		Slots:      []SlotRange{{Start: 0, End: 16383}},
		ReplicaOf:  "",
		State:      NodeStateOnline,
		LastSeen:   time.Now(),
	}

	if n.ID != "node-1" {
		t.Errorf("expected ID 'node-1', got '%s'", n.ID)
	}
}

func TestSlotInfo(t *testing.T) {
	primary := &Node{ID: "primary-1"}
	replica := &Node{ID: "replica-1"}

	si := &SlotInfo{
		Primary:  primary,
		Replicas: []*Node{replica},
	}

	if si.Primary.ID != "primary-1" {
		t.Errorf("expected primary-1, got %s", si.Primary.ID)
	}
	if len(si.Replicas) != 1 {
		t.Errorf("expected 1 replica, got %d", len(si.Replicas))
	}
}

func TestFailoverStateConstants(t *testing.T) {
	if FailoverNone != 0 {
		t.Errorf("expected FailoverNone = 0, got %d", FailoverNone)
	}
	if FailoverWaiting != 1 {
		t.Errorf("expected FailoverWaiting = 1, got %d", FailoverWaiting)
	}
	if FailoverInProgress != 2 {
		t.Errorf("expected FailoverInProgress = 2, got %d", FailoverInProgress)
	}
	if FailoverCompleted != 3 {
		t.Errorf("expected FailoverCompleted = 3, got %d", FailoverCompleted)
	}
}

func TestFailoverManagerStartFailoverNotFound(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	err := fm.StartFailover("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestFailoverManagerStartFailoverNotPrimary(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	replica := &Node{
		ID:    "replica-1",
		Role:  RoleReplica,
		State: NodeStateOnline,
	}
	c.AddNode(replica)

	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	err := fm.StartFailover("replica-1")
	if err == nil {
		t.Error("expected error for replica node")
	}
}

func TestFailoverManagerStartFailoverAlreadyInProgress(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	primary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateOnline,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(primary)

	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.StartFailover("primary-2")
	err := fm.StartFailover("primary-2")
	if err == nil {
		t.Error("expected error for already in progress")
	}
}

func TestFailoverManagerVoteWrongState(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	result := fm.Vote("voter-1", "candidate-1")
	if result {
		t.Error("expected false when not in progress")
	}
}

func TestFailoverManagerVoteWrongCandidate(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	primary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateOnline,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(primary)

	replica := &Node{
		ID:        "replica-1",
		Role:      RoleReplica,
		State:     NodeStateOnline,
		ReplicaOf: "primary-2",
	}
	c.AddNode(replica)

	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.StartFailover("primary-2")
	time.Sleep(10 * time.Millisecond)

	result := fm.Vote("voter-1", "wrong-candidate")
	if result {
		t.Error("expected false for wrong candidate")
	}
}

func TestSlotMigratorStartMigrationNotFound(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	sm := NewSlotMigrator(c)

	err := sm.StartMigration("nonexistent", "node-1", []uint16{0})
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestSlotMigratorStartMigrationTargetNotFound(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	sm := NewSlotMigrator(c)

	err := sm.StartMigration("node-1", "nonexistent", []uint16{0})
	if err == nil {
		t.Error("expected error for nonexistent target")
	}
}

func TestSlotMigratorStartMigrationWrongOwner(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2",
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	sm := NewSlotMigrator(c)

	err := sm.StartMigration("node-1", "node-2", []uint16{0})
	if err == nil {
		t.Error("expected error for wrong owner")
	}
}

func TestSlotMigratorStartMigrationAlreadyInProgress(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	sm := NewSlotMigrator(c)
	sm.StartMigration("node-1", "node-2", []uint16{0})

	err := sm.StartMigration("node-1", "node-2", []uint16{100})
	if err == nil {
		t.Error("expected error for already in progress")
	}
}

func TestSlotMigratorCompleteNotInProgress(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	sm := NewSlotMigrator(c)

	err := sm.Complete()
	if err == nil {
		t.Error("expected error when not in progress")
	}
}

func TestSlotMigratorUpdateProgressExt(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
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

func TestClusterRebalanceNoPrimaries(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().Role = RoleReplica

	result := c.Rebalance()

	if result["error"] == nil {
		t.Error("expected error for no primary nodes")
	}
}

func TestKeySlotWithHashTagExt(t *testing.T) {
	key1 := "{user:1}:profile"
	key2 := "{user:1}:settings"

	slot1 := KeySlot(key1)
	slot2 := KeySlot(key2)

	if slot1 != slot2 {
		t.Errorf("Keys with same hash tag should map to same slot: %d != %d", slot1, slot2)
	}
}

func TestHashSlotRouterGetNodeForKeyExt(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	router := NewHashSlotRouter(c)

	node := router.GetNodeForKey("testkey")
	if node == nil {
		t.Error("expected node for key")
	}
}

func TestHashSlotRouterGetNodeForKeyNoOwner(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	router := NewHashSlotRouter(c)

	node := router.GetNodeForKey("testkey")
	if node != nil {
		t.Error("expected nil when no owner")
	}
}

func TestHashSlotRouterIsLocal(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	router := NewHashSlotRouter(c)

	if !router.IsLocal("testkey") {
		t.Error("expected IsLocal to be true")
	}
}

func TestHashSlotRouterIsLocalNoOwner(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	router := NewHashSlotRouter(c)

	if !router.IsLocal("testkey") {
		t.Error("expected IsLocal to be true when no owner")
	}
}

func TestHashSlotRouterIsLocalFalse(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2",
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)
	c.mu.Lock()
	c.slots[0] = &SlotInfo{Primary: node2}
	c.mu.Unlock()

	router := NewHashSlotRouter(c)

	if router.IsLocal("\x00") {
		t.Error("expected IsLocal to be false when key is owned by other node")
	}
}

func TestHashSlotRouterGetMovedError(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	router := NewHashSlotRouter(c)

	slot, addr, port := router.GetMovedError("testkey")
	if slot >= NumSlots {
		t.Errorf("slot should be < %d", NumSlots)
	}
	if addr == "" {
		t.Error("expected addr")
	}
	if port == 0 {
		t.Error("expected port")
	}
}

func TestHashSlotRouterGetMovedErrorNoOwner(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	router := NewHashSlotRouter(c)

	slot, addr, port := router.GetMovedError("testkey")
	if slot >= NumSlots {
		t.Errorf("slot should be < %d", NumSlots)
	}
	if addr != "" {
		t.Error("expected empty addr when no owner")
	}
	if port != 0 {
		t.Error("expected port 0 when no owner")
	}
}

func TestGossipStruct(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	if g == nil {
		t.Fatal("expected gossip")
	}
	if g.cluster == nil {
		t.Error("expected cluster")
	}
	if g.peers == nil {
		t.Error("expected peers map")
	}
}

func TestGossipMessageStruct(t *testing.T) {
	msg := GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
		Nodes:     []NodeInfo{},
	}

	if msg.Type != "ping" {
		t.Errorf("expected 'ping', got '%s'", msg.Type)
	}
}

func TestNodeInfoStruct(t *testing.T) {
	info := NodeInfo{
		ID:         "node-1",
		Addr:       "127.0.0.1",
		Port:       6379,
		GossipPort: 7946,
		Role:       "master",
		State:      "online",
		ReplicaOf:  "",
	}

	if info.ID != "node-1" {
		t.Errorf("expected 'node-1', got '%s'", info.ID)
	}
}

func TestNumSlotsConstant(t *testing.T) {
	if NumSlots != 16384 {
		t.Errorf("expected NumSlots = 16384, got %d", NumSlots)
	}
}

func TestClusterCheckClusterHealthFailed(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateFailed

	failedNode := &Node{
		ID:    "node-2",
		Role:  RolePrimary,
		State: NodeStateFailed,
	}
	c.AddNode(failedNode)

	health := c.CheckClusterHealth()

	if health["status"] != "fail" {
		t.Errorf("expected 'fail', got '%v'", health["status"])
	}

	issues, ok := health["issues"].([]string)
	if !ok || len(issues) == 0 {
		t.Error("expected issues")
	}
}

func TestClusterCheckClusterHealthDegraded(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	failedNode := &Node{
		ID:    "node-2",
		Role:  RoleReplica,
		State: NodeStateFailed,
	}
	c.AddNode(failedNode)

	health := c.CheckClusterHealth()

	if health["status"] != "degraded" {
		t.Errorf("expected 'degraded', got '%v'", health["status"])
	}
}

func TestClusterGetSlotDistributionEmpty(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	dist := c.GetSlotDistribution()

	if len(dist) != 0 {
		t.Errorf("expected 0 entries, got %d", len(dist))
	}
}
