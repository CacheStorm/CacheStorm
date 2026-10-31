package cluster

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strings"

	"github.com/cachestorm/cachestorm/internal/logger"
)

// unlockAndSave releases the mutex, then saves nodes.conf when a change
// asked for it, sends the failover votes granted, wakes the role goroutine,
// pauses or resumes writers for a manual failover, deletes the keys of slots
// lost to another node and delivers the PUBLISH messages received.
func (c *Cluster) unlockAndSave() {
	save := c.todoSave
	c.todoSave = false
	role, pause, unpause := c.roleDirty, c.mfStartPause, c.mfUnpause
	c.roleDirty, c.mfStartPause, c.mfUnpause = false, false, false
	acks := c.pendingAcks
	c.pendingAcks = nil
	var del []int
	var pubs []*message
	var opts BusOptions
	if b := c.bus; b != nil {
		del, pubs, opts = b.delSlots, b.published, b.opts
		b.delSlots, b.published = nil, nil
		if role {
			select {
			case b.roleWake <- struct{}{}:
			default:
			}
		}
	}
	c.mu.Unlock()
	if save {
		c.persist()
	}
	if len(acks) > 0 {
		// A vote goes out only once lastVoteEpoch is on disk.
		c.mu.Lock()
		for _, n := range acks {
			if c.nodes[n.ID] == n {
				c.sendLocked(n.link, msgFailoverAuthAck, c.headerLocked(msgFailoverAuthAck).encode())
			}
		}
		c.mu.Unlock()
	}
	if unpause && opts.UnpauseWrites != nil {
		opts.UnpauseWrites()
	}
	if pause {
		c.startManualFailoverPause(opts)
	}
	if opts.DelKeysInSlot != nil {
		for _, slot := range del {
			opts.DelKeysInSlot(slot)
		}
	}
	if opts.Publish != nil {
		for _, m := range pubs {
			opts.Publish(string(m.channel), m.payload)
		}
	}
}

// headerLocked returns a message of type typ describing this node.
func (c *Cluster) headerLocked(typ msgType) *message {
	me := c.myself
	m := &message{
		typ: typ, sender: me.ID, currentEpoch: c.currentEpoch, configEpoch: me.ConfigEpoch,
		replOffset: uint64(max(c.myOffset.Load(), 0)),
		flags:      me.Flags, port: me.Port, cport: me.CPort, tlsPort: me.TLSPort, stateOK: c.ok,
		ip: me.IP, hostname: me.Hostname, shardID: me.ShardID, slots: me.slots,
	}
	if !me.replica() && c.mf.end != 0 && c.mf.paused {
		m.mflags |= mflagPaused
	}
	if c.ipLearnable {
		m.ip = ""
	}
	if me.replica() {
		m.masterID = me.MasterID
		if master := c.nodes[me.MasterID]; master != nil {
			m.configEpoch = master.ConfigEpoch
			m.slots = master.slots
		}
	}
	return m
}

func gossipOf(n *Node) gossipEntry {
	return gossipEntry{id: n.ID, ip: n.IP, port: n.Port, cport: n.CPort, tlsPort: n.TLSPort, flags: n.Flags, pingSent: n.PingSent, pongRecv: n.PongRecv}
}

// sendPingLocked sends a PING, PONG or MEET on l with a gossip section of a
// few random nodes and every node in PFAIL, as clusterSendPing does.
func (c *Cluster) sendPingLocked(l *link, typ msgType) {
	if !l.up() {
		return
	}
	m := c.headerLocked(typ)
	wanted := len(c.nodes) / 10
	if wanted < 3 {
		wanted = 3
	}
	if fresh := len(c.nodes) - 2; wanted > fresh {
		wanted = fresh
	}
	var candidates, pfail []*Node
	for _, n := range c.nodes {
		switch {
		case n == c.myself || n.Flags&(FlagHandshake|FlagNoAddr) != 0:
		case n.Flags&FlagPFail != 0:
			pfail = append(pfail, n)
		case n.link == nil && n.numSlots == 0:
		default:
			candidates = append(candidates, n)
		}
	}
	rand.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	if wanted > len(candidates) {
		wanted = len(candidates)
	}
	for _, n := range candidates[:max(wanted, 0)] {
		m.gossip = append(m.gossip, gossipOf(n))
	}
	for _, n := range pfail {
		m.gossip = append(m.gossip, gossipOf(n))
	}
	if !l.inbound && typ == msgPing {
		l.node.PingSent = nowMs()
	}
	c.sendLocked(l, typ, m.encode())
}

// broadcastLocked sends frame to every node with an outbound link.
func (c *Cluster) broadcastLocked(typ msgType, frame []byte) {
	for _, n := range c.nodes {
		if n == c.myself || n.Flags&FlagHandshake != 0 {
			continue
		}
		c.sendLocked(n.link, typ, frame)
	}
}

func (c *Cluster) sendFailLocked(id string) {
	m := c.headerLocked(msgFail)
	m.about = id
	c.broadcastLocked(msgFail, m.encode())
}

// sendUpdateLocked tells the node on l the slots n serves, with n's epoch.
func (c *Cluster) sendUpdateLocked(l *link, n *Node) {
	if !l.up() {
		return
	}
	m := c.headerLocked(msgUpdate)
	m.about, m.updateEpoch, m.updateSlots = n.ID, n.ConfigEpoch, n.slots
	c.sendLocked(l, msgUpdate, m.encode())
}

// Publish broadcasts a PUBLISH to every other node, which delivers it to its
// own subscribers and does not forward it.
func (c *Cluster) Publish(channel string, message []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bus == nil {
		return
	}
	m := c.headerLocked(msgPublish)
	m.channel, m.payload = []byte(channel), message
	frame := m.encode()
	if len(frame) > maxFrameLen {
		logger.Warn().Int("bytes", len(frame)).Msg("cluster bus: PUBLISH message too large to broadcast")
		return
	}
	c.broadcastLocked(msgPublish, frame)
}

// peerIP is the address a node announced, or the address its link comes
// from when it announced none.
func peerIP(l *link, announced string) string {
	if announced != "" {
		return announced
	}
	if l.conn == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(l.conn.RemoteAddr().String())
	if err != nil {
		return ""
	}
	return host
}

// updateAddressLocked is nodeUpdateAddressIfNeeded.
func (c *Cluster) updateAddressLocked(n *Node, l *link, m *message) bool {
	if l == n.link {
		return false
	}
	ip := peerIP(l, m.ip)
	if ip == "" || (n.IP == ip && n.Port == m.port && n.CPort == m.cport && n.TLSPort == m.tlsPort) {
		return false
	}
	n.IP, n.Port, n.CPort, n.TLSPort = ip, m.port, m.cport, m.tlsPort
	c.closeLinkLocked(n.link)
	n.Flags &^= FlagNoAddr
	logger.Info().Str("node", n.ID).Str("ip", ip).Int("port", m.port).Msg("cluster: node address updated")
	return true
}

func (c *Cluster) addNodeLocked(n *Node) {
	if n.ShardID == "" {
		n.ShardID = NewID()
	}
	n.ctime = nowMs()
	c.nodes[n.ID] = n
}

// delNodeLocked removes n: its slots become unassigned, open slots naming it
// close, its failure reports go and its links close.
func (c *Cluster) delNodeLocked(n *Node) {
	for slot := 0; slot < Slots; slot++ {
		if c.importing[slot] == n {
			c.importing[slot] = nil
		}
		if c.migrating[slot] == n {
			c.migrating[slot] = nil
		}
		if c.owner[slot] == n {
			c.delSlotLocked(slot)
		}
	}
	for _, o := range c.nodes {
		delete(o.failReports, n)
	}
	if c.mf.replica == n {
		c.resetManualFailoverLocked()
	}
	c.closeLinkLocked(n.link)
	c.closeLinkLocked(n.inbound)
	delete(c.nodes, n.ID)
	c.todoSave = true
}

func (c *Cluster) renameNodeLocked(n *Node, id string) {
	delete(c.nodes, n.ID)
	n.ID = id
	c.nodes[id] = n
}

func (c *Cluster) blacklistedLocked(id string, now int64) bool {
	for k, exp := range c.blacklist {
		if exp <= now {
			delete(c.blacklist, k)
		}
	}
	_, ok := c.blacklist[id]
	return ok
}

func (c *Cluster) failureReportsLocked(n *Node, now int64) int {
	if c.bus == nil {
		return len(n.failReports)
	}
	maxAge := c.bus.timeout * failReportMult
	for reporter, at := range n.failReports {
		if now-at > maxAge {
			delete(n.failReports, reporter)
		}
	}
	return len(n.failReports)
}

// markFailingIfNeededLocked is markNodeAsFailingIfNeeded: a node in PFAIL
// here that a majority of the masters reports failing becomes FAIL, and
// every reachable node is told.
func (c *Cluster) markFailingIfNeededLocked(n *Node, now int64) {
	if n.Flags&FlagPFail == 0 || n.Flags&FlagFail != 0 {
		return
	}
	failures := c.failureReportsLocked(n, now)
	if !c.myself.replica() {
		failures++
	}
	if failures < c.size/2+1 {
		return
	}
	logger.Info().Str("node", n.ID).Msg("cluster: marking node as failing (quorum reached)")
	n.Flags = n.Flags&^FlagPFail | FlagFail
	n.failTime = now
	c.sendFailLocked(n.ID)
	c.todoSave = true
	c.updateStateLocked()
}

// clearFailureIfNeededLocked is clearNodeFailureIfNeeded, for a FAIL node
// that answered again.
func (c *Cluster) clearFailureIfNeededLocked(n *Node, now int64) {
	if n.replica() || n.numSlots == 0 || now-n.failTime > c.bus.timeout*failUndoMult {
		logger.Info().Str("node", n.ID).Msg("cluster: clearing the FAIL state of a reachable node")
		n.Flags &^= FlagFail
		c.todoSave = true
	}
}

func (c *Cluster) setMasterLocked(n *Node) {
	if !n.replica() && n.Flags&FlagMaster != 0 {
		return
	}
	if n.replica() && n != c.myself {
		n.migrateTo = true
	}
	n.Flags = n.Flags&^FlagSlave | FlagMaster
	n.MasterID = ""
	c.todoSave = true
}

func (c *Cluster) unclaimedLocked(slot int) bool {
	return c.owner[slot] == nil || testBit(c.notClaiming[:], slot)
}

// updateSlotsConfigWithLocked is clusterUpdateSlotsConfigWith: the slots
// sender claims with a greater config epoch than their owner's, or that
// nobody claims, become sender's. Slots being imported are left alone. A
// master left without slots by this becomes a replica of sender, as does a
// replica whose master was; otherwise the keys of the slots this node lost
// are deleted.
func (c *Cluster) updateSlotsConfigWithLocked(sender *Node, epoch uint64, slots *[Slots / 8]byte) {
	me := c.myself
	if sender == me {
		return
	}
	cur := me
	if me.replica() {
		cur = c.nodes[me.MasterID]
	}
	var dirty []int
	var newMaster *Node
	migrated := 0
	for j := 0; j < Slots; j++ {
		if !testBit(slots[:], j) {
			if c.owner[j] == sender {
				setBit(c.notClaiming[:], j)
			}
			continue
		}
		if c.owner[j] == sender {
			clearBit(c.notClaiming[:], j)
			continue
		}
		if c.importing[j] != nil {
			continue
		}
		if c.unclaimedLocked(j) || c.owner[j].ConfigEpoch < epoch {
			if c.owner[j] == me && c.bus != nil && c.bus.opts.KeysInSlot != nil && c.bus.opts.KeysInSlot(j) > 0 {
				dirty = append(dirty, j)
			}
			if cur != nil && c.owner[j] == cur {
				newMaster = sender
				migrated++
			}
			c.addSlotLocked(sender, j)
			c.todoSave = true
		}
	}
	var grand *Node
	if master := c.nodes[me.MasterID]; me.replica() && master != nil && master.replica() {
		grand = c.nodes[master.MasterID]
	}
	switch {
	case newMaster != nil && cur.numSlots == 0:
		logger.Info().Str("master", sender.ID).Msg("cluster: configuration change detected, reconfiguring this node as a replica")
		c.setMyMasterLocked(sender)
	case grand != nil && grand != me:
		logger.Info().Str("master", grand.ID).Msg("cluster: this node is a sub-replica, reconfiguring it as a replica of its grandmaster")
		c.setMyMasterLocked(grand)
	case len(dirty) > 0 && c.bus != nil:
		c.bus.delSlots = append(c.bus.delSlots, dirty...)
	}
	c.updateStateLocked()
}

// handleEpochCollisionLocked is clusterHandleConfigEpochCollision: of two
// masters with the same config epoch, the one with the smaller node ID takes
// a new epoch.
func (c *Cluster) handleEpochCollisionLocked(sender *Node) {
	if sender.ConfigEpoch != c.myself.ConfigEpoch || sender.replica() || c.myself.replica() || sender.ID <= c.myself.ID {
		return
	}
	c.currentEpoch++
	c.myself.ConfigEpoch = c.currentEpoch
	c.todoSave = true
	logger.Info().Uint64("epoch", c.myself.ConfigEpoch).Str("other", sender.ID).Msg("cluster: config epoch collision resolved")
}

func (c *Cluster) processPacket(b *bus, l *link, m *message) {
	c.mu.Lock()
	defer c.unlockAndSave()
	if b.stopped || l.closed {
		return
	}
	if m.typ < msgTypeCount {
		c.stats.recv[m.typ]++
	}
	now := nowMs()
	sender := c.nodes[m.sender]
	if sender != nil && sender.Flags&FlagHandshake != 0 || sender == c.myself {
		sender = nil
	}
	if sender != nil && l.inbound && l.node == nil {
		if sender.inbound != nil && sender.inbound != l {
			c.closeLinkLocked(sender.inbound)
		}
		l.node, sender.inbound = sender, l
	}
	if sender != nil {
		sender.dataReceived = now
		if m.currentEpoch > c.currentEpoch {
			c.currentEpoch = m.currentEpoch
			c.todoSave = true
		}
		if m.configEpoch > sender.ConfigEpoch {
			sender.ConfigEpoch = m.configEpoch
			c.todoSave = true
		}
		if m.shardID != "" && sender.ShardID != m.shardID {
			sender.ShardID = m.shardID
			c.todoSave = true
		}
		sender.replOffset = int64(m.replOffset)
		if c.mf.end != 0 && c.myself.replica() && c.myself.MasterID == sender.ID && m.mflags&mflagPaused != 0 && !c.mf.offsetKnown {
			c.mf.masterOffset, c.mf.offsetKnown = int64(m.replOffset), true
			logger.Info().Int64("offset", c.mf.masterOffset).Msg("cluster: received the replication offset of the paused master for the manual failover")
			c.handleManualFailoverLocked()
		}
		if sender.Hostname != m.hostname {
			sender.Hostname = m.hostname
			c.todoSave = true
		}
	}
	switch m.typ {
	case msgPing, msgPong, msgMeet:
		c.processPingLocked(l, m, sender, now)
	case msgFail:
		if sender == nil {
			break
		}
		if f := c.nodes[m.about]; f != nil && f.Flags&(FlagFail|FlagMyself) == 0 {
			logger.Info().Str("node", f.ID).Str("reporter", sender.ID).Msg("cluster: FAIL message received")
			f.Flags = f.Flags&^FlagPFail | FlagFail
			f.failTime = now
			c.todoSave = true
		}
	case msgPublish:
		if sender != nil {
			b.published = append(b.published, m)
		}
	case msgUpdate:
		if sender == nil {
			break
		}
		n := c.nodes[m.about]
		if n == nil || n.ConfigEpoch >= m.updateEpoch {
			break
		}
		c.setMasterLocked(n)
		n.ConfigEpoch = m.updateEpoch
		c.todoSave = true
		c.updateSlotsConfigWithLocked(n, m.updateEpoch, &m.updateSlots)
	case msgFailoverAuthRequest:
		if sender != nil {
			c.sendFailoverAuthIfNeededLocked(sender, m, now)
		}
	case msgFailoverAuthAck:
		if sender != nil && !sender.replica() && sender.numSlots > 0 && m.currentEpoch >= c.failover.authEpoch {
			c.failover.authCount++
			c.handleReplicaFailoverLocked(now)
		}
	case msgMFStart:
		if sender == nil || !sender.replica() || sender.MasterID != c.myself.ID || c.myself.replica() {
			break
		}
		c.resetManualFailoverLocked()
		c.mf.end = now + mfTimeout
		c.mf.replica = sender
		c.mf.pauseRequested = true
		c.mfStartPause = true
	}
	c.updateStateLocked()
}

// processPingLocked handles PING, PONG and MEET as clusterProcessPacket does.
func (c *Cluster) processPingLocked(l *link, m *message, sender *Node, now int64) {
	if l.inbound && c.ipLearnable && (m.typ == msgMeet || c.myself.IP == "") {
		if host, _, err := net.SplitHostPort(l.conn.LocalAddr().String()); err == nil && host != c.myself.IP {
			c.myself.IP = host
			c.todoSave = true
			logger.Info().Str("ip", host).Msg("cluster: learned this node's IP address")
		}
	}
	if sender == nil && m.typ == msgMeet {
		n := &Node{ID: NewID(), Flags: FlagHandshake, IP: peerIP(l, m.ip), Port: m.port, CPort: m.cport, TLSPort: m.tlsPort}
		c.addNodeLocked(n)
		c.processGossipLocked(m, nil, now)
	}
	if m.typ == msgPing || m.typ == msgMeet {
		c.sendPingLocked(l, msgPong)
	}
	if !l.inbound {
		n := l.node
		if n.Flags&FlagHandshake != 0 {
			if sender != nil {
				c.updateAddressLocked(sender, l, m)
				c.delNodeLocked(n)
				return
			}
			if !ValidID(m.sender) || c.nodes[m.sender] != nil {
				c.delNodeLocked(n)
				return
			}
			c.renameNodeLocked(n, m.sender)
			n.Flags &^= FlagHandshake
			n.Flags |= m.flags & (FlagMaster | FlagSlave)
			if n.Flags&(FlagMaster|FlagSlave) == 0 {
				n.Flags |= FlagMaster
			}
			if m.shardID != "" {
				n.ShardID = m.shardID
			}
			n.Hostname = m.hostname
			c.todoSave = true
			logger.Info().Str("node", n.ID).Msg("cluster: handshake completed")
		} else if n.ID != m.sender {
			logger.Warn().Str("node", n.ID).Str("answered", m.sender).Msg("cluster: a node answered with another ID, marking its address unknown")
			n.Flags |= FlagNoAddr
			n.IP, n.Port, n.CPort = "", 0, 0
			c.closeLinkLocked(l)
			c.todoSave = true
			return
		}
	}
	if sender != nil {
		sender.Flags = sender.Flags&^FlagNoFailover | m.flags&FlagNoFailover
	}
	if sender != nil && m.typ == msgPing && c.updateAddressLocked(sender, l, m) {
		c.todoSave = true
	}
	if !l.inbound && m.typ == msgPong {
		n := l.node
		n.PongRecv, n.PingSent = now, 0
		if n.Flags&FlagPFail != 0 {
			n.Flags &^= FlagPFail
		} else if n.Flags&FlagFail != 0 {
			c.clearFailureIfNeededLocked(n, now)
		}
	}
	if sender == nil {
		return
	}
	if m.flags&FlagMaster != 0 {
		c.setMasterLocked(sender)
	} else if m.flags&FlagSlave != 0 {
		master := c.nodes[m.masterID]
		if master == sender {
			master = nil
		}
		if !sender.replica() {
			// A master turned into a replica. When its new master is of the
			// same shard, that master took its slots over.
			heir := master != nil && master.ShardID == sender.ShardID && sender.ConfigEpoch <= m.configEpoch
			if heir {
				c.setMasterLocked(master)
				if m.configEpoch > master.ConfigEpoch {
					master.ConfigEpoch = m.configEpoch
				}
			}
			for slot := 0; slot < Slots; slot++ {
				if c.owner[slot] == sender {
					if heir {
						c.addSlotLocked(master, slot)
					} else {
						c.delSlotLocked(slot)
					}
				}
			}
			sender.Flags = sender.Flags&^FlagMaster | FlagSlave
			c.todoSave = true
		}
		if m.masterID != "" && sender.MasterID != m.masterID {
			sender.MasterID = m.masterID
			c.todoSave = true
		}
		if master != nil && sender.ShardID != master.ShardID {
			sender.ShardID = master.ShardID
			c.todoSave = true
		}
	}
	senderMaster := sender
	if sender.replica() {
		senderMaster = c.nodes[sender.MasterID]
	}
	dirty := senderMaster != nil && senderMaster.slots != m.slots
	if !sender.replica() && dirty {
		c.updateSlotsConfigWithLocked(sender, m.configEpoch, &m.slots)
	}
	if dirty {
		for j := 0; j < Slots; j++ {
			if !testBit(m.slots[:], j) || c.owner[j] == sender || c.unclaimedLocked(j) {
				continue
			}
			if c.owner[j].ConfigEpoch > m.configEpoch {
				c.sendUpdateLocked(sender.link, c.owner[j])
				break
			}
		}
	}
	if !c.myself.replica() && !sender.replica() && m.configEpoch == c.myself.ConfigEpoch {
		c.handleEpochCollisionLocked(sender)
	}
	c.processGossipLocked(m, sender, now)
}

// processGossipLocked is clusterProcessGossipSection.
func (c *Cluster) processGossipLocked(m *message, sender *Node, now int64) {
	for _, g := range m.gossip {
		n := c.nodes[g.id]
		if n == c.myself {
			continue
		}
		if n != nil {
			if sender != nil && !sender.replica() {
				if g.flags&(FlagFail|FlagPFail) != 0 {
					if n.failReports == nil {
						n.failReports = make(map[*Node]int64)
					}
					n.failReports[sender] = now
					c.markFailingIfNeededLocked(n, now)
				} else {
					delete(n.failReports, sender)
				}
			}
			if g.flags&(FlagFail|FlagPFail) == 0 && n.PingSent == 0 && c.failureReportsLocked(n, now) == 0 &&
				g.pongRecv <= now+pongClockSkewMs && g.pongRecv > n.PongRecv {
				n.PongRecv = g.pongRecv
			}
			if n.failing() && g.flags&(FlagNoAddr|FlagFail|FlagPFail) == 0 && g.ip != "" &&
				(n.IP != g.ip || n.Port != g.port || n.CPort != g.cport) {
				c.closeLinkLocked(n.link)
				n.IP, n.Port, n.CPort, n.TLSPort = g.ip, g.port, g.cport, g.tlsPort
				n.Flags &^= FlagNoAddr
				c.todoSave = true
			}
			continue
		}
		if sender == nil || g.flags&(FlagNoAddr|FlagHandshake) != 0 || g.ip == "" || !ValidID(g.id) || c.blacklistedLocked(g.id, now) {
			continue
		}
		n = &Node{ID: g.id, IP: g.ip, Port: g.port, CPort: g.cport, TLSPort: g.tlsPort, Flags: g.flags & (FlagMaster | FlagSlave | FlagNoFailover)}
		if n.Flags&(FlagMaster|FlagSlave) == 0 {
			n.Flags |= FlagMaster
		}
		c.addNodeLocked(n)
		c.todoSave = true
		logger.Info().Str("node", n.ID).Str("from", sender.ID).Msg("cluster: node learned through gossip")
	}
}

// cron is clusterCron, run every 100ms: it opens the missing links, pings,
// and flags the nodes that do not answer as PFAIL.
func (c *Cluster) cron(b *bus) {
	c.sampleReplication(b.opts)
	c.mu.Lock()
	defer c.unlockAndSave()
	if b.stopped {
		return
	}
	b.iteration++
	now := nowMs()
	handshakeTimeout := max(b.timeout, minHandshakeTime.Milliseconds())
	for _, n := range c.nodes {
		if n == c.myself || n.Flags&FlagNoAddr != 0 {
			continue
		}
		if n.Flags&FlagHandshake != 0 && now-n.ctime > handshakeTimeout {
			c.delNodeLocked(n)
			continue
		}
		if n.link == nil && now >= n.nextDial && n.IP != "" && n.CPort > 0 {
			c.connectLocked(b, n)
		}
	}
	if b.iteration%10 == 0 {
		var oldest *Node
		nodes := make([]*Node, 0, len(c.nodes))
		for _, n := range c.nodes {
			nodes = append(nodes, n)
		}
		for i := 0; i < 5 && len(nodes) > 0; i++ {
			n := nodes[rand.IntN(len(nodes))]
			if n == c.myself || !n.link.up() || n.PingSent != 0 || n.Flags&FlagHandshake != 0 {
				continue
			}
			if oldest == nil || n.PongRecv < oldest.PongRecv {
				oldest = n
			}
		}
		if oldest != nil {
			c.sendPingLocked(oldest.link, msgPing)
		}
	}
	half := b.timeout / 2
	for _, n := range c.nodes {
		if n == c.myself || n.Flags&(FlagNoAddr|FlagHandshake) != 0 {
			continue
		}
		if c.mf.end != 0 && c.mf.paused && !c.myself.replica() && c.mf.replica == n && n.link.up() {
			c.sendPingLocked(n.link, msgPing)
			continue
		}
		pingDelay, dataDelay := now-n.PingSent, now-n.dataReceived
		if n.link.up() && now-n.link.ctime > b.timeout && n.PingSent != 0 && pingDelay > half && dataDelay > half {
			c.closeLinkLocked(n.link)
		}
		if n.link.up() && n.PingSent == 0 && now-n.PongRecv > half {
			c.sendPingLocked(n.link, msgPing)
			continue
		}
		if n.PingSent == 0 {
			continue
		}
		if min(pingDelay, dataDelay) > b.timeout && !n.failing() {
			logger.Info().Str("node", n.ID).Msg("cluster: node is not reachable, marking it PFAIL")
			n.Flags |= FlagPFail
		}
	}
	c.manualFailoverCheckTimeoutLocked(now)
	if c.myself.replica() {
		c.handleManualFailoverLocked()
		c.handleReplicaFailoverLocked(now)
		c.handleReplicaMigrationLocked(now)
	}
	if b.iteration%10 == 0 {
		c.roleDirty = true
	}
	c.updateStateLocked()
}

var errInvalidAddress = errors.New("invalid address")

// Meet implements CLUSTER MEET: a handshake starts with the node at
// ip:port, bus port cport.
func (c *Cluster) Meet(ip string, port, cport int) error {
	parsed := net.ParseIP(ip)
	if parsed == nil || port <= 0 || port > 65535 || cport <= 0 || cport > 65535 {
		return errInvalidAddress
	}
	norm := parsed.String()
	c.mu.Lock()
	defer c.unlockAndSave()
	for _, n := range c.nodes {
		if n.Flags&FlagHandshake != 0 && strings.EqualFold(n.IP, norm) && n.Port == port && n.CPort == cport {
			return nil
		}
	}
	c.addNodeLocked(&Node{ID: NewID(), IP: norm, Port: port, CPort: cport, Flags: FlagHandshake, meet: true})
	return nil
}

// Forget implements CLUSTER FORGET: node id is removed and, for a minute,
// not added back from gossip.
func (c *Cluster) Forget(id string) error {
	id = strings.ToLower(id)
	c.mu.Lock()
	defer c.unlockAndSave()
	now := nowMs()
	n := c.nodes[id]
	switch {
	case n == nil:
		if c.blacklistedLocked(id, now) {
			return nil
		}
		return fmt.Errorf("ERR Unknown node %s", id)
	case n == c.myself:
		return errors.New("ERR I tried hard but I can't forget myself...") //nolint:staticcheck // Redis reply text
	case c.myself.replica() && c.myself.MasterID == n.ID:
		return errors.New("ERR Can't forget my master!") //nolint:staticcheck // Redis reply text
	}
	if c.blacklist == nil {
		c.blacklist = make(map[string]int64)
	}
	c.blacklistedLocked("", now)
	c.blacklist[id] = now + blacklistTTL.Milliseconds()
	c.delNodeLocked(n)
	c.updateStateLocked()
	return nil
}
