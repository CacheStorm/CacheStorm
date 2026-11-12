package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// STATEM, CHAINED, FLAG, COUNTER and ID are keys: they survive a restart
// through an AOF holding only Redis commands.
func TestUtilityNamespacesAreKeys(t *testing.T) {
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
	ok("STATEM.CREATE", "order", "new")
	ok("STATEM.ADDSTATE", "order", "paid")
	ok("STATEM.ADDSTATE", "order", "shipped", "TRUE")
	ok("STATEM.ADDTRANS", "order", "new", "paid", "pay")
	ok("STATEM.ADDTRANS", "order", "paid", "shipped", "ship")
	ok("STATEM.ADDTRANS", "order", "new", "new", "cancel")
	if got := ok("STATEM.TRIGGER", "order", "pay"); got != "paid" {
		t.Fatalf("STATEM.TRIGGER = %q", got)
	}
	ok("CHAINED.SET", "cfg", "a.b", "1")
	ok("FLAG.CREATE", "beta", "new UI")
	ok("FLAG.ENABLE", "beta")
	ok("FLAG.ADDVARIANT", "beta", "color", "blue")
	ok("FLAG.CREATE", "off")
	ok("COUNTER.INCRBY", "hits", "41")
	ok("COUNTER.INCR", "hits")
	ok("COUNTER.SET", "other", "5")
	ok("COUNTER.RESETALL")
	ok("COUNTER.INCR", "hits")
	ok("ID.CREATE", "inv", "100", "5", "PREFIX", "INV-", "PADDING", "6")
	if got := ok("ID.NEXT", "inv"); got != "INV-000100" {
		t.Fatalf("ID.NEXT = %q", got)
	}
	ok("ID.NEXTN", "inv", "2")

	ok("SET", "plain", "v")
	for _, args := range [][]string{{"STATEM.TRIGGER", "plain", "x"}, {"FLAG.ENABLE", "plain"}, {"COUNTER.INCR", "plain"},
		{"ID.NEXT", "plain"}, {"CHAINED.SET", "plain", "a", "b"}, {"COUNTER.INCR", "beta"}} {
		if r := c.do(args...); r.Type != '-' || !strings.HasPrefix(r.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, r)
		}
	}
	for _, args := range [][]string{{"STATEM.TRIGGER", "order", "pay"}, {"COUNTER.INCRBY", "hits", "x"},
		{"ID.CREATE", "i2", "1", "x"}, {"ID.CREATE", "i3", "1", "1", "PADDING", "-1"}, {"FLAG.ADDVARIANT", "nope", "k", "v"}} {
		if r := c.do(args...); r.Type != '-' {
			t.Fatalf("%v accepted: %v", args, r)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"STATEM.", "CHAINED.", "FLAG.", "COUNTER.", "ID."} {
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
	expect("paid", "STATEM.CURRENT", "order")
	expect("ship", "STATEM.EVENTS", "order")
	expect("shipped", "STATEM.TRIGGER", "order", "ship")
	expect("1", "STATEM.ISFINAL", "order")
	expect("order", "STATEM.LIST")
	expect("1", "CHAINED.GET", "cfg", "a.b")
	expect("1", "FLAG.ISENABLED", "beta")
	expect("beta", "FLAG.LISTENABLED")
	expect("beta,off", "FLAG.LIST")
	expect("blue", "FLAG.GETVARIANT", "beta", "color")
	expect("hits,1,other,0", "COUNTER.GETALL")
	expect("INV-000110,110", "ID.CURRENT", "inv")
	expect("INV-000115", "ID.NEXT", "inv")
}
