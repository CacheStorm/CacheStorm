package graph

import (
	"fmt"
	"sync"
)

type Node struct {
	ID         uint64
	Label      string
	Properties map[string]interface{}
}

type Edge struct {
	ID         uint64
	From       uint64
	To         uint64
	Relation   string
	Properties map[string]interface{}
}

type Graph struct {
	Name      string
	Nodes     map[uint64]*Node
	Edges     map[uint64]*Edge
	NodeLabel map[string][]uint64
	EdgeLabel map[string][]uint64
	Adjacency map[uint64][]uint64
	nextNode  uint64
	nextEdge  uint64
	mu        sync.RWMutex
}

func NewGraph(name string) *Graph {
	return &Graph{
		Name:      name,
		Nodes:     make(map[uint64]*Node),
		Edges:     make(map[uint64]*Edge),
		NodeLabel: make(map[string][]uint64),
		EdgeLabel: make(map[string][]uint64),
		Adjacency: make(map[uint64][]uint64),
		nextNode:  1,
		nextEdge:  1,
	}
}

func (g *Graph) AddNode(label string, props map[string]interface{}) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := &Node{
		ID:         g.nextNode,
		Label:      label,
		Properties: props,
	}
	g.nextNode++

	g.Nodes[node.ID] = node
	g.NodeLabel[label] = append(g.NodeLabel[label], node.ID)

	return node
}

func (g *Graph) GetNode(id uint64) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.Nodes[id]
	return node, ok
}

func (g *Graph) DeleteNode(id uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.Nodes[id]
	if !exists {
		return false
	}

	delete(g.Nodes, id)

	nodes := g.NodeLabel[node.Label]
	newNodes := make([]uint64, 0)
	for _, n := range nodes {
		if n != id {
			newNodes = append(newNodes, n)
		}
	}
	g.NodeLabel[node.Label] = newNodes

	for i, edgeID := range g.Edges {
		if edgeID.From == id || edgeID.To == id {
			delete(g.Edges, i)
		}
	}

	delete(g.Adjacency, id)

	return true
}

func (g *Graph) AddEdge(from, to uint64, relation string, props map[string]interface{}) (*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.Nodes[from]; !ok {
		return nil, fmt.Errorf("source node %d not found", from)
	}
	if _, ok := g.Nodes[to]; !ok {
		return nil, fmt.Errorf("target node %d not found", to)
	}

	edge := &Edge{
		ID:         g.nextEdge,
		From:       from,
		To:         to,
		Relation:   relation,
		Properties: props,
	}
	g.nextEdge++

	g.Edges[edge.ID] = edge
	g.EdgeLabel[relation] = append(g.EdgeLabel[relation], edge.ID)
	g.Adjacency[from] = append(g.Adjacency[from], to)

	return edge, nil
}

func (g *Graph) GetEdge(id uint64) (*Edge, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edge, ok := g.Edges[id]
	return edge, ok
}

func (g *Graph) DeleteEdge(id uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	edge, exists := g.Edges[id]
	if !exists {
		return false
	}

	delete(g.Edges, id)

	edges := g.EdgeLabel[edge.Relation]
	newEdges := make([]uint64, 0)
	for _, e := range edges {
		if e != id {
			newEdges = append(newEdges, e)
		}
	}
	g.EdgeLabel[edge.Relation] = newEdges

	return true
}

type QueryResult struct {
	Columns []string
	Data    [][]interface{}
}

func (g *Graph) Query(query string) (*QueryResult, error) {
	return &QueryResult{
		Columns: []string{},
		Data:    [][]interface{}{},
	}, nil
}

func (g *Graph) QueryNodes(label string, filters map[string]interface{}) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node

	if label == "" {
		for _, node := range g.Nodes {
			nodes = append(nodes, node)
		}
	} else {
		ids := g.NodeLabel[label]
		for _, id := range ids {
			if node, ok := g.Nodes[id]; ok {
				nodes = append(nodes, node)
			}
		}
	}

	if len(filters) == 0 {
		return nodes
	}

	result := make([]*Node, 0)
	for _, node := range nodes {
		match := true
		for k, v := range filters {
			if node.Properties[k] != v {
				match = false
				break
			}
		}
		if match {
			result = append(result, node)
		}
	}

	return result
}

func (g *Graph) QueryEdges(relation string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var edges []*Edge

	if relation == "" {
		for _, edge := range g.Edges {
			edges = append(edges, edge)
		}
	} else {
		ids := g.EdgeLabel[relation]
		for _, id := range ids {
			if edge, ok := g.Edges[id]; ok {
				edges = append(edges, edge)
			}
		}
	}

	return edges
}

func (g *Graph) Neighbors(nodeID uint64, relation string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	neighborIDs, ok := g.Adjacency[nodeID]
	if !ok {
		return []*Node{}
	}

	neighbors := make([]*Node, 0)
	for _, id := range neighborIDs {
		if node, ok := g.Nodes[id]; ok {
			if relation == "" {
				neighbors = append(neighbors, node)
			} else {
				for _, edge := range g.Edges {
					if edge.From == nodeID && edge.To == id && edge.Relation == relation {
						neighbors = append(neighbors, node)
						break
					}
				}
			}
		}
	}

	return neighbors
}

func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.Nodes)
}

func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.Edges)
}

func (g *Graph) Info() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"name":      g.Name,
		"nodes":     len(g.Nodes),
		"edges":     len(g.Edges),
		"labels":    len(g.NodeLabel),
		"relations": len(g.EdgeLabel),
	}
}

type GraphManager struct {
	mu     sync.RWMutex
	graphs map[string]*Graph
}

var globalGraphManager = NewGraphManager()

func NewGraphManager() *GraphManager {
	return &GraphManager{
		graphs: make(map[string]*Graph),
	}
}

func GetGraphManager() *GraphManager {
	return globalGraphManager
}

func (gm *GraphManager) Create(name string) *Graph {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if g, exists := gm.graphs[name]; exists {
		return g
	}

	graph := NewGraph(name)
	gm.graphs[name] = graph
	return graph
}

func (gm *GraphManager) Get(name string) (*Graph, bool) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	g, ok := gm.graphs[name]
	return g, ok
}

func (gm *GraphManager) Delete(name string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if _, exists := gm.graphs[name]; !exists {
		return false
	}

	delete(gm.graphs, name)
	return true
}

func (gm *GraphManager) List() []string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	names := make([]string, 0, len(gm.graphs))
	for name := range gm.graphs {
		names = append(names, name)
	}
	return names
}
