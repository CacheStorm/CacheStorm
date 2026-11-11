package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// QUEUE and STACK are lists in the keyspace: their contents survive a
// restart through the AOF, belong to the selected database and go with
// FLUSHALL. They were process-wide maps, shared by every database and lost
// on restart.
func TestQueuesAndStacksAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	bulk := func(args ...string) string {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return string(v.Bulk)
	}
	for _, args := range [][]string{
		{"QUEUE.CREATE", "jobs"},
		{"QUEUE.PUSH", "jobs", "a"}, {"QUEUE.PUSH", "jobs", "b"}, {"QUEUE.PUSH", "jobs", "c"},
		{"QUEUE.POP", "jobs"},
		{"STACK.PUSH", "undo", "x"}, {"STACK.PUSH", "undo", "y"},
		{"QUEUE.PUSH", "gone", "z"}, {"QUEUE.CLEAR", "gone"},
		{"SELECT", "2"}, {"QUEUE.PUSH", "jobs", "db2"}, {"SELECT", "0"},
	} {
		bulk(args...)
	}
	if got := c.do("TYPE", "jobs"); got.Str != "list" {
		t.Fatalf("TYPE jobs: %v", got)
	}
	if got := c.do("SET", "plain", "v"); got.Str != "OK" {
		t.Fatal(got)
	}
	if got := c.do("QUEUE.PUSH", "plain", "x"); got.Type != '-' {
		t.Fatalf("QUEUE.PUSH on a string: %v", got)
	}
	stopReplServer(s)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	if got := bulk("QUEUE.PEEK", "jobs"); got != "b" {
		t.Fatalf("QUEUE.PEEK after restart: %q", got)
	}
	if got := c.do("QUEUE.LEN", "jobs"); got.Int != 2 {
		t.Fatalf("QUEUE.LEN after restart: %v", got)
	}
	if got := bulk("STACK.POP", "undo"); got != "y" {
		t.Fatalf("STACK.POP after restart: %q", got)
	}
	if got := c.do("EXISTS", "gone"); got.Int != 0 {
		t.Fatal("a cleared queue came back")
	}
	bulk("SELECT", "2")
	if got := bulk("QUEUE.POP", "jobs"); got != "db2" {
		t.Fatalf("database 2 queue: %q", got)
	}
	bulk("FLUSHALL")
	bulk("SELECT", "0")
	if got := c.do("QUEUE.LEN", "jobs"); got.Int != 0 {
		t.Fatalf("QUEUE.LEN after FLUSHALL: %v", got)
	}
}

// Queue and stack writes are recorded as the list commands they amount to,
// so Redis can load the AOF and a server started with extension-commands
// none still loads it. Commands written while a namespace was served replay
// after it is turned off.
func TestQueueAOFUsesListCommands(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Server.ExtensionCommands = "all"
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	for _, args := range [][]string{
		{"QUEUE.PUSH", "q", "a", "b"}, {"QUEUE.POP", "q"},
		{"STACK.PUSH", "s", "x", "y"}, {"STACK.POP", "s"},
		{"QUEUE.PUSH", "gone", "z"}, {"QUEUE.CLEAR", "gone"},
		{"LOCK.TRY", "l", "h", "t", "60000"},
		{"RATELIMITER.CREATE", "rl", "5", "60000"},
	} {
		if got := c.do(args...); got.Type == '-' {
			t.Fatalf("%v: %v", args, got.Err)
		}
	}
	stopReplServer(s)
	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"QUEUE.", "STACK.", "LOCK."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands:\n%q", name, data)
		}
	}

	cfg.Server.ExtensionCommands = "none"
	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	for key, want := range map[string]string{"q": "b", "s": "x"} {
		if got := c.do("LRANGE", key, "0", "-1"); len(got.Array) != 1 || string(got.Array[0].Bulk) != want {
			t.Fatalf("LRANGE %s: %+v", key, got)
		}
	}
	if got := c.do("EXISTS", "gone", "l"); got.Int != 1 {
		t.Fatalf("EXISTS gone l: %v", got)
	}
	if got := c.do("RATELIMITER.STATUS", "rl"); got.Type != '-' || !strings.Contains(got.Err, "unknown command") {
		t.Fatalf("RATELIMITER.STATUS under none: %v", got)
	}
}
