package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMoreCommandsFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("item1"), []byte("item2")}}, store.SetOptions{})
	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"member1": {}}}, store.SetOptions{})
	s.Set("zset1", &store.SortedSetValue{Members: map[string]float64{"member1": 1.0}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TOUCH single", "TOUCH", [][]byte{[]byte("key1")}},
		{"TOUCH multiple", "TOUCH", [][]byte{[]byte("key1"), []byte("key2")}},
		{"TOUCH not found", "TOUCH", [][]byte{[]byte("notfound")}},
		{"TOUCH no args", "TOUCH", nil},
		{"EXPIRE", "EXPIRE", [][]byte{[]byte("key1"), []byte("60")}},
		{"EXPIRE no args", "EXPIRE", nil},
		{"PEXPIRE", "PEXPIRE", [][]byte{[]byte("key1"), []byte("60000")}},
		{"PEXPIRE no args", "PEXPIRE", nil},
		{"TTL exists", "TTL", [][]byte{[]byte("key1")}},
		{"TTL not found", "TTL", [][]byte{[]byte("notfound")}},
		{"TTL no args", "TTL", nil},
		{"PTTL exists", "PTTL", [][]byte{[]byte("key1")}},
		{"PTTL not found", "PTTL", [][]byte{[]byte("notfound")}},
		{"PTTL no args", "PTTL", nil},
		{"PERSIST", "PERSIST", [][]byte{[]byte("key1")}},
		{"PERSIST not found", "PERSIST", [][]byte{[]byte("notfound")}},
		{"PERSIST no args", "PERSIST", nil},
		{"EXPIRETIME", "EXPIRETIME", [][]byte{[]byte("key1")}},
		{"EXPIRETIME no args", "EXPIRETIME", nil},
		{"PEXPIRETIME", "PEXPIRETIME", [][]byte{[]byte("key1")}},
		{"PEXPIRETIME no args", "PEXPIRETIME", nil},
		{"RANDOMKEY", "RANDOMKEY", nil},
		{"RANDOMKEY empty", "RANDOMKEY", nil},
		{"KEYS pattern", "KEYS", [][]byte{[]byte("key*")}},
		{"KEYS no args", "KEYS", nil},
		{"SCAN basic", "SCAN", [][]byte{[]byte("0")}},
		{"SCAN no args", "SCAN", nil},
		{"TYPE string", "TYPE", [][]byte{[]byte("key1")}},
		{"TYPE hash", "TYPE", [][]byte{[]byte("hash1")}},
		{"TYPE list", "TYPE", [][]byte{[]byte("list1")}},
		{"TYPE set", "TYPE", [][]byte{[]byte("set1")}},
		{"TYPE zset", "TYPE", [][]byte{[]byte("zset1")}},
		{"TYPE not found", "TYPE", [][]byte{[]byte("notfound")}},
		{"TYPE no args", "TYPE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsDumpRestoreFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("item1")}}, store.SetOptions{})
	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"member1": {}}}, store.SetOptions{})
	s.Set("zset1", &store.SortedSetValue{Members: map[string]float64{"member1": 1.0}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DUMP string", "DUMP", [][]byte{[]byte("key1")}},
		{"DUMP hash", "DUMP", [][]byte{[]byte("hash1")}},
		{"DUMP list", "DUMP", [][]byte{[]byte("list1")}},
		{"DUMP set", "DUMP", [][]byte{[]byte("set1")}},
		{"DUMP zset", "DUMP", [][]byte{[]byte("zset1")}},
		{"DUMP not found", "DUMP", [][]byte{[]byte("notfound")}},
		{"DUMP no args", "DUMP", nil},
		{"RESTORE basic", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("serialized")}},
		{"RESTORE ABSTTL", "RESTORE", [][]byte{[]byte("newkey2"), []byte("0"), []byte("serialized"), []byte("ABSTTL")}},
		{"RESTORE IDLETIME", "RESTORE", [][]byte{[]byte("newkey3"), []byte("0"), []byte("serialized"), []byte("IDLETIME"), []byte("1000")}},
		{"RESTORE FREQ", "RESTORE", [][]byte{[]byte("newkey4"), []byte("0"), []byte("serialized"), []byte("FREQ"), []byte("10")}},
		{"RESTORE no args", "RESTORE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsRenameFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("oldkey", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("existing", &store.StringValue{Data: []byte("existing_value")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RENAME success", "RENAME", [][]byte{[]byte("oldkey"), []byte("newkey")}},
		{"RENAME not found", "RENAME", [][]byte{[]byte("notfound"), []byte("newkey")}},
		{"RENAME no args", "RENAME", nil},
		{"RENAMENX success", "RENAMENX", [][]byte{[]byte("oldkey"), []byte("newkey2")}},
		{"RENAMENX exists", "RENAMENX", [][]byte{[]byte("oldkey"), []byte("existing")}},
		{"RENAMENX not found", "RENAMENX", [][]byte{[]byte("notfound"), []byte("newkey")}},
		{"RENAMENX no args", "RENAMENX", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsSortFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("3"), []byte("1"), []byte("2")}}, store.SetOptions{})
	s.Set("list_alpha", &store.ListValue{Elements: [][]byte{[]byte("c"), []byte("a"), []byte("b")}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SORT numeric", "SORT", [][]byte{[]byte("list1")}},
		{"SORT DESC", "SORT", [][]byte{[]byte("list1"), []byte("DESC")}},
		{"SORT ALPHA", "SORT", [][]byte{[]byte("list_alpha"), []byte("ALPHA")}},
		{"SORT LIMIT", "SORT", [][]byte{[]byte("list1"), []byte("LIMIT"), []byte("0"), []byte("2")}},
		{"SORT no args", "SORT", nil},
		{"SORT not found", "SORT", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsCopyFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("source", &store.StringValue{Data: []byte("source_value")}, store.SetOptions{})
	s.Set("existing", &store.StringValue{Data: []byte("existing_value")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COPY basic", "COPY", [][]byte{[]byte("source"), []byte("dest")}},
		{"COPY NX exists", "COPY", [][]byte{[]byte("source"), []byte("existing"), []byte("NX")}},
		{"COPY NX new", "COPY", [][]byte{[]byte("source"), []byte("newkey"), []byte("NX")}},
		{"COPY REPLACE", "COPY", [][]byte{[]byte("source"), []byte("existing"), []byte("REPLACE")}},
		{"COPY not found", "COPY", [][]byte{[]byte("notfound"), []byte("dest")}},
		{"COPY no args", "COPY", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsMoveFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MOVE success", "MOVE", [][]byte{[]byte("key1"), []byte("1")}},
		{"MOVE not found", "MOVE", [][]byte{[]byte("notfound"), []byte("1")}},
		{"MOVE no args", "MOVE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsExistsDelFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})
	s.Set("key3", &store.StringValue{Data: []byte("value3")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EXISTS single", "EXISTS", [][]byte{[]byte("key1")}},
		{"EXISTS multiple", "EXISTS", [][]byte{[]byte("key1"), []byte("key2"), []byte("notfound")}},
		{"EXISTS not found", "EXISTS", [][]byte{[]byte("notfound")}},
		{"EXISTS no args", "EXISTS", nil},
		{"DEL single", "DEL", [][]byte{[]byte("key1")}},
		{"DEL multiple", "DEL", [][]byte{[]byte("key2"), []byte("key3")}},
		{"DEL not found", "DEL", [][]byte{[]byte("notfound")}},
		{"DEL no args", "DEL", nil},
		{"UNLINK single", "UNLINK", [][]byte{[]byte("key1")}},
		{"UNLINK multiple", "UNLINK", [][]byte{[]byte("key2"), []byte("key3")}},
		{"UNLINK not found", "UNLINK", [][]byte{[]byte("notfound")}},
		{"UNLINK no args", "UNLINK", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
