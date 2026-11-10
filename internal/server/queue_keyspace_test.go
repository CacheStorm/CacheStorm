package server

import (
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
