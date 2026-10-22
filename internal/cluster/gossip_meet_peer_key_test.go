package cluster

import (
	"net"
	"testing"
	"time"
)

func freeGossipPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// TestMeetLearnsTheNodeItMet pins the contract of CLUSTER MEET: the node that
// issues the meet must end up knowing the node it met, not just the other way
// around. Gossip.Meet registers the peer it dialled in g.peers, and every reader
// of that map — validateSender and the lastPong update — looks the peer up by
// NODE ID. The meet reply is a "pong", which is not the "meet" type
// validateSender unconditionally accepts, so it is dropped and updateNodeFromInfo
// never runs: the meeting node never learns its peer.
func TestMeetLearnsTheNodeItMet(t *testing.T) {
	portA := freeGossipPort(t)
	portB := freeGossipPort(t)

	clusterA := New("node-A", "127.0.0.1", 7001, portA, nil)
	gossipA := NewGossip(clusterA)
	if err := gossipA.Start(); err != nil {
		t.Fatalf("start node A gossip: %v", err)
	}
	defer gossipA.Stop()

	clusterB := New("node-B", "127.0.0.1", 7002, portB, nil)
	gossipB := NewGossip(clusterB)
	if err := gossipB.Start(); err != nil {
		t.Fatalf("start node B gossip: %v", err)
	}
	defer gossipB.Stop()

	if err := gossipA.Meet("127.0.0.1", portB); err != nil {
		t.Fatalf("Meet: %v", err)
	}

	// Control: the met node learns the meeting node. handleMessage accepts a
	// "meet" outright, so this direction has always worked.
	if clusterB.GetNode("node-A") == nil {
		t.Fatalf("control failed: node B never learned node A from the meet; " +
			"the handshake itself is broken, so a failure on node A below is not evidence")
	}

	// The failing assertion: the meeting node must learn the node it met.
	if clusterA.GetNode("node-B") == nil {
		t.Fatalf("FAIL: CLUSTER MEET did not add the met node — node A's cluster "+
			"contains only %v; gossip peer table is keyed by %q so validateSender "+
			"never recognises node-B and drops its ping/pong replies",
			clusterNodeIDs(clusterA), gossipPeerKeys(gossipA))
	}
}

// TestLearnedNodeRepliesAreAccepted covers the steady-state half of the same
// root cause: once a node has been learned, its later gossip traffic must pass
// validateSender, otherwise sendPingToAll and checkFailedNodes never see it.
func TestLearnedNodeRepliesAreAccepted(t *testing.T) {
	cluster := New("node-self", "127.0.0.1", 7001, freeGossipPort(t), nil)
	gossip := NewGossip(cluster)

	meet := &GossipMessage{
		Type:      "meet",
		SenderID:  "node-remote",
		Timestamp: time.Now().Unix(),
		Nodes: []NodeInfo{
			{ID: "node-remote", Addr: "127.0.0.2", Port: 7002, GossipPort: 17002, Role: "master", State: "online"},
		},
	}
	gossip.handleMessage(meet)
	if cluster.GetNode("node-remote") == nil {
		t.Fatalf("setup failed: an accepted meet did not add the reporting node")
	}

	// The node is now known, so the follow-up ping/pong must validate. If the
	// peer were registered under anything other than the node ID, this is where
	// it silently stops validating.
	reply := &GossipMessage{Type: "pong", SenderID: "node-remote", Timestamp: time.Now().Unix()}
	if !gossip.validateSender(reply) {
		t.Fatalf("FAIL: validateSender rejected a pong from node-remote after it was "+
			"learned; peer table is keyed %v, so every later ping/pong from a known "+
			"node is dropped", gossipPeerKeys(gossip))
	}
}

func clusterNodeIDs(c *Cluster) []string {
	nodes := c.GetNodes()
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

func gossipPeerKeys(g *Gossip) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	keys := make([]string, 0, len(g.peers))
	for k := range g.peers {
		keys = append(keys, k)
	}
	return keys
}
