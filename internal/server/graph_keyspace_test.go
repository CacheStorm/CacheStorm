package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A graph is a key: it survives a restart through an AOF holding only Redis
// commands, and GRAPH.QUERY reads it.
func TestGraphIsAKey(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("GRAPH.CREATE", "g")
	ok("GRAPH.ADDNODE", "g", "Person", "name", "Alice", "age", "34")
	ok("GRAPH.ADDNODE", "g", "Person", "name", "Bob", "age", "27")
	ok("GRAPH.ADDNODE", "g", "City", "name", "Istanbul")
	ok("GRAPH.ADDNODE", "g", "Person", "name", "Gone")
	ok("GRAPH.ADDEDGE", "g", "1", "2", "KNOWS", "since", "2015")
	ok("GRAPH.ADDEDGE", "g", "1", "3", "LIVES_IN")
	ok("GRAPH.ADDEDGE", "g", "4", "1", "KNOWS")
	if r := c.do("GRAPH.DELNODE", "g", "4"); r.Int != 1 {
		t.Fatalf("GRAPH.DELNODE: %+v", r)
	}

	ok("SET", "plain", "v")
	for _, args := range [][]string{{"GRAPH.ADDNODE", "plain", "X"}, {"GRAPH.QUERY", "plain", "MATCH (n) RETURN n"}, {"GRAPH.INFO", "plain"}} {
		if r := c.do(args...); r.Type != '-' || !strings.HasPrefix(r.Err, "WRONGTYPE") {
			t.Fatalf("%v: %+v", args, r)
		}
	}
	for _, args := range [][]string{{"GRAPH.ADDEDGE", "g", "1", "99", "KNOWS"}, {"GRAPH.GETNODE", "g", "x"}, {"GRAPH.ADDNODE", "g", "L", "odd"},
		{"GRAPH.QUERY", "g", "CREATE (n:X)"}, {"GRAPH.QUERY", "g", "MATCH (n) RETURN m"}, {"GRAPH.INFO", "nograph"}} {
		if r := c.do(args...); r.Type != '-' {
			t.Fatalf("%v accepted: %+v", args, r)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "GRAPH.") {
		t.Fatal("the AOF records GRAPH commands")
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
	expect("g", "GRAPH.LIST")
	expect("name,g,nodes,3,edges,2,labels,2,relations,2", "GRAPH.INFO", "g")
	expect("id,1,label,Person,properties,[age 34 name Alice]", "GRAPH.GETNODE", "g", "1")
	expect("[2 Person],[3 City]", "GRAPH.NEIGHBORS", "g", "1")
	expect("[2 Person]", "GRAPH.NEIGHBORS", "g", "1", "KNOWS")
	q := c.do("GRAPH.QUERY", "g", "MATCH (a:Person)-[r:KNOWS]->(b) RETURN a.name, b.name, r.since, id(b)")
	if len(q.Array) != 3 {
		t.Fatalf("GRAPH.QUERY: %+v", q)
	}
	if got := renderReply(q.Array[0], ","); got != "a.name,b.name,r.since,id(b)" {
		t.Fatalf("header %q", got)
	}
	if got := renderReply(q.Array[1], ","); got != "[Alice Bob 2015 2]" {
		t.Fatalf("rows %q", got)
	}
	expect("5", "GRAPH.ADDNODE", "g", "Person", "name", "Dave")
	expect("4", "GRAPH.ADDEDGE", "g", "5", "1", "KNOWS")
	ro := c.do("GRAPH.RO_QUERY", "g", "MATCH (p:Person) RETURN count(p) AS n")
	if got := renderReply(ro.Array[1], ","); got != "[3]" {
		t.Fatalf("GRAPH.RO_QUERY count %q", got)
	}
	expect("OK", "GRAPH.DELETE", "g")
	expect("0", "EXISTS", "g")
}
