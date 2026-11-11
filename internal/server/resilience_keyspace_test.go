package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The resilience namespaces are keys: their objects survive a restart
// through an AOF holding only Redis commands, and the waiting commands wake
// when another client settles what they wait for.
func TestResilienceNamespacesAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) string {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return string(v.Bulk)
	}
	ok("BULKHEAD.CREATE", "bh", "1")
	ok("BULKHEAD.ACQUIRE", "bh")
	if got := c.do("BULKHEAD.ACQUIRE", "bh"); got.Int != 0 {
		t.Fatalf("a second BULKHEAD.ACQUIRE: %v", got)
	}
	ok("TELEMETRY.RECORD", "cpu", "1000", "0.5", "host", "a")
	prom := ok("PROMISE.CREATE")
	fut := ok("FUTURE.CREATE")
	ok("STREAMPROC.CREATE", "sp")
	ok("STREAMPROC.PUSH", "sp", "a")
	ok("STREAMPROC.PUSH", "sp", "b")
	ok("STREAMPROC.POP", "sp")
	ok("EVENTSOURCING.APPEND", "acct", "opened", "{}")
	ok("EVENTSOURCING.APPEND", "acct", "deposit", "10")
	ok("BACKPRESSURE.CREATE", "bp", "10", "2")
	ok("BACKPRESSURE.CHECK", "bp", "12")
	ok("THROTTLEX.CREATE", "tx", "3600000")
	ok("THROTTLEX.CHECK", "tx")
	ok("DEBOUNCEX.CREATE", "dx", "0")
	ok("DEBOUNCEX.CALL", "dx", "payload")
	ok("COALESCE.CREATE", "co")
	ok("COALESCE.ADD", "co", "first")
	ok("AGGREGATOR.CREATE", "ag", "max")
	ok("AGGREGATOR.ADD", "ag", "3")
	ok("AGGREGATOR.ADD", "ag", "9")
	ok("WINDOWX.CREATE", "wx", "2")
	for _, v := range []string{"1", "2", "3"} {
		ok("WINDOWX.ADD", "wx", v)
	}
	ok("JOINX.CREATE", "j")
	ok("JOINX.ADD", "j", "LEFT", "k", "l1")
	ok("JOINX.ADD", "j", "RIGHT", "k", "r1")
	ok("SHUFFLE.CREATE", "sh")
	ok("SHUFFLE.ADD", "sh", "only")
	ok("PARTITIONX.CREATE", "px", "2")
	part := c.do("PARTITIONX.ADD", "px", "key", "v").Int

	// A waiting PROMISE.AWAIT wakes when another client resolves it.
	got := make(chan string, 1)
	go func() {
		w := dialRepl(t, s)
		got <- string(w.do("PROMISE.AWAIT", prom, "5000").Bulk)
	}()
	time.Sleep(50 * time.Millisecond)
	ok("PROMISE.RESOLVE", prom, "done")
	select {
	case v := <-got:
		if v != "done" {
			t.Fatalf("PROMISE.AWAIT = %q", v)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("PROMISE.AWAIT did not wake")
	}
	ok("SET", "plain", "v")
	for _, args := range [][]string{{"BULKHEAD.ACQUIRE", "plain"}, {"STREAMPROC.PUSH", "plain", "x"}, {"AGGREGATOR.ADD", "sp", "1"}, {"JOINX.ADD", "co", "LEFT", "k", "v"}} {
		if r := c.do(args...); r.Type != '-' || !strings.HasPrefix(r.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, r)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"BULKHEAD.", "TELEMETRY.", "PROMISE.", "FUTURE.", "STREAMPROC.", "EVENTSOURCING.", "BACKPRESSURE.",
		"THROTTLEX.", "DEBOUNCEX.", "COALESCE.", "AGGREGATOR.", "WINDOWX.", "JOINX.", "SHUFFLE.", "PARTITIONX."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if r := renderReply(c.do(args...), ","); r != want {
			t.Fatalf("%v after restart = %q, want %q", args, r, want)
		}
	}
	expect("0", "BULKHEAD.ACQUIRE", "bh")
	expect("[timestamp 1000 value 0.500000]", "TELEMETRY.QUERY", "cpu")
	expect("done", "PROMISE.AWAIT", prom, "1")
	ok("FUTURE.COMPLETE", fut, "42")
	expect("42", "FUTURE.GET", fut)
	expect("b", "STREAMPROC.PEEK", "sp")
	expect("[version 2 type deposit data 10]", "EVENTSOURCING.REPLAY", "acct", "2")
	if r := c.do("BACKPRESSURE.STATUS", "bp"); string(r.Array[9].Bulk) != "true" {
		t.Fatalf("BACKPRESSURE.STATUS after restart: %+v", r)
	}
	expect("0", "THROTTLEX.CHECK", "tx")
	expect("payload", "DEBOUNCEX.FLUSH", "dx")
	expect("first", "COALESCE.GET", "co")
	expect("9.000000", "AGGREGATOR.GET", "ag")
	expect("2.000000,3.000000", "WINDOWX.GET", "wx")
	expect("[key k left l1 right r1]", "JOINX.GET", "j")
	expect("only", "SHUFFLE.GET", "sh")
	expect("v", "PARTITIONX.GET", "px", itoa64(part))
}
