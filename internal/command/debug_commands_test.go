package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllDebugCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDebugCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"DEBUG OBJECT", "DEBUG", [][]byte{[]byte("OBJECT"), []byte("key1")}, func() {
			s.Set("key1", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"DEBUG SEGFAULT", "DEBUG", [][]byte{[]byte("SEGFAULT")}, nil},
		{"DEBUG RELOAD", "DEBUG", [][]byte{[]byte("RELOAD")}, nil},
		{"DEBUG OOM", "DEBUG", [][]byte{[]byte("OOM")}, nil},
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
