package server

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// EVALSHA and FCALL are persisted as themselves, so a restart must restore
// the script cache and the function libraries they name before replaying
// them; otherwise their writes are lost.
func TestAOFRestoresEvalshaAndFcallWrites(t *testing.T) {
	cfg := aofTxConfig(t)
	s, run := aofTxStart(t, cfg)

	loaded := run("SCRIPT", "LOAD", "return redis.call('SET', KEYS[1], ARGV[1])")
	parts := strings.Split(loaded, "\r\n")
	if len(parts) < 2 || len(parts[1]) != 40 {
		t.Fatalf("SCRIPT LOAD reply %q", loaded)
	}
	if got := run("EVALSHA", parts[1], "1", "viasha", "1"); got != "+OK\r\n" {
		t.Fatalf("EVALSHA reply %q", got)
	}
	lib := "#!lua name=aoflib\nredis.register_function('aofset', function(keys, args) return redis.call('SET', keys[1], args[1]) end)"
	if got := run("FUNCTION", "LOAD", lib); got != "$6\r\naoflib\r\n" {
		t.Fatalf("FUNCTION LOAD reply %q", got)
	}
	if got := run("FCALL", "aofset", "1", "viafn", "2"); got != "+OK\r\n" {
		t.Fatalf("FCALL reply %q", got)
	}
	s.aof.Stop()

	// Forget the in-process script cache and libraries, so only the AOF can
	// bring them back. ExecuteSilent skips the persistence hook.
	for _, args := range [][]string{{"SCRIPT", "FLUSH"}, {"FUNCTION", "FLUSH"}} {
		argv := [][]byte{[]byte(args[1])}
		if err := s.router.ExecuteSilent(command.NewContext(args[0], argv, s.store, nil)); err != nil {
			t.Fatal(err)
		}
	}

	restored, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"viasha": "1", "viafn": "2"} {
		entry, ok := restored.store.Get(key)
		if !ok {
			t.Fatalf("%s was not restored", key)
		}
		if sv, ok := entry.Value.(*store.StringValue); !ok || string(sv.Data) != want {
			t.Fatalf("%s restored as %#v, want %s", key, entry.Value, want)
		}
	}
}

func TestPersistedCommandScriptCacheSubcommands(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"SCRIPT", []string{"LOAD", "return 1"}, true},
		{"SCRIPT", []string{"flush"}, true},
		{"SCRIPT", []string{"EXISTS", "x"}, false},
		{"FUNCTION", []string{"LOAD", "code"}, true},
		{"FUNCTION", []string{"DELETE", "lib"}, true},
		{"FUNCTION", []string{"FLUSH"}, true},
		{"FUNCTION", []string{"RESTORE", "payload"}, true},
		{"FUNCTION", []string{"LIST"}, false},
		{"FUNCTION", []string{"STATS"}, false},
		{"EVAL", []string{"return 1", "0"}, true},
		{"GET", []string{"k"}, false},
	}
	for _, c := range cases {
		argv := make([][]byte, len(c.args))
		for i, a := range c.args {
			argv[i] = []byte(a)
		}
		if got := persistedCommand(c.cmd, argv); got != c.want {
			t.Errorf("persistedCommand(%s %v) = %v, want %v", c.cmd, c.args, got, c.want)
		}
	}
}
