package store

import "testing"

func TestStreamRaxNodes(t *testing.T) {
	ids := func(list ...StreamID) []*streamNode {
		out := make([]*streamNode, len(list))
		for i, id := range list {
			out[i] = &streamNode{master: id}
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		nodes []*streamNode
		want  int
	}{
		{"empty", nil, 1},
		{"one", ids(StreamID{1, 0}), 2},
		{"ms differs in the last byte", ids(StreamID{1, 0}, StreamID{2, 0}), 6},
		{"seq differs in the last byte", ids(StreamID{1, 0}, StreamID{1, 5}), 4},
		{"nested branch", ids(StreamID{1, 0}, StreamID{2, 0}, StreamID{256, 0}), 9},
	} {
		if got := streamRaxNodes(tc.nodes); got != tc.want {
			t.Errorf("%s: %d nodes, want %d", tc.name, got, tc.want)
		}
	}
}
