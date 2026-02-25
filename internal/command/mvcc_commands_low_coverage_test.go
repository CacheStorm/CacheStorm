package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMVCCCommandsAnalyticsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ANALYTICS.INCR no args", "ANALYTICS.INCR", nil},
		{"ANALYTICS.INCR missing args", "ANALYTICS.INCR", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.INCR basic", "ANALYTICS.INCR", [][]byte{[]byte("metric1"), []byte("100"), []byte("value1")}},
		{"ANALYTICS.DECR no args", "ANALYTICS.DECR", nil},
		{"ANALYTICS.DECR missing args", "ANALYTICS.DECR", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.DECR basic", "ANALYTICS.DECR", [][]byte{[]byte("metric1"), []byte("100"), []byte("value1")}},
		{"ANALYTICS.GET no args", "ANALYTICS.GET", nil},
		{"ANALYTICS.GET not found", "ANALYTICS.GET", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.GET exists", "ANALYTICS.GET", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.SUM no args", "ANALYTICS.SUM", nil},
		{"ANALYTICS.SUM not found", "ANALYTICS.SUM", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.SUM exists", "ANALYTICS.SUM", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.AVG no args", "ANALYTICS.AVG", nil},
		{"ANALYTICS.AVG not found", "ANALYTICS.AVG", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.AVG exists", "ANALYTICS.AVG", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.MIN no args", "ANALYTICS.MIN", nil},
		{"ANALYTICS.MIN not found", "ANALYTICS.MIN", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.MIN exists", "ANALYTICS.MIN", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.MAX no args", "ANALYTICS.MAX", nil},
		{"ANALYTICS.MAX not found", "ANALYTICS.MAX", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.MAX exists", "ANALYTICS.MAX", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.COUNT no args", "ANALYTICS.COUNT", nil},
		{"ANALYTICS.COUNT not found", "ANALYTICS.COUNT", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.COUNT exists", "ANALYTICS.COUNT", [][]byte{[]byte("metric1")}},
		{"ANALYTICS.CLEAR no args", "ANALYTICS.CLEAR", nil},
		{"ANALYTICS.CLEAR not found", "ANALYTICS.CLEAR", [][]byte{[]byte("notfound")}},
		{"ANALYTICS.CLEAR exists", "ANALYTICS.CLEAR", [][]byte{[]byte("metric1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsSpatialLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SPATIAL.CREATE no args", "SPATIAL.CREATE", nil},
		{"SPATIAL.CREATE index", "SPATIAL.CREATE", [][]byte{[]byte("spatial1")}},
		{"SPATIAL.ADD no args", "SPATIAL.ADD", nil},
		{"SPATIAL.ADD missing args", "SPATIAL.ADD", [][]byte{[]byte("spatial1"), []byte("id1")}},
		{"SPATIAL.ADD point", "SPATIAL.ADD", [][]byte{[]byte("spatial1"), []byte("id1"), []byte("40.7128"), []byte("-74.0060")}},
		{"SPATIAL.NEARBY no args", "SPATIAL.NEARBY", nil},
		{"SPATIAL.NEARBY missing args", "SPATIAL.NEARBY", [][]byte{[]byte("spatial1")}},
		{"SPATIAL.NEARBY search", "SPATIAL.NEARBY", [][]byte{[]byte("spatial1"), []byte("40.7128"), []byte("-74.0060"), []byte("1000")}},
		{"SPATIAL.WITHIN no args", "SPATIAL.WITHIN", nil},
		{"SPATIAL.WITHIN missing args", "SPATIAL.WITHIN", [][]byte{[]byte("spatial1")}},
		{"SPATIAL.WITHIN search", "SPATIAL.WITHIN", [][]byte{[]byte("spatial1"), []byte("40.7128"), []byte("-74.0060"), []byte("1000")}},
		{"SPATIAL.DELETE no args", "SPATIAL.DELETE", nil},
		{"SPATIAL.DELETE not found", "SPATIAL.DELETE", [][]byte{[]byte("notfound"), []byte("id1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsRollupLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ROLLUP.CREATE no args", "ROLLUP.CREATE", nil},
		{"ROLLUP.CREATE rollup", "ROLLUP.CREATE", [][]byte{[]byte("rollup1")}},
		{"ROLLUP.ADD no args", "ROLLUP.ADD", nil},
		{"ROLLUP.ADD not found", "ROLLUP.ADD", [][]byte{[]byte("notfound"), []byte("value")}},
		{"ROLLUP.GET no args", "ROLLUP.GET", nil},
		{"ROLLUP.GET not found", "ROLLUP.GET", [][]byte{[]byte("notfound")}},
		{"ROLLUP.DELETE no args", "ROLLUP.DELETE", nil},
		{"ROLLUP.DELETE not found", "ROLLUP.DELETE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsQuotaLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"QUOTA.SET no args", "QUOTA.SET", nil},
		{"QUOTA.SET quota", "QUOTA.SET", [][]byte{[]byte("quota1"), []byte("1000")}},
		{"QUOTA.CHECK no args", "QUOTA.CHECK", nil},
		{"QUOTA.CHECK not found", "QUOTA.CHECK", [][]byte{[]byte("notfound")}},
		{"QUOTA.USE no args", "QUOTA.USE", nil},
		{"QUOTA.USE not found", "QUOTA.USE", [][]byte{[]byte("notfound"), []byte("100")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsChainLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CHAIN.CREATE no args", "CHAIN.CREATE", nil},
		{"CHAIN.CREATE chain", "CHAIN.CREATE", [][]byte{[]byte("chain1")}},
		{"CHAIN.ADD no args", "CHAIN.ADD", nil},
		{"CHAIN.ADD not found", "CHAIN.ADD", [][]byte{[]byte("notfound"), []byte("block1")}},
		{"CHAIN.GET no args", "CHAIN.GET", nil},
		{"CHAIN.GET not found", "CHAIN.GET", [][]byte{[]byte("notfound"), []byte("0")}},
		{"CHAIN.VALIDATE no args", "CHAIN.VALIDATE", nil},
		{"CHAIN.VALIDATE not found", "CHAIN.VALIDATE", [][]byte{[]byte("notfound")}},
		{"CHAIN.LENGTH no args", "CHAIN.LENGTH", nil},
		{"CHAIN.LENGTH not found", "CHAIN.LENGTH", [][]byte{[]byte("notfound")}},
		{"CHAIN.LAST no args", "CHAIN.LAST", nil},
		{"CHAIN.LAST not found", "CHAIN.LAST", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsMVCC(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MVCC.BEGIN no args", "MVCC.BEGIN", nil},
		{"MVCC.BEGIN tx", "MVCC.BEGIN", [][]byte{[]byte("tx1")}},
		{"MVCC.COMMIT no args", "MVCC.COMMIT", nil},
		{"MVCC.COMMIT not found", "MVCC.COMMIT", [][]byte{[]byte("notfound")}},
		{"MVCC.ROLLBACK no args", "MVCC.ROLLBACK", nil},
		{"MVCC.ROLLBACK not found", "MVCC.ROLLBACK", [][]byte{[]byte("notfound")}},
		{"MVCC.GET no args", "MVCC.GET", nil},
		{"MVCC.GET not found", "MVCC.GET", [][]byte{[]byte("notfound")}},
		{"MVCC.SET no args", "MVCC.SET", nil},
		{"MVCC.SET missing args", "MVCC.SET", [][]byte{[]byte("key1")}},
		{"MVCC.SET value", "MVCC.SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"MVCC.DELETE no args", "MVCC.DELETE", nil},
		{"MVCC.DELETE not found", "MVCC.DELETE", [][]byte{[]byte("notfound")}},
		{"MVCC.STATUS no args", "MVCC.STATUS", nil},
		{"MVCC.STATUS tx", "MVCC.STATUS", [][]byte{[]byte("tx1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
