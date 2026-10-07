package graph_test

import (
	"github.com/cachestorm/cachestorm/internal/graph"
	"reflect"
	"testing"
)

type auditResult struct {
	Name      string
	Want, Got any
}

func auditMust(err error) {
	if err != nil {
		panic(err)
	}
}
func auditCheck(t *testing.T, results []auditResult) {
	t.Helper()
	for _, result := range results {
		if !reflect.DeepEqual(result.Want, result.Got) {
			t.Errorf("%s: got %v, want %v", result.Name, result.Got, result.Want)
		}
	}
}
func Graph(verify bool) []auditResult {
	g := graph.NewGraph("g")
	a := g.AddNode("n", nil)
	b := g.AddNode("n", nil)
	_, err := g.AddEdge(a.ID, b.ID, "knows", nil)
	auditMust(err)
	results := []auditResult{{"control: one relation", 1, len(g.Neighbors(a.ID, "knows"))}}
	other, err := g.AddEdge(a.ID, b.ID, "follows", nil)
	auditMust(err)
	results = append(results, auditResult{"parallel edge of other relation", 1, len(g.Neighbors(a.ID, "knows"))})
	if verify {
		results = append(results, auditResult{"unfiltered parallel edges", 2, len(g.Neighbors(a.ID, ""))}, auditResult{"absent relation", 0, len(g.Neighbors(a.ID, "absent"))})
		g.DeleteEdge(other.ID)
		results = append(results, auditResult{"delete unrelated edge", 1, len(g.Neighbors(a.ID, "knows"))})
		_, err = g.AddEdge(a.ID, b.ID, "knows", nil)
		auditMust(err)
		results = append(results, auditResult{"two matching edges", 2, len(g.Neighbors(a.ID, "knows"))})
		g.DeleteNode(b.ID)
		results = append(results, auditResult{"delete target", 0, len(g.Neighbors(a.ID, "knows"))})
	}
	return results
}

func TestAuditGraphParallelRelationNeighbors(t *testing.T) { auditCheck(t, Graph(true)) }
