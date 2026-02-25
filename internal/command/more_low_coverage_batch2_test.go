package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAdvancedCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RING.NODES no args", "RING.NODES", nil},
		{"RING.NODES empty", "RING.NODES", [][]byte{[]byte("ring1")}},
		{"DAG.ADDEDGE no args", "DAG.ADDEDGE", nil},
		{"DAG.ADDEDGE missing args", "DAG.ADDEDGE", [][]byte{[]byte("dag1"), []byte("node1")}},
		{"DAG.PARENTS no args", "DAG.PARENTS", nil},
		{"DAG.PARENTS not found", "DAG.PARENTS", [][]byte{[]byte("notfound"), []byte("node1")}},
		{"PARALLEL.FILTER no args", "PARALLEL.FILTER", nil},
		{"PARALLEL.FILTER missing args", "PARALLEL.FILTER", [][]byte{[]byte("list1")}},
		{"SECRET.SET no args", "SECRET.SET", nil},
		{"SECRET.SET missing args", "SECRET.SET", [][]byte{[]byte("secret1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestBitmapCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GETBIT no args", "GETBIT", nil},
		{"GETBIT not found", "GETBIT", [][]byte{[]byte("notfound"), []byte("0")}},
		{"BITCOUNT no args", "BITCOUNT", nil},
		{"BITCOUNT not found", "BITCOUNT", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsQuotaXLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"QUOTAX.CHECK no args", "QUOTAX.CHECK", nil},
		{"QUOTAX.CHECK not found", "QUOTAX.CHECK", [][]byte{[]byte("notfound"), []byte("100")}},
		{"SKETCH.UPDATE no args", "SKETCH.UPDATE", nil},
		{"SKETCH.UPDATE not found", "SKETCH.UPDATE", [][]byte{[]byte("notfound"), []byte("item1")}},
		{"PARTITION.ADD no args", "PARTITION.ADD", nil},
		{"PARTITION.ADD not found", "PARTITION.ADD", [][]byte{[]byte("notfound"), []byte("item1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ROUTE.MATCH no args", "ROUTE.MATCH", nil},
		{"ROUTE.MATCH missing args", "ROUTE.MATCH", [][]byte{[]byte("route1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.INFO no args", "CACHE.INFO", nil},
		{"CACHE.FLUSH", "CACHE.FLUSH", nil},
		{"CACHE.STATS no args", "CACHE.STATS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestNamespaceCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterNamespaceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"NAMESPACE.CREATE no args", "NAMESPACE.CREATE", nil},
		{"NAMESPACE.CREATE ns", "NAMESPACE.CREATE", [][]byte{[]byte("ns1")}},
		{"NAMESPACE.DELETE no args", "NAMESPACE.DELETE", nil},
		{"NAMESPACE.DELETE not found", "NAMESPACE.DELETE", [][]byte{[]byte("notfound")}},
		{"NAMESPACE.LIST", "NAMESPACE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSearchCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSearchCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SEARCH.CREATE no args", "SEARCH.CREATE", nil},
		{"SEARCH.CREATE index", "SEARCH.CREATE", [][]byte{[]byte("idx1")}},
		{"SEARCH.ADD no args", "SEARCH.ADD", nil},
		{"SEARCH.ADD not found", "SEARCH.ADD", [][]byte{[]byte("notfound"), []byte("doc1")}},
		{"SEARCH.QUERY no args", "SEARCH.QUERY", nil},
		{"SEARCH.QUERY not found", "SEARCH.QUERY", [][]byte{[]byte("notfound"), []byte("query")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTagCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTagCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TAG.ADD no args", "TAG.ADD", nil},
		{"TAG.ADD key", "TAG.ADD", [][]byte{[]byte("key1"), []byte("tag1")}},
		{"TAG.REMOVE no args", "TAG.REMOVE", nil},
		{"TAG.REMOVE not found", "TAG.REMOVE", [][]byte{[]byte("notfound"), []byte("tag1")}},
		{"TAG.LIST no args", "TAG.LIST", nil},
		{"TAG.LIST not found", "TAG.LIST", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestProbabilisticCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterProbabilisticCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BF.ADD no args", "BF.ADD", nil},
		{"BF.ADD not found", "BF.ADD", [][]byte{[]byte("notfound"), []byte("item")}},
		{"BF.EXISTS no args", "BF.EXISTS", nil},
		{"BF.EXISTS not found", "BF.EXISTS", [][]byte{[]byte("notfound"), []byte("item")}},
		{"CF.ADD no args", "CF.ADD", nil},
		{"CF.ADD not found", "CF.ADD", [][]byte{[]byte("notfound"), []byte("item")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTimeseriesCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTSCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TS.CREATE no args", "TS.CREATE", nil},
		{"TS.CREATE series", "TS.CREATE", [][]byte{[]byte("ts1")}},
		{"TS.ADD no args", "TS.ADD", nil},
		{"TS.ADD not found", "TS.ADD", [][]byte{[]byte("notfound"), []byte("100")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
