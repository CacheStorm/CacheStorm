package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMVCCCommandsExtendedFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MVCC.START", "MVCC.START", nil},
		{"MVCC.COMMIT", "MVCC.COMMIT", [][]byte{[]byte("tx1")}},
		{"MVCC.COMMIT no args", "MVCC.COMMIT", nil},
		{"MVCC.ROLLBACK", "MVCC.ROLLBACK", [][]byte{[]byte("tx1")}},
		{"MVCC.ROLLBACK no args", "MVCC.ROLLBACK", nil},
		{"MVCC.STATUS", "MVCC.STATUS", [][]byte{[]byte("tx1")}},
		{"MVCC.STATUS no args", "MVCC.STATUS", nil},
		{"MVCC.LIST", "MVCC.LIST", nil},
		{"MVCC.SNAPSHOT", "MVCC.SNAPSHOT", nil},
		{"MVCC.RESTORE", "MVCC.RESTORE", [][]byte{[]byte("snapshot1")}},
		{"MVCC.RESTORE no args", "MVCC.RESTORE", nil},
		{"MVCC.GET", "MVCC.GET", [][]byte{[]byte("tx1"), []byte("key1")}},
		{"MVCC.GET no args", "MVCC.GET", nil},
		{"MVCC.SET", "MVCC.SET", [][]byte{[]byte("tx1"), []byte("key1"), []byte("value1")}},
		{"MVCC.SET no args", "MVCC.SET", nil},
		{"MVCC.DEL", "MVCC.DEL", [][]byte{[]byte("tx1"), []byte("key1")}},
		{"MVCC.DEL no args", "MVCC.DEL", nil},
		{"MVCC.EXISTS", "MVCC.EXISTS", [][]byte{[]byte("tx1"), []byte("key1")}},
		{"MVCC.EXISTS no args", "MVCC.EXISTS", nil},
		{"MVCC.KEYS", "MVCC.KEYS", [][]byte{[]byte("tx1"), []byte("pattern*")}},
		{"MVCC.KEYS no args", "MVCC.KEYS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsSnapshotExtendedFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MVCC.SNAPSHOT create", "MVCC.SNAPSHOT", nil},
		{"MVCC.RESTORE snapshot", "MVCC.RESTORE", [][]byte{[]byte("snapshot1")}},
		{"MVCC.RESTORE not found", "MVCC.RESTORE", [][]byte{[]byte("notfound")}},
		{"MVCC.LIST snapshots", "MVCC.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
