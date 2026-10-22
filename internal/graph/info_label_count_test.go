package graph

import "testing"

// Info reports labels and relations as len(NodeLabel)/len(EdgeLabel), so an
// emptied label or relation must lose its map key rather than be left holding
// an empty slice. Otherwise GRAPH.INFO permanently over-reports both counts.
func TestInfoDropsLabelWhenLastNodeDeleted(t *testing.T) {
	g := NewGraph("test")
	g.AddNode("person", nil)
	g.AddNode("city", nil)

	if got := g.Info()["labels"]; got != 2 {
		t.Fatalf("precondition: expected 2 labels, got %v", got)
	}

	if !g.DeleteNode(1) {
		t.Fatal("expected DeleteNode to return true")
	}
	if !g.DeleteNode(2) {
		t.Fatal("expected DeleteNode to return true")
	}

	info := g.Info()
	if got := info["nodes"]; got != 0 {
		t.Errorf("expected 0 nodes, got %v", got)
	}
	if got := info["labels"]; got != 0 {
		t.Errorf("expected 0 labels after deleting every node, got %v", got)
	}
}

func TestInfoDropsRelationWhenLastEdgeDeleted(t *testing.T) {
	g := NewGraph("test")
	a := g.AddNode("person", nil)
	b := g.AddNode("city", nil)
	first, err := g.AddEdge(a.ID, b.ID, "lives_in", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := g.AddEdge(b.ID, a.ID, "knows", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := g.Info()["relations"]; got != 2 {
		t.Fatalf("precondition: expected 2 relations, got %v", got)
	}

	if !g.DeleteEdge(first.ID) {
		t.Fatal("expected DeleteEdge to return true")
	}
	// The other relation must still be counted while an edge of it remains.
	if got := g.Info()["relations"]; got != 1 {
		t.Errorf("expected 1 relation after deleting one of two, got %v", got)
	}

	if !g.DeleteEdge(second.ID) {
		t.Fatal("expected DeleteEdge to return true")
	}
	info := g.Info()
	if got := info["edges"]; got != 0 {
		t.Errorf("expected 0 edges, got %v", got)
	}
	if got := info["relations"]; got != 0 {
		t.Errorf("expected 0 relations after deleting every edge, got %v", got)
	}
	// Edge deletion must leave node state alone.
	if got := info["nodes"]; got != 2 {
		t.Errorf("expected 2 nodes unaffected by edge deletion, got %v", got)
	}
}

func TestInfoKeepsLabelWithSurvivingNode(t *testing.T) {
	g := NewGraph("test")
	g.AddNode("person", nil)
	g.AddNode("person", nil)
	g.AddNode("city", nil)

	g.DeleteNode(1)

	info := g.Info()
	if got := info["nodes"]; got != 2 {
		t.Errorf("expected 2 nodes, got %v", got)
	}
	if got := info["labels"]; got != 2 {
		t.Errorf("expected 2 labels while nodes of both labels remain, got %v", got)
	}
}

func TestInfoKeepsLabelReusedAfterFullDeletion(t *testing.T) {
	g := NewGraph("test")
	g.AddNode("person", nil)
	g.DeleteNode(1)
	g.AddNode("person", nil)

	info := g.Info()
	if got := info["nodes"]; got != 1 {
		t.Errorf("expected 1 node, got %v", got)
	}
	if got := info["labels"]; got != 1 {
		t.Errorf("expected 1 label after re-adding a previously emptied label, got %v", got)
	}
}
