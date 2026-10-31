// Package cluster holds the Redis Cluster state of a node: its identity, the
// known nodes, the owner of each of the 16384 hash slots with the slots being
// migrated or imported, the epochs, and the nodes.conf file they persist to.
package cluster

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/hashslot"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

// Slots is the number of hash slots.
const Slots = hashslot.Count

// IDLen is the length of a node or shard ID: 40 hex characters.
const IDLen = 40

// Flags are the node flags CLUSTER NODES reports.
type Flags uint16

const (
	FlagMyself Flags = 1 << iota
	FlagMaster
	FlagSlave
	FlagPFail
	FlagFail
	FlagHandshake
	FlagNoAddr
	FlagNoFailover
)

var flagNames = []struct {
	flag Flags
	name string
}{
	{FlagMyself, "myself"},
	{FlagMaster, "master"},
	{FlagSlave, "slave"},
	{FlagPFail, "fail?"},
	{FlagFail, "fail"},
	{FlagHandshake, "handshake"},
	{FlagNoAddr, "noaddr"},
	{FlagNoFailover, "nofailover"},
}

func (f Flags) String() string {
	var parts []string
	for _, fn := range flagNames {
		if f&fn.flag != 0 {
			parts = append(parts, fn.name)
		}
	}
	if len(parts) == 0 {
		return "noflags"
	}
	return strings.Join(parts, ",")
}

// Node is a cluster node as this node knows it.
type Node struct {
	ID          string
	IP          string
	Port        int
	CPort       int
	TLSPort     int
	Hostname    string
	ShardID     string
	Flags       Flags
	MasterID    string
	PingSent    int64
	PongRecv    int64
	ConfigEpoch uint64
	numSlots    int
	slots       [Slots / 8]byte

	link         *link
	inbound      *link
	failReports  map[*Node]int64
	failTime     int64
	dataReceived int64
	ctime        int64
	nextDial     int64
	dialBackoff  time.Duration
	meet         bool
	replOffset   int64
	votedTime    int64
	// migrateTo is CLUSTER_NODE_MIGRATE_TO: this master has had replicas,
	// so a replica may migrate to it once it has none. orphanedTime is
	// when it was first seen without a working replica.
	migrateTo    bool
	orphanedTime int64
}

func (n *Node) failing() bool { return n.Flags&(FlagFail|FlagPFail) != 0 }
func (n *Node) replica() bool { return n.Flags&FlagSlave != 0 }

// Addr is the client-facing address of a node.
type Addr struct {
	ID   string
	IP   string
	Port int
}

// Endpoint is the host:port a redirection names.
func (a Addr) Endpoint() string {
	return a.IP + ":" + strconv.Itoa(a.Port)
}

func (n *Node) addr() Addr { return Addr{ID: n.ID, IP: n.IP, Port: n.Port} }

// Options are the cluster settings of the node.
type Options struct {
	RequireFullCoverage bool
	AllowReadsWhenDown  bool
	// ReplicaNoFailover, AllowReplicaMigration and MigrationBarrier are
	// cluster-replica-no-failover, cluster-allow-replica-migration and
	// cluster-migration-barrier.
	ReplicaNoFailover     bool
	AllowReplicaMigration bool
	MigrationBarrier      int
}

// Cluster is the cluster state of this node.
type Cluster struct {
	mu            sync.RWMutex
	saveMu        sync.Mutex
	path          string
	opts          Options
	myself        *Node
	nodes         map[string]*Node
	owner         [Slots]*Node
	migrating     [Slots]*Node
	importing     [Slots]*Node
	notClaiming   [Slots / 8]byte
	currentEpoch  uint64
	lastVoteEpoch uint64
	ok            bool
	size          int
	// minorityTime is when this node last saw fewer than a majority of the
	// masters reachable; writableAt is when a master restarted from
	// nodes.conf may first serve writes.
	minorityTime int64
	writableAt   int64

	ipLearnable bool
	blacklist   map[string]int64
	todoSave    bool
	bus         *bus
	stats       msgStats

	failover failoverState
	mf       manualFailover
	// mfStartPause and mfUnpause ask unlockAndSave to pause or resume the
	// writers for a manual failover.
	mfStartPause bool
	mfUnpause    bool
	// roleDirty asks the role goroutine to bring the replication role in
	// line with this node's role; roleApplied is the master address it
	// last applied ("" for none), and only that goroutine touches it.
	roleDirty   bool
	roleApplied string
	roleSet     bool
	pendingAcks []*Node
	// myOffset is this node's replication offset and dataAge the
	// milliseconds since it last heard from its master, sampled by the cron.
	myOffset atomic.Int64
	dataAge  atomic.Int64
}

// NewID returns a random 40-character hex node ID.
func NewID() string {
	var b [IDLen / 2]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// ValidID reports whether id has the node ID format.
func ValidID(id string) bool {
	if len(id) != IDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// New returns the state of a fresh node: a new identity, no slots, no
// other nodes. path is where Save writes nodes.conf; empty disables saving.
func New(path string, opts Options) *Cluster {
	c := &Cluster{path: path, opts: opts, nodes: make(map[string]*Node)}
	c.myself = &Node{ID: NewID(), ShardID: NewID(), Flags: FlagMyself | FlagMaster}
	c.nodes[c.myself.ID] = c.myself
	c.updateMyselfFlagsLocked()
	c.updateStateLocked()
	return c
}

// Open loads the nodes.conf at path, or creates a fresh node and saves it
// there when the file does not exist. created reports the latter.
func Open(path string, opts Options) (c *Cluster, created bool, err error) {
	data, err := readFile(path)
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		c = New(path, opts)
		err = c.Save()
		return c, true, err
	}
	c, err = Parse(string(data))
	if err != nil {
		return nil, false, fmt.Errorf("cluster config %s: %w", path, err)
	}
	c.path, c.opts = path, opts
	c.writableAt = nowMs() + writableDelay
	c.mu.Lock()
	c.updateMyselfFlagsLocked()
	c.updateStateLocked()
	c.mu.Unlock()
	return c, false, nil
}

// updateMyselfFlagsLocked is clusterUpdateMyselfFlags: the nofailover flag
// follows cluster-replica-no-failover.
func (c *Cluster) updateMyselfFlagsLocked() {
	f := c.myself.Flags &^ FlagNoFailover
	if c.opts.ReplicaNoFailover {
		f |= FlagNoFailover
	}
	if f != c.myself.Flags {
		c.myself.Flags = f
		c.todoSave = true
	}
}

// Options returns the cluster settings in effect.
func (c *Cluster) Options() Options {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.opts
}

// UpdateOptions changes the cluster settings at run time, as CONFIG SET
// does.
func (c *Cluster) UpdateOptions(update func(*Options)) {
	c.mu.Lock()
	defer c.unlockAndSave()
	update(&c.opts)
	c.updateMyselfFlagsLocked()
	c.updateStateLocked()
}

// Path returns the nodes.conf path.
func (c *Cluster) Path() string { return c.path }

// MyID returns this node's ID.
func (c *Cluster) MyID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.myself.ID
}

// MyShardID returns the shard ID of this node.
func (c *Cluster) MyShardID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.myself.ShardID
}

// SetMyself sets the address this node announces. An empty ip keeps the one
// already known.
func (c *Cluster) SetMyself(ip string, port, cport, tlsPort int, hostname string) {
	c.mu.Lock()
	if ip != "" {
		c.myself.IP = ip
	}
	c.ipLearnable = ip == ""
	c.myself.Port, c.myself.CPort, c.myself.TLSPort = port, cport, tlsPort
	c.myself.Hostname = hostname
	c.mu.Unlock()
}

// SlotState is what routing needs to know about one slot.
type SlotState struct {
	// Assigned reports that some node serves the slot; Owner is that node.
	Assigned bool
	Owner    Addr
	// Mine reports that this node serves the slot.
	Mine bool
	// Migrating is the target the slot is migrating to, when MigratingSet.
	MigratingSet bool
	Migrating    Addr
	// Importing reports that this node imports the slot.
	Importing bool
	// Healthy reports that the cluster state is ok.
	Healthy bool
	// AllowReadsWhenDown mirrors the option of the same name.
	AllowReadsWhenDown bool
	// OwnerIsMyMaster reports that this node is a replica of the slot's
	// owner, so it may serve reads of the slot to READONLY clients.
	OwnerIsMyMaster bool
}

// Slot returns the routing state of slot.
func (c *Cluster) Slot(slot int) SlotState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st := SlotState{Healthy: c.ok, AllowReadsWhenDown: c.opts.AllowReadsWhenDown}
	if n := c.owner[slot]; n != nil {
		st.Assigned, st.Owner, st.Mine = true, n.addr(), n == c.myself
		st.OwnerIsMyMaster = c.myself.replica() && c.myself.MasterID == n.ID
	}
	if n := c.migrating[slot]; n != nil && st.Mine {
		st.MigratingSet, st.Migrating = true, n.addr()
	}
	st.Importing = c.importing[slot] != nil
	return st
}

// Healthy reports whether the cluster state is ok.
func (c *Cluster) Healthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ok
}

func (c *Cluster) updateStateLocked() {
	now := nowMs()
	master := !c.myself.replica()
	if master && !c.ok && now < c.writableAt && len(c.nodes) > 1 {
		return
	}
	ok := true
	if c.opts.RequireFullCoverage {
		for _, n := range &c.owner {
			if n == nil || n.Flags&FlagFail != 0 {
				ok = false
				break
			}
		}
	}
	size, reachable := 0, 0
	for _, n := range c.nodes {
		if n.Flags&FlagMaster != 0 && n.numSlots > 0 {
			size++
			if !n.failing() {
				reachable++
			}
		}
	}
	c.size = size
	if reachable < size/2+1 {
		ok = false
		if size > 0 {
			c.minorityTime = now
		}
	}
	if ok && !c.ok && master && c.minorityTime != 0 {
		delay := int64(maxRejoinDelay)
		if c.bus != nil {
			delay = min(max(c.bus.timeout, minRejoinDelay), maxRejoinDelay)
		}
		if now-c.minorityTime < delay {
			return
		}
	}
	c.ok = ok
}

func (c *Cluster) addSlotLocked(n *Node, slot int) {
	c.delSlotLocked(slot)
	c.owner[slot] = n
	n.numSlots++
	setBit(n.slots[:], slot)
}

func (c *Cluster) delSlotLocked(slot int) {
	if n := c.owner[slot]; n != nil {
		n.numSlots--
		clearBit(n.slots[:], slot)
		c.owner[slot] = nil
	}
	clearBit(c.notClaiming[:], slot)
}

func setBit(b []byte, i int)       { b[i>>3] |= 1 << (i & 7) }
func clearBit(b []byte, i int)     { b[i>>3] &^= 1 << (i & 7) }
func testBit(b []byte, i int) bool { return b[i>>3]&(1<<(i&7)) != 0 }

// SlotRange is an inclusive range of slots.
type SlotRange struct{ Start, End int }

// checkAssignments is Redis' checkSlotAssignmentsOrReply.
func (c *Cluster) checkAssignmentsLocked(seen []bool, del bool, r SlotRange) error {
	for slot := r.Start; slot <= r.End; slot++ {
		if del && c.owner[slot] == nil {
			return fmt.Errorf("ERR Slot %d is already unassigned", slot)
		}
		if !del && c.owner[slot] != nil {
			return fmt.Errorf("ERR Slot %d is already busy", slot)
		}
		if seen[slot] {
			return fmt.Errorf("ERR Slot %d specified multiple times", slot)
		}
		seen[slot] = true
	}
	return nil
}

// CheckSlots validates one range of an ADDSLOTS (del false) or DELSLOTS
// call, with seen holding the slots the call named before.
func (c *Cluster) CheckSlots(seen []bool, del bool, r SlotRange) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.checkAssignmentsLocked(seen, del, r)
}

// AddSlots assigns the slots of ranges to this node (ADDSLOTS, ADDSLOTSRANGE).
// Nothing changes when a slot is busy or given twice.
func (c *Cluster) AddSlots(ranges []SlotRange) error {
	return c.updateSlots(ranges, false)
}

// DelSlots unassigns the slots of ranges (DELSLOTS, DELSLOTSRANGE).
func (c *Cluster) DelSlots(ranges []SlotRange) error {
	return c.updateSlots(ranges, true)
}

func (c *Cluster) updateSlots(ranges []SlotRange, del bool) error {
	c.mu.Lock()
	seen := make([]bool, Slots)
	for _, r := range ranges {
		if err := c.checkAssignmentsLocked(seen, del, r); err != nil {
			c.mu.Unlock()
			return err
		}
	}
	for slot, set := range seen {
		if !set {
			continue
		}
		c.importing[slot] = nil
		if del {
			c.delSlotLocked(slot)
		} else {
			c.addSlotLocked(c.myself, slot)
		}
	}
	c.updateStateLocked()
	c.mu.Unlock()
	c.persist()
	return nil
}

// FlushSlots unassigns every slot this node serves.
func (c *Cluster) FlushSlots() {
	c.mu.Lock()
	for slot, n := range &c.owner {
		if n == c.myself {
			c.delSlotLocked(slot)
		}
	}
	c.updateStateLocked()
	c.mu.Unlock()
	c.persist()
}

// ClaimSlot makes this node serve slot when nobody does, or marks it as
// importing from its owner, the way Redis treats keys it finds at startup
// in a slot it does not serve.
func (c *Cluster) ClaimSlot(slot int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch n := c.owner[slot]; {
	case n == c.myself || c.importing[slot] != nil:
	case n == nil:
		logger.Warn().Int("slot", slot).Msg("cluster: I have keys for an unassigned slot, taking responsibility for it")
		c.addSlotLocked(c.myself, slot)
	default:
		logger.Warn().Int("slot", slot).Str("owner", n.ID).Msg("cluster: I have keys for a slot served by another node, setting it to importing")
		c.importing[slot] = n
	}
	c.updateStateLocked()
}

func (c *Cluster) lookupMasterLocked(id string, unknown string) (*Node, error) {
	n := c.nodes[id]
	if n == nil {
		return nil, fmt.Errorf(unknown, id)
	}
	if n.replica() {
		return nil, errors.New("ERR Target node is not a master")
	}
	return n, nil
}

// SetSlot implements CLUSTER SETSLOT slot MIGRATING|IMPORTING|NODE|STABLE.
// keysInSlot counts the keys this node holds in a slot.
//
// NODE follows Redis 7.4: a node giving away a slot must hold no keys for
// it, and its MIGRATING state clears once it holds none; a master left
// without slots becomes a replica of the new owner; the importing node bumps
// its config epoch without consensus and broadcasts a PONG, so the rest of
// the cluster learns the new owner at once.
func (c *Cluster) SetSlot(slot int, action, nodeID string, keysInSlot func(int) int) error {
	c.mu.Lock()
	defer c.unlockAndSave()
	if c.myself.replica() {
		return errors.New("ERR Please use SETSLOT only with masters.") //nolint:staticcheck // Redis reply text
	}
	switch action {
	case "MIGRATING":
		if c.owner[slot] != c.myself {
			return fmt.Errorf("ERR I'm not the owner of hash slot %d", slot)
		}
		n, err := c.lookupMasterLocked(nodeID, "ERR I don't know about node %s")
		if err != nil {
			return err
		}
		c.migrating[slot] = n
	case "IMPORTING":
		if c.owner[slot] == c.myself {
			return fmt.Errorf("ERR I'm already the owner of hash slot %d", slot)
		}
		n, err := c.lookupMasterLocked(nodeID, "ERR I don't know about node %s")
		if err != nil {
			return err
		}
		c.importing[slot] = n
	case "STABLE":
		c.importing[slot], c.migrating[slot] = nil, nil
	case "NODE":
		n, err := c.lookupMasterLocked(nodeID, "ERR Unknown node %s")
		if err != nil {
			return err
		}
		keys := keysInSlot(slot)
		if c.owner[slot] == c.myself && n != c.myself && keys != 0 {
			return fmt.Errorf("ERR Can't assign hashslot %d to a different node while I still hold keys for this hash slot.", slot) //nolint:staticcheck // Redis reply text
		}
		if keys == 0 {
			c.migrating[slot] = nil
		}
		wasMine := c.owner[slot] == c.myself
		c.delSlotLocked(slot)
		c.addSlotLocked(n, slot)
		if wasMine && n != c.myself && c.myself.numSlots == 0 {
			logger.Info().Str("master", n.ID).Msg("cluster: configuration change detected, reconfiguring this node as a replica")
			c.setMyMasterLocked(n)
		}
		if n == c.myself && c.importing[slot] != nil {
			if c.bumpEpochLocked() {
				logger.Info().Uint64("epoch", c.myself.ConfigEpoch).Int("slot", slot).Msg("cluster: configEpoch updated after importing slot")
			}
			c.importing[slot] = nil
			if c.bus != nil {
				c.broadcastPongLocked(false)
			}
		}
	}
	c.todoSave = true
	c.updateStateLocked()
	return nil
}

func (c *Cluster) maxEpochLocked() uint64 {
	max := c.currentEpoch
	for _, n := range c.nodes {
		if n.ConfigEpoch > max {
			max = n.ConfigEpoch
		}
	}
	return max
}

// bumpEpochLocked is clusterBumpConfigEpochWithoutConsensus.
func (c *Cluster) bumpEpochLocked() bool {
	if c.myself.ConfigEpoch == 0 || c.myself.ConfigEpoch != c.maxEpochLocked() {
		c.currentEpoch++
		c.myself.ConfigEpoch = c.currentEpoch
		return true
	}
	return false
}

// BumpEpoch implements CLUSTER BUMPEPOCH: it reports whether the config
// epoch changed and the epoch this node now has.
func (c *Cluster) BumpEpoch() (bool, uint64) {
	c.mu.Lock()
	bumped := c.bumpEpochLocked()
	epoch := c.myself.ConfigEpoch
	c.mu.Unlock()
	if bumped {
		c.persist()
	}
	return bumped, epoch
}

// SetConfigEpoch implements CLUSTER SET-CONFIG-EPOCH.
func (c *Cluster) SetConfigEpoch(epoch int64) error {
	c.mu.Lock()
	switch {
	case epoch < 0:
		c.mu.Unlock()
		return fmt.Errorf("ERR Invalid config epoch specified: %d", epoch)
	case len(c.nodes) > 1:
		c.mu.Unlock()
		return errors.New("ERR The user can assign a config epoch only when the node does not know any other node.") //nolint:staticcheck // Redis reply text
	case c.myself.ConfigEpoch != 0:
		c.mu.Unlock()
		return errors.New("ERR Node config epoch is already non-zero")
	}
	c.myself.ConfigEpoch = uint64(epoch)
	if c.currentEpoch < uint64(epoch) {
		c.currentEpoch = uint64(epoch)
	}
	c.updateStateLocked()
	c.mu.Unlock()
	c.persist()
	return nil
}

// Reset implements CLUSTER RESET: this node becomes a master serving no
// slots and forgets every other node; a hard reset also zeroes the epochs and
// takes a new node ID.
func (c *Cluster) Reset(hard bool) {
	c.mu.Lock()
	c.myself.Flags = c.myself.Flags&^FlagSlave | FlagMaster
	c.myself.MasterID = ""
	c.resetManualFailoverLocked()
	c.failover.promote = false
	c.roleDirty = true
	for slot := range c.owner {
		c.migrating[slot], c.importing[slot] = nil, nil
		c.delSlotLocked(slot)
	}
	for _, n := range c.nodes {
		if n != c.myself {
			c.delNodeLocked(n)
		}
	}
	if hard {
		c.currentEpoch, c.lastVoteEpoch, c.myself.ConfigEpoch = 0, 0, 0
		delete(c.nodes, c.myself.ID)
		c.myself.ID, c.myself.ShardID = NewID(), NewID()
		c.nodes[c.myself.ID] = c.myself
	}
	c.updateStateLocked()
	c.todoSave = true
	c.unlockAndSave()
}

// IsReplica reports whether this node is a replica.
func (c *Cluster) IsReplica() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.myself.replica()
}

// CountFailureReports implements CLUSTER COUNT-FAILURE-REPORTS: the number
// of masters that recently reported node id as failing.
func (c *Cluster) CountFailureReports(id string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := c.nodes[id]
	if n == nil {
		return 0, fmt.Errorf("ERR Unknown node %s", id)
	}
	return c.failureReportsLocked(n, nowMs()), nil
}

// Info is the CLUSTER INFO text.
func (c *Cluster) Info() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	assigned, okSlots, pfail, fail := 0, 0, 0, 0
	for _, n := range &c.owner {
		if n == nil {
			continue
		}
		assigned++
		switch {
		case n.Flags&FlagFail != 0:
			fail++
		case n.Flags&FlagPFail != 0:
			pfail++
		default:
			okSlots++
		}
	}
	state := "ok"
	if !c.ok {
		state = "fail"
	}
	myEpoch := c.myself.ConfigEpoch
	if m := c.nodes[c.myself.MasterID]; c.myself.replica() && m != nil {
		myEpoch = m.ConfigEpoch
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cluster_state:%s\r\n", state)
	fmt.Fprintf(&b, "cluster_slots_assigned:%d\r\n", assigned)
	fmt.Fprintf(&b, "cluster_slots_ok:%d\r\n", okSlots)
	fmt.Fprintf(&b, "cluster_slots_pfail:%d\r\n", pfail)
	fmt.Fprintf(&b, "cluster_slots_fail:%d\r\n", fail)
	fmt.Fprintf(&b, "cluster_known_nodes:%d\r\n", len(c.nodes))
	fmt.Fprintf(&b, "cluster_size:%d\r\n", c.size)
	fmt.Fprintf(&b, "cluster_current_epoch:%d\r\n", c.currentEpoch)
	fmt.Fprintf(&b, "cluster_my_epoch:%d\r\n", myEpoch)
	c.stats.write(&b)
	return b.String()
}

// sortedNodesLocked returns myself first, then the other nodes by ID.
func (c *Cluster) sortedNodesLocked() []*Node {
	out := make([]*Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		if n != c.myself {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return append([]*Node{c.myself}, out...)
}

// rangesLocked returns the slot ranges each node serves.
func (c *Cluster) rangesLocked() map[*Node][]SlotRange {
	out := make(map[*Node][]SlotRange)
	for slot := 0; slot < Slots; {
		n := c.owner[slot]
		if n == nil {
			slot++
			continue
		}
		end := slot
		for end+1 < Slots && c.owner[end+1] == n {
			end++
		}
		out[n] = append(out[n], SlotRange{slot, end})
		slot = end + 1
	}
	return out
}

// nodeLineLocked is one line of CLUSTER NODES, or of nodes.conf when
// forConfig is set (with the aux fields and without the trailing newline).
func (c *Cluster) nodeLineLocked(n *Node, ranges []SlotRange, forConfig bool) string {
	var b strings.Builder
	b.WriteString(n.ID)
	fmt.Fprintf(&b, " %s:%d@%d", n.IP, n.Port, n.CPort)
	if n.Hostname != "" {
		b.WriteString("," + n.Hostname)
	}
	if forConfig {
		if n.Hostname == "" {
			b.WriteString(",")
		}
		if n.TLSPort != 0 {
			fmt.Fprintf(&b, ",tls-port=%d", n.TLSPort)
		}
		b.WriteString(",shard-id=" + n.ShardID)
	}
	b.WriteString(" " + n.Flags.String() + " ")
	if n.MasterID != "" {
		b.WriteString(n.MasterID)
	} else {
		b.WriteString("-")
	}
	epoch := n.ConfigEpoch
	if m := c.nodes[n.MasterID]; n.replica() && m != nil {
		epoch = m.ConfigEpoch
	}
	link := "disconnected"
	if n == c.myself || n.link.up() {
		link = "connected"
	}
	fmt.Fprintf(&b, " %d %d %d %s", n.PingSent, n.PongRecv, epoch, link)
	for _, r := range ranges {
		if r.Start == r.End {
			fmt.Fprintf(&b, " %d", r.Start)
		} else {
			fmt.Fprintf(&b, " %d-%d", r.Start, r.End)
		}
	}
	if n == c.myself {
		for slot := 0; slot < Slots; slot++ {
			if m := c.migrating[slot]; m != nil {
				fmt.Fprintf(&b, " [%d->-%s]", slot, m.ID)
			} else if m := c.importing[slot]; m != nil {
				fmt.Fprintf(&b, " [%d-<-%s]", slot, m.ID)
			}
		}
	}
	return b.String()
}

// Nodes is the CLUSTER NODES text.
func (c *Cluster) Nodes() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ranges := c.rangesLocked()
	var b strings.Builder
	for _, n := range c.sortedNodesLocked() {
		b.WriteString(c.nodeLineLocked(n, ranges[n], false))
		b.WriteString("\n")
	}
	return b.String()
}

// Replicas returns the CLUSTER NODES lines of the replicas of node id.
func (c *Cluster) Replicas(id string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := c.nodes[id]
	if m == nil {
		return nil, fmt.Errorf("ERR Unknown node %s", id)
	}
	if m.replica() {
		return nil, errors.New("ERR The specified node is not a master")
	}
	ranges := c.rangesLocked()
	var out []string
	for _, n := range c.sortedNodesLocked() {
		if n.replica() && n.MasterID == id {
			out = append(out, c.nodeLineLocked(n, ranges[n], false))
		}
	}
	return out, nil
}

// NodeView is what CLUSTER SLOTS and CLUSTER SHARDS report of a node.
type NodeView struct {
	ID       string
	IP       string
	Hostname string
	Port     int
	TLSPort  int
	Replica  bool
	Failing  bool
	Myself   bool
	// ReplOffset is the replication offset the node last announced.
	ReplOffset int64
}

func (c *Cluster) viewLocked(n *Node) NodeView {
	return NodeView{ID: n.ID, IP: n.IP, Hostname: n.Hostname, Port: n.Port, TLSPort: n.TLSPort,
		Replica: n.replica(), Failing: n.Flags&FlagFail != 0, Myself: n == c.myself, ReplOffset: c.offsetOfLocked(n)}
}

func (c *Cluster) offsetOfLocked(n *Node) int64 {
	if n == c.myself {
		return c.myOffset.Load()
	}
	return n.replOffset
}

// replicaAvailableLocked is Redis' isReplicaAvailable: a replica that is not
// failing and has received data.
func (c *Cluster) replicaAvailableLocked(n *Node) bool {
	return n.Flags&FlagFail == 0 && c.offsetOfLocked(n) != 0
}

// SlotsEntry is one entry of CLUSTER SLOTS.
type SlotsEntry struct {
	Start, End int
	Nodes      []NodeView
}

// SlotMap returns the CLUSTER SLOTS entries: each range of slots with its
// master and the master's available replicas, by start slot.
func (c *Cluster) SlotMap() []SlotsEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []SlotsEntry
	nodes := c.sortedNodesLocked()
	for n, ranges := range c.rangesLocked() {
		views := []NodeView{c.viewLocked(n)}
		for _, r := range nodes {
			if r.replica() && r.MasterID == n.ID && c.replicaAvailableLocked(r) {
				views = append(views, c.viewLocked(r))
			}
		}
		for _, r := range ranges {
			out = append(out, SlotsEntry{Start: r.Start, End: r.End, Nodes: views})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// Shard is one entry of CLUSTER SHARDS.
type Shard struct {
	Slots []SlotRange
	Nodes []NodeView
}

// Shards returns the CLUSTER SHARDS entries: every shard with the slots its
// master serves and its nodes, master first.
func (c *Cluster) Shards() []Shard {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ranges := c.rangesLocked()
	byID := make(map[string]*Shard)
	var order []string
	for _, n := range c.sortedNodesLocked() {
		sid := n.ShardID
		if m := c.nodes[n.MasterID]; n.replica() && m != nil {
			sid = m.ShardID
		}
		sh := byID[sid]
		if sh == nil {
			sh = &Shard{}
			byID[sid] = sh
			order = append(order, sid)
		}
		if n.replica() {
			sh.Nodes = append(sh.Nodes, c.viewLocked(n))
		} else {
			sh.Slots = ranges[n]
			sh.Nodes = append([]NodeView{c.viewLocked(n)}, sh.Nodes...)
		}
	}
	out := make([]Shard, 0, len(order))
	for _, sid := range order {
		out = append(out, *byID[sid])
	}
	return out
}

// Config returns the nodes.conf text.
func (c *Cluster) Config() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ranges := c.rangesLocked()
	var b strings.Builder
	for _, n := range c.sortedNodesLocked() {
		if n.Flags&FlagHandshake != 0 {
			continue
		}
		b.WriteString(c.nodeLineLocked(n, ranges[n], true))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "vars currentEpoch %d lastVoteEpoch %d\n", c.currentEpoch, c.lastVoteEpoch)
	return b.String()
}

// Save writes nodes.conf atomically. Saves are serialized, so the file
// always ends up with the latest state.
func (c *Cluster) Save() error {
	if c.path == "" {
		return nil
	}
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
	return persistence.WriteFileAtomic(c.path, []byte(c.Config()))
}

// persist saves nodes.conf after a state change. The change stands even
// when the write fails; the failure is logged.
func (c *Cluster) persist() {
	if err := c.Save(); err != nil {
		logger.Error().Err(err).Str("path", c.path).Msg("cluster: saving the node config failed")
	}
}
