package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllHashCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"HSET new", "HSET", [][]byte{[]byte("hash1"), []byte("field1"), []byte("value1")}, nil},
		{"HSET existing", "HSET", [][]byte{[]byte("hash2"), []byte("field1"), []byte("newvalue")}, func() {
			s.Set("hash2", &store.HashValue{Fields: map[string][]byte{"field1": []byte("oldvalue")}}, store.SetOptions{})
		}},
		{"HSET multiple", "HSET", [][]byte{[]byte("hash3"), []byte("f1"), []byte("v1"), []byte("f2"), []byte("v2")}, nil},
		{"HGET existing", "HGET", [][]byte{[]byte("hash4"), []byte("field1")}, func() {
			s.Set("hash4", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
		}},
		{"HGET nonexistent field", "HGET", [][]byte{[]byte("hash5"), []byte("nofield")}, func() {
			s.Set("hash5", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
		}},
		{"HGET nonexistent hash", "HGET", [][]byte{[]byte("nohash"), []byte("field")}, nil},
		{"HDEL single", "HDEL", [][]byte{[]byte("hash6"), []byte("field1")}, func() {
			s.Set("hash6", &store.HashValue{Fields: map[string][]byte{"field1": []byte("v1"), "field2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HDEL multiple", "HDEL", [][]byte{[]byte("hash7"), []byte("f1"), []byte("f2")}, func() {
			s.Set("hash7", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2"), "f3": []byte("v3")}}, store.SetOptions{})
		}},
		{"HEXISTS true", "HEXISTS", [][]byte{[]byte("hash8"), []byte("field1")}, func() {
			s.Set("hash8", &store.HashValue{Fields: map[string][]byte{"field1": []byte("v1")}}, store.SetOptions{})
		}},
		{"HEXISTS false", "HEXISTS", [][]byte{[]byte("hash9"), []byte("nofield")}, func() {
			s.Set("hash9", &store.HashValue{Fields: map[string][]byte{"other": []byte("v")}}, store.SetOptions{})
		}},
		{"HLEN with fields", "HLEN", [][]byte{[]byte("hash10")}, func() {
			s.Set("hash10", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HLEN empty", "HLEN", [][]byte{[]byte("hashempty")}, func() {
			s.Set("hashempty", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
		}},
		{"HKEYS", "HKEYS", [][]byte{[]byte("hash11")}, func() {
			s.Set("hash11", &store.HashValue{Fields: map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HVALS", "HVALS", [][]byte{[]byte("hash12")}, func() {
			s.Set("hash12", &store.HashValue{Fields: map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HGETALL", "HGETALL", [][]byte{[]byte("hash13")}, func() {
			s.Set("hash13", &store.HashValue{Fields: map[string][]byte{"k1": []byte("v1"), "k2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HMSET", "HMSET", [][]byte{[]byte("hash14"), []byte("f1"), []byte("v1"), []byte("f2"), []byte("v2")}, nil},
		{"HMGET", "HMGET", [][]byte{[]byte("hash15"), []byte("f1"), []byte("f2")}, func() {
			s.Set("hash15", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HINCRBY new", "HINCRBY", [][]byte{[]byte("hash16"), []byte("counter"), []byte("5")}, nil},
		{"HINCRBY existing", "HINCRBY", [][]byte{[]byte("hash17"), []byte("counter"), []byte("3")}, func() {
			s.Set("hash17", &store.HashValue{Fields: map[string][]byte{"counter": []byte("10")}}, store.SetOptions{})
		}},
		{"HINCRBYFLOAT", "HINCRBYFLOAT", [][]byte{[]byte("hash18"), []byte("float"), []byte("1.5")}, nil},
		{"HSETNX new", "HSETNX", [][]byte{[]byte("hash20"), []byte("field"), []byte("value")}, nil},
		{"HSETNX existing", "HSETNX", [][]byte{[]byte("hash20"), []byte("field"), []byte("newval")}, func() {
			s.Set("hash20", &store.HashValue{Fields: map[string][]byte{"field": []byte("oldval")}}, store.SetOptions{})
		}},
		{"HSTRLEN", "HSTRLEN", [][]byte{[]byte("hash21"), []byte("field")}, func() {
			s.Set("hash21", &store.HashValue{Fields: map[string][]byte{"field": []byte("Hello World")}}, store.SetOptions{})
		}},
		{"HRANDFIELD single", "HRANDFIELD", [][]byte{[]byte("hash22")}, func() {
			s.Set("hash22", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})
		}},
		{"HRANDFIELD with count", "HRANDFIELD", [][]byte{[]byte("hash23"), []byte("2")}, func() {
			s.Set("hash23", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2"), "f3": []byte("v3")}}, store.SetOptions{})
		}},
		{"HRANDFIELD with values", "HRANDFIELD", [][]byte{[]byte("hash24"), []byte("2"), []byte("WITHVALUES")}, func() {
			s.Set("hash24", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2"), "f3": []byte("v3")}}, store.SetOptions{})
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
