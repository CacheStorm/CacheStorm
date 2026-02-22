package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllConfigCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterConfigCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONFIG GET", "CONFIG", [][]byte{[]byte("GET"), []byte("*")}},
		{"CONFIG GET maxmemory", "CONFIG", [][]byte{[]byte("GET"), []byte("maxmemory")}},
		{"CONFIG SET", "CONFIG", [][]byte{[]byte("SET"), []byte("maxmemory"), []byte("100mb")}},
		{"CONFIG REWRITE", "CONFIG", [][]byte{[]byte("REWRITE")}},
		{"CONFIG RESETSTAT", "CONFIG", [][]byte{[]byte("RESETSTAT")}},
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
