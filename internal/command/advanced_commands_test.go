package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllAdvancedCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"ACTOR.CREATE", "ACTOR.CREATE", [][]byte{[]byte("actor1")}, nil},
		{"ACTOR.DELETE", "ACTOR.DELETE", [][]byte{[]byte("actor1")}, func() {
			s.Set("actor:actor1", &store.StringValue{Data: []byte("state")}, store.SetOptions{})
		}},
		{"ACTOR.SEND", "ACTOR.SEND", [][]byte{[]byte("actor1"), []byte("message")}, nil},
		{"ACTOR.RECV", "ACTOR.RECV", [][]byte{[]byte("actor1")}, func() {
			s.Set("actor:actor1", &store.StringValue{Data: []byte("mailbox")}, store.SetOptions{})
		}},
		{"ACTOR.POKE", "ACTOR.POKE", [][]byte{[]byte("actor1")}, nil},
		{"ACTOR.PEEK", "ACTOR.PEEK", [][]byte{[]byte("actor1")}, nil},
		{"ACTOR.LEN", "ACTOR.LEN", [][]byte{[]byte("actor1")}, nil},
		{"ACTOR.LIST", "ACTOR.LIST", nil, nil},
		{"ACTOR.CLEAR", "ACTOR.CLEAR", nil, nil},
		{"DAG.CREATE", "DAG.CREATE", [][]byte{[]byte("dag1")}, nil},
		{"DAG.ADDNODE", "DAG.ADDNODE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("data")}, func() {
			s.Set("dag:dag1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"DAG.ADDEDGE", "DAG.ADDEDGE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("node2")}, func() {
			s.Set("dag:dag1:nodes:node1", &store.StringValue{Data: []byte("data")}, store.SetOptions{})
			s.Set("dag:dag1:nodes:node2", &store.StringValue{Data: []byte("data")}, store.SetOptions{})
		}},
		{"DAG.TOPO", "DAG.TOPO", [][]byte{[]byte("dag1")}, func() {
			s.Set("dag:dag1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"DAG.PARENTS", "DAG.PARENTS", [][]byte{[]byte("dag1"), []byte("node1")}, func() {
			s.Set("dag:dag1:nodes:node1", &store.StringValue{Data: []byte("data")}, store.SetOptions{})
		}},
		{"DAG.CHILDREN", "DAG.CHILDREN", [][]byte{[]byte("dag1"), []byte("node1")}, func() {
			s.Set("dag:dag1:nodes:node1", &store.StringValue{Data: []byte("data")}, store.SetOptions{})
		}},
		{"DAG.DELETE", "DAG.DELETE", [][]byte{[]byte("dag1")}, func() {
			s.Set("dag:dag1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"DAG.LIST", "DAG.LIST", nil, nil},
		{"PARALLEL.EXEC", "PARALLEL.EXEC", [][]byte{[]byte("command1"), []byte("command2")}, nil},
		{"PARALLEL.MAP", "PARALLEL.MAP", [][]byte{[]byte("key1"), []byte("key2"), []byte("UPPER")}, func() {
			s.Set("key1", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("world")}, store.SetOptions{})
		}},
		{"PARALLEL.REDUCE", "PARALLEL.REDUCE", [][]byte{[]byte("key1"), []byte("key2"), []byte("SUM")}, func() {
			s.Set("key1", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("20")}, store.SetOptions{})
		}},
		{"PARALLEL.FILTER", "PARALLEL.FILTER", [][]byte{[]byte("key1"), []byte("key2"), []byte("NONEMPTY")}, func() {
			s.Set("key1", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("")}, store.SetOptions{})
		}},
		{"SECRET.SET", "SECRET.SET", [][]byte{[]byte("secret1"), []byte("value")}, nil},
		{"SECRET.GET", "SECRET.GET", [][]byte{[]byte("secret1")}, func() {
			s.Set("secret:secret1", &store.StringValue{Data: []byte("encrypted")}, store.SetOptions{})
		}},
		{"SECRET.DELETE", "SECRET.DELETE", [][]byte{[]byte("secret1")}, func() {
			s.Set("secret:secret1", &store.StringValue{Data: []byte("encrypted")}, store.SetOptions{})
		}},
		{"SECRET.LIST", "SECRET.LIST", nil, nil},
		{"SECRET.ROTATE", "SECRET.ROTATE", [][]byte{[]byte("secret1")}, func() {
			s.Set("secret:secret1", &store.StringValue{Data: []byte("encrypted")}, store.SetOptions{})
		}},
		{"SECRET.VERSION", "SECRET.VERSION", [][]byte{[]byte("secret1")}, func() {
			s.Set("secret:secret1:version", &store.StringValue{Data: []byte("1")}, store.SetOptions{})
		}},
		{"CONFIG.SET", "CONFIG.SET", [][]byte{[]byte("key1"), []byte("value1")}, nil},
		{"CONFIG.GET", "CONFIG.GET", [][]byte{[]byte("key1")}, func() {
			s.Set("config:key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
		}},
		{"CONFIG.DELETE", "CONFIG.DELETE", [][]byte{[]byte("key1")}, func() {
			s.Set("config:key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
		}},
		{"CONFIG.LIST", "CONFIG.LIST", nil, nil},
		{"CONFIG.NAMESPACE", "CONFIG.NAMESPACE", [][]byte{[]byte("mynamespace")}, nil},
		{"CONFIG.IMPORT", "CONFIG.IMPORT", [][]byte{[]byte(`{"key":"value"}`)}, nil},
		{"CONFIG.EXPORT", "CONFIG.EXPORT", nil, nil},
		{"TRIE.ADD", "TRIE.ADD", [][]byte{[]byte("trie1"), []byte("word")}, nil},
		{"TRIE.SEARCH", "TRIE.SEARCH", [][]byte{[]byte("trie1"), []byte("word")}, func() {
			s.Set("trie:trie1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"TRIE.PREFIX", "TRIE.PREFIX", [][]byte{[]byte("trie1"), []byte("wo")}, func() {
			s.Set("trie:trie1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"TRIE.DELETE", "TRIE.DELETE", [][]byte{[]byte("trie1"), []byte("word")}, func() {
			s.Set("trie:trie1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"TRIE.AUTOCOMPLETE", "TRIE.AUTOCOMPLETE", [][]byte{[]byte("trie1"), []byte("wo")}, func() {
			s.Set("trie:trie1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"RING.CREATE", "RING.CREATE", [][]byte{[]byte("ring1")}, nil},
		{"RING.ADD", "RING.ADD", [][]byte{[]byte("ring1"), []byte("node1")}, func() {
			s.Set("ring:ring1", &store.StringValue{Data: []byte("{}")}, store.SetOptions{})
		}},
		{"RING.GET", "RING.GET", [][]byte{[]byte("ring1"), []byte("key1")}, func() {
			s.Set("ring:ring1", &store.StringValue{Data: []byte(`{"nodes":["node1"]}`)}, store.SetOptions{})
		}},
		{"RING.NODES", "RING.NODES", [][]byte{[]byte("ring1")}, func() {
			s.Set("ring:ring1", &store.StringValue{Data: []byte(`{"nodes":["node1"]}`)}, store.SetOptions{})
		}},
		{"RING.REMOVE", "RING.REMOVE", [][]byte{[]byte("ring1"), []byte("node1")}, func() {
			s.Set("ring:ring1", &store.StringValue{Data: []byte(`{"nodes":["node1"]}`)}, store.SetOptions{})
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
