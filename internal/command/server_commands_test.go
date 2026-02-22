package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllServerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"PING", "PING", nil, nil},
		{"PING with message", "PING", [][]byte{[]byte("hello")}, nil},
		{"ECHO", "ECHO", [][]byte{[]byte("Hello World")}, nil},
		{"TIME", "TIME", nil, nil},
		{"DBSIZE", "DBSIZE", nil, func() {
			s.Set("key1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("v2")}, store.SetOptions{})
		}},
		{"FLUSHDB", "FLUSHDB", nil, func() {
			s.Set("flushkey1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
			s.Set("flushkey2", &store.StringValue{Data: []byte("v2")}, store.SetOptions{})
		}},
		{"FLUSHALL", "FLUSHALL", nil, func() {
			s.Set("allkey1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
		}},
		{"SWAPDB", "SWAPDB", [][]byte{[]byte("0"), []byte("1")}, nil},
		{"INFO", "INFO", nil, nil},
		{"INFO section", "INFO", [][]byte{[]byte("server")}, nil},
		{"COMMAND", "COMMAND", nil, nil},
		{"COMMAND COUNT", "COMMAND", [][]byte{[]byte("COUNT")}, nil},
		{"COMMAND INFO", "COMMAND", [][]byte{[]byte("INFO"), []byte("GET"), []byte("SET")}, nil},
		{"LOLWUT", "LOLWUT", nil, nil},
		{"MONITOR", "MONITOR", nil, nil},
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
