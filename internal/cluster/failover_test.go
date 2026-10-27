package cluster

import (
	"strings"
	"testing"
)

func TestFailoverVoteRules(t *testing.T) {
	c := New("", Options{})
	c.bus = &bus{timeout: 1000}
	c.mu.Lock()
	me := c.myself
	c.addSlotLocked(me, 0)
	node := func(id string, flags Flags, master string, epoch uint64) *Node {
		n := &Node{ID: strings.Repeat(id, 40), Flags: flags, MasterID: master, ConfigEpoch: epoch}
		c.addNodeLocked(n)
		return n
	}
	failed := node("1", FlagMaster|FlagFail, "", 1)
	c.addSlotLocked(failed, 1)
	up := node("2", FlagMaster, "", 2)
	c.addSlotLocked(up, 2)
	newer := node("3", FlagMaster, "", 10)
	c.addSlotLocked(newer, 3)
	r1 := node("4", FlagSlave, failed.ID, 1)
	r2 := node("5", FlagSlave, failed.ID, 1)
	r3 := node("6", FlagSlave, up.ID, 2)

	request := func(epoch, configEpoch uint64, force bool, slots ...int) *message {
		m := &message{currentEpoch: epoch, configEpoch: configEpoch}
		if force {
			m.mflags = mflagForceAck
		}
		for _, s := range slots {
			setBit(m.slots[:], s)
		}
		return m
	}
	votes := func() int {
		n := len(c.pendingAcks)
		c.pendingAcks = nil
		return n
	}
	now := nowMs()

	c.currentEpoch = 5
	c.sendFailoverAuthIfNeededLocked(r1, request(5, 1, false, 1), now)
	if votes() != 1 || c.lastVoteEpoch != 5 || failed.votedTime != now {
		t.Fatalf("first vote: lastVoteEpoch %d", c.lastVoteEpoch)
	}
	c.sendFailoverAuthIfNeededLocked(r2, request(5, 1, false, 1), now)
	if votes() != 0 {
		t.Fatal("voted twice in one epoch")
	}
	c.currentEpoch = 6
	c.sendFailoverAuthIfNeededLocked(r2, request(6, 1, false, 1), now+1999)
	if votes() != 0 {
		t.Fatal("voted for the same master within two node timeouts")
	}
	c.sendFailoverAuthIfNeededLocked(r2, request(6, 1, false, 1), now+2001)
	if votes() != 1 || c.lastVoteEpoch != 6 {
		t.Fatal("no vote once two node timeouts passed")
	}

	c.currentEpoch = 7
	c.sendFailoverAuthIfNeededLocked(r3, request(6, 2, true, 2), now+5000)
	if votes() != 0 {
		t.Fatal("voted for a request with an old epoch")
	}
	c.sendFailoverAuthIfNeededLocked(r3, request(7, 2, false, 2), now+5000)
	if votes() != 0 {
		t.Fatal("voted for a replica whose master is up")
	}
	c.sendFailoverAuthIfNeededLocked(up, request(7, 2, true, 2), now+5000)
	if votes() != 0 {
		t.Fatal("voted for a master")
	}
	c.sendFailoverAuthIfNeededLocked(r1, request(7, 1, false, 1, 3), now+9000)
	if votes() != 0 {
		t.Fatal("voted for a replica claiming a slot with a newer config epoch")
	}
	c.sendFailoverAuthIfNeededLocked(r3, request(7, 2, true, 2), now+5000)
	if votes() != 1 || c.lastVoteEpoch != 7 {
		t.Fatal("no vote for a manual failover (FORCEACK)")
	}

	me.Flags = me.Flags&^FlagMaster | FlagSlave
	c.currentEpoch = 8
	c.sendFailoverAuthIfNeededLocked(r1, request(8, 1, false, 1), now+20000)
	if votes() != 0 {
		t.Fatal("a replica voted")
	}
	c.mu.Unlock()
	if !strings.Contains(c.Config(), "lastVoteEpoch 7") {
		t.Fatalf("lastVoteEpoch is not in nodes.conf:\n%s", c.Config())
	}
	again, err := Parse(c.Config())
	if err != nil || again.lastVoteEpoch != 7 {
		t.Fatalf("lastVoteEpoch does not survive a reload: %v", err)
	}
}

func TestReplicaRankAndSlots(t *testing.T) {
	c := New("", Options{})
	c.mu.Lock()
	master := &Node{ID: strings.Repeat("1", 40), Flags: FlagMaster}
	c.addNodeLocked(master)
	c.addSlotLocked(master, 5)
	c.setMyMasterLocked(master)
	c.myOffset.Store(100)
	for i, off := range []int64{50, 150, 200} {
		n := &Node{ID: strings.Repeat(string(rune('2'+i)), 40), Flags: FlagSlave, MasterID: master.ID, replOffset: off}
		c.nodes[n.ID] = n
	}
	if r := c.replicaRankLocked(); r != 2 {
		t.Fatalf("rank %d", r)
	}
	if !c.myself.replica() || c.myself.ShardID != master.ShardID || !c.roleDirty {
		t.Fatal("setMyMaster did not make this node a replica of the master's shard")
	}
	c.mu.Unlock()
	if st := c.Slot(5); !st.OwnerIsMyMaster || st.Mine {
		t.Fatalf("slot state %+v", st)
	}
}
