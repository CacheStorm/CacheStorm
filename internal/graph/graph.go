// Package graph holds the graph model the GRAPH commands keep in a hash key
// and the read-only Cypher subset GRAPH.QUERY runs over it.
package graph

import "sort"

// Node is a graph node: an id, one label and string properties.
type Node struct {
	ID         uint64
	Label      string
	Properties map[string]string
}

// Edge is a directed relationship between two nodes.
type Edge struct {
	ID         uint64
	From       uint64
	To         uint64
	Relation   string
	Properties map[string]string
}

// Snapshot is a whole graph as a query reads it.
type Snapshot struct {
	nodes map[uint64]*Node
	edges map[uint64]*Edge
	// order lists node ids and edge ids ascending; out and in list each
	// node's edges by id.
	nodeOrder []uint64
	out, in   map[uint64][]*Edge
	edgeOrder []uint64
}

// NewSnapshot indexes nodes and edges for querying. Edges whose endpoints
// are missing are left out.
func NewSnapshot(nodes []*Node, edges []*Edge) *Snapshot {
	s := &Snapshot{
		nodes: make(map[uint64]*Node, len(nodes)),
		edges: make(map[uint64]*Edge, len(edges)),
		out:   make(map[uint64][]*Edge),
		in:    make(map[uint64][]*Edge),
	}
	for _, n := range nodes {
		s.nodes[n.ID] = n
		s.nodeOrder = append(s.nodeOrder, n.ID)
	}
	sort.Slice(s.nodeOrder, func(i, j int) bool { return s.nodeOrder[i] < s.nodeOrder[j] })
	sorted := append([]*Edge(nil), edges...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, e := range sorted {
		if s.nodes[e.From] == nil || s.nodes[e.To] == nil {
			continue
		}
		s.edges[e.ID] = e
		s.edgeOrder = append(s.edgeOrder, e.ID)
		s.out[e.From] = append(s.out[e.From], e)
		s.in[e.To] = append(s.in[e.To], e)
	}
	return s
}

// Node returns the node with id.
func (s *Snapshot) Node(id uint64) *Node { return s.nodes[id] }

// Nodes returns the nodes in id order.
func (s *Snapshot) Nodes() []*Node {
	out := make([]*Node, len(s.nodeOrder))
	for i, id := range s.nodeOrder {
		out[i] = s.nodes[id]
	}
	return out
}

// Out returns the edges leaving id, in id order.
func (s *Snapshot) Out(id uint64) []*Edge { return s.out[id] }
