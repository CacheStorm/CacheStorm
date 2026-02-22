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
