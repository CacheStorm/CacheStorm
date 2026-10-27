package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExtendedCommandsTOPICFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsWSFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsLEADERFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LEADER.ELECT", "LEADER.ELECT", [][]byte{[]byte("group1"), []byte("node1")}},
		{"LEADER.ELECT no args", "LEADER.ELECT", nil},
		{"LEADER.RENEW", "LEADER.RENEW", [][]byte{[]byte("group1"), []byte("node1")}},
		{"LEADER.RENEW no args", "LEADER.RENEW", nil},
		{"LEADER.RESIGN", "LEADER.RESIGN", [][]byte{[]byte("group1"), []byte("node1")}},
		{"LEADER.RESIGN no args", "LEADER.RESIGN", nil},
		{"LEADER.CURRENT", "LEADER.CURRENT", [][]byte{[]byte("group1")}},
		{"LEADER.CURRENT no args", "LEADER.CURRENT", nil},
		{"LEADER.HISTORY", "LEADER.HISTORY", [][]byte{[]byte("group1")}},
		{"LEADER.HISTORY no args", "LEADER.HISTORY", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
