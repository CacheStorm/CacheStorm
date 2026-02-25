package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExtendedCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"VECTOR.CREATE no args", "VECTOR.CREATE", nil},
		{"VECTOR.CREATE vector", "VECTOR.CREATE", [][]byte{[]byte("vec1"), []byte("[1,2,3]")}},
		{"VECTOR.ADD no args", "VECTOR.ADD", nil},
		{"VECTOR.ADD missing args", "VECTOR.ADD", [][]byte{[]byte("vec1")}},
		{"VECTOR.GET no args", "VECTOR.GET", nil},
		{"VECTOR.GET not found", "VECTOR.GET", [][]byte{[]byte("notfound")}},
		{"VECTOR.DELETE no args", "VECTOR.DELETE", nil},
		{"VECTOR.DELETE not found", "VECTOR.DELETE", [][]byte{[]byte("notfound")}},
		{"VECTOR.SEARCH no args", "VECTOR.SEARCH", nil},
		{"VECTOR.SEARCH not found", "VECTOR.SEARCH", [][]byte{[]byte("notfound"), []byte("[1,2,3]")}},
		{"VECTOR.SIMILARITY no args", "VECTOR.SIMILARITY", nil},
		{"VECTOR.SIMILARITY missing args", "VECTOR.SIMILARITY", [][]byte{[]byte("vec1")}},
		{"VECTOR.NORMALIZE no args", "VECTOR.NORMALIZE", nil},
		{"VECTOR.NORMALIZE not found", "VECTOR.NORMALIZE", [][]byte{[]byte("notfound")}},
		{"VECTOR.DIMENSIONS no args", "VECTOR.DIMENSIONS", nil},
		{"VECTOR.DIMENSIONS not found", "VECTOR.DIMENSIONS", [][]byte{[]byte("notfound")}},
		{"VECTOR.MERGE no args", "VECTOR.MERGE", nil},
		{"VECTOR.MERGE missing args", "VECTOR.MERGE", [][]byte{[]byte("vec1")}},
		{"VECTOR.STATS no args", "VECTOR.STATS", nil},
		{"VECTOR.STATS not found", "VECTOR.STATS", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsAdvanced(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"QUEUE.CREATE no args", "QUEUE.CREATE", nil},
		{"QUEUE.CREATE queue", "QUEUE.CREATE", [][]byte{[]byte("queue1")}},
		{"QUEUE.ENQUEUE no args", "QUEUE.ENQUEUE", nil},
		{"QUEUE.ENQUEUE not found", "QUEUE.ENQUEUE", [][]byte{[]byte("notfound"), []byte("item")}},
		{"QUEUE.DEQUEUE no args", "QUEUE.DEQUEUE", nil},
		{"QUEUE.DEQUEUE not found", "QUEUE.DEQUEUE", [][]byte{[]byte("notfound")}},
		{"QUEUE.PEEK no args", "QUEUE.PEEK", nil},
		{"QUEUE.PEEK not found", "QUEUE.PEEK", [][]byte{[]byte("notfound")}},
		{"QUEUE.SIZE no args", "QUEUE.SIZE", nil},
		{"QUEUE.SIZE not found", "QUEUE.SIZE", [][]byte{[]byte("notfound")}},
		{"STACK.CREATE no args", "STACK.CREATE", nil},
		{"STACK.CREATE stack", "STACK.CREATE", [][]byte{[]byte("stack1")}},
		{"STACK.PUSH no args", "STACK.PUSH", nil},
		{"STACK.PUSH not found", "STACK.PUSH", [][]byte{[]byte("notfound"), []byte("item")}},
		{"STACK.POP no args", "STACK.POP", nil},
		{"STACK.POP not found", "STACK.POP", [][]byte{[]byte("notfound")}},
		{"STACK.PEEK no args", "STACK.PEEK", nil},
		{"STACK.PEEK not found", "STACK.PEEK", [][]byte{[]byte("notfound")}},
		{"STACK.SIZE no args", "STACK.SIZE", nil},
		{"STACK.SIZE not found", "STACK.SIZE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsPriority(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PRIORITY.CREATE no args", "PRIORITY.CREATE", nil},
		{"PRIORITY.CREATE pq", "PRIORITY.CREATE", [][]byte{[]byte("pq1")}},
		{"PRIORITY.PUSH no args", "PRIORITY.PUSH", nil},
		{"PRIORITY.PUSH missing args", "PRIORITY.PUSH", [][]byte{[]byte("pq1")}},
		{"PRIORITY.POP no args", "PRIORITY.POP", nil},
		{"PRIORITY.POP not found", "PRIORITY.POP", [][]byte{[]byte("notfound")}},
		{"PRIORITY.PEEK no args", "PRIORITY.PEEK", nil},
		{"PRIORITY.PEEK not found", "PRIORITY.PEEK", [][]byte{[]byte("notfound")}},
		{"PRIORITY.SIZE no args", "PRIORITY.SIZE", nil},
		{"PRIORITY.SIZE not found", "PRIORITY.SIZE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsBloom(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BLOOM.CREATE no args", "BLOOM.CREATE", nil},
		{"BLOOM.CREATE bloom", "BLOOM.CREATE", [][]byte{[]byte("bloom1"), []byte("1000"), []byte("0.01")}},
		{"BLOOM.ADD no args", "BLOOM.ADD", nil},
		{"BLOOM.ADD not found", "BLOOM.ADD", [][]byte{[]byte("notfound"), []byte("item")}},
		{"BLOOM.EXISTS no args", "BLOOM.EXISTS", nil},
		{"BLOOM.EXISTS not found", "BLOOM.EXISTS", [][]byte{[]byte("notfound"), []byte("item")}},
		{"BLOOM.COUNT no args", "BLOOM.COUNT", nil},
		{"BLOOM.COUNT not found", "BLOOM.COUNT", [][]byte{[]byte("notfound")}},
		{"BLOOM.CLEAR no args", "BLOOM.CLEAR", nil},
		{"BLOOM.CLEAR not found", "BLOOM.CLEAR", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsHyperLogLog(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HLL.CREATE no args", "HLL.CREATE", nil},
		{"HLL.CREATE hll", "HLL.CREATE", [][]byte{[]byte("hll1")}},
		{"HLL.ADD no args", "HLL.ADD", nil},
		{"HLL.ADD not found", "HLL.ADD", [][]byte{[]byte("notfound"), []byte("item")}},
		{"HLL.COUNT no args", "HLL.COUNT", nil},
		{"HLL.COUNT not found", "HLL.COUNT", [][]byte{[]byte("notfound")}},
		{"HLL.MERGE no args", "HLL.MERGE", nil},
		{"HLL.MERGE missing args", "HLL.MERGE", [][]byte{[]byte("hll1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsCache(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LRU.CREATE no args", "LRU.CREATE", nil},
		{"LRU.CREATE lru", "LRU.CREATE", [][]byte{[]byte("lru1"), []byte("100")}},
		{"LRU.GET no args", "LRU.GET", nil},
		{"LRU.GET not found", "LRU.GET", [][]byte{[]byte("notfound"), []byte("key")}},
		{"LRU.PUT no args", "LRU.PUT", nil},
		{"LRU.PUT missing args", "LRU.PUT", [][]byte{[]byte("lru1"), []byte("key")}},
		{"LRU.SIZE no args", "LRU.SIZE", nil},
		{"LRU.SIZE not found", "LRU.SIZE", [][]byte{[]byte("notfound")}},
		{"LFU.CREATE no args", "LFU.CREATE", nil},
		{"LFU.CREATE lfu", "LFU.CREATE", [][]byte{[]byte("lfu1"), []byte("100")}},
		{"LFU.GET no args", "LFU.GET", nil},
		{"LFU.GET not found", "LFU.GET", [][]byte{[]byte("notfound"), []byte("key")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
