package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllScriptCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"EVAL hello", "EVAL", [][]byte{[]byte(`return "Hello World"`), []byte("0")}, nil},
		{"EVAL with keys", "EVAL", [][]byte{[]byte(`return KEYS[1]`), []byte("1"), []byte("mykey")}, nil},
		{"EVAL with args", "EVAL", [][]byte{[]byte(`return ARGV[1]`), []byte("0"), []byte("arg1")}, nil},
		{"EVALSHA", "EVALSHA", [][]byte{[]byte("abc123"), []byte("0")}, nil},
		{"SCRIPT EXISTS", "SCRIPT", [][]byte{[]byte("EXISTS"), []byte("abc123")}, nil},
		{"SCRIPT FLUSH", "SCRIPT", [][]byte{[]byte("FLUSH")}, nil},
		{"SCRIPT LOAD", "SCRIPT", [][]byte{[]byte("LOAD"), []byte(`return 1+1`)}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
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
