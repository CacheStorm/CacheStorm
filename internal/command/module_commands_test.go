package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllModuleCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterModuleCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MODULE LIST", "MODULE", [][]byte{[]byte("LIST")}},
		{"MODULE LOAD", "MODULE", [][]byte{[]byte("LOAD"), []byte("/path/to/module.so")}},
		{"MODULE UNLOAD", "MODULE", [][]byte{[]byte("UNLOAD"), []byte("mymodule")}},
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
