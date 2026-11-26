package server

import (
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

const (
	busyEval     = "BUSY Redis is busy running a script. You can only call SCRIPT KILL or SHUTDOWN NOSAVE."
	busyFunction = "BUSY Redis is busy running a script. You can only call FUNCTION KILL or SHUTDOWN NOSAVE."
)

// startScript sends a command running a script without waiting for its
// reply, and waits until it has run past busy-reply-threshold.
func startScript(t *testing.T, c *replClient, args ...string) {
	t.Helper()
	_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := c.conn.Write(encodeArgs(args...)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
}

func replyOf(t *testing.T, c *replClient) *resp.Value {
	t.Helper()
	v, err := c.rd.ReadValue()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// A script runs until it ends, however long, as in Redis; past
// busy-reply-threshold other clients get BUSY but for the commands Redis
// lets through, and SCRIPT KILL / FUNCTION KILL stop a script that has not
// written.
func TestBusyScriptX(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	admin.str("CONFIG", "SET", "busy-reply-threshold", "100")
	t.Cleanup(func() { dialRepl(t, s).do("CONFIG", "SET", "busy-reply-threshold", "5000") })
	runner, other := dialRepl(t, s), dialRepl(t, s)

	// A script longer than the threshold completes.
	startScript(t, runner, "EVAL", "local t = redis.call('TIME') local s = t[1]*1000000+t[2] repeat local n = redis.call('TIME') until n[1]*1000000+n[2]-s > 500000 return 'done'", "0")
	if got := other.do("GET", "k"); got.Type != resp.TypeError || got.Err != busyEval {
		t.Fatalf("GET while a script is busy = %s", str(got))
	}
	if got := replyOf(t, runner); str(got) != "done" {
		t.Fatalf("a script past the threshold ended with %s", str(got))
	}
	if got := other.do("GET", "k"); got.Type == resp.TypeError {
		t.Fatalf("GET after the script = %s", got.Err)
	}

	// SCRIPT KILL stops an EVAL; FUNCTION KILL and FUNCTION STATS refuse.
	startScript(t, runner, "EVAL", "local i = 0 while true do i = i + 1 end", "0")
	for _, cmd := range [][]string{{"FUNCTION", "KILL"}, {"FUNCTION", "STATS"}, {"PING"}, {"SHUTDOWN"}} {
		if got := other.do(cmd...); got.Type != resp.TypeError || got.Err != busyEval {
			t.Fatalf("%v while an EVAL is busy = %s", cmd, str(got))
		}
	}
	if got := other.str("MULTI"); got != "OK" {
		t.Fatalf("MULTI while busy = %s", got)
	}
	other.do("SET", "k", "v") // BUSY: the transaction fails at EXEC
	if got := admin.str("SCRIPT", "KILL"); got != "OK" {
		t.Fatalf("SCRIPT KILL = %s", got)
	}
	if got := replyOf(t, runner); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "ERR Script killed by user with SCRIPT KILL... script: ") {
		t.Fatalf("killed EVAL = %s", str(got))
	}
	if got := other.do("EXEC"); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "EXECABORT") {
		t.Fatalf("EXEC of a transaction a BUSY broke = %s", str(got))
	}
	if got := other.do("SCRIPT", "KILL"); got.Err != "NOTBUSY No scripts in execution right now." {
		t.Fatalf("SCRIPT KILL with none running = %s", str(got))
	}

	// FUNCTION STATS shows a running function; FUNCTION KILL stops it.
	admin.str("FUNCTION", "LOAD", "#!lua name=spin\nredis.register_function{function_name='spin', callback=function() local i = 0 while true do i = i + 1 end end, flags={'no-writes'}}")
	startScript(t, runner, "FCALL", "spin", "0")
	stats := other.str("FUNCTION", "STATS")
	if !strings.Contains(stats, "running_script [name spin command [FCALL spin 0] duration_ms ") {
		t.Fatalf("FUNCTION STATS while spin runs = %s", stats)
	}
	if got := other.do("SCRIPT", "KILL"); got.Err != busyFunction {
		t.Fatalf("SCRIPT KILL on a function = %s", str(got))
	}
	if got := other.str("FUNCTION", "KILL"); got != "OK" {
		t.Fatalf("FUNCTION KILL = %s", got)
	}
	if got := replyOf(t, runner); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "ERR Script killed by user with SCRIPT KILL... script: spin, on @user_function:") {
		t.Fatalf("killed function = %s", str(got))
	}

	// A script that has written cannot be killed; SHUTDOWN NOSAVE ends it.
	startScript(t, runner, "EVAL", "redis.call('SET', 'w', '1') local i = 0 while true do i = i + 1 end", "0")
	if got := other.do("SCRIPT", "KILL"); !strings.HasPrefix(got.Err, "UNKILLABLE Sorry the script already executed write commands") {
		t.Fatalf("SCRIPT KILL of a script that wrote = %s", str(got))
	}
	_ = other.conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := other.conn.Write(encodeArgs("SHUTDOWN", "NOSAVE")); err != nil {
		t.Fatal(err)
	}
	if _, err := other.rd.ReadValue(); err == nil {
		t.Fatal("SHUTDOWN NOSAVE answered instead of closing the connection")
	}
	select {
	case <-s.shutdownCh:
	case <-time.After(5 * time.Second):
		t.Fatal("SHUTDOWN NOSAVE did not ask the server to stop")
	}
}
