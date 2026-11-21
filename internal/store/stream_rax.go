package store

import "encoding/binary"

// streamRaxNodes is the number of nodes Redis' radix tree needs for the stream's
// node keys, XINFO STREAM's radix-tree-nodes. Each key is a node's master
// ID, 16 big-endian bytes. The tree is compressed: a run of bytes with one
// child is one node, a byte position where keys diverge is a branching
// node, and every key ends in a node of its own; the root always exists.
func streamRaxNodes(nodes []*streamNode) int {
	if len(nodes) == 0 {
		return 1
	}
	keys := make([][16]byte, len(nodes))
	for i, n := range nodes {
		binary.BigEndian.PutUint64(keys[i][:8], n.master.Ms)
		binary.BigEndian.PutUint64(keys[i][8:], n.master.Seq)
	}
	return streamRaxSubtree(keys, 0)
}

// streamRaxSubtree counts the nodes below depth for sorted keys that share their
// first depth bytes.
func streamRaxSubtree(keys [][16]byte, depth int) int {
	if len(keys) == 1 {
		if depth == 16 {
			return 1
		}
		return 2
	}
	// Sorted keys share what the first and last share.
	first, last := &keys[0], &keys[len(keys)-1]
	p := depth
	for first[p] == last[p] {
		p++
	}
	n := 1
	if p > depth {
		n++
	}
	for i := 0; i < len(keys); {
		j := i + 1
		for j < len(keys) && keys[j][p] == keys[i][p] {
			j++
		}
		n += streamRaxSubtree(keys[i:j], p+1)
		i = j
	}
	return n
}
