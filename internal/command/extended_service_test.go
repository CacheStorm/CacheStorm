package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExtendedCommandsSERVICEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SERVICE.REGISTER", "SERVICE.REGISTER", [][]byte{[]byte("myservice"), []byte("instance1"), []byte("localhost"), []byte("8080")}},
		{"SERVICE.REGISTER with weight", "SERVICE.REGISTER", [][]byte{[]byte("myservice"), []byte("instance2"), []byte("localhost"), []byte("8081"), []byte("50")}},
		{"SERVICE.REGISTER no args", "SERVICE.REGISTER", nil},
		{"SERVICE.DEREGISTER", "SERVICE.DEREGISTER", [][]byte{[]byte("myservice"), []byte("instance1")}},
		{"SERVICE.DEREGISTER not found", "SERVICE.DEREGISTER", [][]byte{[]byte("notfound"), []byte("instance1")}},
		{"SERVICE.DEREGISTER no args", "SERVICE.DEREGISTER", nil},
		{"SERVICE.DISCOVER", "SERVICE.DISCOVER", [][]byte{[]byte("myservice")}},
		{"SERVICE.DISCOVER not found", "SERVICE.DISCOVER", [][]byte{[]byte("notfound")}},
		{"SERVICE.DISCOVER no args", "SERVICE.DISCOVER", nil},
		{"SERVICE.HEARTBEAT", "SERVICE.HEARTBEAT", [][]byte{[]byte("myservice"), []byte("instance1")}},
		{"SERVICE.HEARTBEAT not found", "SERVICE.HEARTBEAT", [][]byte{[]byte("notfound"), []byte("instance1")}},
		{"SERVICE.HEARTBEAT no args", "SERVICE.HEARTBEAT", nil},
		{"SERVICE.LIST", "SERVICE.LIST", nil},
		{"SERVICE.WEIGHT", "SERVICE.WEIGHT", [][]byte{[]byte("myservice"), []byte("instance1"), []byte("75")}},
		{"SERVICE.WEIGHT not found", "SERVICE.WEIGHT", [][]byte{[]byte("notfound"), []byte("instance1"), []byte("75")}},
		{"SERVICE.WEIGHT no args", "SERVICE.WEIGHT", nil},
		{"SERVICE.TAGS", "SERVICE.TAGS", [][]byte{[]byte("myservice"), []byte("instance1"), []byte("tag1"), []byte("tag2")}},
		{"SERVICE.TAGS no args", "SERVICE.TAGS", nil},
		{"SERVICE.HEALTH", "SERVICE.HEALTH", [][]byte{[]byte("myservice")}},
		{"SERVICE.HEALTH no args", "SERVICE.HEALTH", nil},
		{"SERVICE.METRICS", "SERVICE.METRICS", [][]byte{[]byte("myservice")}},
		{"SERVICE.METRICS no args", "SERVICE.METRICS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "SERVICE.DEREGISTER" || tt.name == "SERVICE.HEARTBEAT" || tt.name == "SERVICE.WEIGHT" || tt.name == "SERVICE.TAGS" {
				servicesMu.Lock()
				if _, exists := services["myservice"]; !exists {
					services["myservice"] = make(map[string]*ServiceInstance)
				}
				services["myservice"]["instance1"] = &ServiceInstance{
					ID: "instance1", Name: "myservice", Address: "localhost", Port: 8080,
					Weight: 100, Tags: []string{}, Metadata: make(map[string]string),
				}
				servicesMu.Unlock()
			}
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsHEALTHXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HEALTHX.REGISTER", "HEALTHX.REGISTER", [][]byte{[]byte("check1"), []byte("http://localhost:8080/health")}},
		{"HEALTHX.REGISTER no args", "HEALTHX.REGISTER", nil},
		{"HEALTHX.CHECK", "HEALTHX.CHECK", [][]byte{[]byte("check1")}},
		{"HEALTHX.CHECK not found", "HEALTHX.CHECK", [][]byte{[]byte("notfound")}},
		{"HEALTHX.CHECK no args", "HEALTHX.CHECK", nil},
		{"HEALTHX.STATUS", "HEALTHX.STATUS", [][]byte{[]byte("check1")}},
		{"HEALTHX.STATUS not found", "HEALTHX.STATUS", [][]byte{[]byte("notfound")}},
		{"HEALTHX.STATUS no args", "HEALTHX.STATUS", nil},
		{"HEALTHX.LIST", "HEALTHX.LIST", nil},
		{"HEALTHX.UNREGISTER", "HEALTHX.UNREGISTER", [][]byte{[]byte("check1")}},
		{"HEALTHX.UNREGISTER no args", "HEALTHX.UNREGISTER", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsSENTINELXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SENTINELX.REGISTER", "SENTINELX.REGISTER", [][]byte{[]byte("master1"), []byte("127.0.0.1"), []byte("6379")}},
		{"SENTINELX.REGISTER no args", "SENTINELX.REGISTER", nil},
		{"SENTINELX.WATCH", "SENTINELX.WATCH", [][]byte{[]byte("master1")}},
		{"SENTINELX.WATCH not found", "SENTINELX.WATCH", [][]byte{[]byte("notfound")}},
		{"SENTINELX.WATCH no args", "SENTINELX.WATCH", nil},
		{"SENTINELX.UNWATCH", "SENTINELX.UNWATCH", [][]byte{[]byte("master1")}},
		{"SENTINELX.UNWATCH no args", "SENTINELX.UNWATCH", nil},
		{"SENTINELX.FAILOVER", "SENTINELX.FAILOVER", [][]byte{[]byte("master1")}},
		{"SENTINELX.FAILOVER no args", "SENTINELX.FAILOVER", nil},
		{"SENTINELX.ALERTS", "SENTINELX.ALERTS", [][]byte{[]byte("master1")}},
		{"SENTINELX.ALERTS not found", "SENTINELX.ALERTS", [][]byte{[]byte("notfound")}},
		{"SENTINELX.ALERTS no args", "SENTINELX.ALERTS", nil},
		{"SENTINELX.MASTERS", "SENTINELX.MASTERS", nil},
		{"SENTINELX.SLAVES", "SENTINELX.SLAVES", [][]byte{[]byte("master1")}},
		{"SENTINELX.SLAVES no args", "SENTINELX.SLAVES", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsREPLAYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLAY.START", "REPLAY.START", [][]byte{[]byte("log1")}},
		{"REPLAY.START no args", "REPLAY.START", nil},
		{"REPLAY.STOP", "REPLAY.STOP", [][]byte{[]byte("log1")}},
		{"REPLAY.STOP not found", "REPLAY.STOP", [][]byte{[]byte("notfound")}},
		{"REPLAY.STOP no args", "REPLAY.STOP", nil},
		{"REPLAY.PAUSE", "REPLAY.PAUSE", [][]byte{[]byte("log1")}},
		{"REPLAY.PAUSE no args", "REPLAY.PAUSE", nil},
		{"REPLAY.RESUME", "REPLAY.RESUME", [][]byte{[]byte("log1")}},
		{"REPLAY.RESUME no args", "REPLAY.RESUME", nil},
		{"REPLAY.STATUS", "REPLAY.STATUS", [][]byte{[]byte("log1")}},
		{"REPLAY.STATUS not found", "REPLAY.STATUS", [][]byte{[]byte("notfound")}},
		{"REPLAY.STATUS no args", "REPLAY.STATUS", nil},
		{"REPLAY.SPEED", "REPLAY.SPEED", [][]byte{[]byte("log1"), []byte("2.0")}},
		{"REPLAY.SPEED no args", "REPLAY.SPEED", nil},
		{"REPLAY.SEEK", "REPLAY.SEEK", [][]byte{[]byte("log1"), []byte("1000")}},
		{"REPLAY.SEEK no args", "REPLAY.SEEK", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsMEMOCACHEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MEMOCACHE.GET hit", "MEMOCACHE.GET", [][]byte{[]byte("key1")}},
		{"MEMOCACHE.GET miss", "MEMOCACHE.GET", [][]byte{[]byte("notfound")}},
		{"MEMOCACHE.GET no args", "MEMOCACHE.GET", nil},
		{"MEMOCACHE.SET", "MEMOCACHE.SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"MEMOCACHE.SET no args", "MEMOCACHE.SET", nil},
		{"MEMOCACHE.DELETE", "MEMOCACHE.DELETE", [][]byte{[]byte("key1")}},
		{"MEMOCACHE.DELETE no args", "MEMOCACHE.DELETE", nil},
		{"MEMOCACHE.CLEAR", "MEMOCACHE.CLEAR", nil},
		{"MEMOCACHE.STATS", "MEMOCACHE.STATS", nil},
		{"MEMOCACHE.WARM", "MEMOCACHE.WARM", [][]byte{[]byte("pattern*")}},
		{"MEMOCACHE.WARM no args", "MEMOCACHE.WARM", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsWSFullCoverage2(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WS.BROADCAST", "WS.BROADCAST", [][]byte{[]byte("message")}},
		{"WS.BROADCAST no args", "WS.BROADCAST", nil},
		{"WS.ROOM.BROADCAST", "WS.ROOM.BROADCAST", [][]byte{[]byte("room1"), []byte("message")}},
		{"WS.ROOM.BROADCAST no args", "WS.ROOM.BROADCAST", nil},
		{"WS.ROOM.LIST", "WS.ROOM.LIST", [][]byte{[]byte("room1")}},
		{"WS.ROOM.LIST no args", "WS.ROOM.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
