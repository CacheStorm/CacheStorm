package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExtraCommandsReplayFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLAYX.START", "REPLAYX.START", [][]byte{[]byte("log1")}},
		{"REPLAYX.START no args", "REPLAYX.START", nil},
		{"REPLAYX.STOP", "REPLAYX.STOP", [][]byte{[]byte("log1")}},
		{"REPLAYX.STOP not found", "REPLAYX.STOP", [][]byte{[]byte("notfound")}},
		{"REPLAYX.STOP no args", "REPLAYX.STOP", nil},
		{"REPLAYX.PAUSE", "REPLAYX.PAUSE", [][]byte{[]byte("log1")}},
		{"REPLAYX.PAUSE no args", "REPLAYX.PAUSE", nil},
		{"REPLAYX.SPEED", "REPLAYX.SPEED", [][]byte{[]byte("log1"), []byte("2.0")}},
		{"REPLAYX.SPEED no args", "REPLAYX.SPEED", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGridFinalCoverage(t *testing.T) {
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

func TestExtraCommandsTapeFinalCoverage(t *testing.T) {
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
		{"TAPE.READ", "TAPE.READ", [][]byte{[]byte("tape1"), []byte("100")}},
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

func TestExtraCommandsSliceFinalCoverage(t *testing.T) {
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

func TestExtraCommandsRageFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RAGE.TEST", "RAGE.TEST", [][]byte{[]byte("test1"), []byte("100")}},
		{"RAGE.TEST no args", "RAGE.TEST", nil},
		{"RAGE.STOP", "RAGE.STOP", [][]byte{[]byte("test1")}},
		{"RAGE.STOP no args", "RAGE.STOP", nil},
		{"RAGE.STATS", "RAGE.STATS", [][]byte{[]byte("test1")}},
		{"RAGE.STATS no args", "RAGE.STATS", nil},
		{"RAGE.RESET", "RAGE.RESET", [][]byte{[]byte("test1")}},
		{"RAGE.RESET no args", "RAGE.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsBookmarkFinalCoverage(t *testing.T) {
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
		{"BOOKMARK.GET not found", "BOOKMARK.GET", [][]byte{[]byte("notfound")}},
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

func TestExtraCommandsSwitchFinalCoverage(t *testing.T) {
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

func TestExtraCommandsThresholdFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"THRESHOLD.SET", "THRESHOLD.SET", [][]byte{[]byte("metric1"), []byte("100")}},
		{"THRESHOLD.SET no args", "THRESHOLD.SET", nil},
		{"THRESHOLD.CHECK", "THRESHOLD.CHECK", [][]byte{[]byte("metric1"), []byte("50")}},
		{"THRESHOLD.CHECK no args", "THRESHOLD.CHECK", nil},
		{"THRESHOLD.LIST", "THRESHOLD.LIST", nil},
		{"THRESHOLD.DELETE", "THRESHOLD.DELETE", [][]byte{[]byte("metric1")}},
		{"THRESHOLD.DELETE no args", "THRESHOLD.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsGatewayFinalCoverage(t *testing.T) {
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

func TestExtraCommandsRouteFinalCoverage(t *testing.T) {
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

func TestExtraCommandsProbeFinalCoverage(t *testing.T) {
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

func TestExtraCommandsCanaryFinalCoverage(t *testing.T) {
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

func TestExtraCommandsGhostFinalCoverage(t *testing.T) {
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

func TestExtraCommandsBeaconFinalCoverage(t *testing.T) {
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

func TestExtraCommandsRollupFinalCoverage(t *testing.T) {
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
