package cluster

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GossipMessage struct {
	Type      string     `json:"type"`
	SenderID  string     `json:"sender_id"`
	Timestamp int64      `json:"timestamp"`
	Nodes     []NodeInfo `json:"nodes,omitempty"`
	Slot      uint16     `json:"slot,omitempty"`
	TargetID  string     `json:"target_id,omitempty"`
}

type NodeInfo struct {
	ID         string `json:"id"`
	Addr       string `json:"addr"`
	Port       int    `json:"port"`
	GossipPort int    `json:"gossip_port"`
	Role       string `json:"role"`
	State      string `json:"state"`
	ReplicaOf  string `json:"replica_of,omitempty"`
}

type Gossip struct {
	cluster    *Cluster
	mu         sync.RWMutex
	peers      map[string]*gossipPeer
	stopCh     chan struct{}
	wg         sync.WaitGroup
	interval   time.Duration
	knownNodes map[string]bool // Track known node IDs for validation
	listener   net.Listener
}

type gossipPeer struct {
	addr     string
	port     int
	lastPing time.Time
	lastPong time.Time
}

func NewGossip(c *Cluster) *Gossip {
	return &Gossip{
		cluster:    c,
		peers:      make(map[string]*gossipPeer),
		knownNodes: make(map[string]bool),
		stopCh:     make(chan struct{}),
		interval:   1 * time.Second,
	}
}

func (g *Gossip) Start() error {
	self := g.cluster.Self()
	addr := fmt.Sprintf("%s:%d", self.Addr, self.GossipPort)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start gossip listener: %v", err)
	}
	g.listener = ln

	g.wg.Add(1)
	go g.acceptLoop(ln)

	g.wg.Add(1)
	go g.gossipLoop()

	return nil
}

func (g *Gossip) Stop() {
	close(g.stopCh)
	if g.listener != nil {
		g.listener.Close() // Unblocks acceptLoop
	}
	g.wg.Wait()
}

func (g *Gossip) acceptLoop(ln net.Listener) {
	defer g.wg.Done()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-g.stopCh:
				return
			default:
				continue
			}
		}

		g.wg.Add(1)
		go g.handleConnection(conn)
	}
}

func (g *Gossip) handleConnection(conn net.Conn) {
	defer g.wg.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-g.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg GossipMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		response := g.handleMessage(&msg)
		if response != nil {
			data, err := json.Marshal(response)
			if err != nil {
				continue
			}
			if _, err := conn.Write(data); err != nil {
				return
			}
			if _, err := conn.Write([]byte("\n")); err != nil {
				return
			}
		}
	}
}

func (g *Gossip) validateSender(msg *GossipMessage) bool {
	// Reject messages with empty sender ID
	if msg.SenderID == "" {
		return false
	}

	// Reject messages from self (loop detection)
	if msg.SenderID == g.cluster.Self().ID {
		return false
	}

	// Check if sender is a known node or in our peer list
	g.mu.RLock()
	_, isKnownPeer := g.peers[msg.SenderID]
	g.mu.RUnlock()

	if isKnownPeer {
		return true
	}

	// For "meet" messages, allow new nodes
	if msg.Type == "meet" {
		return true
	}

	// For other messages, check known nodes
	g.mu.RLock()
	_, isKnown := g.knownNodes[msg.SenderID]
	g.mu.RUnlock()

	return isKnown
}

func (g *Gossip) handleMessage(msg *GossipMessage) *GossipMessage {
	// Validate sender
	if !g.validateSender(msg) {
		return nil
	}

	switch msg.Type {
	case "ping":
		g.updateNodeFromInfo(msg.Nodes)
		return &GossipMessage{
			Type:      "pong",
			SenderID:  g.cluster.Self().ID,
			Timestamp: time.Now().Unix(),
			Nodes:     g.getNodeInfoList(),
		}

	case "pong":
		g.updateNodeFromInfo(msg.Nodes)
		g.mu.Lock()
		if peer, ok := g.peers[msg.SenderID]; ok {
			peer.lastPong = time.Now()
		}
		g.mu.Unlock()
		return nil

	case "meet":
		g.updateNodeFromInfo(msg.Nodes)
		return &GossipMessage{
			Type:      "pong",
			SenderID:  g.cluster.Self().ID,
			Timestamp: time.Now().Unix(),
			Nodes:     g.getNodeInfoList(),
		}

	case "fail":
		if msg.TargetID != "" {
			node := g.cluster.GetNode(msg.TargetID)
			if node != nil {
				node.State = NodeStateFailed
			}
		}
		return nil

	case "slot_migrate":
		return nil

	default:
		return nil
	}
}

func (g *Gossip) updateNodeFromInfo(nodes []NodeInfo) {
	for _, info := range nodes {
		if info.ID == g.cluster.Self().ID {
			continue
		}

		// Validate node info from gossip to prevent injection
		if info.ID == "" || info.Addr == "" || info.Port < 1 || info.Port > 65535 || info.GossipPort < 1 || info.GossipPort > 65535 {
			continue
		}
		if net.ParseIP(info.Addr) == nil {
			continue
		}

		existing := g.cluster.GetNode(info.ID)
		if existing == nil {
			role := RolePrimary
			if info.Role == "slave" {
				role = RoleReplica
			}

			state := NodeStateOnline
			if info.State == "failed" {
				state = NodeStateFailed
			} else if info.State == "joining" {
				state = NodeStateJoining
			}

			g.cluster.AddNode(&Node{
				ID:         info.ID,
				Addr:       info.Addr,
				Port:       info.Port,
				GossipPort: info.GossipPort,
				Role:       role,
				State:      state,
				ReplicaOf:  info.ReplicaOf,
				LastSeen:   time.Now(),
			})

			g.mu.Lock()
			g.peers[info.ID] = &gossipPeer{
				addr:     info.Addr,
				port:     info.GossipPort,
				lastPing: time.Now(),
			}
			g.mu.Unlock()
		} else {
			existing.LastSeen = time.Now()
		}
	}
}

func (g *Gossip) getNodeInfoList() []NodeInfo {
	nodes := g.cluster.GetNodes()
	result := make([]NodeInfo, 0, len(nodes))

	for _, n := range nodes {
		role := "master"
		if n.Role == RoleReplica {
			role = "slave"
		}

		result = append(result, NodeInfo{
			ID:         n.ID,
			Addr:       n.Addr,
			Port:       n.Port,
			GossipPort: n.GossipPort,
			Role:       role,
			State:      n.State.String(),
			ReplicaOf:  n.ReplicaOf,
		})
	}

	return result
}

func (g *Gossip) gossipLoop() {
	defer g.wg.Done()

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.sendPingToAll()
			g.checkFailedNodes()
		}
	}
}

func (g *Gossip) sendPingToAll() {
	msg := &GossipMessage{
		Type:      "ping",
		SenderID:  g.cluster.Self().ID,
		Timestamp: time.Now().Unix(),
		Nodes:     g.getNodeInfoList(),
	}

	g.mu.RLock()
	peers := make([]*gossipPeer, 0, len(g.peers))
	peerAddrs := make([]string, 0, len(g.peers))
	for _, p := range g.peers {
		peers = append(peers, p)
		peerAddrs = append(peerAddrs, fmt.Sprintf("%s:%d", p.addr, p.port))
	}
	g.mu.RUnlock()

	for i, peerAddr := range peerAddrs {
		go g.sendMessage(peerAddr, msg)
		peers[i].lastPing = time.Now()
	}
}

func (g *Gossip) sendMessage(addr string, msg *GossipMessage) {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if _, err := conn.Write(data); err != nil {
		return
	}
	if _, err := conn.Write([]byte("\n")); err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	var response GossipMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &response); err == nil {
		g.handleMessage(&response)
	}
}

func (g *Gossip) checkFailedNodes() {
	threshold := 5 * time.Second

	nodes := g.cluster.GetNodes()
	for _, node := range nodes {
		if node.ID == g.cluster.Self().ID {
			continue
		}

		if time.Since(node.LastSeen) > threshold && node.State != NodeStateFailed {
			node.State = NodeStateFailed
			g.broadcastFail(node.ID)
		}
	}
}

func (g *Gossip) broadcastFail(nodeID string) {
	msg := &GossipMessage{
		Type:      "fail",
		SenderID:  g.cluster.Self().ID,
		Timestamp: time.Now().Unix(),
		TargetID:  nodeID,
	}

	g.mu.RLock()
	for _, p := range g.peers {
		addr := fmt.Sprintf("%s:%d", p.addr, p.port)
		go g.sendMessage(addr, msg)
	}
	g.mu.RUnlock()
}

func (g *Gossip) Meet(addr string, port int) error {
	msg := &GossipMessage{
		Type:      "meet",
		SenderID:  g.cluster.Self().ID,
		Timestamp: time.Now().Unix(),
		Nodes:     g.getNodeInfoList(),
	}

	peerAddr := fmt.Sprintf("%s:%d", addr, port)
	g.sendMessage(peerAddr, msg)

	g.mu.Lock()
	g.peers[addr+":"+strconv.Itoa(port)] = &gossipPeer{
		addr:     addr,
		port:     port,
		lastPing: time.Now(),
	}
	g.mu.Unlock()

	return nil
}
