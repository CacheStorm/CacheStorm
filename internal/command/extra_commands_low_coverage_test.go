package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExtraCommandsAntiEntropyLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ANTI_ENTROPY.SYNC basic", "ANTI_ENTROPY.SYNC", [][]byte{[]byte("state1"), []byte("1")}},
		{"ANTI_ENTROPY.SYNC no args", "ANTI_ENTROPY.SYNC", nil},
		{"ANTI_ENTROPY.SYNC missing args", "ANTI_ENTROPY.SYNC", [][]byte{[]byte("state1")}},
		{"ANTI_ENTROPY.DIFF sync needed not found", "ANTI_ENTROPY.DIFF", [][]byte{[]byte("notfound"), []byte("1")}},
		{"ANTI_ENTROPY.DIFF sync needed version", "ANTI_ENTROPY.DIFF", [][]byte{[]byte("state1"), []byte("2")}},
		{"ANTI_ENTROPY.DIFF no sync needed", "ANTI_ENTROPY.DIFF", [][]byte{[]byte("state1"), []byte("1")}},
		{"ANTI_ENTROPY.DIFF no args", "ANTI_ENTROPY.DIFF", nil},
		{"ANTI_ENTROPY.MERGE update existing", "ANTI_ENTROPY.MERGE", [][]byte{[]byte("state1"), []byte("2")}},
		{"ANTI_ENTROPY.MERGE create new", "ANTI_ENTROPY.MERGE", [][]byte{[]byte("state2"), []byte("1")}},
		{"ANTI_ENTROPY.MERGE no args", "ANTI_ENTROPY.MERGE", nil},
		{"ANTI_ENTROPY.STATUS not found", "ANTI_ENTROPY.STATUS", [][]byte{[]byte("notfound")}},
		{"ANTI_ENTROPY.STATUS exists", "ANTI_ENTROPY.STATUS", [][]byte{[]byte("state1")}},
		{"ANTI_ENTROPY.STATUS no args", "ANTI_ENTROPY.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsRaftLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RAFT.STATE empty", "RAFT.STATE", nil},
		{"RAFT.LEADER empty", "RAFT.LEADER", nil},
		{"RAFT.TERM empty", "RAFT.TERM", nil},
		{"RAFT.VOTE no args", "RAFT.VOTE", nil},
		{"RAFT.VOTE candidate", "RAFT.VOTE", [][]byte{[]byte("node1"), []byte("1")}},
		{"RAFT.APPEND no args", "RAFT.APPEND", nil},
		{"RAFT.APPEND entry", "RAFT.APPEND", [][]byte{[]byte("1"), []byte("data")}},
		{"RAFT.COMMIT no args", "RAFT.COMMIT", nil},
		{"RAFT.COMMIT index", "RAFT.COMMIT", [][]byte{[]byte("1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGossipLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GOSSIP.BROADCAST no args", "GOSSIP.BROADCAST", nil},
		{"GOSSIP.BROADCAST message", "GOSSIP.BROADCAST", [][]byte{[]byte("test message")}},
		{"GOSSIP.JOIN no args", "GOSSIP.JOIN", nil},
		{"GOSSIP.JOIN node", "GOSSIP.JOIN", [][]byte{[]byte("node1"), []byte("192.168.1.1:7946")}},
		{"GOSSIP.LEAVE no args", "GOSSIP.LEAVE", nil},
		{"GOSSIP.LEAVE node", "GOSSIP.LEAVE", [][]byte{[]byte("node1")}},
		{"GOSSIP.GET no args", "GOSSIP.GET", nil},
		{"GOSSIP.GET key", "GOSSIP.GET", [][]byte{[]byte("key1")}},
		{"GOSSIP.MEMBERS empty", "GOSSIP.MEMBERS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsVectorClockLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"VECTOR_CLOCK.CREATE no args", "VECTOR_CLOCK.CREATE", nil},
		{"VECTOR_CLOCK.CREATE clock", "VECTOR_CLOCK.CREATE", [][]byte{[]byte("clock1")}},
		{"VECTOR_CLOCK.INCREMENT no args", "VECTOR_CLOCK.INCREMENT", nil},
		{"VECTOR_CLOCK.INCREMENT clock", "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("clock1"), []byte("node1")}},
		{"VECTOR_CLOCK.COMPARE no args", "VECTOR_CLOCK.COMPARE", nil},
		{"VECTOR_CLOCK.COMPARE missing args", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("clock1")}},
		{"VECTOR_CLOCK.COMPARE clocks", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("clock1"), []byte("clock2")}},
		{"VECTOR_CLOCK.MERGE no args", "VECTOR_CLOCK.MERGE", nil},
		{"VECTOR_CLOCK.MERGE clocks", "VECTOR_CLOCK.MERGE", [][]byte{[]byte("clock1"), []byte("clock2")}},
		{"VECTOR_CLOCK.GET not found", "VECTOR_CLOCK.GET", [][]byte{[]byte("notfound")}},
		{"VECTOR_CLOCK.GET no args", "VECTOR_CLOCK.GET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsShardLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SHARD.MAP no args", "SHARD.MAP", nil},
		{"SHARD.MAP key", "SHARD.MAP", [][]byte{[]byte("key1")}},
		{"SHARD.MOVE no args", "SHARD.MOVE", nil},
		{"SHARD.MOVE key", "SHARD.MOVE", [][]byte{[]byte("key1"), []byte("shard1")}},
		{"SHARD.REBALANCE no args", "SHARD.REBALANCE", nil},
		{"SHARD.LIST empty", "SHARD.LIST", nil},
		{"SHARD.STATUS empty", "SHARD.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGatewayLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GATEWAY.CREATE no args", "GATEWAY.CREATE", nil},
		{"GATEWAY.CREATE gateway", "GATEWAY.CREATE", [][]byte{[]byte("gw1")}},
		{"GATEWAY.DELETE no args", "GATEWAY.DELETE", nil},
		{"GATEWAY.DELETE not found", "GATEWAY.DELETE", [][]byte{[]byte("notfound")}},
		{"GATEWAY.ROUTE no args", "GATEWAY.ROUTE", nil},
		{"GATEWAY.ROUTE missing args", "GATEWAY.ROUTE", [][]byte{[]byte("gw1")}},
		{"GATEWAY.LIST empty", "GATEWAY.LIST", nil},
		{"GATEWAY.METRICS no args", "GATEWAY.METRICS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsThresholdLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"THRESHOLD.SET no args", "THRESHOLD.SET", nil},
		{"THRESHOLD.SET threshold", "THRESHOLD.SET", [][]byte{[]byte("th1"), []byte("100")}},
		{"THRESHOLD.CHECK no args", "THRESHOLD.CHECK", nil},
		{"THRESHOLD.CHECK threshold", "THRESHOLD.CHECK", [][]byte{[]byte("th1"), []byte("50")}},
		{"THRESHOLD.LIST empty", "THRESHOLD.LIST", nil},
		{"THRESHOLD.DELETE no args", "THRESHOLD.DELETE", nil},
		{"THRESHOLD.DELETE not found", "THRESHOLD.DELETE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsSwitchLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SWITCH.STATE no args", "SWITCH.STATE", nil},
		{"SWITCH.STATE switch", "SWITCH.STATE", [][]byte{[]byte("sw1")}},
		{"SWITCH.TOGGLE no args", "SWITCH.TOGGLE", nil},
		{"SWITCH.TOGGLE switch", "SWITCH.TOGGLE", [][]byte{[]byte("sw1")}},
		{"SWITCH.ON no args", "SWITCH.ON", nil},
		{"SWITCH.ON switch", "SWITCH.ON", [][]byte{[]byte("sw1")}},
		{"SWITCH.OFF no args", "SWITCH.OFF", nil},
		{"SWITCH.OFF switch", "SWITCH.OFF", [][]byte{[]byte("sw1")}},
		{"SWITCH.LIST empty", "SWITCH.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
