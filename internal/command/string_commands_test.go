package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllStringCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	tests := []struct {
		name    string
		cmd     string
		args    [][]byte
		wantErr bool
		setup   func()
	}{
		{"SET basic", "SET", [][]byte{[]byte("key1"), []byte("value1")}, false, nil},
		{"SET with EX", "SET", [][]byte{[]byte("key2"), []byte("value2"), []byte("EX"), []byte("10")}, false, nil},
		{"SET with PX", "SET", [][]byte{[]byte("key3"), []byte("value3"), []byte("PX"), []byte("10000")}, false, nil},
		{"SET NX", "SET", [][]byte{[]byte("key1"), []byte("newvalue"), []byte("NX")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		}},
		{"SET XX", "SET", [][]byte{[]byte("nonexistent"), []byte("value"), []byte("XX")}, false, nil},
		{"SET with KEEPTTL", "SET", [][]byte{[]byte("key4"), []byte("value4"), []byte("KEEPTTL")}, false, nil},
		{"SET GET", "SET", [][]byte{[]byte("key1"), []byte("newvalue"), []byte("GET")}, false, nil},
		{"SET insufficient args", "SET", [][]byte{[]byte("key")}, false, nil},
		{"SET invalid EX", "SET", [][]byte{[]byte("key"), []byte("value"), []byte("EX"), []byte("invalid")}, false, nil},
		{"GET existing", "GET", [][]byte{[]byte("key1")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("testval")}, store.SetOptions{})
		}},
		{"GET nonexistent", "GET", [][]byte{[]byte("nonexistent")}, false, nil},
		{"DEL single", "DEL", [][]byte{[]byte("key1")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
		}},
		{"DEL multiple", "DEL", [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("val1")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("val2")}, store.SetOptions{})
			s.Set("key3", &store.StringValue{Data: []byte("val3")}, store.SetOptions{})
		}},
		{"EXISTS single", "EXISTS", [][]byte{[]byte("key1")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
		}},
		{"EXISTS multiple", "EXISTS", [][]byte{[]byte("key1"), []byte("key2")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("val1")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("val2")}, store.SetOptions{})
		}},
		{"MSET", "MSET", [][]byte{[]byte("key1"), []byte("val1"), []byte("key2"), []byte("val2")}, false, nil},
		{"MSET odd args", "MSET", [][]byte{[]byte("key1"), []byte("val1"), []byte("key2")}, false, nil},
		{"MGET", "MGET", [][]byte{[]byte("key1"), []byte("key2"), []byte("nonexistent")}, false, func() {
			s.Set("key1", &store.StringValue{Data: []byte("val1")}, store.SetOptions{})
			s.Set("key2", &store.StringValue{Data: []byte("val2")}, store.SetOptions{})
		}},
		{"INCR new", "INCR", [][]byte{[]byte("counter1")}, false, nil},
		{"INCR existing", "INCR", [][]byte{[]byte("counter2")}, false, func() {
			s.Set("counter2", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
		}},
		{"DECR new", "DECR", [][]byte{[]byte("decrcounter")}, false, nil},
		{"DECR existing", "DECR", [][]byte{[]byte("decrcounter2")}, false, func() {
			s.Set("decrcounter2", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
		}},
		{"INCRBY", "INCRBY", [][]byte{[]byte("incrcounter"), []byte("5")}, false, nil},
		{"DECRBY", "DECRBY", [][]byte{[]byte("decrcounter"), []byte("5")}, false, nil},
		{"INCRBYFLOAT", "INCRBYFLOAT", [][]byte{[]byte("floatcounter"), []byte("1.5")}, false, nil},
		{"APPEND empty", "APPEND", [][]byte{[]byte("appendkey"), []byte("Hello")}, false, nil},
		{"APPEND existing", "APPEND", [][]byte{[]byte("appendkey2"), []byte(" World")}, false, func() {
			s.Set("appendkey2", &store.StringValue{Data: []byte("Hello")}, store.SetOptions{})
		}},
		{"STRLEN empty", "STRLEN", [][]byte{[]byte("nonexistent")}, false, nil},
		{"STRLEN existing", "STRLEN", [][]byte{[]byte("strkey")}, false, func() {
			s.Set("strkey", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
		}},
		{"GETRANGE full", "GETRANGE", [][]byte{[]byte("rangekey"), []byte("0"), []byte("-1")}, false, func() {
			s.Set("rangekey", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
		}},
		{"GETRANGE partial", "GETRANGE", [][]byte{[]byte("rangekey2"), []byte("0"), []byte("4")}, false, func() {
			s.Set("rangekey2", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
		}},
		{"GETRANGE negative", "GETRANGE", [][]byte{[]byte("rangekey3"), []byte("-5"), []byte("-1")}, false, func() {
			s.Set("rangekey3", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
		}},
		{"SETRANGE", "SETRANGE", [][]byte{[]byte("setrangekey"), []byte("6"), []byte("Redis")}, false, func() {
			s.Set("setrangekey", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
		}},
		{"SETNX new", "SETNX", [][]byte{[]byte("setnxkey"), []byte("value")}, false, nil},
		{"SETNX existing", "SETNX", [][]byte{[]byte("setnxkey2"), []byte("value")}, false, func() {
			s.Set("setnxkey2", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		}},
		{"SETEX", "SETEX", [][]byte{[]byte("setexkey"), []byte("10"), []byte("value")}, false, nil},
		{"PSETEX", "PSETEX", [][]byte{[]byte("psetexkey"), []byte("10000"), []byte("value")}, false, nil},
		{"MSETNX all new", "MSETNX", [][]byte{[]byte("msetnx1"), []byte("val1"), []byte("msetnx2"), []byte("val2")}, false, nil},
		{"MSETNX one exists", "MSETNX", [][]byte{[]byte("msetnx3"), []byte("val3"), []byte("msetnxexists"), []byte("val4")}, false, func() {
			s.Set("msetnxexists", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		}},
		{"GETSET", "GETSET", [][]byte{[]byte("getsetkey"), []byte("newvalue")}, false, func() {
			s.Set("getsetkey", &store.StringValue{Data: []byte("oldvalue")}, store.SetOptions{})
		}},
		{"GETDEL", "GETDEL", [][]byte{[]byte("getdelkey")}, false, func() {
			s.Set("getdelkey", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"GETEX", "GETEX", [][]byte{[]byte("getexkey"), []byte("EX"), []byte("10")}, false, func() {
			s.Set("getexkey", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"GETEX PX", "GETEX", [][]byte{[]byte("getexkey2"), []byte("PX"), []byte("10000")}, false, func() {
			s.Set("getexkey2", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"GETEX EXAT", "GETEX", [][]byte{[]byte("getexkey3"), []byte("EXAT"), []byte("1893456000")}, false, func() {
			s.Set("getexkey3", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"GETEX PXAT", "GETEX", [][]byte{[]byte("getexkey4"), []byte("PXAT"), []byte("1893456000000")}, false, func() {
			s.Set("getexkey4", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"GETEX PERSIST", "GETEX", [][]byte{[]byte("getexkey5"), []byte("PERSIST")}, false, func() {
			s.Set("getexkey5", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		}},
		{"LCS basic", "LCS", [][]byte{[]byte("lcskey1"), []byte("lcskey2")}, false, func() {
			s.Set("lcskey1", &store.StringValue{Data: []byte("hello world")}, store.SetOptions{})
			s.Set("lcskey2", &store.StringValue{Data: []byte("hello redis")}, store.SetOptions{})
		}},
		{"SUBSTR alias", "SUBSTR", [][]byte{[]byte("substrkey"), []byte("0"), []byte("4")}, false, func() {
			s.Set("substrkey", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
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
			err := handler.Handler(ctx)

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
