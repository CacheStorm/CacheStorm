package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestEventCommandsCOMPRESSFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COMPRESS.GZIP", "COMPRESS.GZIP", [][]byte{[]byte("test data")}},
		{"COMPRESS.GZIP no args", "COMPRESS.GZIP", nil},
		{"COMPRESS.GUNZIP valid", "COMPRESS.GUNZIP", [][]byte{[]byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00, 0x86, 0xa6, 0x10, 0x36, 0x05, 0x00, 0x00, 0x00}}},
		{"COMPRESS.GUNZIP invalid", "COMPRESS.GUNZIP", [][]byte{[]byte("invalid gzip")}},
		{"COMPRESS.GUNZIP no args", "COMPRESS.GUNZIP", nil},
		{"COMPRESS.DEFLATE", "COMPRESS.DEFLATE", [][]byte{[]byte("test data")}},
		{"COMPRESS.DEFLATE no args", "COMPRESS.DEFLATE", nil},
		{"COMPRESS.INFLATE valid", "COMPRESS.INFLATE", [][]byte{[]byte{0x78, 0x9c, 0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00, 0x00, 0x00, 0xff, 0xff}}},
		{"COMPRESS.INFLATE invalid", "COMPRESS.INFLATE", [][]byte{[]byte("invalid deflate")}},
		{"COMPRESS.INFLATE no args", "COMPRESS.INFLATE", nil},
		{"COMPRESS.ZLIB", "COMPRESS.ZLIB", [][]byte{[]byte("test data")}},
		{"COMPRESS.ZLIB no args", "COMPRESS.ZLIB", nil},
		{"COMPRESS.UNZLIB valid", "COMPRESS.UNZLIB", [][]byte{[]byte{0x78, 0x9c, 0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00, 0x00, 0x00, 0xff, 0xff}}},
		{"COMPRESS.UNZLIB invalid", "COMPRESS.UNZLIB", [][]byte{[]byte("invalid zlib")}},
		{"COMPRESS.UNZLIB no args", "COMPRESS.UNZLIB", nil},
		{"COMPRESS.CUSTOM", "COMPRESS.CUSTOM", [][]byte{[]byte("algo1"), []byte("test data")}},
		{"COMPRESS.CUSTOM no args", "COMPRESS.CUSTOM", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEventCommandsWEBHOOKFullCoverage2(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WEBHOOK.TRIGGER", "WEBHOOK.TRIGGER", [][]byte{[]byte("wh1"), []byte("event1"), []byte("payload")}},
		{"WEBHOOK.TRIGGER no args", "WEBHOOK.TRIGGER", nil},
		{"WEBHOOK.RETRY", "WEBHOOK.RETRY", [][]byte{[]byte("wh1"), []byte("5")}},
		{"WEBHOOK.RETRY no args", "WEBHOOK.RETRY", nil},
		{"WEBHOOK.QUEUE", "WEBHOOK.QUEUE", [][]byte{[]byte("wh1")}},
		{"WEBHOOK.QUEUE no args", "WEBHOOK.QUEUE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEventCommandsQUEUEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"QUEUE.PUSH", "QUEUE.PUSH", [][]byte{[]byte("queue1"), []byte("item1")}},
		{"QUEUE.PUSH no args", "QUEUE.PUSH", nil},
		{"QUEUE.POP", "QUEUE.POP", [][]byte{[]byte("queue1")}},
		{"QUEUE.POP empty", "QUEUE.POP", [][]byte{[]byte("empty_queue")}},
		{"QUEUE.POP no args", "QUEUE.POP", nil},
		{"QUEUE.PEEK", "QUEUE.PEEK", [][]byte{[]byte("queue1")}},
		{"QUEUE.PEEK no args", "QUEUE.PEEK", nil},
		{"QUEUE.LEN", "QUEUE.LEN", [][]byte{[]byte("queue1")}},
		{"QUEUE.LEN no args", "QUEUE.LEN", nil},
		{"QUEUE.CLEAR", "QUEUE.CLEAR", [][]byte{[]byte("queue1")}},
		{"QUEUE.CLEAR no args", "QUEUE.CLEAR", nil},
		{"QUEUE.LIST", "QUEUE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEventCommandsSTACKFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"STACK.PUSH", "STACK.PUSH", [][]byte{[]byte("stack1"), []byte("item1")}},
		{"STACK.PUSH no args", "STACK.PUSH", nil},
		{"STACK.POP", "STACK.POP", [][]byte{[]byte("stack1")}},
		{"STACK.POP empty", "STACK.POP", [][]byte{[]byte("empty_stack")}},
		{"STACK.POP no args", "STACK.POP", nil},
		{"STACK.PEEK", "STACK.PEEK", [][]byte{[]byte("stack1")}},
		{"STACK.PEEK no args", "STACK.PEEK", nil},
		{"STACK.LEN", "STACK.LEN", [][]byte{[]byte("stack1")}},
		{"STACK.LEN no args", "STACK.LEN", nil},
		{"STACK.CLEAR", "STACK.CLEAR", [][]byte{[]byte("stack1")}},
		{"STACK.CLEAR no args", "STACK.CLEAR", nil},
		{"STACK.LIST", "STACK.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
