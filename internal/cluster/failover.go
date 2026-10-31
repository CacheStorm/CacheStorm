package cluster

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

// Message flags, as Redis' CLUSTERMSG_FLAG0_*.
const (
	mflagPaused   = 1 << 0
	mflagForceAck = 1 << 1
)

const (
	// mfTimeout is CLUSTER_MF_TIMEOUT: how long a manual failover may take.
	mfTimeout = 5000
	// mfPauseMult is CLUSTER_MF_PAUSE_MULT: the master pauses its writers
	// for mfTimeout times this.
	mfPauseMult = 2
	// replicaMigrationDelay is CLUSTER_SLAVE_MIGRATION_DELAY: how long a
	// master must stay without a working replica before one migrates to it.
	replicaMigrationDelay = 5000
)

// failoverState is the election state of a replica, as Redis keeps it in
// clusterState (failover_auth_*).
type failoverState struct {
	authTime  int64
	authCount int
	authSent  bool
	authRank  int
	authEpoch uint64
	cantLog   int64
	// promote is set once the election is won (or for a TAKEOVER): the
	// role goroutine stops replication, then takes the master's slots.
	promote bool
}

// manualFailover is the CLUSTER FAILOVER state (mf_*). end is when it
// expires, 0 when none runs. On the master, replica is the replica that
// asked and paused reports that the writers are paused; on the replica,
// masterOffset is the offset the paused master announced.
type manualFailover struct {
	end            int64
	canStart       bool
	replica        *Node
	masterOffset   int64
	offsetKnown    bool
	paused         bool
	pauseRequested bool
}

// resetManualFailoverLocked is resetManualFailover: a master that paused
// its writers resumes them.
func (c *Cluster) resetManualFailoverLocked() {
	if c.mf.paused {
		c.mfUnpause = true
	}
	c.mf = manualFailover{}
}

// setMyMasterLocked is clusterSetMaster: this node becomes a replica of n.
// The replication link follows once the lock is released.
func (c *Cluster) setMyMasterLocked(n *Node) {
	me := c.myself
	if !me.replica() {
		me.Flags = me.Flags&^FlagMaster | FlagSlave
		for slot := 0; slot < Slots; slot++ {
			c.migrating[slot], c.importing[slot] = nil, nil
		}
	}
	me.MasterID = n.ID
	me.ShardID = n.ShardID
	c.failover.promote = false
	c.resetManualFailoverLocked()
	c.roleDirty = true
	c.todoSave = true
	logger.Info().Str("master", n.ID).Msg("cluster: this node is now a replica")
}

// Replicate implements CLUSTER REPLICATE: this node becomes a replica of
// node id. empty reports that this node holds no keys.
func (c *Cluster) Replicate(id string, empty bool) error {
	c.mu.Lock()
	defer c.unlockAndSave()
	n := c.nodes[id]
	switch {
	case n == nil:
		return fmt.Errorf("ERR Unknown node %s", id)
	case n == c.myself:
		return errors.New("ERR Can't replicate myself")
	case n.replica():
		return errors.New("ERR I can only replicate a master, not a replica.") //nolint:staticcheck // Redis reply text
	case !c.myself.replica() && (c.myself.numSlots != 0 || !empty):
		return errors.New("ERR To set a master the node must be empty and without assigned slots.") //nolint:staticcheck // Redis reply text
	}
	c.setMyMasterLocked(n)
	c.updateStateLocked()
	return nil
}

// Failover implements CLUSTER FAILOVER [FORCE|TAKEOVER] on a replica.
func (c *Cluster) Failover(force, takeover bool) error {
	c.mu.Lock()
	defer c.unlockAndSave()
	me := c.myself
	master := c.nodes[me.MasterID]
	switch {
	case !me.replica():
		return errors.New("ERR You should send CLUSTER FAILOVER to a replica")
	case master == nil:
		return errors.New("ERR I'm a replica but my master is unknown to me")
	case !force && (master.Flags&FlagFail != 0 || !master.link.up()):
		return errors.New("ERR Master is down or failed, please use CLUSTER FAILOVER FORCE")
	}
	c.resetManualFailoverLocked()
	c.mf.end = nowMs() + mfTimeout
	switch {
	case takeover:
		logger.Info().Msg("cluster: taking over the master (user request)")
		c.bumpEpochLocked()
		c.failover.promote = true
		c.roleDirty = true
	case force:
		logger.Info().Msg("cluster: forced failover user request accepted")
		c.mf.canStart = true
	default:
		logger.Info().Msg("cluster: manual failover user request accepted")
		m := c.headerLocked(msgMFStart)
		c.sendLocked(master.link, msgMFStart, m.encode())
	}
	return nil
}

// failoverReplaceMasterLocked is clusterFailoverReplaceYourMaster: this
// replica becomes a master, takes its master's slots and tells everyone.
func (c *Cluster) failoverReplaceMasterLocked() {
	me := c.myself
	old := c.nodes[me.MasterID]
	if !me.replica() || old == nil {
		return
	}
	c.setMasterLocked(me)
	for slot := 0; slot < Slots; slot++ {
		if c.owner[slot] == old {
			c.addSlotLocked(me, slot)
		}
	}
	c.roleDirty = true
	c.todoSave = true
	c.updateStateLocked()
	c.broadcastPongLocked(false)
	c.resetManualFailoverLocked()
	logger.Info().Uint64("epoch", me.ConfigEpoch).Str("old", old.ID).Msg("cluster: failover done, this node is now a master")
}

// broadcastPongLocked is clusterBroadcastPong: to every node, or to the
// other replicas of this node's master only.
func (c *Cluster) broadcastPongLocked(siblingsOnly bool) {
	for _, n := range c.nodes {
		if n == c.myself || n.Flags&FlagHandshake != 0 || !n.link.up() {
			continue
		}
		if siblingsOnly && (!n.replica() || n.MasterID != c.myself.MasterID) {
			continue
		}
		c.sendPingLocked(n.link, msgPong)
	}
}

// replicaRankLocked is clusterGetSlaveRank: how many other replicas of the
// same master that may fail over have a greater replication offset.
func (c *Cluster) replicaRankLocked() int {
	me := c.myself
	mine := c.myOffset.Load()
	rank := 0
	for _, n := range c.nodes {
		if n != me && n.replica() && n.MasterID == me.MasterID && n.Flags&FlagNoFailover == 0 && n.replOffset > mine {
			rank++
		}
	}
	return rank
}

func (c *Cluster) logCantFailoverLocked(now int64, why string) {
	if now-c.failover.cantLog < 10000 {
		return
	}
	c.failover.cantLog = now
	logger.Info().Str("reason", why).Msg("cluster: currently unable to failover")
}

// handleReplicaFailoverLocked is clusterHandleSlaveFailover: a replica
// whose master is FAIL, or that runs a manual failover, schedules an
// election, asks the masters for their vote and, with a majority, replaces
// its master.
func (c *Cluster) handleReplicaFailoverLocked(now int64) {
	b := c.bus
	me := c.myself
	if b == nil || !me.replica() || c.failover.promote {
		return
	}
	master := c.nodes[me.MasterID]
	manual := c.mf.end != 0 && c.mf.canStart
	if master == nil || (master.Flags&FlagFail == 0 && !manual) || (me.Flags&FlagNoFailover != 0 && !manual) || master.numSlots == 0 {
		return
	}
	timeout := b.timeout
	authTimeout := max(timeout*2, 2000)
	authRetry := authTimeout * 2
	authAge := now - c.failover.authTime

	dataAge := c.dataAge.Load()
	if dataAge > timeout {
		dataAge -= timeout
	}
	if f := int64(b.opts.ReplicaValidityFactor); f > 0 && dataAge > b.opts.ReplPingPeriod.Milliseconds()+timeout*f && !manual {
		c.logCantFailoverLocked(now, "my data is too old")
		return
	}

	if authAge > authRetry {
		c.failover.authTime = now + 500 + rand.Int64N(500)
		c.failover.authCount = 0
		c.failover.authSent = false
		c.failover.authRank = c.replicaRankLocked()
		c.failover.authTime += int64(c.failover.authRank) * 1000
		if c.mf.end != 0 {
			c.failover.authTime = now
			c.failover.authRank = 0
		}
		logger.Info().Int64("delay_ms", c.failover.authTime-now).Int("rank", c.failover.authRank).
			Int64("offset", c.myOffset.Load()).Msg("cluster: start of election delayed")
		c.broadcastPongLocked(true)
		if c.mf.end == 0 {
			return
		}
		authAge = now - c.failover.authTime
	}

	if !c.failover.authSent && c.mf.end == 0 {
		if rank := c.replicaRankLocked(); rank > c.failover.authRank {
			c.failover.authTime += int64(rank-c.failover.authRank) * 1000
			c.failover.authRank = rank
			logger.Info().Int("rank", rank).Msg("cluster: replica rank updated, election delayed")
		}
	}
	if now < c.failover.authTime {
		c.logCantFailoverLocked(now, "waiting for the election delay")
		return
	}
	if authAge > authTimeout {
		c.logCantFailoverLocked(now, "the failover attempt expired")
		return
	}
	if !c.failover.authSent {
		c.currentEpoch++
		c.failover.authEpoch = c.currentEpoch
		logger.Info().Uint64("epoch", c.currentEpoch).Msg("cluster: starting a failover election")
		m := c.headerLocked(msgFailoverAuthRequest)
		if c.mf.end != 0 {
			m.mflags |= mflagForceAck
		}
		c.broadcastLocked(msgFailoverAuthRequest, m.encode())
		c.failover.authSent = true
		c.todoSave = true
		return
	}
	if c.failover.authCount >= c.size/2+1 {
		logger.Info().Msg("cluster: failover election won, this node is the new master")
		if me.ConfigEpoch < c.failover.authEpoch {
			me.ConfigEpoch = c.failover.authEpoch
		}
		c.failover.promote = true
		c.failover.authSent = false
		c.failover.authCount = 0
		c.roleDirty = true
		return
	}
	c.logCantFailoverLocked(now, "waiting for votes")
}

// sendFailoverAuthIfNeededLocked is clusterSendFailoverAuthIfNeeded: a
// master votes for node, a replica asking for the vote, at most once per
// epoch and per failed master.
func (c *Cluster) sendFailoverAuthIfNeededLocked(node *Node, m *message, now int64) {
	me := c.myself
	if me.replica() || me.numSlots == 0 {
		return
	}
	deny := func(why string) {
		logger.Info().Str("node", node.ID).Uint64("epoch", c.currentEpoch).Str("reason", why).Msg("cluster: failover auth denied")
	}
	if m.currentEpoch < c.currentEpoch {
		deny("the request epoch is older than the current epoch")
		return
	}
	if c.lastVoteEpoch == c.currentEpoch {
		deny("already voted for this epoch")
		return
	}
	master := c.nodes[node.MasterID]
	force := m.mflags&mflagForceAck != 0
	switch {
	case !node.replica():
		deny("it is a master node")
		return
	case master == nil:
		deny("its master is unknown")
		return
	case master.Flags&FlagFail == 0 && !force:
		deny("its master is up")
		return
	}
	if now-master.votedTime < c.bus.timeout*2 {
		deny("already voted about this master recently")
		return
	}
	for j := 0; j < Slots; j++ {
		if !testBit(m.slots[:], j) || c.unclaimedLocked(j) || c.owner[j].ConfigEpoch <= m.configEpoch {
			continue
		}
		deny("a claimed slot has a newer config epoch")
		return
	}
	c.lastVoteEpoch = c.currentEpoch
	master.votedTime = now
	c.todoSave = true
	c.pendingAcks = append(c.pendingAcks, node)
	logger.Info().Str("node", node.ID).Uint64("epoch", c.currentEpoch).Msg("cluster: failover auth granted")
}

// handleManualFailoverLocked is clusterHandleManualFailover: once the
// replica has processed the stream up to the offset of its paused master,
// the election may start.
func (c *Cluster) handleManualFailoverLocked() {
	if c.mf.end == 0 || c.mf.canStart || !c.mf.offsetKnown {
		return
	}
	if c.myOffset.Load() >= c.mf.masterOffset {
		c.mf.canStart = true
		logger.Info().Msg("cluster: all of the master's replication stream processed, manual failover can start")
	}
}

// manualFailoverCheckTimeoutLocked is manualFailoverCheckTimeout.
func (c *Cluster) manualFailoverCheckTimeoutLocked(now int64) {
	if c.mf.end != 0 && c.mf.end < now {
		logger.Warn().Msg("cluster: manual failover timed out")
		c.resetManualFailoverLocked()
	}
}

// roleLoop runs the replication role changes, one at a time, when asked
// and once a second.
func (c *Cluster) roleLoop(b *bus) {
	defer b.wg.Done()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-b.roleWake:
		case <-t.C:
		}
		c.applyRole(b.opts)
	}
}

// applyRole points the replication link at this node's master, or stops
// it when this node is a master. A replica that won an election first stops
// replicating, as Redis' replicationUnsetMaster, and only then takes its
// master's slots, so it never serves them as a read-only replica.
func (c *Cluster) applyRole(opts BusOptions) {
	if opts.SetMaster == nil {
		return
	}
	c.mu.RLock()
	me := c.myself
	promote := c.failover.promote
	want, ip, port, known := "", "", 0, true
	if me.replica() && !promote {
		m := c.nodes[me.MasterID]
		if m == nil || m.IP == "" || m.Port == 0 {
			known = false
		} else {
			ip, port = m.IP, m.Port
			want = net.JoinHostPort(ip, strconv.Itoa(port))
		}
	}
	c.mu.RUnlock()
	if !known || (c.roleSet && c.roleApplied == want && !promote) {
		return
	}
	opts.SetMaster(ip, port)
	c.roleApplied, c.roleSet = want, true
	if !promote {
		return
	}
	c.mu.Lock()
	if c.failover.promote {
		c.failover.promote = false
		c.failoverReplaceMasterLocked()
	}
	// Still a replica when another master took over meanwhile: the next
	// round points replication at it again.
	c.roleDirty = c.myself.replica()
	c.unlockAndSave()
}

// startManualFailoverPause pauses the writers of a master a replica asked
// to fail over, then tells that replica the offset it must reach.
func (c *Cluster) startManualFailoverPause(opts BusOptions) {
	if opts.PauseWrites != nil {
		opts.PauseWrites(time.Now().Add(mfTimeout * mfPauseMult * time.Millisecond))
	}
	var off int64
	if opts.ReplOffset != nil {
		off = opts.ReplOffset()
	}
	c.mu.Lock()
	if c.mf.end == 0 || c.mf.replica == nil || c.myself.replica() || !c.mf.pauseRequested {
		c.mu.Unlock()
		if opts.UnpauseWrites != nil {
			opts.UnpauseWrites()
		}
		return
	}
	c.mf.pauseRequested = false
	c.mf.paused = true
	c.myOffset.Store(off)
	logger.Info().Str("replica", c.mf.replica.ID).Int64("offset", off).Msg("cluster: manual failover requested by a replica, writers paused")
	c.sendPingLocked(c.mf.replica.link, msgPing)
	c.mu.Unlock()
}

// sampleReplication records this node's replication offset and data age
// for the headers and the failover checks.
func (c *Cluster) sampleReplication(opts BusOptions) {
	if opts.ReplOffset != nil {
		c.myOffset.Store(opts.ReplOffset())
	}
	if opts.ReplDataAge != nil {
		c.dataAge.Store(opts.ReplDataAge().Milliseconds())
	}
}

// handleReplicaMigrationLocked is the replica migration of clusterCron and
// clusterHandleSlaveMigration: when a master serving slots has lost all its
// working replicas, a replica of the master with the most working replicas
// (more than cluster-migration-barrier) moves over to it. Of those replicas
// the one with the smallest node ID moves, after the orphan has waited
// replicaMigrationDelay.
func (c *Cluster) handleReplicaMigrationLocked(now int64) {
	me := c.myself
	okReplicas := make(map[*Node]int)
	replicas := make(map[*Node][]*Node)
	for _, n := range c.nodes {
		if !n.replica() {
			continue
		}
		master := c.nodes[n.MasterID]
		if master == nil || master.replica() {
			continue
		}
		master.migrateTo = true
		replicas[master] = append(replicas[master], n)
		if n.Flags&FlagFail == 0 {
			okReplicas[master]++
		}
	}
	orphaned, maxReplicas, mine := false, 0, 0
	for _, n := range c.nodes {
		if n == me || n.replica() || n.Flags&(FlagNoAddr|FlagHandshake|FlagFail) != 0 {
			continue
		}
		ok := okReplicas[n]
		if ok == 0 && n.numSlots > 0 && n.migrateTo {
			orphaned = true
		}
		maxReplicas = max(maxReplicas, ok)
		if n.ID == me.MasterID {
			mine = ok
		}
	}
	if !orphaned || maxReplicas < 2 || mine != maxReplicas || !c.opts.AllowReplicaMigration || !c.ok {
		return
	}
	master := c.nodes[me.MasterID]
	if master == nil {
		return
	}
	working := 0
	for _, r := range replicas[master] {
		if !r.failing() {
			working++
		}
	}
	if working <= c.opts.MigrationBarrier {
		return
	}
	var target *Node
	candidate := me
	for _, n := range c.nodes {
		orphan := !n.replica() && n.Flags&FlagFail == 0 && n.migrateTo && okReplicas[n] == 0
		if orphan {
			if target == nil && n.numSlots > 0 {
				target = n
			}
			if n.orphanedTime == 0 {
				n.orphanedTime = now
			}
		} else {
			n.orphanedTime = 0
		}
		if !n.replica() && okReplicas[n] == maxReplicas {
			for _, r := range replicas[n] {
				if r.ID < candidate.ID {
					candidate = r
				}
			}
		}
	}
	if target != nil && candidate == me && now-target.orphanedTime > replicaMigrationDelay {
		logger.Info().Str("master", target.ID).Msg("cluster: migrating to an orphaned master")
		c.setMyMasterLocked(target)
	}
}
