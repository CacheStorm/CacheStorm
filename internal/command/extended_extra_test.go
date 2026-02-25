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
	}{
		{"TOPIC.SUBSCRIBE", "TOPIC.SUBSCRIBE", [][]byte{[]byte("topic1"), []byte("subscriber1")}},
		{"TOPIC.SUBSCRIBE no args", "TOPIC.SUBSCRIBE", nil},
		{"TOPIC.UNSUBSCRIBE", "TOPIC.UNSUBSCRIBE", [][]byte{[]byte("topic1"), []byte("subscriber1")}},
		{"TOPIC.UNSUBSCRIBE no args", "TOPIC.UNSUBSCRIBE", nil},
		{"TOPIC.PUBLISH", "TOPIC.PUBLISH", [][]byte{[]byte("topic1"), []byte("message")}},
		{"TOPIC.PUBLISH no args", "TOPIC.PUBLISH", nil},
		{"TOPIC.SUBSCRIBERS", "TOPIC.SUBSCRIBERS", [][]byte{[]byte("topic1")}},
		{"TOPIC.SUBSCRIBERS no args", "TOPIC.SUBSCRIBERS", nil},
		{"TOPIC.LIST", "TOPIC.LIST", nil},
		{"TOPIC.HISTORY", "TOPIC.HISTORY", [][]byte{[]byte("topic1"), []byte("10")}},
		{"TOPIC.HISTORY no args", "TOPIC.HISTORY", nil},
	}

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
	}{
		{"WS.CONNECT", "WS.CONNECT", [][]byte{[]byte("client1")}},
		{"WS.CONNECT no args", "WS.CONNECT", nil},
		{"WS.DISCONNECT", "WS.DISCONNECT", [][]byte{[]byte("client1")}},
		{"WS.DISCONNECT no args", "WS.DISCONNECT", nil},
		{"WS.SEND", "WS.SEND", [][]byte{[]byte("client1"), []byte("message")}},
		{"WS.SEND no args", "WS.SEND", nil},
		{"WS.BROADCAST", "WS.BROADCAST", [][]byte{[]byte("message")}},
		{"WS.BROADCAST no args", "WS.BROADCAST", nil},
		{"WS.LIST", "WS.LIST", nil},
		{"WS.ROOMS", "WS.ROOMS", nil},
		{"WS.JOIN", "WS.JOIN", [][]byte{[]byte("room1"), []byte("client1")}},
		{"WS.JOIN no args", "WS.JOIN", nil},
		{"WS.LEAVE", "WS.LEAVE", [][]byte{[]byte("room1"), []byte("client1")}},
		{"WS.LEAVE no args", "WS.LEAVE", nil},
	}

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
