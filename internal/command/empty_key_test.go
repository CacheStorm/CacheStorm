package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The empty string is a key like any other in Redis: it is written, read,
// renamed and moved to, and holds every type.
func TestEmptyKeyIsAKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	RegisterHashCommands(router)
	RegisterSetCommands(router)
	RegisterServerCommands(router)
	for _, step := range []struct {
		cmd  string
		args []string
		want string
	}{
		{"SET", []string{"", "v"}, "+OK\r\n"},
		{"GET", []string{""}, "$1\r\nv\r\n"},
		{"MSET", []string{"", "w", "k", "x"}, "+OK\r\n"},
		{"MGET", []string{"", "k"}, "*2\r\n$1\r\nw\r\n$1\r\nx\r\n"},
		{"RENAME", []string{"", "r"}, "+OK\r\n"},
		{"RENAME", []string{"r", ""}, "+OK\r\n"},
		{"EXISTS", []string{""}, ":1\r\n"},
		{"DEL", []string{""}, ":1\r\n"},
		{"HSET", []string{"", "f", "v"}, ":1\r\n"},
		{"HGET", []string{"", "f"}, "$1\r\nv\r\n"},
		{"DEL", []string{""}, ":1\r\n"},
		{"SADD", []string{"s", "m"}, ":1\r\n"},
		{"SMOVE", []string{"s", "", "m"}, ":1\r\n"},
		{"SMEMBERS", []string{""}, "*1\r\n$1\r\nm\r\n"},
	} {
		if got := runCmd(t, s, router, step.cmd, step.args...); got != step.want {
			t.Fatalf("%s %q: %q, want %q", step.cmd, step.args, got, step.want)
		}
	}
}
