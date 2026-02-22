package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllSortedSetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"ZADD single", "ZADD", [][]byte{[]byte("zset1"), []byte("1"), []byte("member1")}, nil},
		{"ZADD multiple", "ZADD", [][]byte{[]byte("zset2"), []byte("1"), []byte("a"), []byte("2"), []byte("b"), []byte("3"), []byte("c")}, nil},
		{"ZADD with options", "ZADD", [][]byte{[]byte("zset3"), []byte("NX"), []byte("1"), []byte("newmember")}, nil},
		{"ZREM single", "ZREM", [][]byte{[]byte("zset4"), []byte("a")}, func() {
			s.Set("zset4", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZREM multiple", "ZREM", [][]byte{[]byte("zset5"), []byte("a"), []byte("b")}, func() {
			s.Set("zset5", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZCARD empty", "ZCARD", [][]byte{[]byte("emptyzset")}, nil},
		{"ZCARD with members", "ZCARD", [][]byte{[]byte("zset6")}, func() {
			s.Set("zset6", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0}}, store.SetOptions{})
		}},
		{"ZSCORE exists", "ZSCORE", [][]byte{[]byte("zset7"), []byte("a")}, func() {
			s.Set("zset7", &store.SortedSetValue{Members: map[string]float64{"a": 1.5, "b": 2.5}}, store.SetOptions{})
		}},
		{"ZSCORE not exists", "ZSCORE", [][]byte{[]byte("zset8"), []byte("c")}, func() {
			s.Set("zset8", &store.SortedSetValue{Members: map[string]float64{"a": 1.0}}, store.SetOptions{})
		}},
		{"ZRANK", "ZRANK", [][]byte{[]byte("zset9"), []byte("b")}, func() {
			s.Set("zset9", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZREVRANK", "ZREVRANK", [][]byte{[]byte("zset10"), []byte("b")}, func() {
			s.Set("zset10", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZRANGE", "ZRANGE", [][]byte{[]byte("zset11"), []byte("0"), []byte("-1")}, func() {
			s.Set("zset11", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZRANGE with scores", "ZRANGE", [][]byte{[]byte("zset12"), []byte("0"), []byte("-1"), []byte("WITHSCORES")}, func() {
			s.Set("zset12", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0}}, store.SetOptions{})
		}},
		{"ZREVRANGE", "ZREVRANGE", [][]byte{[]byte("zset13"), []byte("0"), []byte("-1")}, func() {
			s.Set("zset13", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZCOUNT", "ZCOUNT", [][]byte{[]byte("zset14"), []byte("1"), []byte("2")}, func() {
			s.Set("zset14", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZINCRBY", "ZINCRBY", [][]byte{[]byte("zset15"), []byte("5"), []byte("a")}, func() {
			s.Set("zset15", &store.SortedSetValue{Members: map[string]float64{"a": 10.0}}, store.SetOptions{})
		}},
		{"ZINCRBY new member", "ZINCRBY", [][]byte{[]byte("zset16"), []byte("5"), []byte("newmember")}, nil},
		{"ZPOPMIN", "ZPOPMIN", [][]byte{[]byte("zset17")}, func() {
			s.Set("zset17", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZPOPMIN with count", "ZPOPMIN", [][]byte{[]byte("zset18"), []byte("2")}, func() {
			s.Set("zset18", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZPOPMAX", "ZPOPMAX", [][]byte{[]byte("zset19")}, func() {
			s.Set("zset19", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZPOPMAX with count", "ZPOPMAX", [][]byte{[]byte("zset20"), []byte("2")}, func() {
			s.Set("zset20", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZUNION", "ZUNION", [][]byte{[]byte("2"), []byte("zset21"), []byte("zset22")}, func() {
			s.Set("zset21", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0}}, store.SetOptions{})
			s.Set("zset22", &store.SortedSetValue{Members: map[string]float64{"b": 2.0, "c": 3.0}}, store.SetOptions{})
		}},
		{"ZINTER", "ZINTER", [][]byte{[]byte("2"), []byte("zset23"), []byte("zset24")}, func() {
			s.Set("zset23", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
			s.Set("zset24", &store.SortedSetValue{Members: map[string]float64{"b": 2.0, "c": 3.0, "d": 4.0}}, store.SetOptions{})
		}},
		{"ZDIFF", "ZDIFF", [][]byte{[]byte("2"), []byte("zset25"), []byte("zset26")}, func() {
			s.Set("zset25", &store.SortedSetValue{Members: map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}}, store.SetOptions{})
			s.Set("zset26", &store.SortedSetValue{Members: map[string]float64{"b": 2.0, "c": 3.0}}, store.SetOptions{})
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
