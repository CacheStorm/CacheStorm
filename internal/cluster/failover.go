package cluster

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type FailoverState int

const (
	FailoverNone FailoverState = iota
	FailoverWaiting
	FailoverInProgress
	FailoverCompleted
)

type FailoverManager struct {
	mu            sync.RWMutex
	cluster       *Cluster
	gossip        *Gossip
	state         FailoverState
	failedNode    string
	failedSlots   []uint16
	startTime     time.Time
	votes         map[string]bool
	quorum        int
	leader        string
	electionTimer *time.Timer
}

func NewFailoverManager(c *Cluster, g *Gossip) *FailoverManager {
	return &FailoverManager{
		cluster: c,
		gossip:  g,
		votes:   make(map[string]bool),
	}
}

func (f *FailoverManager) StartFailover(failedNodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != FailoverNone {
		return fmt.Errorf("failover already in progress")
	}

	node := f.cluster.GetNode(failedNodeID)
	if node == nil {
		return fmt.Errorf("node not found")
	}

	if node.Role != RolePrimary {
		return fmt.Errorf("only primaries can be failed over")
	}

	f.failedNode = failedNodeID
	f.failedSlots = f.getNodeSlots(node)
	f.state = FailoverWaiting
	f.startTime = time.Now()
	f.votes = make(map[string]bool)
	f.leader = ""

	primaries := f.getPrimaryCount()
	f.quorum = (primaries / 2) + 1

	f.electionTimer = time.AfterFunc(2*time.Second, f.runElection)

	return nil
}

func (f *FailoverManager) getNodeSlots(node *Node) []uint16 {
	slots := make([]uint16, 0)
	for _, sr := range node.Slots {
		for i := sr.Start; i <= sr.End; i++ {
			slots = append(slots, i)
		}
	}
	return slots
}

func (f *FailoverManager) getPrimaryCount() int {
	count := 0
	for _, n := range f.cluster.GetNodes() {
		if n.Role == RolePrimary {
			count++
		}
	}
	return count
}

func (f *FailoverManager) runElection() {
	f.mu.Lock()
	if f.state != FailoverWaiting {
		f.mu.Unlock()
		return
	}

	replicas := f.getReplicasOf(f.failedNode)
	if len(replicas) == 0 {
		f.state = FailoverNone
		f.mu.Unlock()
		return
	}

	var winner *Node
	var maxOffset int64 = -1
	for _, r := range replicas {
		if r.State == NodeStateOnline {
			offset := f.getReplicaOffset(r.ID)
			if offset > maxOffset {
				maxOffset = offset
				winner = r
			}
		}
	}

	if winner == nil {
		f.state = FailoverNone
		f.mu.Unlock()
		return
	}

	f.leader = winner.ID
	f.state = FailoverInProgress
	f.mu.Unlock()

	go f.requestVotes()
}

func (f *FailoverManager) getReplicasOf(primaryID string) []*Node {
	replicas := make([]*Node, 0)
	for _, n := range f.cluster.GetNodes() {
		if n.Role == RoleReplica && n.ReplicaOf == primaryID {
			replicas = append(replicas, n)
		}
	}
	return replicas
}

func (f *FailoverManager) getReplicaOffset(replicaID string) int64 {
	return time.Now().UnixNano()
}

func (f *FailoverManager) requestVotes() {
}

func (f *FailoverManager) Vote(voterID string, candidateID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != FailoverInProgress {
		return false
	}

	if candidateID != f.leader {
		return false
	}

	f.votes[voterID] = true

	yesVotes := 0
	for _, voted := range f.votes {
		if voted {
			yesVotes++
		}
	}

	if yesVotes >= f.quorum {
		f.completeFailover()
		return true
	}

	return false
}

func (f *FailoverManager) completeFailover() {
	if f.leader == "" {
		return
	}

	newPrimary := f.cluster.GetNode(f.leader)
	if newPrimary == nil {
		return
	}

	newPrimary.Role = RolePrimary
	newPrimary.ReplicaOf = ""

	failedNode := f.cluster.GetNode(f.failedNode)
	if failedNode != nil {
		newPrimary.Slots = failedNode.Slots
		failedNode.Role = RoleReplica
		failedNode.ReplicaOf = f.leader
	}

	for _, slot := range f.failedSlots {
		f.cluster.slots[slot] = &SlotInfo{Primary: newPrimary}
	}

	f.state = FailoverCompleted
	f.leader = ""
	f.failedNode = ""
	f.failedSlots = nil
}

func (f *FailoverManager) GetState() FailoverState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

type SlotMigrator struct {
	mu        sync.RWMutex
	cluster   *Cluster
	source    string
	target    string
	slots     []uint16
	state     string
	progress  int
	bytesSent int64
	startTime time.Time
}

func NewSlotMigrator(c *Cluster) *SlotMigrator {
	return &SlotMigrator{
		cluster: c,
		state:   "none",
	}
}

func (m *SlotMigrator) StartMigration(sourceID, targetID string, slots []uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == "migrating" {
		return fmt.Errorf("migration already in progress")
	}

	source := m.cluster.GetNode(sourceID)
	if source == nil {
		return fmt.Errorf("source node not found")
	}

	target := m.cluster.GetNode(targetID)
	if target == nil {
		return fmt.Errorf("target node not found")
	}

	for _, slot := range slots {
		owner := m.cluster.GetSlotOwner(slot)
		if owner == nil || owner.ID != sourceID {
			return fmt.Errorf("slot %d not owned by source", slot)
		}
	}

	m.source = sourceID
	m.target = targetID
	m.slots = slots
	m.state = "migrating"
	m.progress = 0
	m.bytesSent = 0
	m.startTime = time.Now()

	return nil
}

func (m *SlotMigrator) UpdateProgress(progress int, bytesSent int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress = progress
	m.bytesSent = bytesSent
}

func (m *SlotMigrator) Complete() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != "migrating" {
		return fmt.Errorf("no migration in progress")
	}

	target := m.cluster.GetNode(m.target)
	if target == nil {
		return fmt.Errorf("target node not found")
	}

	for _, slot := range m.slots {
		m.cluster.slots[slot] = &SlotInfo{Primary: target}
	}

	for _, slot := range m.slots {
		for i, sr := range target.Slots {
			if slot >= sr.Start && slot <= sr.End {
				continue
			}
			if slot < sr.Start {
				if i > 0 && slot <= target.Slots[i-1].End+1 {
					target.Slots[i-1].End = slot
				} else {
					target.Slots = append(target.Slots[:i], append([]SlotRange{{Start: slot, End: slot}}, target.Slots[i:]...)...)
				}
			}
		}
	}

	m.state = "completed"
	m.progress = 100

	return nil
}

func (m *SlotMigrator) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = "cancelled"
	m.progress = 0
	m.bytesSent = 0
}

func (m *SlotMigrator) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"state":      m.state,
		"source":     m.source,
		"target":     m.target,
		"slots":      len(m.slots),
		"progress":   m.progress,
		"bytes_sent": m.bytesSent,
		"duration":   time.Since(m.startTime).Seconds(),
	}
}

func (m *SlotMigrator) IsMigrating() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == "migrating"
}

func (c *Cluster) Rebalance() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	primaries := make([]*Node, 0)
	for _, n := range c.nodes {
		if n.Role == RolePrimary && n.State == NodeStateOnline {
			primaries = append(primaries, n)
		}
	}

	if len(primaries) == 0 {
		return map[string]interface{}{"error": "no primary nodes available"}
	}

	slotsPerNode := 16384 / len(primaries)
	remainder := 16384 % len(primaries)

	movements := make([]map[string]interface{}, 0)
	slot := uint16(0)

	for i, node := range primaries {
		count := slotsPerNode
		if i < remainder {
			count++
		}

		newSlots := []SlotRange{}
		if count > 0 {
			newSlots = append(newSlots, SlotRange{Start: slot, End: slot + uint16(count) - 1})
		}

		for _, sr := range node.Slots {
			for j := sr.Start; j <= sr.End; j++ {
				c.slots[j] = nil
			}
		}

		node.Slots = newSlots
		for _, sr := range newSlots {
			for j := sr.Start; j <= sr.End; j++ {
				c.slots[j] = &SlotInfo{Primary: node}
			}
		}

		slot += uint16(count)
	}

	return map[string]interface{}{
		"ok":             true,
		"primaries":      len(primaries),
		"slots_per_node": slotsPerNode,
		"movements":      len(movements),
		"rebalanced_at":  time.Now().Unix(),
	}
}

func (c *Cluster) GetSlotDistribution() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dist := make(map[string]int)
	for i := 0; i < 16384; i++ {
		if c.slots[i] != nil && c.slots[i].Primary != nil {
			dist[c.slots[i].Primary.ID]++
		}
	}

	return dist
}

func (c *Cluster) CheckClusterHealth() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	healthyNodes := 0
	failedNodes := 0
	onlinePrimaries := 0
	onlineReplicas := 0

	for _, n := range c.nodes {
		if n.State == NodeStateOnline {
			healthyNodes++
			if n.Role == RolePrimary {
				onlinePrimaries++
			} else {
				onlineReplicas++
			}
		} else if n.State == NodeStateFailed {
			failedNodes++
		}
	}

	coveredSlots := 0
	for i := 0; i < 16384; i++ {
		if c.slots[i] != nil && c.slots[i].Primary != nil {
			coveredSlots++
		}
	}

	status := "ok"
	issues := make([]string, 0)

	if failedNodes > 0 {
		status = "degraded"
		issues = append(issues, fmt.Sprintf("%d nodes failed", failedNodes))
	}

	if coveredSlots < 16384 {
		status = "fail"
		issues = append(issues, fmt.Sprintf("%d slots not covered", 16384-coveredSlots))
	}

	if onlinePrimaries == 0 {
		status = "fail"
		issues = append(issues, "no online primaries")
	}

	return map[string]interface{}{
		"status":           status,
		"issues":           issues,
		"healthy_nodes":    healthyNodes,
		"failed_nodes":     failedNodes,
		"online_primaries": onlinePrimaries,
		"online_replicas":  onlineReplicas,
		"covered_slots":    coveredSlots,
		"coverage_pct":     float64(coveredSlots) / 16384 * 100,
	}
}

func (c *Cluster) GetClusterStats() map[string]interface{} {
	dist := c.GetSlotDistribution()
	health := c.CheckClusterHealth()

	minSlots := math.MaxInt32
	maxSlots := 0
	for _, count := range dist {
		if count < minSlots {
			minSlots = count
		}
		if count > maxSlots {
			maxSlots = count
		}
	}

	if len(dist) == 0 {
		minSlots = 0
	}

	return map[string]interface{}{
		"health":             health,
		"slot_distribution":  dist,
		"min_slots_per_node": minSlots,
		"max_slots_per_node": maxSlots,
		"avg_slots_per_node": float64(16384) / float64(max(1, len(dist))),
	}
}
