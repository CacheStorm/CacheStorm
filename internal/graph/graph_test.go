package graph

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph("test")
	if g.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", g.Name)
	}
	if g.Nodes == nil {
		t.Error("Nodes map should be initialized")
	}
	if g.Edges == nil {
		t.Error("Edges map should be initialized")
	}
	if g.NodeLabel == nil {
		t.Error("NodeLabel map should be initialized")
	}
	if g.EdgeLabel == nil {
		t.Error("EdgeLabel map should be initialized")
	}
	if g.Adjacency == nil {
		t.Error("Adjacency map should be initialized")
	}
	if g.nextNode != 1 {
		t.Errorf("expected nextNode 1, got %d", g.nextNode)
	}
	if g.nextEdge != 1 {
		t.Errorf("expected nextEdge 1, got %d", g.nextEdge)
	}
}

func TestAddNode(t *testing.T) {
	g := NewGraph("test")

	node := g.AddNode("person", map[string]interface{}{"name": "Alice", "age": 30})
	if node.ID != 1 {
		t.Errorf("expected node ID 1, got %d", node.ID)
	}
	if node.Label != "person" {
		t.Errorf("expected label 'person', got '%s'", node.Label)
	}
	if node.Properties["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got '%v'", node.Properties["name"])
	}

	node2 := g.AddNode("person", map[string]interface{}{"name": "Bob"})
	if node2.ID != 2 {
		t.Errorf("expected node ID 2, got %d", node2.ID)
	}

	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
	}

	if len(g.NodeLabel["person"]) != 2 {
		t.Errorf("expected 2 nodes with 'person' label, got %d", len(g.NodeLabel["person"]))
	}
}

func TestGetNode(t *testing.T) {
	g := NewGraph("test")
	node := g.AddNode("person", map[string]interface{}{"name": "Alice"})

	retrieved, ok := g.GetNode(node.ID)
	if !ok {
		t.Error("expected to find node")
	}
	if retrieved.Properties["name"] != node.Properties["name"] {
		t.Errorf("expected name '%s', got '%s'", node.Properties["name"], retrieved.Properties["name"])
	}

	_, ok = g.GetNode(999)
	if ok {
		t.Error("should not find non-existent node")
	}
}

func TestDeleteNode(t *testing.T) {
	g := NewGraph("test")
	node := g.AddNode("person", map[string]interface{}{"name": "Alice"})

	if !g.DeleteNode(node.ID) {
		t.Error("expected delete to return true")
	}

	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}

	if g.DeleteNode(999) {
		t.Error("expected delete of non-existent node to return false")
	}
}

func TestDeleteNodeRemovesEdges(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)
	_, _ = g.AddEdge(node1.ID, node2.ID, "knows", nil)

	g.DeleteNode(node1.ID)

	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges after deleting node, got %d", g.EdgeCount())
	}
}

func TestAddEdge(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)

	edge, err := g.AddEdge(node1.ID, node2.ID, "knows", map[string]interface{}{"since": 2020})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge.ID != 1 {
		t.Errorf("expected edge ID 1, got %d", edge.ID)
	}
	if edge.From != node1.ID {
		t.Errorf("expected From %d, got %d", node1.ID, edge.From)
	}
	if edge.To != node2.ID {
		t.Errorf("expected To %d, got %d", node2.ID, edge.To)
	}
	if edge.Relation != "knows" {
		t.Errorf("expected relation 'knows', got '%s'", edge.Relation)
	}

	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}

	if len(g.Adjacency[node1.ID]) != 1 {
		t.Errorf("expected 1 adjacency for node1, got %d", len(g.Adjacency[node1.ID]))
	}
}

func TestAddEdgeNonExistentNode(t *testing.T) {
	g := NewGraph("test")

	_, err := g.AddEdge(1, 2, "knows", nil)
	if err == nil {
		t.Error("expected error for non-existent source node")
	}
}

func TestGetEdge(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)
	edge, _ := g.AddEdge(node1.ID, node2.ID, "knows", nil)

	retrieved, ok := g.GetEdge(edge.ID)
	if !ok {
		t.Error("expected to find edge")
	}
	if retrieved.ID != edge.ID {
		t.Errorf("expected edge ID %d, got %d", edge.ID, retrieved.ID)
	}

	_, ok = g.GetEdge(999)
	if ok {
		t.Error("should not find non-existent edge")
	}
}

func TestDeleteEdge(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)
	edge, _ := g.AddEdge(node1.ID, node2.ID, "knows", nil)

	if !g.DeleteEdge(edge.ID) {
		t.Error("expected delete to return true")
	}

	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}

	if g.DeleteEdge(999) {
		t.Error("expected delete of non-existent edge to return false")
	}
}

func TestQuery(t *testing.T) {
	g := NewGraph("test")
	result, err := g.Query("MATCH (n) RETURN n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected result")
	}
}

func TestQueryNodesAll(t *testing.T) {
	g := NewGraph("test")
	g.AddNode("person", map[string]interface{}{"name": "Alice"})
	g.AddNode("person", map[string]interface{}{"name": "Bob"})
	g.AddNode("city", map[string]interface{}{"name": "NYC"})

	nodes := g.QueryNodes("", nil)
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestQueryNodesByLabel(t *testing.T) {
	g := NewGraph("test")
	g.AddNode("person", nil)
	g.AddNode("person", nil)
	g.AddNode("city", nil)

	nodes := g.QueryNodes("person", nil)
	if len(nodes) != 2 {
		t.Errorf("expected 2 person nodes, got %d", len(nodes))
	}
}

func TestQueryNodesWithFilters(t *testing.T) {
	g := NewGraph("test")
	g.AddNode("person", map[string]interface{}{"name": "Alice", "age": 30})
	g.AddNode("person", map[string]interface{}{"name": "Bob", "age": 25})

	nodes := g.QueryNodes("person", map[string]interface{}{"name": "Alice"})
	if len(nodes) != 1 {
		t.Errorf("expected 1 node with filter, got %d", len(nodes))
	}
}

func TestQueryEdgesAll(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)
	node3 := g.AddNode("person", nil)
	g.AddEdge(node1.ID, node2.ID, "knows", nil)
	g.AddEdge(node2.ID, node3.ID, "follows", nil)

	edges := g.QueryEdges("")
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestQueryEdgesByRelation(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)
	node3 := g.AddNode("person", nil)
	g.AddEdge(node1.ID, node2.ID, "knows", nil)
	g.AddEdge(node2.ID, node3.ID, "follows", nil)

	edges := g.QueryEdges("knows")
	if len(edges) != 1 {
		t.Errorf("expected 1 edge with 'knows' relation, got %d", len(edges))
	}
}

func TestNeighbors(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", map[string]interface{}{"name": "Alice"})
	node2 := g.AddNode("person", map[string]interface{}{"name": "Bob"})
	node3 := g.AddNode("person", map[string]interface{}{"name": "Charlie"})
	g.AddEdge(node1.ID, node2.ID, "knows", nil)
	g.AddEdge(node1.ID, node3.ID, "knows", nil)

	neighbors := g.Neighbors(node1.ID, "")
	if len(neighbors) != 2 {
		t.Errorf("expected 2 neighbors, got %d", len(neighbors))
	}
}

func TestNeighborsWithRelation(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)
	node3 := g.AddNode("person", nil)
	g.AddEdge(node1.ID, node2.ID, "knows", nil)
	g.AddEdge(node1.ID, node3.ID, "follows", nil)

	neighbors := g.Neighbors(node1.ID, "knows")
	if len(neighbors) != 1 {
		t.Errorf("expected 1 neighbor with 'knows' relation, got %d", len(neighbors))
	}
}

func TestNeighborsNoNode(t *testing.T) {
	g := NewGraph("test")
	neighbors := g.Neighbors(999, "")
	if len(neighbors) != 0 {
		t.Errorf("expected 0 neighbors for non-existent node, got %d", len(neighbors))
	}
}

func TestNodeCount(t *testing.T) {
	g := NewGraph("test")
	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}
	g.AddNode("person", nil)
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
}

func TestEdgeCount(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("person", nil)

	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
	g.AddEdge(node1.ID, node2.ID, "knows", nil)
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestGraphInfo(t *testing.T) {
	g := NewGraph("test")
	node1 := g.AddNode("person", nil)
	node2 := g.AddNode("city", nil)
	g.AddEdge(node1.ID, node2.ID, "lives_in", nil)

	info := g.Info()
	if info["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", info["name"])
	}
	if info["nodes"] != 2 {
		t.Errorf("expected 2 nodes, got %v", info["nodes"])
	}
	if info["edges"] != 1 {
		t.Errorf("expected 1 edge, got %v", info["edges"])
	}
	if info["labels"] != 2 {
		t.Errorf("expected 2 labels, got %v", info["labels"])
	}
	if info["relations"] != 1 {
		t.Errorf("expected 1 relation, got %v", info["relations"])
	}
}

func TestGraphManager(t *testing.T) {
	gm := NewGraphManager()

	g1 := gm.Create("graph1")
	if g1 == nil {
		t.Error("expected graph to be created")
	}
	if g1.Name != "graph1" {
		t.Errorf("expected name 'graph1', got '%s'", g1.Name)
	}

	g1Again := gm.Create("graph1")
	if g1Again != g1 {
		t.Error("expected same graph instance")
	}

	gm.Create("graph2")
	list := gm.List()
	if len(list) != 2 {
		t.Errorf("expected 2 graphs, got %d", len(list))
	}

	retrieved, ok := gm.Get("graph1")
	if !ok {
		t.Error("expected to find graph1")
	}
	if retrieved != g1 {
		t.Error("expected same graph instance")
	}

	_, ok = gm.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent graph")
	}

	if !gm.Delete("graph1") {
		t.Error("expected delete to return true")
	}
	if gm.Delete("graph1") {
		t.Error("expected second delete to return false")
	}

	list = gm.List()
	if len(list) != 1 {
		t.Errorf("expected 1 graph after delete, got %d", len(list))
	}
}

func TestGetGraphManager(t *testing.T) {
	gm1 := GetGraphManager()
	gm2 := GetGraphManager()
	if gm1 != gm2 {
		t.Error("expected same global manager instance")
	}
}
