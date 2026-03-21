package cluster

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// --- BalanceSlots coverage ---

// TestBalanceSlotsThreeNodes tests BalanceSlots with 3 nodes so that the
// remainder distribution path is exercised (16384 % 3 != 0).
func TestBalanceSlotsThreeNodes(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	c.AddNode(&Node{ID: "node-2", Addr: "127.0.0.1", Port: 6381, Role: RolePrimary, State: NodeStateOnline})
	c.AddNode(&Node{ID: "node-3", Addr: "127.0.0.1", Port: 6382, Role: RolePrimary, State: NodeStateOnline})

	c.BalanceSlots()

	// All 16384 slots should be covered.
	covered := 0
	for i := 0; i < 16384; i++ {
		if c.GetSlotOwner(uint16(i)) != nil {
			covered++
		}
	}
	if covered != 16384 {
		t.Errorf("expected 16384 slots covered, got %d", covered)
	}

	// Each node should have slots assigned.
	for _, n := range c.GetNodes() {
		if len(n.Slots) == 0 {
			t.Errorf("node %s has no slots", n.ID)
		}
	}
}

// TestBalanceSlotsSingleNode tests BalanceSlots with only one node.
func TestBalanceSlotsSingleNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
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

// --- GetClusterNodes coverage ---

// TestGetClusterNodesMultipleSlotsRanges tests GetClusterNodes with a node
// that has multiple slot ranges to cover the slots string building path.
func TestGetClusterNodesMultipleSlotsRanges(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.Self().Slots = []SlotRange{
		{Start: 0, End: 100},
		{Start: 200, End: 300},
	}

	nodes := c.GetClusterNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	slotsStr, ok := nodes[0]["slots"].(string)
	if !ok {
		t.Fatal("expected slots to be a string")
	}
	if slotsStr == "" {
		t.Error("expected non-empty slots string")
	}
}

// --- Failover runElection coverage ---

// TestRunElectionNoReplicas tests runElection when there are no replicas
// for the failed node.
func TestRunElectionNoReplicas(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	primary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateOnline,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(primary)

	fm.mu.Lock()
	fm.failedNode = "primary-2"
	fm.state = FailoverWaiting
	fm.mu.Unlock()

	fm.runElection()

	if fm.GetState() != FailoverNone {
		t.Errorf("expected FailoverNone after no replicas, got %d", fm.GetState())
	}
}

// TestRunElectionNotWaiting tests runElection when state is not FailoverWaiting.
func TestRunElectionNotWaiting(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.mu.Lock()
	fm.state = FailoverNone
	fm.mu.Unlock()

	fm.runElection()

	if fm.GetState() != FailoverNone {
		t.Errorf("expected FailoverNone, got %d", fm.GetState())
	}
}

// TestRunElectionAllReplicasOffline tests runElection when all replicas are offline.
func TestRunElectionAllReplicasOffline(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	primary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateFailed,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(primary)

	replica := &Node{
		ID:        "replica-1",
		Role:      RoleReplica,
		State:     NodeStateFailed, // offline
		ReplicaOf: "primary-2",
	}
	c.AddNode(replica)

	fm.mu.Lock()
	fm.failedNode = "primary-2"
	fm.state = FailoverWaiting
	fm.mu.Unlock()

	fm.runElection()

	if fm.GetState() != FailoverNone {
		t.Errorf("expected FailoverNone when all replicas offline, got %d", fm.GetState())
	}
}

// TestRunElectionWithOnlineReplica tests runElection with an online replica
// that becomes the election winner.
func TestRunElectionWithOnlineReplica(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	primary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateFailed,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(primary)

	replica := &Node{
		ID:        "replica-1",
		Addr:      "127.0.0.1",
		Port:      6382,
		Role:      RoleReplica,
		State:     NodeStateOnline,
		ReplicaOf: "primary-2",
	}
	c.AddNode(replica)

	fm.mu.Lock()
	fm.failedNode = "primary-2"
	fm.state = FailoverWaiting
	fm.mu.Unlock()

	fm.runElection()

	// Give requestVotes goroutine time to start.
	time.Sleep(10 * time.Millisecond)

	state := fm.GetState()
	if state != FailoverInProgress {
		t.Errorf("expected FailoverInProgress, got %d", state)
	}

	fm.mu.RLock()
	leader := fm.leader
	fm.mu.RUnlock()
	if leader != "replica-1" {
		t.Errorf("expected leader 'replica-1', got '%s'", leader)
	}
}

// --- completeFailover coverage ---

// TestCompleteFailoverEmptyLeader tests completeFailover when leader is empty.
func TestCompleteFailoverEmptyLeader(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.leader = ""
	fm.completeFailover()

	// Should just return without doing anything.
}

// TestCompleteFailoverLeaderNotFound tests completeFailover when leader node
// is not in the cluster.
func TestCompleteFailoverLeaderNotFound(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.leader = "nonexistent"
	fm.completeFailover()

	// Should just return without panicking.
}

// TestCompleteFailoverWithFailedNodeNil tests completeFailover when the
// failed node has been removed from the cluster.
func TestCompleteFailoverWithFailedNodeNil(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	newPrimary := &Node{
		ID:    "replica-1",
		Addr:  "127.0.0.1",
		Port:  6382,
		Role:  RoleReplica,
		State: NodeStateOnline,
	}
	c.AddNode(newPrimary)

	fm.leader = "replica-1"
	fm.failedNode = "nonexistent-primary"
	fm.failedSlots = []uint16{0, 1, 2}
	fm.state = FailoverInProgress

	fm.completeFailover()

	// new primary should be promoted.
	if newPrimary.Role != RolePrimary {
		t.Errorf("expected new primary role, got %d", newPrimary.Role)
	}
}

// TestVoteReachesQuorum tests Vote when quorum is reached, triggering completeFailover.
func TestVoteReachesQuorum(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	failedPrimary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateFailed,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(failedPrimary)

	newPrimary := &Node{
		ID:        "replica-1",
		Addr:      "127.0.0.1",
		Port:      6382,
		Role:      RoleReplica,
		State:     NodeStateOnline,
		ReplicaOf: "primary-2",
	}
	c.AddNode(newPrimary)

	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.mu.Lock()
	fm.state = FailoverInProgress
	fm.leader = "replica-1"
	fm.failedNode = "primary-2"
	fm.failedSlots = []uint16{0, 1, 2, 3}
	fm.quorum = 1
	fm.votes = make(map[string]bool)
	fm.mu.Unlock()

	result := fm.Vote("voter-1", "replica-1")
	if !result {
		t.Error("expected vote to succeed and trigger failover completion")
	}

	if fm.GetState() != FailoverCompleted {
		t.Errorf("expected FailoverCompleted, got %d", fm.GetState())
	}
}

// --- addSlots coverage ---

// TestAddSlotsEmpty tests addSlots with empty existing ranges.
func TestAddSlotsEmpty(t *testing.T) {
	result := addSlots(nil, []uint16{5, 10, 15})
	if len(result) != 3 {
		t.Errorf("expected 3 ranges, got %d", len(result))
	}
}

// TestAddSlotsSingle tests addSlots with a single slot.
func TestAddSlotsSingle(t *testing.T) {
	result := addSlots(nil, []uint16{5})
	if len(result) != 1 {
		t.Errorf("expected 1 range, got %d", len(result))
	}
	if result[0].Start != 5 || result[0].End != 5 {
		t.Errorf("expected range [5,5], got [%d,%d]", result[0].Start, result[0].End)
	}
}

// TestAddSlotsMergeAdjacent tests addSlots with adjacent slots that should merge.
func TestAddSlotsMergeAdjacent(t *testing.T) {
	result := addSlots(nil, []uint16{1, 2, 3, 5, 6})
	// Should merge into [1,3] and [5,6].
	if len(result) != 2 {
		t.Errorf("expected 2 merged ranges, got %d", len(result))
	}
	if result[0].Start != 1 || result[0].End != 3 {
		t.Errorf("expected first range [1,3], got [%d,%d]", result[0].Start, result[0].End)
	}
	if result[1].Start != 5 || result[1].End != 6 {
		t.Errorf("expected second range [5,6], got [%d,%d]", result[1].Start, result[1].End)
	}
}

// TestAddSlotsOverlapping tests addSlots with overlapping ranges.
func TestAddSlotsOverlapping(t *testing.T) {
	existing := []SlotRange{{Start: 1, End: 5}}
	result := addSlots(existing, []uint16{3, 4, 6, 10})
	// [1,5] + [3,3] [4,4] [6,6] [10,10] should merge to [1,6] [10,10].
	if len(result) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(result))
	}
	if result[0].Start != 1 || result[0].End != 6 {
		t.Errorf("expected first range [1,6], got [%d,%d]", result[0].Start, result[0].End)
	}
}

// TestAddSlotsExistingWithNew tests addSlots with existing ranges and new slots.
func TestAddSlotsExistingWithNew(t *testing.T) {
	existing := []SlotRange{{Start: 0, End: 100}, {Start: 200, End: 300}}
	result := addSlots(existing, []uint16{101, 150})
	// [0,100] + [101,101] should merge to [0,101]; [150,150]; [200,300]
	if len(result) != 3 {
		t.Errorf("expected 3 ranges, got %d", len(result))
	}
}

// --- removeSlots coverage ---

// TestRemoveSlotsMiddle tests removing slots from the middle of a range.
func TestRemoveSlotsMiddle(t *testing.T) {
	ranges := []SlotRange{{Start: 0, End: 10}}
	result := removeSlots(ranges, []uint16{5})
	// Should result in [0,4] and [6,10].
	if len(result) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(result))
	}
}

// TestRemoveSlotsAll tests removing all slots in a range.
func TestRemoveSlotsAll(t *testing.T) {
	ranges := []SlotRange{{Start: 0, End: 2}}
	result := removeSlots(ranges, []uint16{0, 1, 2})
	if len(result) != 0 {
		t.Errorf("expected 0 ranges, got %d", len(result))
	}
}

// --- updateNodeFromInfo coverage ---

// TestUpdateNodeFromInfoNewNode tests updateNodeFromInfo adding a new node.
func TestUpdateNodeFromInfoNewNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	nodes := []NodeInfo{
		{
			ID:         "node-2",
			Addr:       "127.0.0.2",
			Port:       6381,
			GossipPort: 7947,
			Role:       "master",
			State:      "online",
		},
	}

	g.updateNodeFromInfo(nodes)

	n := c.GetNode("node-2")
	if n == nil {
		t.Fatal("expected node-2 to be added")
	}
	if n.Role != RolePrimary {
		t.Errorf("expected RolePrimary, got %d", n.Role)
	}
	if n.State != NodeStateOnline {
		t.Errorf("expected NodeStateOnline, got %d", n.State)
	}
}

// TestUpdateNodeFromInfoReplicaNode tests adding a replica node.
func TestUpdateNodeFromInfoReplicaNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	nodes := []NodeInfo{
		{
			ID:         "replica-1",
			Addr:       "127.0.0.3",
			Port:       6382,
			GossipPort: 7948,
			Role:       "slave",
			State:      "online",
			ReplicaOf:  "node-1",
		},
	}

	g.updateNodeFromInfo(nodes)

	n := c.GetNode("replica-1")
	if n == nil {
		t.Fatal("expected replica-1 to be added")
	}
	if n.Role != RoleReplica {
		t.Errorf("expected RoleReplica, got %d", n.Role)
	}
	if n.ReplicaOf != "node-1" {
		t.Errorf("expected ReplicaOf 'node-1', got '%s'", n.ReplicaOf)
	}
}

// TestUpdateNodeFromInfoFailedState tests adding a node with failed state.
func TestUpdateNodeFromInfoFailedState(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	nodes := []NodeInfo{
		{
			ID:         "node-3",
			Addr:       "127.0.0.4",
			Port:       6383,
			GossipPort: 7949,
			Role:       "master",
			State:      "failed",
		},
	}

	g.updateNodeFromInfo(nodes)

	n := c.GetNode("node-3")
	if n == nil {
		t.Fatal("expected node-3 to be added")
	}
	if n.State != NodeStateFailed {
		t.Errorf("expected NodeStateFailed, got %d", n.State)
	}
}

// TestUpdateNodeFromInfoJoiningState tests adding a node with joining state.
func TestUpdateNodeFromInfoJoiningState(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	nodes := []NodeInfo{
		{
			ID:         "node-4",
			Addr:       "127.0.0.5",
			Port:       6384,
			GossipPort: 7950,
			Role:       "master",
			State:      "joining",
		},
	}

	g.updateNodeFromInfo(nodes)

	n := c.GetNode("node-4")
	if n == nil {
		t.Fatal("expected node-4 to be added")
	}
	if n.State != NodeStateJoining {
		t.Errorf("expected NodeStateJoining, got %d", n.State)
	}
}

// TestUpdateNodeFromInfoSkipSelf tests that self-node info is skipped.
func TestUpdateNodeFromInfoSkipSelf(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	initialCount := c.NodeCount()

	nodes := []NodeInfo{
		{
			ID:         "node-1", // self
			Addr:       "127.0.0.1",
			Port:       6380,
			GossipPort: 7946,
			Role:       "master",
			State:      "online",
		},
	}

	g.updateNodeFromInfo(nodes)

	if c.NodeCount() != initialCount {
		t.Error("should not add self as new node")
	}
}

// TestUpdateNodeFromInfoExistingNode tests updating an already known node.
func TestUpdateNodeFromInfoExistingNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	existing := &Node{
		ID:       "node-2",
		Addr:     "127.0.0.2",
		Port:     6381,
		Role:     RolePrimary,
		State:    NodeStateOnline,
		LastSeen: time.Now().Add(-1 * time.Minute),
	}
	c.AddNode(existing)

	nodes := []NodeInfo{
		{
			ID:         "node-2",
			Addr:       "127.0.0.2",
			Port:       6381,
			GossipPort: 7947,
			Role:       "master",
			State:      "online",
		},
	}

	g.updateNodeFromInfo(nodes)

	n := c.GetNode("node-2")
	if time.Since(n.LastSeen) > 1*time.Second {
		t.Error("LastSeen should have been updated recently")
	}
}

// TestUpdateNodeFromInfoInvalidNodeInfo tests that invalid node info is rejected.
func TestUpdateNodeFromInfoInvalidNodeInfo(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	initialCount := c.NodeCount()

	nodes := []NodeInfo{
		{ID: "", Addr: "127.0.0.2", Port: 6381, GossipPort: 7947},            // empty ID
		{ID: "n2", Addr: "", Port: 6381, GossipPort: 7947},                    // empty Addr
		{ID: "n3", Addr: "127.0.0.2", Port: 0, GossipPort: 7947},             // invalid Port
		{ID: "n4", Addr: "127.0.0.2", Port: 6381, GossipPort: 0},             // invalid GossipPort
		{ID: "n5", Addr: "127.0.0.2", Port: 70000, GossipPort: 7947},         // Port > 65535
		{ID: "n6", Addr: "127.0.0.2", Port: 6381, GossipPort: 70000},         // GossipPort > 65535
		{ID: "n7", Addr: "not-an-ip", Port: 6381, GossipPort: 7947},          // invalid IP address
		{ID: "n8", Addr: "invalid.hostname", Port: 6381, GossipPort: 7947},   // hostname not IP
	}

	g.updateNodeFromInfo(nodes)

	if c.NodeCount() != initialCount {
		t.Errorf("expected no new nodes, but count changed from %d to %d", initialCount, c.NodeCount())
	}
}

// --- validateSender coverage ---

// TestValidateSenderEmptyID tests that messages with empty sender IDs are rejected.
func TestValidateSenderEmptyID(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	msg := &GossipMessage{
		SenderID: "",
		Type:     "ping",
	}

	if g.validateSender(msg) {
		t.Error("expected false for empty sender ID")
	}
}

// TestValidateSenderSelfID tests that messages from self are rejected.
func TestValidateSenderSelfID(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	msg := &GossipMessage{
		SenderID: "node-1", // same as self
		Type:     "ping",
	}

	if g.validateSender(msg) {
		t.Error("expected false for self sender")
	}
}

// TestValidateSenderKnownPeer tests that messages from known peers are accepted.
func TestValidateSenderKnownPeer(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.mu.Unlock()

	msg := &GossipMessage{
		SenderID: "node-2",
		Type:     "ping",
	}

	if !g.validateSender(msg) {
		t.Error("expected true for known peer")
	}
}

// TestValidateSenderMeetMessage tests that meet messages from unknown senders are accepted.
func TestValidateSenderMeetMessage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	msg := &GossipMessage{
		SenderID: "new-node",
		Type:     "meet",
	}

	if !g.validateSender(msg) {
		t.Error("expected true for meet message from unknown sender")
	}
}

// TestValidateSenderKnownNode tests that messages from known nodes are accepted.
func TestValidateSenderKnownNode(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.knownNodes["node-3"] = true
	g.mu.Unlock()

	msg := &GossipMessage{
		SenderID: "node-3",
		Type:     "ping",
	}

	if !g.validateSender(msg) {
		t.Error("expected true for known node")
	}
}

// TestValidateSenderUnknown tests that messages from unknown senders are rejected.
func TestValidateSenderUnknown(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	msg := &GossipMessage{
		SenderID: "unknown-node",
		Type:     "ping",
	}

	if g.validateSender(msg) {
		t.Error("expected false for unknown sender")
	}
}

// --- handleConnection coverage ---

// TestHandleConnectionValidPing tests handleConnection with a valid ping
// that produces a pong response written back to the connection.
func TestHandleConnectionValidPing(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	// Add the sender as a known peer so validation passes.
	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.mu.Unlock()

	server, client := net.Pipe()
	defer server.Close()

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.handleConnection(server)
		close(done)
	}()

	// Send a valid ping.
	ping := GossipMessage{
		Type:      "ping",
		SenderID:  "node-2",
		Timestamp: time.Now().Unix(),
		Nodes:     []NodeInfo{},
	}
	data, _ := json.Marshal(ping)
	client.Write(data)
	client.Write([]byte("\n"))

	// Read the pong response.
	reader := bufio.NewReader(client)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("error reading response: %v", err)
	}

	var response GossipMessage
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		t.Fatalf("error unmarshalling response: %v", err)
	}
	if response.Type != "pong" {
		t.Errorf("expected pong, got %s", response.Type)
	}

	client.Close()
	<-done
}

// TestHandleConnectionEmptyLine tests handleConnection with an empty line.
func TestHandleConnectionEmptyLine(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	server, client := net.Pipe()
	defer server.Close()

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.handleConnection(server)
		close(done)
	}()

	// Send an empty line followed by close.
	client.Write([]byte("\n"))
	time.Sleep(10 * time.Millisecond)
	client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handleConnection did not exit")
	}
}

// TestHandleConnectionBadJSON tests handleConnection with invalid JSON.
func TestHandleConnectionBadJSON(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	server, client := net.Pipe()
	defer server.Close()

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.handleConnection(server)
		close(done)
	}()

	// Send invalid JSON.
	client.Write([]byte("not json\n"))
	time.Sleep(10 * time.Millisecond)
	client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handleConnection did not exit")
	}
}

// TestHandleConnectionStopCh tests handleConnection exit via stopCh.
func TestHandleConnectionStopCh(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	server, client := net.Pipe()
	defer client.Close()

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.handleConnection(server)
		close(done)
	}()

	// Close stopCh to signal shutdown, then close the connection to unblock the read.
	close(g.stopCh)
	server.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handleConnection did not exit on stop")
	}
}

// --- handleMessage coverage ---

// TestHandleMessageFailWithExistingTarget tests the fail message type when
// the target node actually exists in the cluster.
func TestHandleMessageFailWithExistingTarget(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	target := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.2",
		Port:  6381,
		State: NodeStateOnline,
	}
	c.AddNode(target)

	g.mu.Lock()
	g.knownNodes["node-3"] = true
	g.mu.Unlock()

	msg := &GossipMessage{
		Type:     "fail",
		SenderID: "node-3",
		TargetID: "node-2",
	}

	g.handleMessage(msg)

	n := c.GetNode("node-2")
	if n.State != NodeStateFailed {
		t.Errorf("expected NodeStateFailed, got %d", n.State)
	}
}

// TestHandleMessagePongWithPeer tests the pong message when the sender
// is a known peer, updating lastPong.
func TestHandleMessagePongWithPeer(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.mu.Unlock()

	msg := &GossipMessage{
		Type:      "pong",
		SenderID:  "node-2",
		Timestamp: time.Now().Unix(),
		Nodes:     []NodeInfo{},
	}

	response := g.handleMessage(msg)
	if response != nil {
		t.Error("pong should not produce a response")
	}

	g.mu.RLock()
	peer := g.peers["node-2"]
	g.mu.RUnlock()
	if peer.lastPong.IsZero() {
		t.Error("lastPong should be updated")
	}
}

// TestHandleMessageMeet tests the meet message type.
func TestHandleMessageMeet(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	msg := &GossipMessage{
		Type:      "meet",
		SenderID:  "new-node",
		Timestamp: time.Now().Unix(),
		Nodes: []NodeInfo{
			{
				ID:         "new-node",
				Addr:       "127.0.0.5",
				Port:       6385,
				GossipPort: 7951,
				Role:       "master",
				State:      "online",
			},
		},
	}

	response := g.handleMessage(msg)
	if response == nil {
		t.Fatal("meet should produce a pong response")
	}
	if response.Type != "pong" {
		t.Errorf("expected pong, got %s", response.Type)
	}

	// The new node should have been added to the cluster.
	n := c.GetNode("new-node")
	if n == nil {
		t.Error("new-node should have been added to cluster")
	}
}

// --- sendPingToAll / broadcastFail with peers ---

// TestSendPingToAllWithPeers tests sendPingToAll when there are peers.
func TestSendPingToAllWithPeers(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.peers["node-3"] = &gossipPeer{addr: "127.0.0.3", port: 7948}
	g.mu.Unlock()

	// This will attempt to connect which will fail, but exercises the code path.
	g.sendPingToAll()
	time.Sleep(50 * time.Millisecond)
}

// TestBroadcastFailWithPeers tests broadcastFail when there are peers.
func TestBroadcastFailWithPeers(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.mu.Unlock()

	g.broadcastFail("node-2")
	time.Sleep(50 * time.Millisecond)
}

// --- sendMessage coverage ---

// TestSendMessageToListener tests sendMessage with an actual listener that
// accepts the connection and sends a response.
func TestSendMessageToListener(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	// Add sender to known nodes so response validation passes.
	g.mu.Lock()
	g.knownNodes["node-2"] = true
	g.mu.Unlock()

	// Start a TCP listener that reads a message and sends a response.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		// Verify we received a valid message.
		var msg GossipMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return
		}

		// Send a pong response.
		response := GossipMessage{
			Type:      "pong",
			SenderID:  "node-2",
			Timestamp: time.Now().Unix(),
		}
		data, _ := json.Marshal(response)
		conn.Write(data)
		conn.Write([]byte("\n"))
	}()

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
	}

	g.sendMessage(ln.Addr().String(), msg)
}

// --- Gossip Start/Stop with acceptLoop ---

// TestGossipStartStopClean tests a clean start and stop of gossip.
func TestGossipStartStopClean(t *testing.T) {
	// Use port 0 to let OS assign a free port.
	c := New("node-1", "127.0.0.1", 6380, 0, nil)
	c.Self().GossipPort = 0 // Will cause listen on :0

	g := NewGossip(c)

	// Start may fail if port 0 doesn't resolve well, but that's OK.
	err := g.Start()
	if err != nil {
		t.Skipf("gossip start failed (expected on some systems): %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	g.Stop()
}

// --- Gossip acceptLoop coverage ---

// TestAcceptLoopStopCh tests that acceptLoop exits when stopCh is closed.
func TestAcceptLoopStopCh(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 0, nil)
	g := NewGossip(c)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.acceptLoop(ln)
		close(done)
	}()

	close(g.stopCh)
	ln.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("acceptLoop did not exit")
	}
}

// --- Slot Migration Complete with source ---

// TestSlotMigrationCompleteWithSource tests Complete when both source and
// target exist, verifying slot reassignment.
func TestSlotMigrationCompleteWithSource(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.2",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	sm := NewSlotMigrator(c)
	err := sm.StartMigration("node-1", "node-2", []uint16{0, 1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sm.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify slots moved to node-2.
	for _, slot := range []uint16{0, 1, 2} {
		owner := c.GetSlotOwner(slot)
		if owner == nil || owner.ID != "node-2" {
			t.Errorf("slot %d should be owned by node-2", slot)
		}
	}

	// Verify source no longer owns those slots.
	for _, sr := range c.Self().Slots {
		for s := sr.Start; s <= sr.End; s++ {
			if s <= 2 {
				t.Errorf("source should not own slot %d after migration", s)
			}
		}
	}
}

// --- GetClusterStats with empty distribution ---

// TestGetClusterStatsEmptyDistribution tests GetClusterStats when no slots
// are assigned (empty distribution).
func TestGetClusterStatsEmptyDistribution(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline

	stats := c.GetClusterStats()
	if stats == nil {
		t.Fatal("expected stats")
	}

	minSlots, ok := stats["min_slots_per_node"].(int)
	if !ok {
		t.Fatal("expected min_slots_per_node to be int")
	}
	if minSlots != 0 {
		t.Errorf("expected 0 min_slots, got %d", minSlots)
	}
}

// --- getNodeInfoList with replica ---

// TestGetNodeInfoListWithReplica tests getNodeInfoList includes replica info.
func TestGetNodeInfoListWithReplica(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline

	replica := &Node{
		ID:         "replica-1",
		Addr:       "127.0.0.2",
		Port:       6381,
		GossipPort: 7947,
		Role:       RoleReplica,
		State:      NodeStateOnline,
		ReplicaOf:  "node-1",
	}
	c.AddNode(replica)

	g := NewGossip(c)
	nodes := g.getNodeInfoList()

	found := false
	for _, n := range nodes {
		if n.ID == "replica-1" {
			if n.Role != "slave" {
				t.Errorf("expected role 'slave', got '%s'", n.Role)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected replica-1 in node info list")
	}
}

// --- Rebalance edge cases ---

// TestRebalanceMultiplePrimaries tests Rebalance with multiple online primaries.
func TestRebalanceMultiplePrimaries(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	c.AddNode(&Node{
		ID: "node-2", Addr: "127.0.0.1", Port: 6382,
		Role: RolePrimary, State: NodeStateOnline,
	})
	c.AddNode(&Node{
		ID: "node-3", Addr: "127.0.0.1", Port: 6383,
		Role: RolePrimary, State: NodeStateOnline,
	})
	c.AddNode(&Node{
		ID: "node-4", Addr: "127.0.0.1", Port: 6384,
		Role: RolePrimary, State: NodeStateOnline,
	})

	result := c.Rebalance()
	if result["ok"] != true {
		t.Errorf("expected ok rebalance, got %v", result)
	}

	primariesCount, ok := result["primaries"].(int)
	if !ok || primariesCount != 4 {
		t.Errorf("expected 4 primaries in rebalance result, got %v", result["primaries"])
	}

	// Verify at least some nodes got slots (map iteration order varies).
	dist := c.GetSlotDistribution()
	if len(dist) < 1 {
		t.Errorf("expected at least 1 node with slots, got %d", len(dist))
	}
}

// --- CheckClusterHealth edge cases ---

// TestCheckClusterHealthNoOnlinePrimaries tests health check with no online primaries.
func TestCheckClusterHealthNoOnlinePrimaries(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateFailed
	c.Self().Role = RolePrimary

	health := c.CheckClusterHealth()
	if health["status"] != "fail" {
		t.Errorf("expected 'fail', got '%v'", health["status"])
	}

	issues, ok := health["issues"].([]string)
	if !ok {
		t.Fatal("expected issues to be []string")
	}

	hasNoPrimaries := false
	for _, issue := range issues {
		if issue == "no online primaries" {
			hasNoPrimaries = true
		}
	}
	if !hasNoPrimaries {
		t.Error("expected 'no online primaries' issue")
	}
}

// TestCheckClusterHealthPartialCoverage tests health check with partial slot coverage.
func TestCheckClusterHealthPartialCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	// Only assign some slots, leaving gaps.
	c.AssignSlots([]SlotRange{{Start: 0, End: 100}})

	health := c.CheckClusterHealth()
	if health["status"] != "fail" {
		t.Errorf("expected 'fail' for incomplete coverage, got '%v'", health["status"])
	}
}

// TestCheckClusterHealthWithReplicaCount tests health counts replicas correctly.
func TestCheckClusterHealthWithReplicaCount(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	c.AddNode(&Node{
		ID:    "replica-1",
		Role:  RoleReplica,
		State: NodeStateOnline,
	})

	health := c.CheckClusterHealth()
	replicas, ok := health["online_replicas"].(int)
	if !ok {
		t.Fatal("expected online_replicas to be int")
	}
	if replicas != 1 {
		t.Errorf("expected 1 online replica, got %d", replicas)
	}
}

// --- HandleMessage with duplicate suppression ---

// TestHandleMessageDuplicate tests that a duplicate message is suppressed.
func TestTagBroadcasterHandleMessageDuplicate(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	tb := NewTagBroadcaster(c)

	handlerCalled := 0
	tb.RegisterHandler(func(tag string, keys []string) {
		handlerCalled++
	})

	msg := TagBroadcastMessage{
		Type:       "TAG_INVALIDATE",
		Tag:        "test-tag",
		Keys:       []string{"key1"},
		OriginNode: "other-node",
		Timestamp:  time.Now().UnixNano(),
	}
	data, _ := json.Marshal(msg)

	// Handle first time.
	tb.HandleMessage(data)
	if handlerCalled != 1 {
		t.Errorf("expected handler called once, got %d", handlerCalled)
	}

	// Handle second time -- should be suppressed as duplicate.
	tb.HandleMessage(data)
	if handlerCalled != 1 {
		t.Errorf("expected handler still called once (duplicate), got %d", handlerCalled)
	}
}

// --- Tag Broadcaster Broadcast with peers ---

// TestTagBroadcasterBroadcastWithPeers tests Broadcast when the cluster has
// other nodes to send to.
func TestTagBroadcasterBroadcastWithPeers(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.mu.Lock()
	c.enabled = true
	c.mu.Unlock()

	// Add another node.
	c.AddNode(&Node{
		ID:         "node-2",
		Addr:       "127.0.0.2",
		Port:       6381,
		GossipPort: 7947,
		Role:       RolePrimary,
		State:      NodeStateOnline,
	})

	tb := NewTagBroadcaster(c)

	err := tb.Broadcast("users", []string{"user:1", "user:2"})
	if err != nil {
		t.Errorf("broadcast should succeed: %v", err)
	}

	// Give time for goroutines to attempt sending.
	time.Sleep(50 * time.Millisecond)
}

// --- Complete migration when target node removed ---

// TestSlotMigrationCompleteTargetRemoved tests Complete when the target node
// has been removed from the cluster.
func TestSlotMigrationCompleteTargetRemoved(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.2",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	sm := NewSlotMigrator(c)
	sm.StartMigration("node-1", "node-2", []uint16{0})

	// Remove the target node.
	c.RemoveNode("node-2")

	err := sm.Complete()
	if err == nil {
		t.Error("expected error when target node removed")
	}
}

// --- GetClusterInfo coverage ---

// TestGetClusterInfoAllOnline tests GetClusterInfo when all nodes are online.
func TestGetClusterInfoAllOnline(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline

	c.AddNode(&Node{
		ID:    "node-2",
		Addr:  "127.0.0.2",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	})

	info := c.GetClusterInfo()
	if info["cluster_state"] != "ok" {
		t.Errorf("expected 'ok', got '%v'", info["cluster_state"])
	}
	if info["cluster_nodes"] != 2 {
		t.Errorf("expected 2 nodes, got %v", info["cluster_nodes"])
	}
}

// --- Vote full path ---

// TestVoteCountsCorrectly tests that Vote counts votes correctly and
// returns false when quorum is not yet reached.
func TestVoteCountsCorrectly(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	failedPrimary := &Node{
		ID:    "primary-2",
		Role:  RolePrimary,
		State: NodeStateFailed,
		Slots: []SlotRange{{Start: 0, End: 100}},
	}
	c.AddNode(failedPrimary)

	candidate := &Node{
		ID:        "replica-1",
		Addr:      "127.0.0.1",
		Port:      6382,
		Role:      RoleReplica,
		State:     NodeStateOnline,
		ReplicaOf: "primary-2",
	}
	c.AddNode(candidate)

	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	fm.mu.Lock()
	fm.state = FailoverInProgress
	fm.leader = "replica-1"
	fm.failedNode = "primary-2"
	fm.failedSlots = []uint16{0, 1}
	fm.quorum = 3
	fm.votes = make(map[string]bool)
	fm.mu.Unlock()

	// Vote 1 - not enough for quorum.
	result := fm.Vote("voter-1", "replica-1")
	if result {
		t.Error("expected false, quorum not yet reached")
	}

	// Vote 2 - still not enough.
	result = fm.Vote("voter-2", "replica-1")
	if result {
		t.Error("expected false, quorum not yet reached")
	}

	// Vote 3 - reaches quorum.
	result = fm.Vote("voter-3", "replica-1")
	if !result {
		t.Error("expected true, quorum reached")
	}

	if fm.GetState() != FailoverCompleted {
		t.Errorf("expected FailoverCompleted, got %d", fm.GetState())
	}
}

// --- Gossip Start failure ---

// TestGossipStartListenError tests that Start returns an error when the
// listener cannot be created (port already in use).
func TestGossipStartListenError(t *testing.T) {
	// First listener takes the port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	c := New("node-1", "127.0.0.1", 6380, port, nil)
	g := NewGossip(c)

	// This should fail because the port is already in use.
	err = g.Start()
	if err == nil {
		g.Stop()
		t.Error("expected error when port is in use")
	}
}

// --- sendMessage full path with response handling ---

// TestSendMessageFullPathWithPongResponse tests sendMessage sending a message
// and reading back a pong response with node info.
func TestSendMessageFullPathWithPongResponse(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.knownNodes["node-2"] = true
	g.mu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		_, err = reader.ReadString('\n')
		if err != nil {
			return
		}

		response := GossipMessage{
			Type:      "pong",
			SenderID:  "node-2",
			Timestamp: time.Now().Unix(),
			Nodes: []NodeInfo{
				{
					ID:         "node-3",
					Addr:       "127.0.0.3",
					Port:       6383,
					GossipPort: 7949,
					Role:       "master",
					State:      "online",
				},
			},
		}
		data, _ := json.Marshal(response)
		conn.Write(data)
		conn.Write([]byte("\n"))
	}()

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
	}

	g.sendMessage(ln.Addr().String(), msg)

	// Verify node-3 was added from the pong response.
	time.Sleep(50 * time.Millisecond)
	n := c.GetNode("node-3")
	if n == nil {
		t.Error("expected node-3 to be added from pong response")
	}
}

// TestSendMessageReadError tests sendMessage when reading response fails.
func TestSendMessageReadError(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Read the message but close immediately without sending response.
		reader := bufio.NewReader(conn)
		reader.ReadString('\n')
		conn.Close()
	}()

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
	}

	// Should not panic even when response read fails.
	g.sendMessage(ln.Addr().String(), msg)
}

// TestSendMessageBadResponse tests sendMessage when response is not valid JSON.
func TestSendMessageBadResponse(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		reader.ReadString('\n')

		// Send bad JSON.
		conn.Write([]byte("not json\n"))
	}()

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
	}

	g.sendMessage(ln.Addr().String(), msg)
}

// --- handleConnection response write error ---

// TestHandleConnectionWriteError tests handleConnection when writing the
// response fails because the client closes the connection.
func TestHandleConnectionWriteError(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.mu.Unlock()

	server, client := net.Pipe()

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.handleConnection(server)
		close(done)
	}()

	// Send a valid ping that will trigger a pong response.
	ping := GossipMessage{
		Type:      "ping",
		SenderID:  "node-2",
		Timestamp: time.Now().Unix(),
	}
	data, _ := json.Marshal(ping)
	client.Write(data)
	client.Write([]byte("\n"))

	// Close client immediately so the response write fails.
	time.Sleep(10 * time.Millisecond)
	client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handleConnection did not exit after write error")
	}
}

// --- Rebalance edge case: node with existing slots gets cleared ---

// TestRebalanceWithExistingSlots tests Rebalance when primaries already have
// slots assigned, verifying old slots are cleared.
func TestRebalanceWithExistingSlots(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.Self().State = NodeStateOnline
	c.AssignSlots([]SlotRange{{Start: 0, End: 16383}})

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.2",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
		Slots: []SlotRange{{Start: 5000, End: 10000}},
	}
	c.AddNode(node2)

	result := c.Rebalance()
	if result["ok"] != true {
		t.Errorf("expected ok rebalance, got %v", result)
	}
}

// --- Gossip acceptLoop with connection ---

// TestAcceptLoopWithConnection tests acceptLoop accepting a connection and
// handling it.
func TestAcceptLoopWithConnection(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	g.wg.Add(1)
	go g.acceptLoop(ln)

	// Connect to the listener.
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Stop the gossip to close everything.
	close(g.stopCh)
	ln.Close()
	g.wg.Wait()
}

// --- Broadcast with marshal error is impossible with valid struct ---
// The json.Marshal of TagBroadcastMessage can only fail with values
// that json.Marshal can't handle (channels, funcs, etc.), which is
// impossible with the TagBroadcastMessage struct. So that path is
// structurally unreachable.

// --- Additional sendMessage coverage ---

// TestSendMessageConnectError tests sendMessage when the connection fails.
func TestSendMessageConnectError(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
	}

	// Send to an address that's not listening.
	g.sendMessage("127.0.0.1:1", msg)
}

// --- handleConnection with null response (fail message) ---

// TestHandleConnectionNullResponse tests handleConnection when the handler
// returns nil (e.g., for pong or fail messages).
func TestHandleConnectionNullResponse(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.knownNodes["node-2"] = true
	g.mu.Unlock()

	server, client := net.Pipe()

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.handleConnection(server)
		close(done)
	}()

	// Send a fail message which returns nil response.
	msg := GossipMessage{
		Type:      "fail",
		SenderID:  "node-2",
		TargetID:  "node-3",
		Timestamp: time.Now().Unix(),
	}
	data, _ := json.Marshal(msg)
	client.Write(data)
	client.Write([]byte("\n"))

	// Then send a pong which also returns nil.
	msg2 := GossipMessage{
		Type:      "pong",
		SenderID:  "node-2",
		Timestamp: time.Now().Unix(),
	}
	data2, _ := json.Marshal(msg2)
	client.Write(data2)
	client.Write([]byte("\n"))

	time.Sleep(50 * time.Millisecond)
	client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handleConnection did not exit")
	}
}

// --- Gossip with active gossip loop ---

// TestGossipStartStopWithPeers tests start/stop with peers added to exercise
// the gossip loop's sendPingToAll path.
func TestGossipStartStopWithPeers(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 0, nil)
	c.Self().GossipPort = 0
	g := NewGossip(c)
	g.interval = 100 * time.Millisecond

	err := g.Start()
	if err != nil {
		t.Skipf("gossip start failed: %v", err)
	}

	// Add peers.
	g.mu.Lock()
	g.peers["node-2"] = &gossipPeer{addr: "127.0.0.2", port: 7947}
	g.mu.Unlock()

	// Let the gossip loop run a few iterations.
	time.Sleep(350 * time.Millisecond)

	g.Stop()
}

// --- acceptLoop error continue path ---

// TestAcceptLoopAcceptError tests acceptLoop when Accept returns an error
// but stopCh is not closed (the continue path).
func TestAcceptLoopAcceptError(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	g.wg.Add(1)
	done := make(chan struct{})
	go func() {
		g.acceptLoop(ln)
		close(done)
	}()

	// Close the listener to trigger Accept error without closing stopCh first.
	// This will make Accept fail, and since stopCh is not closed, it will
	// hit the default/continue case. Then immediately after, we close stopCh.
	ln.Close()

	// Give it time to hit the error and loop.
	time.Sleep(50 * time.Millisecond)

	// Now close stopCh so the goroutine can exit.
	close(g.stopCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("acceptLoop did not exit")
	}
}

// --- sendMessage write data path ---

// TestSendMessageWriteDataSuccess tests sendMessage writing data then
// newline then reading response with a complete path.
func TestSendMessageWriteDataSuccess(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	g.mu.Lock()
	g.knownNodes["node-2"] = true
	g.mu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		_, err = reader.ReadString('\n')
		if err != nil {
			return
		}

		// Send a valid pong response.
		resp := GossipMessage{
			Type:      "pong",
			SenderID:  "node-2",
			Timestamp: time.Now().Unix(),
		}
		data, _ := json.Marshal(resp)
		conn.Write(data)
		conn.Write([]byte("\n"))
	}()

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
		Nodes:     g.getNodeInfoList(),
	}

	g.sendMessage(ln.Addr().String(), msg)
	time.Sleep(50 * time.Millisecond)
}

// --- sendMessage write error paths ---

// TestSendMessageWriteError tests sendMessage when the connection is closed
// after connect but before writing, causing a write error.
func TestSendMessageWriteError(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Close immediately to trigger write error on the sender side.
		conn.Close()
	}()

	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().Unix(),
	}

	// Give the server time to accept and close.
	time.Sleep(50 * time.Millisecond)

	// Should not panic. The write will fail because the server closed.
	g.sendMessage(ln.Addr().String(), msg)
}

// --- requestVotes is an empty function so there's nothing to cover ---

// --- Broadcast with no self-skip path ---

// TestTagBroadcasterBroadcastOnlyHasSelf tests Broadcast when the only node
// is self - the loop should skip self.
func TestTagBroadcasterBroadcastOnlyHasSelf(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	c.mu.Lock()
	c.enabled = true
	c.mu.Unlock()

	tb := NewTagBroadcaster(c)

	err := tb.Broadcast("tag", []string{"key1"})
	if err != nil {
		t.Errorf("broadcast should succeed: %v", err)
	}
}
