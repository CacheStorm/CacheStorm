package cluster

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const otherID = "1111111111111111111111111111111111111111"

func TestOpenCreatesAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.conf")
	c, created, err := Open(path, Options{RequireFullCoverage: true})
	if err != nil || !created {
		t.Fatalf("Open: %v created=%v", err, created)
	}
	if !ValidID(c.MyID()) || !ValidID(c.MyShardID()) {
		t.Fatalf("ids %q %q", c.MyID(), c.MyShardID())
	}
	if c.Healthy() {
		t.Fatal("a node without slots must be in the fail state")
	}
	c.SetMyself("127.0.0.1", 7000, 17000, 0, "")
	if err := c.AddSlots([]SlotRange{{0, 16383}}); err != nil {
		t.Fatal(err)
	}
	if !c.Healthy() {
		t.Fatal("full coverage must be ok")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := c.MyID() + " 127.0.0.1:7000@17000,,shard-id=" + c.MyShardID() + " myself,master - 0 0 0 connected 0-16383\nvars currentEpoch 0 lastVoteEpoch 0\n"
	if string(data) != want {
		t.Fatalf("nodes.conf:\n%q\nwant\n%q", data, want)
	}
	again, created, err := Open(path, Options{RequireFullCoverage: true})
	if err != nil || created {
		t.Fatalf("reopen: %v created=%v", err, created)
	}
	if again.MyID() != c.MyID() || again.Config() != c.Config() || !again.Healthy() {
		t.Fatalf("round trip:\n%s\n%s", again.Config(), c.Config())
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".cachestorm-tmp-*"))
	if len(matches) != 0 {
		t.Fatalf("temporary files left: %v", matches)
	}
}

func TestParseRedisNodesConf(t *testing.T) {
	myID := "e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca"
	replicaID := "2222222222222222222222222222222222222222"
	text := myID + " 127.0.0.1:30001@40001,,tls-port=0,shard-id=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa myself,master - 0 0 1 connected 0-5460 5462 [5463->-" + otherID + "] [9000-<-" + otherID + "]\n" +
		otherID + " 127.0.0.2:30002@40002,host2,shard-id=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb master - 0 1700000000000 2 connected 5461 5463-16383\n" +
		replicaID + " 127.0.0.3:30003@40003 slave " + otherID + " 0 0 2 connected\n" +
		"vars currentEpoch 7 lastVoteEpoch 3\n"
	c, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	c.updateStateLocked()
	if c.MyID() != myID || !c.Healthy() {
		t.Fatalf("myself %s healthy=%v", c.MyID(), c.Healthy())
	}
	st := c.Slot(5463)
	if !st.Assigned || st.Mine || st.Owner.Endpoint() != "127.0.0.2:30002" || st.MigratingSet {
		t.Fatalf("slot 5463: %+v", st)
	}
	if st := c.Slot(9000); !st.Importing || st.Mine {
		t.Fatalf("slot 9000: %+v", st)
	}
	conf := c.Config()
	for _, want := range []string{
		"vars currentEpoch 7 lastVoteEpoch 3\n",
		" myself,master - 0 0 1 connected 0-5460 5462 [5463->-" + otherID + "] [9000-<-" + otherID + "]\n",
		" 127.0.0.2:30002@40002,host2,shard-id=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb master - 0 1700000000000 2 disconnected 5461 5463-16383\n",
		replicaID + " 127.0.0.3:30003@40003,,shard-id=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb slave " + otherID + " 0 0 2 disconnected\n",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config lacks %q:\n%s", want, conf)
		}
	}
	nodes := c.Nodes()
	if !strings.HasPrefix(nodes, myID+" 127.0.0.1:30001@40001 myself,master - 0 0 1 connected 0-5460 5462 [5463->-") {
		t.Fatalf("nodes:\n%s", nodes)
	}
	if strings.Contains(nodes, "shard-id") || !strings.Contains(nodes, " 127.0.0.2:30002@40002,host2 master") {
		t.Fatalf("CLUSTER NODES must not show aux fields:\n%s", nodes)
	}
	again, err := Parse(conf)
	if err != nil || again.Config() != conf {
		t.Fatalf("round trip: %v\n%s", err, again.Config())
	}
	shards := c.Shards()
	if len(shards) != 2 || len(shards[1].Nodes) != 2 || shards[1].Nodes[0].ID != otherID || shards[1].Nodes[1].ID != replicaID {
		t.Fatalf("shards: %+v", shards)
	}
	slots := c.SlotMap()
	if len(slots) != 4 || slots[0].Start != 0 || slots[0].End != 5460 || len(slots[1].Nodes) != 2 {
		t.Fatalf("slots: %+v", slots)
	}
	if reps, err := c.Replicas(otherID); err != nil || len(reps) != 1 || !strings.HasPrefix(reps[0], replicaID) {
		t.Fatalf("replicas %v %v", reps, err)
	}
	if _, err := c.Replicas(replicaID); err == nil || err.Error() != "ERR The specified node is not a master" {
		t.Fatalf("replicas of a replica: %v", err)
	}
}

func TestParseRejectsCorruptFiles(t *testing.T) {
	for _, text := range []string{
		"garbage\n",
		otherID + " 127.0.0.1:1@2 master - 0 0 0 connected\n",
		otherID + " 127.0.0.1:1@2 myself,master - 0 0 0 connected 16384\n",
		otherID + " 127.0.0.1:1@2 myself,bogus - 0 0 0 connected\n",
	} {
		if _, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) accepted a corrupt file", text)
		}
	}
}

func TestSlotCommands(t *testing.T) {
	c, err := Parse("0000000000000000000000000000000000000001 127.0.0.1:7000@17000 myself,master - 0 0 0 connected\n" +
		otherID + " 127.0.0.1:7001@17001 master - 0 0 0 connected\n")
	if err != nil {
		t.Fatal(err)
	}
	c.opts.RequireFullCoverage = true
	for _, tc := range []struct {
		ranges []SlotRange
		del    bool
		want   string
	}{
		{[]SlotRange{{0, 1}, {1, 1}}, false, "ERR Slot 1 specified multiple times"},
		{[]SlotRange{{5, 5}}, true, "ERR Slot 5 is already unassigned"},
	} {
		var err error
		if tc.del {
			err = c.DelSlots(tc.ranges)
		} else {
			err = c.AddSlots(tc.ranges)
		}
		if err == nil || err.Error() != tc.want {
			t.Fatalf("%v: %v, want %s", tc.ranges, err, tc.want)
		}
	}
	if c.Slot(0).Assigned {
		t.Fatal("a refused ADDSLOTS assigned slots")
	}
	if err := c.AddSlots([]SlotRange{{0, 16383}}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddSlots([]SlotRange{{7, 7}}); err == nil || err.Error() != "ERR Slot 7 is already busy" {
		t.Fatalf("busy: %v", err)
	}
	keys := map[int]int{100: 1}
	count := func(slot int) int { return keys[slot] }
	if err := c.SetSlot(100, "NODE", otherID, count); err == nil || !strings.Contains(err.Error(), "while I still hold keys") {
		t.Fatalf("SETSLOT NODE with keys: %v", err)
	}
	if err := c.SetSlot(200, "NODE", "nope", count); err == nil || err.Error() != "ERR Unknown node nope" {
		t.Fatalf("unknown node: %v", err)
	}
	if err := c.SetSlot(200, "MIGRATING", otherID, count); err != nil {
		t.Fatal(err)
	}
	if st := c.Slot(200); !st.Mine || !st.MigratingSet || st.Migrating.ID != otherID {
		t.Fatalf("migrating: %+v", st)
	}
	if err := c.SetSlot(200, "NODE", otherID, count); err != nil {
		t.Fatal(err)
	}
	if st := c.Slot(200); st.Mine || st.MigratingSet || st.Owner.ID != otherID {
		t.Fatalf("after NODE: %+v", st)
	}
	if err := c.SetSlot(200, "MIGRATING", otherID, count); err == nil || err.Error() != "ERR I'm not the owner of hash slot 200" {
		t.Fatalf("migrating a foreign slot: %v", err)
	}
	if err := c.SetSlot(200, "IMPORTING", otherID, count); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSlot(200, "NODE", c.MyID(), count); err != nil {
		t.Fatal(err)
	}
	if st := c.Slot(200); !st.Mine || st.Importing {
		t.Fatalf("after import: %+v", st)
	}
	if !strings.Contains(c.Info(), "cluster_my_epoch:1\r\n") || !strings.Contains(c.Info(), "cluster_current_epoch:1\r\n") {
		t.Fatalf("closing an import must bump the epoch:\n%s", c.Info())
	}
	if bumped, epoch := c.BumpEpoch(); bumped || epoch != 1 {
		t.Fatalf("BUMPEPOCH with the highest epoch: %v %d", bumped, epoch)
	}
	if err := c.SetConfigEpoch(5); err == nil {
		t.Fatal("SET-CONFIG-EPOCH with other nodes known")
	}
	c.Reset(true)
	info := c.Info()
	for _, want := range []string{"cluster_state:fail\r\n", "cluster_known_nodes:1\r\n", "cluster_slots_assigned:0\r\n", "cluster_current_epoch:0\r\n"} {
		if !strings.Contains(info, want) {
			t.Fatalf("after RESET HARD lacks %q:\n%s", want, info)
		}
	}
	if c.MyID() == "0000000000000000000000000000000000000001" {
		t.Fatal("RESET HARD kept the node ID")
	}
	if err := c.SetConfigEpoch(5); err != nil {
		t.Fatal(err)
	}
	if err := c.SetConfigEpoch(6); err == nil || err.Error() != "ERR Node config epoch is already non-zero" {
		t.Fatalf("second SET-CONFIG-EPOCH: %v", err)
	}
	if bumped, epoch := c.BumpEpoch(); bumped || epoch != 5 {
		t.Fatalf("BUMPEPOCH: %v %d", bumped, epoch)
	}
}

func TestClaimSlot(t *testing.T) {
	c, err := Parse("0000000000000000000000000000000000000001 127.0.0.1:7000@17000 myself,master - 0 0 0 connected\n" +
		otherID + " 127.0.0.1:7001@17001 master - 0 0 0 connected 10\n")
	if err != nil {
		t.Fatal(err)
	}
	c.ClaimSlot(5)
	c.ClaimSlot(10)
	if !c.Slot(5).Mine || !c.Slot(10).Importing || c.Slot(10).Mine {
		t.Fatalf("%+v %+v", c.Slot(5), c.Slot(10))
	}
}
