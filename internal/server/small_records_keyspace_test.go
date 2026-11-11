package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// THRESHOLD, SWITCH, BOOKMARK, DEADLINE, ROUTE, GATEWAY, GRID, GHOST, TAPE,
// SLICE and ROLLUPX objects are keys: they survive a restart through an AOF
// holding only hash commands.
func TestSmallRecordsAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("THRESHOLD.SET", "cpu", "80")
	ok("SWITCH.ON", "feature")
	ok("SWITCH.TOGGLE", "other")
	ok("SWITCH.TOGGLE", "other")
	ok("BOOKMARK.SET", "bm", "page-7")
	ok("DEADLINE.SET", "job", "3600000", "notify")
	ok("ROUTE.ADD", "web", "/api/*", "api")
	ok("ROUTE.ADD", "web", "/api/health", "health")
	ok("ROUTE.ADD", "web", "/old", "gone")
	ok("ROUTE.REMOVE", "web", "/old")
	gw := c.do("GATEWAY.CREATE", "edge")
	ok("GATEWAY.ROUTE", string(gw.Bulk), "/x", "svc")
	ok("GRID.CREATE", "board", "3", "2")
	ok("GRID.SET", "board", "2", "1", "x")
	ok("GRID.SET", "board", "0", "0", "o")
	ok("GHOST.CREATE", "g")
	ok("GHOST.WRITE", "g", "a")
	ok("GHOST.WRITE", "g", "b")
	ok("TAPE.CREATE", "t")
	ok("TAPE.WRITE", "t", "first")
	ok("TAPE.WRITE", "t", "second")
	ok("TAPE.SEEK", "t", "1")
	ok("SLICE.APPEND", "sl", "p", "q", "r")
	ok("ROLLUPX.CREATE", "r")
	ok("ROLLUPX.ADD", "r", "1")
	ok("ROLLUPX.ADD", "r", "2")
	ok("SET", "plain", "v")
	for _, args := range [][]string{
		{"THRESHOLD.SET", "plain", "1"}, {"SWITCH.ON", "cpu"}, {"BOOKMARK.GET", "feature"}, {"DEADLINE.CHECK", "bm"},
		{"ROUTE.ADD", "board", "a", "b"}, {"GRID.SET", "g", "0", "0", "v"}, {"GHOST.WRITE", "t", "v"},
		{"TAPE.READ", "sl"}, {"SLICE.APPEND", "r", "v"}, {"ROLLUPX.ADD", "web", "1"},
	} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	for _, args := range [][]string{
		{"GRID.SET", "board", "3", "0", "v"}, {"GRID.CREATE", "b2", "0", "1"}, {"TAPE.SEEK", "t", "5"},
		{"ROLLUPX.ADD", "r", "x"}, {"THRESHOLD.SET", "x", "y"}, {"DEADLINE.SET", "d", "-1"},
	} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"THRESHOLD.", "SWITCH.", "BOOKMARK.", "DEADLINE.", "ROUTE.", "GATEWAY.", "GRID.", "GHOST.", "TAPE.", "SLICE.", "ROLLUPX."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := renderReply(c.do(args...), ","); got != want {
			t.Fatalf("%v after restart = %q, want %q", args, got, want)
		}
	}
	expect("1", "THRESHOLD.CHECK", "cpu", "80")
	expect("0", "THRESHOLD.CHECK", "cpu", "79")
	expect("1", "SWITCH.STATE", "feature")
	expect("0", "SWITCH.STATE", "other")
	expect("[name feature state on],[name other state off]", "SWITCH.LIST")
	expect("page-7", "BOOKMARK.GET", "bm")
	expect("job", "DEADLINE.LIST")
	expect("health", "ROUTE.MATCH", "web", "/api/health")
	expect("api", "ROUTE.MATCH", "web", "/api/users")
	expect("[pattern /api/* target api],[pattern /api/health target health]", "ROUTE.LIST", "web")
	expect("[id "+string(gw.Bulk)+" name edge]", "GATEWAY.LIST")
	expect("x", "GRID.GET", "board", "2", "1")
	expect("[coord 0,0 value o],[coord 2,1 value x]", "GRID.QUERY", "board")
	expect("a,b", "GHOST.READ", "g")
	expect("second", "TAPE.READ", "t")
	expect("p,q,r", "SLICE.GET", "sl")
	expect("1.500000", "ROLLUPX.GET", "r")
	if got := c.do("DEADLINE.CHECK", "job"); got.Int <= 0 || got.Int > 3600000 {
		t.Fatalf("DEADLINE.CHECK after restart: %v", got)
	}
	ok("GRID.CLEAR", "board")
	expect("", "GRID.QUERY", "board")
	for _, args := range [][]string{
		{"THRESHOLD.DELETE", "cpu"}, {"BOOKMARK.DELETE", "bm"}, {"DEADLINE.CANCEL", "job"}, {"GATEWAY.DELETE", string(gw.Bulk)},
		{"GRID.DELETE", "board"}, {"GHOST.DELETE", "g"}, {"TAPE.DELETE", "t"}, {"SLICE.DELETE", "sl"}, {"ROLLUPX.DELETE", "r"},
		{"ROUTE.REMOVE", "web", "/api/*"}, {"ROUTE.REMOVE", "web", "/api/health"},
	} {
		if got := c.do(args...); got.Int != 1 {
			t.Fatalf("%v: %v", args, got)
		}
	}
	if got := c.do("DBSIZE"); got.Int != 3 {
		t.Fatalf("DBSIZE once the objects are deleted: %v (want plain and the two switches)", got)
	}
}

// renderReply writes a reply compactly: integers in decimal, bulk strings as
// they are, and nested arrays bracketed with their items space-separated.
func renderReply(v *resp.Value, sep string) string {
	switch v.Type {
	case ':':
		return strconv.FormatInt(v.Int, 10)
	case '*':
		parts := make([]string, len(v.Array))
		for i, e := range v.Array {
			parts[i] = renderReply(e, " ")
			if e.Type == '*' {
				parts[i] = "[" + parts[i] + "]"
			}
		}
		return strings.Join(parts, sep)
	case '$':
		return string(v.Bulk)
	}
	return v.Str + v.Err
}
