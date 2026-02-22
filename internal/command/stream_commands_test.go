package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllStreamCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"XADD basic", "XADD", [][]byte{[]byte("stream1"), []byte("*"), []byte("field1"), []byte("value1")}, nil},
		{"XADD with ID", "XADD", [][]byte{[]byte("stream2"), []byte("1000-0"), []byte("name"), []byte("John")}, nil},
		{"XADD multiple fields", "XADD", [][]byte{[]byte("stream3"), []byte("*"), []byte("f1"), []byte("v1"), []byte("f2"), []byte("v2")}, nil},
		{"XADD with MAXLEN", "XADD", [][]byte{[]byte("stream4"), []byte("MAXLEN"), []byte("~"), []byte("1000"), []byte("*"), []byte("data"), []byte("test")}, nil},
		{"XLEN empty", "XLEN", [][]byte{[]byte("emptystream")}, nil},
		{"XLEN with entries", "XLEN", [][]byte{[]byte("stream5")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream5", stream, store.SetOptions{})
		}},
		{"XRANGE", "XRANGE", [][]byte{[]byte("stream6"), []byte("-"), []byte("+")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f1": []byte("v1")})
			stream.Add("1001-0", map[string][]byte{"f2": []byte("v2")})
			s.Set("stream6", stream, store.SetOptions{})
		}},
		{"XRANGE with COUNT", "XRANGE", [][]byte{[]byte("stream7"), []byte("-"), []byte("+"), []byte("COUNT"), []byte("1")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f1": []byte("v1")})
			stream.Add("1001-0", map[string][]byte{"f2": []byte("v2")})
			s.Set("stream7", stream, store.SetOptions{})
		}},
		{"XREVRANGE", "XREVRANGE", [][]byte{[]byte("stream8"), []byte("+"), []byte("-")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f1": []byte("v1")})
			stream.Add("1001-0", map[string][]byte{"f2": []byte("v2")})
			s.Set("stream8", stream, store.SetOptions{})
		}},
		{"XREAD single stream", "XREAD", [][]byte{[]byte("STREAMS"), []byte("stream9"), []byte("0")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream9", stream, store.SetOptions{})
		}},
		{"XREAD multiple streams", "XREAD", [][]byte{[]byte("STREAMS"), []byte("stream10"), []byte("stream11"), []byte("0"), []byte("0")}, func() {
			stream1 := store.NewStreamValue(0)
			stream1.Add("1000-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream10", stream1, store.SetOptions{})
			stream2 := store.NewStreamValue(0)
			stream2.Add("1001-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream11", stream2, store.SetOptions{})
		}},
		{"XREAD COUNT", "XREAD", [][]byte{[]byte("COUNT"), []byte("1"), []byte("STREAMS"), []byte("stream12"), []byte("0")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			stream.Add("1001-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream12", stream, store.SetOptions{})
		}},
		{"XGROUP CREATE", "XGROUP", [][]byte{[]byte("CREATE"), []byte("stream13"), []byte("mygroup"), []byte("$")}, func() {
			stream := store.NewStreamValue(0)
			s.Set("stream13", stream, store.SetOptions{})
		}},
		{"XGROUP DESTROY", "XGROUP", [][]byte{[]byte("DESTROY"), []byte("stream14"), []byte("mygroup")}, func() {
			stream := store.NewStreamValue(0)
			stream.CreateGroup("mygroup", "0")
			s.Set("stream14", stream, store.SetOptions{})
		}},
		{"XGROUP DELCONSUMER", "XGROUP", [][]byte{[]byte("DELCONSUMER"), []byte("stream15"), []byte("mygroup"), []byte("consumer1")}, func() {
			stream := store.NewStreamValue(0)
			stream.CreateGroup("mygroup", "0")
			s.Set("stream15", stream, store.SetOptions{})
		}},
		{"XREADGROUP", "XREADGROUP", [][]byte{[]byte("GROUP"), []byte("mygroup"), []byte("consumer1"), []byte("STREAMS"), []byte("stream16"), []byte(">")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			stream.CreateGroup("mygroup", "0")
			s.Set("stream16", stream, store.SetOptions{})
		}},
		{"XPENDING", "XPENDING", [][]byte{[]byte("stream17"), []byte("mygroup")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			stream.CreateGroup("mygroup", "0")
			s.Set("stream17", stream, store.SetOptions{})
		}},
		{"XACK", "XACK", [][]byte{[]byte("stream18"), []byte("mygroup"), []byte("1000-0")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			stream.CreateGroup("mygroup", "0")
			s.Set("stream18", stream, store.SetOptions{})
		}},
		{"XCLAIM", "XCLAIM", [][]byte{[]byte("stream19"), []byte("mygroup"), []byte("consumer2"), []byte("0"), []byte("1000-0")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			stream.CreateGroup("mygroup", "0")
			s.Set("stream19", stream, store.SetOptions{})
		}},
		{"XDEL", "XDEL", [][]byte{[]byte("stream20"), []byte("1000-0")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream20", stream, store.SetOptions{})
		}},
		{"XTRIM", "XTRIM", [][]byte{[]byte("stream21"), []byte("MAXLEN"), []byte("~"), []byte("100")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream21", stream, store.SetOptions{})
		}},
		{"XINFO STREAM", "XINFO", [][]byte{[]byte("STREAM"), []byte("stream22")}, func() {
			stream := store.NewStreamValue(0)
			stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
			s.Set("stream22", stream, store.SetOptions{})
		}},
		{"XINFO GROUPS", "XINFO", [][]byte{[]byte("GROUPS"), []byte("stream23")}, func() {
			stream := store.NewStreamValue(0)
			stream.CreateGroup("group1", "0")
			s.Set("stream23", stream, store.SetOptions{})
		}},
		{"XINFO CONSUMERS", "XINFO", [][]byte{[]byte("CONSUMERS"), []byte("stream24"), []byte("mygroup")}, func() {
			stream := store.NewStreamValue(0)
			stream.CreateGroup("mygroup", "0")
			s.Set("stream24", stream, store.SetOptions{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}

			ctx := newTestContext(tt.cmd, tt.args, s)
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestStreamValueOperations(t *testing.T) {
	t.Run("Stream Add", func(t *testing.T) {
		stream := store.NewStreamValue(0)
		entry, err := stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
		if err != nil {
			t.Errorf("Add failed: %v", err)
		}
		if entry == nil {
			t.Error("Add returned nil entry")
		}

		if stream.Len() != 1 {
			t.Errorf("Expected length 1, got %d", stream.Len())
		}
	})

	t.Run("Stream Consumer Group", func(t *testing.T) {
		stream := store.NewStreamValue(0)

		// Create group
		err := stream.CreateGroup("mygroup", "0")
		if err != nil {
			t.Errorf("CreateGroup failed: %v", err)
		}

		// Get group
		group := stream.GetGroup("mygroup")
		if group == nil {
			t.Error("GetGroup returned nil")
		}

		// Destroy group
		if !stream.DestroyGroup("mygroup") {
			t.Error("DestroyGroup should return true")
		}

		if stream.GetGroup("mygroup") != nil {
			t.Error("Group should be destroyed")
		}
	})

	t.Run("Stream GetRange", func(t *testing.T) {
		stream := store.NewStreamValue(0)
		stream.Add("1000-0", map[string][]byte{"f1": []byte("v1")})
		stream.Add("1001-0", map[string][]byte{"f2": []byte("v2")})
		stream.Add("1002-0", map[string][]byte{"f3": []byte("v3")})

		entries := stream.GetRange("-", "+", 10)
		if len(entries) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(entries))
		}
	})

	t.Run("Stream Delete", func(t *testing.T) {
		stream := store.NewStreamValue(0)
		stream.Add("1000-0", map[string][]byte{"f": []byte("v")})
		stream.Add("1001-0", map[string][]byte{"f": []byte("v")})

		deleted := stream.Delete("1000-0")
		if deleted != 1 {
			t.Errorf("Expected 1 deleted, got %d", deleted)
		}

		if stream.Len() != 1 {
			t.Errorf("Expected length 1, got %d", stream.Len())
		}
	})
}
