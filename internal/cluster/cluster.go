package cluster

import (
	"sync"
	"time"
)

type NodeRole int

const (
	RolePrimary NodeRole = iota
	RoleReplica
)

type NodeState int

const (
	NodeStateJoining NodeState = iota
	NodeStateOnline
	NodeStateFailed
	NodeStateLeaving
)

type SlotRange struct {
	Start uint16
	End   uint16
}

type Node struct {
	ID         string
	Addr       string
	Port       int
	GossipPort int
	Role       NodeRole
	Slots      []SlotRange
	ReplicaOf  string
	State      NodeState
	LastSeen   time.Time
}

type SlotInfo struct {
	Primary  *Node
	Replicas []*Node
}

type Cluster struct {
	mu      sync.RWMutex
	self    *Node
	nodes   map[string]*Node
	slots   [16384]*SlotInfo
	enabled bool
	seeds   []string
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func New(nodeID, addr string, port, gossipPort int, seeds []string) *Cluster {
	c := &Cluster{
		self: &Node{
			ID:         nodeID,
			Addr:       addr,
			Port:       port,
			GossipPort: gossipPort,
			Role:       RolePrimary,
			State:      NodeStateJoining,
		},
		nodes:  make(map[string]*Node),
		seeds:  seeds,
		stopCh: make(chan struct{}),
	}
	c.nodes[nodeID] = c.self
	return c
}

func (c *Cluster) Start() error {
	c.mu.Lock()
	c.enabled = true
	c.mu.Unlock()
	return nil
}

func (c *Cluster) Stop() {
	c.mu.Lock()
	c.enabled = false
	c.mu.Unlock()
	close(c.stopCh)
	c.wg.Wait()
}

func (c *Cluster) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *Cluster) GetNode(id string) *Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodes[id]
}

func (c *Cluster) GetNodes() []*Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	nodes := make([]*Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

func (c *Cluster) AddNode(n *Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[n.ID] = n
}

func (c *Cluster) RemoveNode(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, id)
}

func (c *Cluster) Self() *Node {
	return c.self
}

func (c *Cluster) NodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.nodes)
}

func (c *Cluster) AssignSlots(slots []SlotRange) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.self.Slots = slots
	for _, sr := range slots {
		for i := sr.Start; i <= sr.End; i++ {
			c.slots[i] = &SlotInfo{Primary: c.self}
		}
	}
}

func (c *Cluster) GetSlotOwner(slot uint16) *Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.slots[slot] == nil {
		return nil
	}
	return c.slots[slot].Primary
}

func (c *Cluster) BalanceSlots() {
	c.mu.Lock()
	defer c.mu.Unlock()

	nodeCount := len(c.nodes)
	if nodeCount == 0 {
		return
	}

	slotsPerNode := 16384 / nodeCount
	remainder := 16384 % nodeCount

	i := 0
	for _, node := range c.nodes {
		count := slotsPerNode
		if remainder > 0 {
			count++
			remainder--
		}

		start := uint16(i)
		end := uint16(i + count - 1)
		node.Slots = []SlotRange{{Start: start, End: end}}

		for j := start; j <= end; j++ {
			c.slots[j] = &SlotInfo{Primary: node}
		}

		i += count
	}
}

func (c *Cluster) GetClusterInfo() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	okNodes := 0
	for _, n := range c.nodes {
		if n.State == NodeStateOnline {
			okNodes++
		}
	}

	state := "ok"
	if okNodes < len(c.nodes) {
		state = "fail"
	}

	return map[string]interface{}{
		"cluster_state": state,
		"cluster_slots": 16384,
		"cluster_nodes": len(c.nodes),
		"cluster_my_id": c.self.ID,
		"cluster_known": len(c.nodes),
		"cluster_size":  len(c.nodes),
	}
}

func (c *Cluster) GetClusterNodes() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(c.nodes))
	for _, n := range c.nodes {
		slots := ""
		for _, sr := range n.Slots {
			if slots != "" {
				slots += " "
			}
			slots += string(rune(sr.Start)) + "-" + string(rune(sr.End))
		}

		role := "master"
		if n.Role == RoleReplica {
			role = "slave"
		}

		result = append(result, map[string]interface{}{
			"id":    n.ID,
			"addr":  n.Addr + ":" + string(rune(n.Port)),
			"role":  role,
			"slots": slots,
			"state": n.State.String(),
		})
	}
	return result
}

func (s NodeState) String() string {
	switch s {
	case NodeStateJoining:
		return "joining"
	case NodeStateOnline:
		return "online"
	case NodeStateFailed:
		return "failed"
	case NodeStateLeaving:
		return "leaving"
	default:
		return "unknown"
	}
}
