package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestCacheWarmCommandsBATCHEXECFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHEXEC GET", "BATCHEXEC", [][]byte{[]byte("GET"), []byte("key1")}},
		{"BATCHEXEC GET missing key", "BATCHEXEC", [][]byte{[]byte("GET"), []byte("notfound")}},
		{"BATCHEXEC GET no key", "BATCHEXEC", [][]byte{[]byte("GET")}},
		{"BATCHEXEC SET", "BATCHEXEC", [][]byte{[]byte("SET"), []byte("newkey"), []byte("newvalue")}},
		{"BATCHEXEC SET missing value", "BATCHEXEC", [][]byte{[]byte("SET"), []byte("newkey")}},
		{"BATCHEXEC DEL", "BATCHEXEC", [][]byte{[]byte("DEL"), []byte("key1")}},
		{"BATCHEXEC DEL missing key", "BATCHEXEC", [][]byte{[]byte("DEL"), []byte("notfound")}},
		{"BATCHEXEC DEL no key", "BATCHEXEC", [][]byte{[]byte("DEL")}},
		{"BATCHEXEC EXISTS true", "BATCHEXEC", [][]byte{[]byte("EXISTS"), []byte("key1")}},
		{"BATCHEXEC EXISTS false", "BATCHEXEC", [][]byte{[]byte("EXISTS"), []byte("notfound")}},
		{"BATCHEXEC EXISTS no key", "BATCHEXEC", [][]byte{[]byte("EXISTS")}},
		{"BATCHEXEC unknown command", "BATCHEXEC", [][]byte{[]byte("UNKNOWN"), []byte("key1")}},
		{"BATCHEXEC no args", "BATCHEXEC", nil},
		{"BATCHEXEC multiple commands", "BATCHEXEC", [][]byte{[]byte("GET"), []byte("key1"), []byte("GET"), []byte("key2"), []byte("SET"), []byte("key3"), []byte("value3")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheWarmCommandsKEYOBJECTFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	// Setup test data with different types
	s.Set("str1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("item1"), []byte("item2")}}, store.SetOptions{})
	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"member1": {}}}, store.SetOptions{})
	s.Set("zset1", &store.SortedSetValue{Members: map[string]float64{"member1": 1.0}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"KEYOBJECT string", "KEYOBJECT", [][]byte{[]byte("str1")}},
		{"KEYOBJECT hash", "KEYOBJECT", [][]byte{[]byte("hash1")}},
		{"KEYOBJECT list", "KEYOBJECT", [][]byte{[]byte("list1")}},
		{"KEYOBJECT set", "KEYOBJECT", [][]byte{[]byte("set1")}},
		{"KEYOBJECT zset", "KEYOBJECT", [][]byte{[]byte("zset1")}},
		{"KEYOBJECT not found", "KEYOBJECT", [][]byte{[]byte("notfound")}},
		{"KEYOBJECT no args", "KEYOBJECT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheWarmCommandsKeyOperationsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	// Setup test data
	s.Set("oldkey", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("existing", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"KEYRENAME success", "KEYRENAME", [][]byte{[]byte("oldkey"), []byte("newkey")}},
		{"KEYRENAME not found", "KEYRENAME", [][]byte{[]byte("notfound"), []byte("newkey")}},
		{"KEYRENAME no args", "KEYRENAME", nil},
		{"KEYRENAMENX success", "KEYRENAMENX", [][]byte{[]byte("oldkey"), []byte("newkey2")}},
		{"KEYRENAMENX exists", "KEYRENAMENX", [][]byte{[]byte("oldkey"), []byte("existing")}},
		{"KEYRENAMENX not found", "KEYRENAMENX", [][]byte{[]byte("notfound"), []byte("newkey")}},
		{"KEYRENAMENX no args", "KEYRENAMENX", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheWarmCommandsBatchOperationsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})
	s.Set("key3", &store.StringValue{Data: []byte("value3")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHGET", "BATCHGET", [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}},
		{"BATCHGET no args", "BATCHGET", nil},
		{"BATCHSET", "BATCHSET", [][]byte{[]byte("key4"), []byte("value4"), []byte("key5"), []byte("value5")}},
		{"BATCHSET no args", "BATCHSET", nil},
		{"BATCHDEL", "BATCHDEL", [][]byte{[]byte("key1"), []byte("key2")}},
		{"BATCHDEL no args", "BATCHDEL", nil},
		{"BATCHEXISTS", "BATCHEXISTS", [][]byte{[]byte("key1"), []byte("notfound")}},
		{"BATCHEXISTS no args", "BATCHEXISTS", nil},
		{"BATCHMDEL", "BATCHMDEL", [][]byte{[]byte("key1"), []byte("key2")}},
		{"BATCHMDEL no args", "BATCHMDEL", nil},
		{"PIPELINEEXEC", "PIPELINEEXEC", [][]byte{[]byte("GET"), []byte("key1")}},
		{"PIPELINEEXEC no args", "PIPELINEEXEC", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
