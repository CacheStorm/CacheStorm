package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllClusterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER NODES", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"CLUSTER INFO", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER SLOTS", "CLUSTER", [][]byte{[]byte("SLOTS")}},
		{"CLUSTER KEYSLOT", "CLUSTER", [][]byte{[]byte("KEYSLOT"), []byte("mykey")}},
		{"CLUSTER MEET", "CLUSTER", [][]byte{[]byte("MEET"), []byte("127.0.0.1"), []byte("6379")}},
		{"CLUSTER REPLICATE", "CLUSTER", [][]byte{[]byte("REPLICATE"), []byte("node123")}},
		{"CLUSTER FAILOVER", "CLUSTER", [][]byte{[]byte("FAILOVER")}},
		{"CLUSTER FORGET", "CLUSTER", [][]byte{[]byte("FORGET"), []byte("node456")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			ctx := newTestContext(tt.cmd, tt.args, s)
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestClusterCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER COUNTKEYSINSLOT", "CLUSTER", [][]byte{[]byte("COUNTKEYSINSLOT"), []byte("100")}},
		{"CLUSTER COUNTKEYSINSLOT no args", "CLUSTER", [][]byte{[]byte("COUNTKEYSINSLOT")}},
		{"CLUSTER GETKEYSINSLOT", "CLUSTER", [][]byte{[]byte("GETKEYSINSLOT"), []byte("100"), []byte("10")}},
		{"CLUSTER GETKEYSINSLOT no args", "CLUSTER", [][]byte{[]byte("GETKEYSINSLOT")}},
		{"CLUSTER ADDSLOTS", "CLUSTER", [][]byte{[]byte("ADDSLOTS"), []byte("100")}},
		{"CLUSTER ADDSLOTS no args", "CLUSTER", [][]byte{[]byte("ADDSLOTS")}},
		{"CLUSTER DELSLOTS", "CLUSTER", [][]byte{[]byte("DELSLOTS"), []byte("100")}},
		{"CLUSTER DELSLOTS no args", "CLUSTER", [][]byte{[]byte("DELSLOTS")}},
		{"CLUSTER FLUSHSLOTS", "CLUSTER", [][]byte{[]byte("FLUSHSLOTS")}},
		{"CLUSTER MEET no args", "CLUSTER", [][]byte{[]byte("MEET")}},
		{"CLUSTER FORGET no args", "CLUSTER", [][]byte{[]byte("FORGET")}},
		{"CLUSTER REPLICATE no args", "CLUSTER", [][]byte{[]byte("REPLICATE")}},
		{"CLUSTER SLAVES", "CLUSTER", [][]byte{[]byte("SLAVES"), []byte("node1")}},
		{"CLUSTER SLAVES no args", "CLUSTER", [][]byte{[]byte("SLAVES")}},
		{"CLUSTER FAILOVER FORCE", "CLUSTER", [][]byte{[]byte("FAILOVER"), []byte("FORCE")}},
		{"CLUSTER FAILOVER TAKEOVER", "CLUSTER", [][]byte{[]byte("FAILOVER"), []byte("TAKEOVER")}},
		{"CLUSTER no args", "CLUSTER", nil},
		{"CLUSTER unknown subcommand", "CLUSTER", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
