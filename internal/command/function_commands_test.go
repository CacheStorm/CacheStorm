package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllFunctionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FUNCTION LOAD", "FUNCTION", [][]byte{[]byte("LOAD"), []byte(`#!lua name=mylib
redis.register_function('myfunc', function(keys, args) return 1 end)`)}},
		{"FUNCTION LIST", "FUNCTION", [][]byte{[]byte("LIST")}},
		{"FUNCTION FLUSH", "FUNCTION", [][]byte{[]byte("FLUSH")}},
		{"FUNCTION DELETE", "FUNCTION", [][]byte{[]byte("DELETE"), []byte("mylib")}},
		{"FUNCTION KILL", "FUNCTION", [][]byte{[]byte("KILL")}},
		{"FUNCTION STATS", "FUNCTION", [][]byte{[]byte("STATS")}},
		{"FCALL", "FCALL", [][]byte{[]byte("myfunc"), []byte("0")}},
		{"FCALL_RO", "FCALL_RO", [][]byte{[]byte("myfunc"), []byte("0")}},
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
