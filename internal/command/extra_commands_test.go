package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExtraCommandsSWIMFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SWIM.JOIN", "SWIM.JOIN", [][]byte{[]byte("node1"), []byte("192.168.1.1:7946")}},
		{"SWIM.JOIN no args", "SWIM.JOIN", nil},
		{"SWIM.LEAVE", "SWIM.LEAVE", [][]byte{[]byte("node1")}},
		{"SWIM.LEAVE not found", "SWIM.LEAVE", [][]byte{[]byte("notfound")}},
		{"SWIM.LEAVE no args", "SWIM.LEAVE", nil},
		{"SWIM.MEMBERS empty", "SWIM.MEMBERS", nil},
		{"SWIM.MEMBERS with data", "SWIM.MEMBERS", nil},
		{"SWIM.PING", "SWIM.PING", [][]byte{[]byte("node1")}},
		{"SWIM.PING not found", "SWIM.PING", [][]byte{[]byte("notfound")}},
		{"SWIM.PING no args", "SWIM.PING", nil},
		{"SWIM.SUSPECT", "SWIM.SUSPECT", [][]byte{[]byte("node1")}},
		{"SWIM.SUSPECT not found", "SWIM.SUSPECT", [][]byte{[]byte("notfound")}},
		{"SWIM.SUSPECT no args", "SWIM.SUSPECT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "SWIM.MEMBERS with data" || tt.name == "SWIM.LEAVE" || tt.name == "SWIM.PING" || tt.name == "SWIM.SUSPECT" {
				swimMembersMu.Lock()
				swimMembers["node1"] = &SwimMember{ID: "node1", Addr: "192.168.1.1:7946", Status: "alive", LastSeen: 0, Incarnation: 0}
				swimMembersMu.Unlock()
			}
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGOSSIPFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GOSSIP.JOIN", "GOSSIP.JOIN", [][]byte{[]byte("node1"), []byte("192.168.1.1:7946")}},
		{"GOSSIP.JOIN no args", "GOSSIP.JOIN", nil},
		{"GOSSIP.LEAVE", "GOSSIP.LEAVE", [][]byte{[]byte("node1")}},
		{"GOSSIP.LEAVE not found", "GOSSIP.LEAVE", [][]byte{[]byte("notfound")}},
		{"GOSSIP.LEAVE no args", "GOSSIP.LEAVE", nil},
		{"GOSSIP.BROADCAST", "GOSSIP.BROADCAST", [][]byte{[]byte("message")}},
		{"GOSSIP.BROADCAST no args", "GOSSIP.BROADCAST", nil},
		{"GOSSIP.GET", "GOSSIP.GET", [][]byte{[]byte("key1")}},
		{"GOSSIP.GET not found", "GOSSIP.GET", [][]byte{[]byte("notfound")}},
		{"GOSSIP.GET no args", "GOSSIP.GET", nil},
		{"GOSSIP.MEMBERS empty", "GOSSIP.MEMBERS", nil},
		{"GOSSIP.MEMBERS with data", "GOSSIP.MEMBERS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "GOSSIP.MEMBERS with data" || tt.name == "GOSSIP.LEAVE" || tt.name == "GOSSIP.GET" {
				gossipMu.Lock()
				gossipMembers["node1"] = &GossipMember{ID: "node1", Addr: "192.168.1.1:7946"}
				gossipData["key1"] = "value1"
				gossipMu.Unlock()
			}
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsCRDTFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CRDT.LWW.SET", "CRDT.LWW.SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"CRDT.LWW.SET no args", "CRDT.LWW.SET", nil},
		{"CRDT.LWW.GET", "CRDT.LWW.GET", [][]byte{[]byte("key1")}},
		{"CRDT.LWW.GET not found", "CRDT.LWW.GET", [][]byte{[]byte("notfound")}},
		{"CRDT.LWW.GET no args", "CRDT.LWW.GET", nil},
		{"CRDT.LWW.DELETE", "CRDT.LWW.DELETE", [][]byte{[]byte("key1")}},
		{"CRDT.LWW.DELETE no args", "CRDT.LWW.DELETE", nil},
		{"CRDT.GCOUNTER.INCR", "CRDT.GCOUNTER.INCR", [][]byte{[]byte("counter1"), []byte("5")}},
		{"CRDT.GCOUNTER.INCR no args", "CRDT.GCOUNTER.INCR", nil},
		{"CRDT.GCOUNTER.GET", "CRDT.GCOUNTER.GET", [][]byte{[]byte("counter1")}},
		{"CRDT.GCOUNTER.GET not found", "CRDT.GCOUNTER.GET", [][]byte{[]byte("notfound")}},
		{"CRDT.GCOUNTER.GET no args", "CRDT.GCOUNTER.GET", nil},
		{"CRDT.PNCounter.INCR", "CRDT.PNCounter.INCR", [][]byte{[]byte("pcounter1"), []byte("5")}},
		{"CRDT.PNCounter.INCR no args", "CRDT.PNCounter.INCR", nil},
		{"CRDT.PNCounter.DECR", "CRDT.PNCounter.DECR", [][]byte{[]byte("pcounter1"), []byte("3")}},
		{"CRDT.PNCounter.DECR no args", "CRDT.PNCounter.DECR", nil},
		{"CRDT.PNCounter.GET", "CRDT.PNCounter.GET", [][]byte{[]byte("pcounter1")}},
		{"CRDT.PNCounter.GET not found", "CRDT.PNCounter.GET", [][]byte{[]byte("notfound")}},
		{"CRDT.PNCounter.GET no args", "CRDT.PNCounter.GET", nil},
		{"CRDT.GSET.ADD", "CRDT.GSET.ADD", [][]byte{[]byte("gset1"), []byte("item1")}},
		{"CRDT.GSET.ADD no args", "CRDT.GSET.ADD", nil},
		{"CRDT.GSET.GET", "CRDT.GSET.GET", [][]byte{[]byte("gset1")}},
		{"CRDT.GSET.GET not found", "CRDT.GSET.GET", [][]byte{[]byte("notfound")}},
		{"CRDT.GSET.GET no args", "CRDT.GSET.GET", nil},
		{"CRDT.ORSET.ADD", "CRDT.ORSET.ADD", [][]byte{[]byte("orset1"), []byte("item1")}},
		{"CRDT.ORSET.ADD no args", "CRDT.ORSET.ADD", nil},
		{"CRDT.ORSET.REMOVE", "CRDT.ORSET.REMOVE", [][]byte{[]byte("orset1"), []byte("item1")}},
		{"CRDT.ORSET.REMOVE no args", "CRDT.ORSET.REMOVE", nil},
		{"CRDT.ORSET.GET", "CRDT.ORSET.GET", [][]byte{[]byte("orset1")}},
		{"CRDT.ORSET.GET not found", "CRDT.ORSET.GET", [][]byte{[]byte("notfound")}},
		{"CRDT.ORSET.GET no args", "CRDT.ORSET.GET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsMERKLEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MERKLE.CREATE", "MERKLE.CREATE", [][]byte{[]byte("mtree1")}},
		{"MERKLE.CREATE no args", "MERKLE.CREATE", nil},
		{"MERKLE.ADD", "MERKLE.ADD", [][]byte{[]byte("mtree1"), []byte("data1")}},
		{"MERKLE.ADD no args", "MERKLE.ADD", nil},
		{"MERKLE.VERIFY", "MERKLE.VERIFY", [][]byte{[]byte("mtree1"), []byte("data1")}},
		{"MERKLE.VERIFY no args", "MERKLE.VERIFY", nil},
		{"MERKLE.PROOF", "MERKLE.PROOF", [][]byte{[]byte("mtree1"), []byte("data1")}},
		{"MERKLE.PROOF no args", "MERKLE.PROOF", nil},
		{"MERKLE.ROOT", "MERKLE.ROOT", [][]byte{[]byte("mtree1")}},
		{"MERKLE.ROOT no args", "MERKLE.ROOT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsRAFTFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RAFT.STATE", "RAFT.STATE", nil},
		{"RAFT.LEADER", "RAFT.LEADER", nil},
		{"RAFT.TERM", "RAFT.TERM", nil},
		{"RAFT.VOTE", "RAFT.VOTE", [][]byte{[]byte("node1"), []byte("1")}},
		{"RAFT.VOTE no args", "RAFT.VOTE", nil},
		{"RAFT.APPEND", "RAFT.APPEND", [][]byte{[]byte("1"), []byte("data")}},
		{"RAFT.APPEND no args", "RAFT.APPEND", nil},
		{"RAFT.COMMIT", "RAFT.COMMIT", [][]byte{[]byte("1")}},
		{"RAFT.COMMIT no args", "RAFT.COMMIT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsSHARDFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SHARD.MAP", "SHARD.MAP", nil},
		{"SHARD.MOVE", "SHARD.MOVE", [][]byte{[]byte("1"), []byte("node2")}},
		{"SHARD.MOVE no args", "SHARD.MOVE", nil},
		{"SHARD.REBALANCE", "SHARD.REBALANCE", nil},
		{"SHARD.LIST", "SHARD.LIST", nil},
		{"SHARD.STATUS", "SHARD.STATUS", [][]byte{[]byte("1")}},
		{"SHARD.STATUS no args", "SHARD.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsCOMPRESSIONFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COMPRESSION.COMPRESS", "COMPRESSION.COMPRESS", [][]byte{[]byte("testdata")}},
		{"COMPRESSION.COMPRESS no args", "COMPRESSION.COMPRESS", nil},
		{"COMPRESSION.DECOMPRESS", "COMPRESSION.DECOMPRESS", [][]byte{[]byte("compresseddata")}},
		{"COMPRESSION.DECOMPRESS no args", "COMPRESSION.DECOMPRESS", nil},
		{"COMPRESSION.INFO", "COMPRESSION.INFO", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsDEDUPFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEDUP.ADD", "DEDUP.ADD", [][]byte{[]byte("item1")}},
		{"DEDUP.ADD no args", "DEDUP.ADD", nil},
		{"DEDUP.CHECK", "DEDUP.CHECK", [][]byte{[]byte("item1")}},
		{"DEDUP.CHECK no args", "DEDUP.CHECK", nil},
		{"DEDUP.EXPIRE", "DEDUP.EXPIRE", [][]byte{[]byte("1000")}},
		{"DEDUP.EXPIRE no args", "DEDUP.EXPIRE", nil},
		{"DEDUP.CLEAR", "DEDUP.CLEAR", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsBATCHFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCH.SUBMIT", "BATCH.SUBMIT", [][]byte{[]byte("cmd1"), []byte("arg1")}},
		{"BATCH.SUBMIT no args", "BATCH.SUBMIT", nil},
		{"BATCH.STATUS", "BATCH.STATUS", [][]byte{[]byte("batch1")}},
		{"BATCH.STATUS no args", "BATCH.STATUS", nil},
		{"BATCH.CANCEL", "BATCH.CANCEL", [][]byte{[]byte("batch1")}},
		{"BATCH.CANCEL no args", "BATCH.CANCEL", nil},
		{"BATCH.LIST", "BATCH.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsDEADLINEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEADLINE.SET", "DEADLINE.SET", [][]byte{[]byte("task1"), []byte("5000")}},
		{"DEADLINE.SET no args", "DEADLINE.SET", nil},
		{"DEADLINE.CHECK", "DEADLINE.CHECK", [][]byte{[]byte("task1")}},
		{"DEADLINE.CHECK no args", "DEADLINE.CHECK", nil},
		{"DEADLINE.CANCEL", "DEADLINE.CANCEL", [][]byte{[]byte("task1")}},
		{"DEADLINE.CANCEL no args", "DEADLINE.CANCEL", nil},
		{"DEADLINE.LIST", "DEADLINE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsSANITIZEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SANITIZE.STRING", "SANITIZE.STRING", [][]byte{[]byte("test<script>alert(1)</script>")}},
		{"SANITIZE.STRING no args", "SANITIZE.STRING", nil},
		{"SANITIZE.HTML", "SANITIZE.HTML", [][]byte{[]byte("<p>Hello</p><script>alert(1)</script>")}},
		{"SANITIZE.HTML no args", "SANITIZE.HTML", nil},
		{"SANITIZE.JSON", "SANITIZE.JSON", [][]byte{[]byte(`{"key":"value"}`)}},
		{"SANITIZE.JSON no args", "SANITIZE.JSON", nil},
		{"SANITIZE.SQL", "SANITIZE.SQL", [][]byte{[]byte("SELECT * FROM users WHERE id = 1")}},
		{"SANITIZE.SQL no args", "SANITIZE.SQL", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsMASKFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MASK.CARD", "MASK.CARD", [][]byte{[]byte("1234567890123456")}},
		{"MASK.CARD no args", "MASK.CARD", nil},
		{"MASK.EMAIL", "MASK.EMAIL", [][]byte{[]byte("test@example.com")}},
		{"MASK.EMAIL no args", "MASK.EMAIL", nil},
		{"MASK.PHONE", "MASK.PHONE", [][]byte{[]byte("+1234567890")}},
		{"MASK.PHONE no args", "MASK.PHONE", nil},
		{"MASK.IP", "MASK.IP", [][]byte{[]byte("192.168.1.1")}},
		{"MASK.IP no args", "MASK.IP", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGATEWAYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GATEWAY.CREATE", "GATEWAY.CREATE", [][]byte{[]byte("gw1"), []byte("http://localhost:8080")}},
		{"GATEWAY.CREATE no args", "GATEWAY.CREATE", nil},
		{"GATEWAY.DELETE", "GATEWAY.DELETE", [][]byte{[]byte("gw1")}},
		{"GATEWAY.DELETE no args", "GATEWAY.DELETE", nil},
		{"GATEWAY.ROUTE", "GATEWAY.ROUTE", [][]byte{[]byte("gw1"), []byte("/api")}},
		{"GATEWAY.ROUTE no args", "GATEWAY.ROUTE", nil},
		{"GATEWAY.LIST", "GATEWAY.LIST", nil},
		{"GATEWAY.METRICS", "GATEWAY.METRICS", [][]byte{[]byte("gw1")}},
		{"GATEWAY.METRICS no args", "GATEWAY.METRICS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsTHRESHOLDFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"THRESHOLD.SET", "THRESHOLD.SET", [][]byte{[]byte("cpu"), []byte("80")}},
		{"THRESHOLD.SET no args", "THRESHOLD.SET", nil},
		{"THRESHOLD.CHECK", "THRESHOLD.CHECK", [][]byte{[]byte("cpu"), []byte("75")}},
		{"THRESHOLD.CHECK no args", "THRESHOLD.CHECK", nil},
		{"THRESHOLD.LIST", "THRESHOLD.LIST", nil},
		{"THRESHOLD.DELETE", "THRESHOLD.DELETE", [][]byte{[]byte("cpu")}},
		{"THRESHOLD.DELETE no args", "THRESHOLD.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsSWITCHFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SWITCH.STATE", "SWITCH.STATE", [][]byte{[]byte("switch1")}},
		{"SWITCH.STATE no args", "SWITCH.STATE", nil},
		{"SWITCH.TOGGLE", "SWITCH.TOGGLE", [][]byte{[]byte("switch1")}},
		{"SWITCH.TOGGLE no args", "SWITCH.TOGGLE", nil},
		{"SWITCH.ON", "SWITCH.ON", [][]byte{[]byte("switch1")}},
		{"SWITCH.ON no args", "SWITCH.ON", nil},
		{"SWITCH.OFF", "SWITCH.OFF", [][]byte{[]byte("switch1")}},
		{"SWITCH.OFF no args", "SWITCH.OFF", nil},
		{"SWITCH.LIST", "SWITCH.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsBOOKMARKFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BOOKMARK.SET", "BOOKMARK.SET", [][]byte{[]byte("bm1"), []byte("/path/to/key")}},
		{"BOOKMARK.SET no args", "BOOKMARK.SET", nil},
		{"BOOKMARK.GET", "BOOKMARK.GET", [][]byte{[]byte("bm1")}},
		{"BOOKMARK.GET no args", "BOOKMARK.GET", nil},
		{"BOOKMARK.DELETE", "BOOKMARK.DELETE", [][]byte{[]byte("bm1")}},
		{"BOOKMARK.DELETE no args", "BOOKMARK.DELETE", nil},
		{"BOOKMARK.LIST", "BOOKMARK.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsROUTEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ROUTE.ADD", "ROUTE.ADD", [][]byte{[]byte("/api"), []byte("handler1")}},
		{"ROUTE.ADD no args", "ROUTE.ADD", nil},
		{"ROUTE.REMOVE", "ROUTE.REMOVE", [][]byte{[]byte("/api")}},
		{"ROUTE.REMOVE no args", "ROUTE.REMOVE", nil},
		{"ROUTE.MATCH", "ROUTE.MATCH", [][]byte{[]byte("/api/users")}},
		{"ROUTE.MATCH no args", "ROUTE.MATCH", nil},
		{"ROUTE.LIST", "ROUTE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGHOSTFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GHOST.CREATE", "GHOST.CREATE", [][]byte{[]byte("ghost1")}},
		{"GHOST.CREATE no args", "GHOST.CREATE", nil},
		{"GHOST.WRITE", "GHOST.WRITE", [][]byte{[]byte("ghost1"), []byte("data")}},
		{"GHOST.WRITE no args", "GHOST.WRITE", nil},
		{"GHOST.READ", "GHOST.READ", [][]byte{[]byte("ghost1")}},
		{"GHOST.READ no args", "GHOST.READ", nil},
		{"GHOST.DELETE", "GHOST.DELETE", [][]byte{[]byte("ghost1")}},
		{"GHOST.DELETE no args", "GHOST.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsPROBEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PROBE.CREATE", "PROBE.CREATE", [][]byte{[]byte("probe1"), []byte("http://localhost:8080")}},
		{"PROBE.CREATE no args", "PROBE.CREATE", nil},
		{"PROBE.DELETE", "PROBE.DELETE", [][]byte{[]byte("probe1")}},
		{"PROBE.DELETE no args", "PROBE.DELETE", nil},
		{"PROBE.RUN", "PROBE.RUN", [][]byte{[]byte("probe1")}},
		{"PROBE.RUN no args", "PROBE.RUN", nil},
		{"PROBE.RESULTS", "PROBE.RESULTS", [][]byte{[]byte("probe1")}},
		{"PROBE.RESULTS no args", "PROBE.RESULTS", nil},
		{"PROBE.LIST", "PROBE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsCANARYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CANARY.CREATE", "CANARY.CREATE", [][]byte{[]byte("canary1"), []byte("10")}},
		{"CANARY.CREATE no args", "CANARY.CREATE", nil},
		{"CANARY.DELETE", "CANARY.DELETE", [][]byte{[]byte("canary1")}},
		{"CANARY.DELETE no args", "CANARY.DELETE", nil},
		{"CANARY.CHECK", "CANARY.CHECK", [][]byte{[]byte("canary1")}},
		{"CANARY.CHECK no args", "CANARY.CHECK", nil},
		{"CANARY.STATUS", "CANARY.STATUS", [][]byte{[]byte("canary1")}},
		{"CANARY.STATUS no args", "CANARY.STATUS", nil},
		{"CANARY.LIST", "CANARY.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGRIDFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GRID.CREATE", "GRID.CREATE", [][]byte{[]byte("grid1"), []byte("10"), []byte("10")}},
		{"GRID.CREATE no args", "GRID.CREATE", nil},
		{"GRID.SET", "GRID.SET", [][]byte{[]byte("grid1"), []byte("0"), []byte("0"), []byte("value")}},
		{"GRID.SET no args", "GRID.SET", nil},
		{"GRID.GET", "GRID.GET", [][]byte{[]byte("grid1"), []byte("0"), []byte("0")}},
		{"GRID.GET no args", "GRID.GET", nil},
		{"GRID.DELETE", "GRID.DELETE", [][]byte{[]byte("grid1"), []byte("0"), []byte("0")}},
		{"GRID.DELETE no args", "GRID.DELETE", nil},
		{"GRID.QUERY", "GRID.QUERY", [][]byte{[]byte("grid1"), []byte("0"), []byte("0"), []byte("5"), []byte("5")}},
		{"GRID.QUERY no args", "GRID.QUERY", nil},
		{"GRID.CLEAR", "GRID.CLEAR", [][]byte{[]byte("grid1")}},
		{"GRID.CLEAR no args", "GRID.CLEAR", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsTAPEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TAPE.CREATE", "TAPE.CREATE", [][]byte{[]byte("tape1")}},
		{"TAPE.CREATE no args", "TAPE.CREATE", nil},
		{"TAPE.WRITE", "TAPE.WRITE", [][]byte{[]byte("tape1"), []byte("data")}},
		{"TAPE.WRITE no args", "TAPE.WRITE", nil},
		{"TAPE.READ", "TAPE.READ", [][]byte{[]byte("tape1"), []byte("10")}},
		{"TAPE.READ no args", "TAPE.READ", nil},
		{"TAPE.SEEK", "TAPE.SEEK", [][]byte{[]byte("tape1"), []byte("0")}},
		{"TAPE.SEEK no args", "TAPE.SEEK", nil},
		{"TAPE.DELETE", "TAPE.DELETE", [][]byte{[]byte("tape1")}},
		{"TAPE.DELETE no args", "TAPE.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsSLICEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SLICE.CREATE", "SLICE.CREATE", [][]byte{[]byte("slice1"), []byte("1024")}},
		{"SLICE.CREATE no args", "SLICE.CREATE", nil},
		{"SLICE.APPEND", "SLICE.APPEND", [][]byte{[]byte("slice1"), []byte("data")}},
		{"SLICE.APPEND no args", "SLICE.APPEND", nil},
		{"SLICE.GET", "SLICE.GET", [][]byte{[]byte("slice1")}},
		{"SLICE.GET no args", "SLICE.GET", nil},
		{"SLICE.DELETE", "SLICE.DELETE", [][]byte{[]byte("slice1")}},
		{"SLICE.DELETE no args", "SLICE.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsROLLUPXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ROLLUPX.CREATE", "ROLLUPX.CREATE", [][]byte{[]byte("rollup1"), []byte("3600000")}},
		{"ROLLUPX.CREATE no args", "ROLLUPX.CREATE", nil},
		{"ROLLUPX.ADD", "ROLLUPX.ADD", [][]byte{[]byte("rollup1"), []byte("100")}},
		{"ROLLUPX.ADD no args", "ROLLUPX.ADD", nil},
		{"ROLLUPX.GET", "ROLLUPX.GET", [][]byte{[]byte("rollup1")}},
		{"ROLLUPX.GET no args", "ROLLUPX.GET", nil},
		{"ROLLUPX.DELETE", "ROLLUPX.DELETE", [][]byte{[]byte("rollup1")}},
		{"ROLLUPX.DELETE no args", "ROLLUPX.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsBEACONFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BEACON.START", "BEACON.START", [][]byte{[]byte("beacon1"), []byte("5000")}},
		{"BEACON.START no args", "BEACON.START", nil},
		{"BEACON.STOP", "BEACON.STOP", [][]byte{[]byte("beacon1")}},
		{"BEACON.STOP no args", "BEACON.STOP", nil},
		{"BEACON.LIST", "BEACON.LIST", nil},
		{"BEACON.CHECK", "BEACON.CHECK", [][]byte{[]byte("beacon1")}},
		{"BEACON.CHECK no args", "BEACON.CHECK", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
