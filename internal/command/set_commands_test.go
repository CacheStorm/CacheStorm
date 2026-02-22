package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllSetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSetCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"SADD single", "SADD", [][]byte{[]byte("set1"), []byte("member1")}, nil},
		{"SADD multiple", "SADD", [][]byte{[]byte("set2"), []byte("a"), []byte("b"), []byte("c")}, nil},
		{"SADD existing", "SADD", [][]byte{[]byte("set3"), []byte("new")}, func() {
			s.Set("set3", &store.SetValue{Members: map[string]struct{}{"existing": {}}}, store.SetOptions{})
		}},
		{"SREM single", "SREM", [][]byte{[]byte("set4"), []byte("a")}, func() {
			s.Set("set4", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SREM multiple", "SREM", [][]byte{[]byte("set5"), []byte("a"), []byte("b")}, func() {
			s.Set("set5", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SCARD empty", "SCARD", [][]byte{[]byte("emptyset")}, nil},
		{"SCARD with members", "SCARD", [][]byte{[]byte("set6")}, func() {
			s.Set("set6", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SISMEMBER true", "SISMEMBER", [][]byte{[]byte("set7"), []byte("a")}, func() {
			s.Set("set7", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		}},
		{"SISMEMBER false", "SISMEMBER", [][]byte{[]byte("set8"), []byte("c")}, func() {
			s.Set("set8", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		}},
		{"SMEMBERS", "SMEMBERS", [][]byte{[]byte("set9")}, func() {
			s.Set("set9", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SPOP single", "SPOP", [][]byte{[]byte("set10")}, func() {
			s.Set("set10", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SPOP multiple", "SPOP", [][]byte{[]byte("set11"), []byte("2")}, func() {
			s.Set("set11", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		}},
		{"SRANDMEMBER single", "SRANDMEMBER", [][]byte{[]byte("set12")}, func() {
			s.Set("set12", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SRANDMEMBER multiple", "SRANDMEMBER", [][]byte{[]byte("set13"), []byte("2")}, func() {
			s.Set("set13", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		}},
		{"SMOVE success", "SMOVE", [][]byte{[]byte("set14"), []byte("set15"), []byte("a")}, func() {
			s.Set("set14", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
			s.Set("set15", &store.SetValue{Members: map[string]struct{}{"c": {}}}, store.SetOptions{})
		}},
		{"SMOVE member not exists", "SMOVE", [][]byte{[]byte("set16"), []byte("set17"), []byte("x")}, func() {
			s.Set("set16", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		}},
		{"SUNION", "SUNION", [][]byte{[]byte("set18"), []byte("set19")}, func() {
			s.Set("set18", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
			s.Set("set19", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SUNIONSTORE", "SUNIONSTORE", [][]byte{[]byte("result1"), []byte("set20"), []byte("set21")}, func() {
			s.Set("set20", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
			s.Set("set21", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}}}, store.SetOptions{})
		}},
		{"SINTER", "SINTER", [][]byte{[]byte("set22"), []byte("set23")}, func() {
			s.Set("set22", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
			s.Set("set23", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		}},
		{"SINTERSTORE", "SINTERSTORE", [][]byte{[]byte("result2"), []byte("set24"), []byte("set25")}, func() {
			s.Set("set24", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
			s.Set("set25", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		}},
		{"SDIFF", "SDIFF", [][]byte{[]byte("set26"), []byte("set27")}, func() {
			s.Set("set26", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
			s.Set("set27", &store.SetValue{Members: map[string]struct{}{"b": {}, "d": {}}}, store.SetOptions{})
		}},
		{"SDIFFSTORE", "SDIFFSTORE", [][]byte{[]byte("result3"), []byte("set28"), []byte("set29")}, func() {
			s.Set("set28", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
			s.Set("set29", &store.SetValue{Members: map[string]struct{}{"b": {}, "d": {}}}, store.SetOptions{})
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
