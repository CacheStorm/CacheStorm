package graph

import (
	"fmt"
	"strings"
	"testing"
)

func sample() *Snapshot {
	nodes := []*Node{
		{ID: 1, Label: "Person", Properties: map[string]string{"name": "Alice", "age": "34"}},
		{ID: 2, Label: "Person", Properties: map[string]string{"name": "Bob", "age": "27"}},
		{ID: 3, Label: "Person", Properties: map[string]string{"name": "Carol", "age": "41"}},
		{ID: 4, Label: "City", Properties: map[string]string{"name": "Istanbul"}},
	}
	edges := []*Edge{
		{ID: 1, From: 1, To: 2, Relation: "KNOWS", Properties: map[string]string{"since": "2015"}},
		{ID: 2, From: 2, To: 3, Relation: "KNOWS", Properties: map[string]string{"since": "2020"}},
		{ID: 3, From: 1, To: 4, Relation: "LIVES_IN", Properties: map[string]string{}},
		{ID: 4, From: 3, To: 4, Relation: "LIVES_IN", Properties: map[string]string{}},
		{ID: 5, From: 9, To: 1, Relation: "DANGLING", Properties: map[string]string{}},
	}
	return NewSnapshot(nodes, edges)
}

// render writes a result as "col|col; v|v; ..." with nodes and edges by id.
func render(r *Result) string {
	var rows []string
	rows = append(rows, strings.Join(r.Columns, "|"))
	for _, row := range r.Rows {
		var cells []string
		for _, v := range row {
			cells = append(cells, show(v))
		}
		rows = append(rows, strings.Join(cells, "|"))
	}
	return strings.Join(rows, "; ")
}

func show(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case *Node:
		return fmt.Sprintf("(%d)", x.ID)
	case *Edge:
		return fmt.Sprintf("[%d]", x.ID)
	case []any:
		var parts []string
		for _, it := range x {
			parts = append(parts, show(it))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case float64:
		return FormatFloat(x)
	}
	return fmt.Sprint(v)
}

func TestQueries(t *testing.T) {
	g := sample()
	for _, tc := range []struct{ q, want string }{
		{"MATCH (n) RETURN n", "n; (1); (2); (3); (4)"},
		{"MATCH (n:Person) RETURN n.name ORDER BY n.name DESC", "n.name; Carol; Bob; Alice"},
		{"MATCH (n:Person {name: 'Bob'}) RETURN id(n), n.age", "id(n)|n.age; 2|27"},
		{"MATCH (a)-[:KNOWS]->(b) RETURN a.name, b.name", "a.name|b.name; Alice|Bob; Bob|Carol"},
		{"MATCH (a)<-[r:KNOWS]-(b) RETURN a.name, b.name, r.since", "a.name|b.name|r.since; Bob|Alice|2015; Carol|Bob|2020"},
		{"MATCH (a:Person)-[:KNOWS]-(b) WHERE a.name = 'Bob' RETURN b.name ORDER BY b.name", "b.name; Alice; Carol"},
		{"MATCH (a)-[:KNOWS]->(b)-[:KNOWS]->(c) RETURN a.name, c.name", "a.name|c.name; Alice|Carol"},
		{"MATCH (p:Person)-[:LIVES_IN]->(c:City) RETURN c.name, count(p) AS people", "c.name|people; Istanbul|2"},
		{"MATCH (p:Person) WHERE p.age > 30 RETURN p.name ORDER BY p.age", "p.name; Alice; Carol"},
		{"MATCH (p:Person) RETURN sum(p.age), avg(p.age), min(p.age), max(p.name)", "sum(p.age)|avg(p.age)|min(p.age)|max(p.name); 102|34|27|Carol"},
		{"MATCH (p:Person) WHERE p.name STARTS WITH 'A' OR p.name ENDS WITH 'l' RETURN p.name", "p.name; Alice; Carol"},
		{"MATCH (p:Person) WHERE p.name IN ['Bob', 'Zed'] RETURN p.name", "p.name; Bob"},
		{"MATCH (p:Person) WHERE NOT p.name CONTAINS 'o' RETURN p.name", "p.name; Alice"},
		{"MATCH (p:Person) RETURN p.name ORDER BY p.age DESC SKIP 1 LIMIT 1", "p.name; Alice"},
		{"MATCH (n) RETURN DISTINCT labels(n)", "labels(n); [Person]; [City]"},
		{"MATCH (n:Nobody) RETURN count(*)", "count(*); 0"},
		{"MATCH (n:Nobody) RETURN n.name, count(*)", "n.name|count(*)"},
		{"MATCH ()-[r]->() RETURN type(r), count(*) AS c ORDER BY c DESC, type(r)", "type(r)|c; KNOWS|2; LIVES_IN|2"},
		{"MATCH (a)-[r]->(b) WHERE id(r) = 3 RETURN r, a, b", "r|a|b; [3]|(1)|(4)"},
		{"MATCH (p:Person) RETURN p.age + 1 AS next, toUpper(p.name) ORDER BY next LIMIT 1", "next|toUpper(p.name); 28|BOB"},
		{"MATCH (p:Person) WHERE p.missing IS NULL AND p.age >= 34 RETURN collect(p.name)", "collect(p.name); [Alice,Carol]"},
		{"MATCH (a:Person), (c:City) WHERE a.age < 30 RETURN a.name, c.name", "a.name|c.name; Bob|Istanbul"},
		{"MATCH (a {name: 'Alice'})-[:KNOWS]->(b) MATCH (b)-[:KNOWS]->(c) RETURN c.name", "c.name; Carol"},
		{"MATCH (a)-[r1]->(b)<-[r2]-(c) RETURN a.name, c.name", "a.name|c.name; Alice|Carol; Carol|Alice"},
		{"MATCH (n:Delete) RETURN n.set", "n.set"},
		{"MATCH (p:Person) RETURN count(DISTINCT p.age > 30)", "count(DISTINCT p.age > 30); 2"},
	} {
		r, err := Query(g, tc.q)
		if err != nil {
			t.Errorf("%s: %v", tc.q, err)
			continue
		}
		if got := render(r); got != tc.want {
			t.Errorf("%s\n got %s\nwant %s", tc.q, got, tc.want)
		}
	}
}

func TestQueryErrors(t *testing.T) {
	g := sample()
	for _, tc := range []struct{ q, want string }{
		{"CREATE (n:Person {name: 'x'})", "read queries"},
		{"MATCH (n) DELETE n", "read queries"},
		{"MATCH (n) SET n.x = 1 RETURN n", "read queries"},
		{"MATCH (n) RETURN m", "m not defined"},
		{"MATCH (n)-[*1..3]->(m) RETURN m", "variable-length"},
		{"MATCH (n) RETURN count(n) + 1", "whole RETURN item"},
		{"MATCH (n) RETURN nosuch(n)", "unknown function"},
		{"MATCH (n RETURN n", "expected ')'"},
		{"RETURN 1", "starts with MATCH"},
		{"MATCH (n)-[n]->(m) RETURN n", "both as a node"},
		{"MATCH (n) RETURN n.name, count(*) ORDER BY n.age", "must name a RETURN item"},
		{"MATCH (n) WHERE n.name = 'x RETURN n", "unterminated string"},
		{"MATCH (n) RETURN n LIMIT -1", "non-negative integer"},
		{"MATCH (a), (b), (c), (d), (e), (f), (g), (h), (i), (j) RETURN count(*)", "more than"},
	} {
		_, err := Query(g, tc.q)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.q, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one containing %q", tc.q, err, tc.want)
		}
	}
}

func TestQueryRowLimit(t *testing.T) {
	var nodes []*Node
	for i := 1; i <= 200; i++ {
		nodes = append(nodes, &Node{ID: uint64(i), Label: "N", Properties: map[string]string{}})
	}
	g := NewSnapshot(nodes, nil)
	if _, err := Query(g, "MATCH (a), (b), (c) RETURN count(*)"); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("an 8M-row product: %v", err)
	}
}

// Every prefix of a valid query parses or fails cleanly, without a panic.
func TestQueryPrefixesDoNotPanic(t *testing.T) {
	g := sample()
	for _, q := range []string{
		"MATCH (a:Person {name: 'Alice'})-[r:KNOWS|LIKES]->(b)<-[:X]-(c) WHERE a.age >= 3 AND NOT b.name IN ['x', \"y\"] OR c.v IS NOT NULL RETURN DISTINCT a.name AS n, count(DISTINCT b), toUpper(c.name) ORDER BY n DESC SKIP 1 LIMIT 2",
		"MATCH (n)--(m), (o) MATCH (p)-->(q) WHERE n.x STARTS WITH 'a' XOR -n.y * 2 % 3 / 1.5e2 < 4 RETURN collect(n), sum(n.y), [1, 2] + 3",
	} {
		rs := []rune(q)
		for i := 0; i <= len(rs); i++ {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("%q panicked: %v", string(rs[:i]), r)
					}
				}()
				_, _ = Query(g, string(rs[:i]))
			}()
		}
	}
}
