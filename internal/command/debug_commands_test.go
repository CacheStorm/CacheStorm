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

func TestDebugPopulate(t *testing.T) {
	r := newMemAcctRouter()
	RegisterDebugCommands(r)
	s := store.NewStore()
	run := func(args ...string) string { return memAcctRun(r, s, nil, args...) }
	run("SET", "key:0", "keep")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"DEBUG", "POPULATE", "3"}, "+OK\r\n"},
		{[]string{"DEBUG", "POPULATE", "2", "p", "10"}, "+OK\r\n"},
		{[]string{"DEBUG", "POPULATE", "2", "q", "3"}, "+OK\r\n"},
		{[]string{"DEBUG", "POPULATE", "x"}, "-ERR value is out of range, must be positive\r\n"},
		{[]string{"DEBUG", "populate"}, "-ERR unknown subcommand or wrong number of arguments for 'populate'. Try DEBUG HELP.\r\n"},
		{[]string{"GET", "key:0"}, "$4\r\nkeep\r\n"},
		{[]string{"GET", "key:2"}, "$7\r\nvalue:2\r\n"},
		{[]string{"GET", "p:1"}, "$10\r\nvalue:1\x00\x00\x00\r\n"},
		{[]string{"GET", "q:1"}, "$3\r\nval\r\n"},
		{[]string{"DBSIZE"}, ":7\r\n"},
	} {
		if got := run(c.args...); got != c.want {
			t.Fatalf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
}
