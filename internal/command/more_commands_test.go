package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMoreCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COPY", "COPY", [][]byte{[]byte("source"), []byte("destination")}},
		{"COPY no args", "COPY", nil},
		{"COPY NX", "COPY", [][]byte{[]byte("source"), []byte("destination"), []byte("NX")}},
		{"COPY REPLACE", "COPY", [][]byte{[]byte("source"), []byte("destination"), []byte("REPLACE")}},
		{"MOVE", "MOVE", [][]byte{[]byte("source"), []byte("destination")}},
		{"MOVE no args", "MOVE", nil},
		{"EXPIREAT", "EXPIREAT", [][]byte{[]byte("key1"), []byte("1893456000")}},
		{"EXPIREAT no args", "EXPIREAT", nil},
		{"PEXPIREAT", "PEXPIREAT", [][]byte{[]byte("key1"), []byte("1893456000000")}},
		{"PEXPIREAT no args", "PEXPIREAT", nil},
		{"PERSIST", "PERSIST", [][]byte{[]byte("key1")}},
		{"PERSIST no args", "PERSIST", nil},
		{"PTTL", "PTTL", [][]byte{[]byte("key1")}},
		{"PTTL no args", "PTTL", nil},
		{"TTL", "TTL", [][]byte{[]byte("key1")}},
		{"TTL no args", "TTL", nil},
		{"EXPIRETIME", "EXPIRETIME", [][]byte{[]byte("key1")}},
		{"EXPIRETIME no args", "EXPIRETIME", nil},
		{"PEXPIRETIME", "PEXPIRETIME", [][]byte{[]byte("key1")}},
		{"PEXPIRETIME no args", "PEXPIRETIME", nil},
		{"RENAME", "RENAME", [][]byte{[]byte("key1"), []byte("key2")}},
		{"RENAME no args", "RENAME", nil},
		{"RENAMENX", "RENAMENX", [][]byte{[]byte("key1"), []byte("key2")}},
		{"RENAMENX no args", "RENAMENX", nil},
		{"SORT", "SORT", [][]byte{[]byte("list1")}},
		{"SORT no args", "SORT", nil},
		{"SORT DESC", "SORT", [][]byte{[]byte("list1"), []byte("DESC")}},
		{"SORT ALPHA", "SORT", [][]byte{[]byte("list1"), []byte("ALPHA")}},
		{"SORT LIMIT", "SORT", [][]byte{[]byte("list1"), []byte("LIMIT"), []byte("0"), []byte("10")}},
		{"DEL", "DEL", [][]byte{[]byte("key1"), []byte("key2")}},
		{"DEL no args", "DEL", nil},
		{"UNLINK", "UNLINK", [][]byte{[]byte("key1"), []byte("key2")}},
		{"UNLINK no args", "UNLINK", nil},
		{"RANDOMKEY", "RANDOMKEY", nil},
		{"EXISTS", "EXISTS", [][]byte{[]byte("key1"), []byte("key2")}},
		{"EXISTS no args", "EXISTS", nil},
		{"TYPE", "TYPE", [][]byte{[]byte("key1")}},
		{"TYPE no args", "TYPE", nil},
		{"KEYS", "KEYS", [][]byte{[]byte("pattern*")}},
		{"KEYS no args", "KEYS", nil},
		{"SCAN", "SCAN", [][]byte{[]byte("0")}},
		{"SCAN no args", "SCAN", nil},
		{"SCAN MATCH", "SCAN", [][]byte{[]byte("0"), []byte("MATCH"), []byte("pattern*")}},
		{"SCAN COUNT", "SCAN", [][]byte{[]byte("0"), []byte("COUNT"), []byte("10")}},
		{"TOUCH", "TOUCH", [][]byte{[]byte("key1"), []byte("key2")}},
		{"TOUCH no args", "TOUCH", nil},
	}

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("source", &store.StringValue{Data: []byte("source_value")}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("c"), []byte("a"), []byte("b")}}, store.SetOptions{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsDumpRestoreFullCoverage(t *testing.T) {
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
		{"DUMP", "DUMP", [][]byte{[]byte("key1")}},
		{"DUMP not found", "DUMP", [][]byte{[]byte("notfound")}},
		{"DUMP no args", "DUMP", nil},
		{"RESTORE", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("serialized_data")}},
		{"RESTORE no args", "RESTORE", nil},
		{"RESTORE ABSTTL", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("serialized_data"), []byte("ABSTTL")}},
		{"RESTORE IDLETIME", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("serialized_data"), []byte("IDLETIME"), []byte("1000")}},
		{"RESTORE FREQ", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("serialized_data"), []byte("FREQ"), []byte("10")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsMigrateFullCoverage(t *testing.T) {
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
		{"MIGRATE", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte("key1"), []byte("0"), []byte("1000")}},
		{"MIGRATE no args", "MIGRATE", nil},
		{"MIGRATE COPY", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte("key1"), []byte("0"), []byte("1000"), []byte("COPY")}},
		{"MIGRATE REPLACE", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte("key1"), []byte("0"), []byte("1000"), []byte("REPLACE")}},
		{"MIGRATE AUTH", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte("key1"), []byte("0"), []byte("1000"), []byte("AUTH"), []byte("password")}},
		{"MIGRATE KEYS", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte(""), []byte("0"), []byte("1000"), []byte("KEYS"), []byte("key1"), []byte("key2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsObjectFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

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
		{"OBJECT ENCODING string", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("str1")}},
		{"OBJECT ENCODING hash", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("hash1")}},
		{"OBJECT ENCODING list", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("list1")}},
		{"OBJECT ENCODING set", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("set1")}},
		{"OBJECT ENCODING zset", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("zset1")}},
		{"OBJECT ENCODING not found", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("notfound")}},
		{"OBJECT IDLETIME", "OBJECT", [][]byte{[]byte("IDLETIME"), []byte("str1")}},
		{"OBJECT IDLETIME no args", "OBJECT", [][]byte{[]byte("IDLETIME")}},
		{"OBJECT FREQ", "OBJECT", [][]byte{[]byte("FREQ"), []byte("str1")}},
		{"OBJECT FREQ no args", "OBJECT", [][]byte{[]byte("FREQ")}},
		{"OBJECT REFCOUNT", "OBJECT", [][]byte{[]byte("REFCOUNT"), []byte("str1")}},
		{"OBJECT REFCOUNT no args", "OBJECT", [][]byte{[]byte("REFCOUNT")}},
		{"OBJECT unknown", "OBJECT", [][]byte{[]byte("UNKNOWN"), []byte("str1")}},
		{"OBJECT no args", "OBJECT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsMemoryFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MEMORY USAGE", "MEMORY", [][]byte{[]byte("USAGE"), []byte("key1")}},
		{"MEMORY USAGE not found", "MEMORY", [][]byte{[]byte("USAGE"), []byte("notfound")}},
		{"MEMORY USAGE SAMPLES", "MEMORY", [][]byte{[]byte("USAGE"), []byte("key1"), []byte("SAMPLES"), []byte("5")}},
		{"MEMORY USAGE no args", "MEMORY", [][]byte{[]byte("USAGE")}},
		{"MEMORY STATS", "MEMORY", [][]byte{[]byte("STATS")}},
		{"MEMORY MALLOC-STATS", "MEMORY", [][]byte{[]byte("MALLOC-STATS")}},
		{"MEMORY DOCTOR", "MEMORY", [][]byte{[]byte("DOCTOR")}},
		{"MEMORY PURGE", "MEMORY", [][]byte{[]byte("PURGE")}},
		{"MEMORY no args", "MEMORY", nil},
		{"MEMORY unknown", "MEMORY", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsWaitSelectFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WAIT", "WAIT", [][]byte{[]byte("1"), []byte("1000")}},
		{"WAIT no args", "WAIT", nil},
		{"SELECT", "SELECT", [][]byte{[]byte("0")}},
		{"SELECT no args", "SELECT", nil},
		{"SWAPDB", "SWAPDB", [][]byte{[]byte("0"), []byte("1")}},
		{"SWAPDB no args", "SWAPDB", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsDebugFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEBUG SEGFAULT", "DEBUG", [][]byte{[]byte("SEGFAULT")}},
		{"DEBUG OBJECT", "DEBUG", [][]byte{[]byte("OBJECT"), []byte("key1")}},
		{"DEBUG OBJECT no args", "DEBUG", [][]byte{[]byte("OBJECT")}},
		{"DEBUG SLEEP", "DEBUG", [][]byte{[]byte("SLEEP"), []byte("100")}},
		{"DEBUG SLEEP no args", "DEBUG", [][]byte{[]byte("SLEEP")}},
		{"DEBUG unknown", "DEBUG", [][]byte{[]byte("UNKNOWN")}},
		{"DEBUG no args", "DEBUG", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
