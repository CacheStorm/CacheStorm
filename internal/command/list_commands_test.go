package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllListCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"LPUSH single", "LPUSH", [][]byte{[]byte("list1"), []byte("item1")}, nil},
		{"LPUSH multiple", "LPUSH", [][]byte{[]byte("list2"), []byte("item1"), []byte("item2"), []byte("item3")}, nil},
		{"RPUSH single", "RPUSH", [][]byte{[]byte("list3"), []byte("item1")}, nil},
		{"RPUSH multiple", "RPUSH", [][]byte{[]byte("list4"), []byte("item1"), []byte("item2")}, nil},
		{"LPOP single", "LPOP", [][]byte{[]byte("list5")}, func() {
			s.Set("list5", &store.ListValue{Elements: [][]byte{[]byte("first"), []byte("second")}}, store.SetOptions{})
		}},
		{"LPOP multiple", "LPOP", [][]byte{[]byte("list6"), []byte("2")}, func() {
			s.Set("list6", &store.ListValue{Elements: [][]byte{[]byte("1"), []byte("2"), []byte("3"), []byte("4")}}, store.SetOptions{})
		}},
		{"RPOP single", "RPOP", [][]byte{[]byte("list7")}, func() {
			s.Set("list7", &store.ListValue{Elements: [][]byte{[]byte("first"), []byte("second")}}, store.SetOptions{})
		}},
		{"RPOP multiple", "RPOP", [][]byte{[]byte("list8"), []byte("2")}, func() {
			s.Set("list8", &store.ListValue{Elements: [][]byte{[]byte("1"), []byte("2"), []byte("3"), []byte("4")}}, store.SetOptions{})
		}},
		{"LLEN empty", "LLEN", [][]byte{[]byte("emptylist")}, nil},
		{"LLEN with items", "LLEN", [][]byte{[]byte("list9")}, func() {
			s.Set("list9", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LRANGE full", "LRANGE", [][]byte{[]byte("list10"), []byte("0"), []byte("-1")}, func() {
			s.Set("list10", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LRANGE partial", "LRANGE", [][]byte{[]byte("list11"), []byte("0"), []byte("1")}, func() {
			s.Set("list11", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LINDEX", "LINDEX", [][]byte{[]byte("list12"), []byte("1")}, func() {
			s.Set("list12", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LSET", "LSET", [][]byte{[]byte("list13"), []byte("1"), []byte("newvalue")}, func() {
			s.Set("list13", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LREM", "LREM", [][]byte{[]byte("list14"), []byte("0"), []byte("b")}, func() {
			s.Set("list14", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LTRIM", "LTRIM", [][]byte{[]byte("list15"), []byte("1"), []byte("2")}, func() {
			s.Set("list15", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")}}, store.SetOptions{})
		}},
		{"LINSERT BEFORE", "LINSERT", [][]byte{[]byte("list16"), []byte("BEFORE"), []byte("b"), []byte("new")}, func() {
			s.Set("list16", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LINSERT AFTER", "LINSERT", [][]byte{[]byte("list17"), []byte("AFTER"), []byte("b"), []byte("new")}, func() {
			s.Set("list17", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LPOS single", "LPOS", [][]byte{[]byte("list18"), []byte("b")}, func() {
			s.Set("list18", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LPOS with RANK", "LPOS", [][]byte{[]byte("list19"), []byte("b"), []byte("RANK"), []byte("1")}, func() {
			s.Set("list19", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("b"), []byte("c")}}, store.SetOptions{})
		}},
		{"LPUSHX existing", "LPUSHX", [][]byte{[]byte("list20"), []byte("newitem")}, func() {
			s.Set("list20", &store.ListValue{Elements: [][]byte{[]byte("existing")}}, store.SetOptions{})
		}},
		{"LPUSHX nonexisting", "LPUSHX", [][]byte{[]byte("newlist"), []byte("item")}, nil},
		{"RPUSHX existing", "RPUSHX", [][]byte{[]byte("list21"), []byte("newitem")}, func() {
			s.Set("list21", &store.ListValue{Elements: [][]byte{[]byte("existing")}}, store.SetOptions{})
		}},
		{"RPUSHX nonexisting", "RPUSHX", [][]byte{[]byte("newlist2"), []byte("item")}, nil},
		{"LMOVE", "LMOVE", [][]byte{[]byte("list22"), []byte("list23"), []byte("LEFT"), []byte("RIGHT")}, func() {
			s.Set("list22", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}, store.SetOptions{})
		}},
		{"BLPOP", "BLPOP", [][]byte{[]byte("list24"), []byte("1")}, func() {
			s.Set("list24", &store.ListValue{Elements: [][]byte{[]byte("item")}}, store.SetOptions{})
		}},
		{"BRPOP", "BRPOP", [][]byte{[]byte("list25"), []byte("1")}, func() {
			s.Set("list25", &store.ListValue{Elements: [][]byte{[]byte("item")}}, store.SetOptions{})
		}},
		{"BLMOVE", "BLMOVE", [][]byte{[]byte("list26"), []byte("list27"), []byte("LEFT"), []byte("RIGHT"), []byte("1")}, func() {
			s.Set("list26", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
		}},
		{"LMPOP single", "LMPOP", [][]byte{[]byte("1"), []byte("list28"), []byte("LEFT")}, func() {
			s.Set("list28", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}, store.SetOptions{})
		}},
		{"LMPOP multiple", "LMPOP", [][]byte{[]byte("1"), []byte("list29"), []byte("LEFT"), []byte("COUNT"), []byte("2")}, func() {
			s.Set("list29", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
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
