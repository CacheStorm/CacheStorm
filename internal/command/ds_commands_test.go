package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestDSCommandsSLIDINGWINDOWFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDataStructuresCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SLIDINGWINDOW.CREATE", "SLIDINGWINDOW.CREATE", [][]byte{[]byte("sw1"), []byte("10"), []byte("60000")}},
		{"SLIDINGWINDOW.CREATE no args", "SLIDINGWINDOW.CREATE", nil},
		{"SLIDINGWINDOW.INCR", "SLIDINGWINDOW.INCR", [][]byte{[]byte("sw1")}},
		{"SLIDINGWINDOW.INCR not found", "SLIDINGWINDOW.INCR", [][]byte{[]byte("notfound")}},
		{"SLIDINGWINDOW.INCR no args", "SLIDINGWINDOW.INCR", nil},
		{"SLIDINGWINDOW.DECR", "SLIDINGWINDOW.DECR", [][]byte{[]byte("sw1")}},
		{"SLIDINGWINDOW.DECR not found", "SLIDINGWINDOW.DECR", [][]byte{[]byte("notfound")}},
		{"SLIDINGWINDOW.DECR no args", "SLIDINGWINDOW.DECR", nil},
		{"SLIDINGWINDOW.GET", "SLIDINGWINDOW.GET", [][]byte{[]byte("sw1")}},
		{"SLIDINGWINDOW.GET not found", "SLIDINGWINDOW.GET", [][]byte{[]byte("notfound")}},
		{"SLIDINGWINDOW.GET no args", "SLIDINGWINDOW.GET", nil},
		{"SLIDINGWINDOW.DELETE", "SLIDINGWINDOW.DELETE", [][]byte{[]byte("sw1")}},
		{"SLIDINGWINDOW.DELETE no args", "SLIDINGWINDOW.DELETE", nil},
		{"SLIDINGWINDOW.LIST", "SLIDINGWINDOW.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDSCommandsDEBOUNCEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDataStructuresCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEBOUNCE.CREATE", "DEBOUNCE.CREATE", [][]byte{[]byte("deb1"), []byte("1000")}},
		{"DEBOUNCE.CREATE no args", "DEBOUNCE.CREATE", nil},
		{"DEBOUNCE.SET", "DEBOUNCE.SET", [][]byte{[]byte("deb1"), []byte("key1"), []byte("value1")}},
		{"DEBOUNCE.SET not found", "DEBOUNCE.SET", [][]byte{[]byte("notfound"), []byte("key1"), []byte("value1")}},
		{"DEBOUNCE.SET no args", "DEBOUNCE.SET", nil},
		{"DEBOUNCE.CALL", "DEBOUNCE.CALL", [][]byte{[]byte("deb1"), []byte("key1")}},
		{"DEBOUNCE.CALL not found", "DEBOUNCE.CALL", [][]byte{[]byte("notfound"), []byte("key1")}},
		{"DEBOUNCE.CALL no args", "DEBOUNCE.CALL", nil},
		{"DEBOUNCE.GET", "DEBOUNCE.GET", [][]byte{[]byte("deb1"), []byte("key1")}},
		{"DEBOUNCE.GET not found", "DEBOUNCE.GET", [][]byte{[]byte("notfound"), []byte("key1")}},
		{"DEBOUNCE.GET no args", "DEBOUNCE.GET", nil},
		{"DEBOUNCE.DELETE", "DEBOUNCE.DELETE", [][]byte{[]byte("deb1"), []byte("key1")}},
		{"DEBOUNCE.DELETE no args", "DEBOUNCE.DELETE", nil},
		{"DEBOUNCE.LIST", "DEBOUNCE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
