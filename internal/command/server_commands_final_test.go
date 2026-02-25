package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestServerCommandsFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DBSIZE", "DBSIZE", nil},
		{"FLUSHDB", "FLUSHDB", nil},
		{"FLUSHALL", "FLUSHALL", nil},
		{"SAVE", "SAVE", nil},
		{"BGSAVE", "BGSAVE", nil},
		{"BGREWRITEAOF", "BGREWRITEAOF", nil},
		{"INFO", "INFO", nil},
		{"INFO section", "INFO", [][]byte{[]byte("server")}},
		{"INFO all", "INFO", [][]byte{[]byte("all")}},
		{"CONFIG GET", "CONFIG", [][]byte{[]byte("GET"), []byte("*")}},
		{"CONFIG GET pattern", "CONFIG", [][]byte{[]byte("GET"), []byte("maxmemory")}},
		{"CONFIG SET", "CONFIG", [][]byte{[]byte("SET"), []byte("maxmemory"), []byte("100mb")}},
		{"CONFIG no args", "CONFIG", nil},
		{"CONFIG unknown", "CONFIG", [][]byte{[]byte("UNKNOWN")}},
		{"MONITOR", "MONITOR", nil},
		{"SYNC", "SYNC", nil},
		{"PSYNC", "PSYNC", [][]byte{[]byte("?"), []byte("-1")}},
		{"PSYNC no args", "PSYNC", nil},
		{"SLOWLOG LEN", "SLOWLOG", [][]byte{[]byte("LEN")}},
		{"SLOWLOG RESET", "SLOWLOG", [][]byte{[]byte("RESET")}},
		{"SLOWLOG no args", "SLOWLOG", nil},
		{"TIME", "TIME", nil},
		{"LASTSAVE", "LASTSAVE", nil},
		{"CLIENT LIST", "CLIENT", [][]byte{[]byte("LIST")}},
		{"CLIENT no args", "CLIENT", nil},
		{"CLIENT unknown", "CLIENT", [][]byte{[]byte("UNKNOWN")}},
		{"COMMAND", "COMMAND", nil},
		{"COMMAND INFO", "COMMAND", [][]byte{[]byte("INFO"), []byte("GET")}},
		{"COMMAND COUNT", "COMMAND", [][]byte{[]byte("COUNT")}},
		{"LATENCY", "LATENCY", nil},
		{"LATENCY DOCTOR", "LATENCY", [][]byte{[]byte("DOCTOR")}},
		{"LATENCY GRAPH", "LATENCY", [][]byte{[]byte("GRAPH"), []byte("command")}},
		{"LATENCY HISTORY", "LATENCY", [][]byte{[]byte("HISTORY"), []byte("command")}},
		{"LATENCY RESET", "LATENCY", [][]byte{[]byte("RESET")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsAUTHFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"AUTH with password", "AUTH", [][]byte{[]byte("password")}},
		{"AUTH no args", "AUTH", nil},
		{"AUTH username password", "AUTH", [][]byte{[]byte("username"), []byte("password")}},
		{"PING", "PING", nil},
		{"PING message", "PING", [][]byte{[]byte("hello")}},
		{"ECHO", "ECHO", [][]byte{[]byte("hello world")}},
		{"ECHO no args", "ECHO", nil},
		{"SELECT 0", "SELECT", [][]byte{[]byte("0")}},
		{"SELECT 15", "SELECT", [][]byte{[]byte("15")}},
		{"SELECT invalid", "SELECT", [][]byte{[]byte("999")}},
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

func TestServerCommandsPersistenceFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SAVE", "SAVE", nil},
		{"BGSAVE", "BGSAVE", nil},
		{"BGREWRITEAOF", "BGREWRITEAOF", nil},
		{"LASTSAVE", "LASTSAVE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsInfoFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"INFO server", "INFO", [][]byte{[]byte("server")}},
		{"INFO clients", "INFO", [][]byte{[]byte("clients")}},
		{"INFO memory", "INFO", [][]byte{[]byte("memory")}},
		{"INFO persistence", "INFO", [][]byte{[]byte("persistence")}},
		{"INFO stats", "INFO", [][]byte{[]byte("stats")}},
		{"INFO replication", "INFO", [][]byte{[]byte("replication")}},
		{"INFO cpu", "INFO", [][]byte{[]byte("cpu")}},
		{"INFO commandstats", "INFO", [][]byte{[]byte("commandstats")}},
		{"INFO cluster", "INFO", [][]byte{[]byte("cluster")}},
		{"INFO keyspace", "INFO", [][]byte{[]byte("keyspace")}},
		{"INFO all", "INFO", [][]byte{[]byte("all")}},
		{"INFO default", "INFO", [][]byte{[]byte("default")}},
		{"INFO no args", "INFO", nil},
		{"INFO unknown", "INFO", [][]byte{[]byte("unknown")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsConfigFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONFIG GET maxmemory", "CONFIG", [][]byte{[]byte("GET"), []byte("maxmemory")}},
		{"CONFIG GET maxclients", "CONFIG", [][]byte{[]byte("GET"), []byte("maxclients")}},
		{"CONFIG GET timeout", "CONFIG", [][]byte{[]byte("GET"), []byte("timeout")}},
		{"CONFIG GET dir", "CONFIG", [][]byte{[]byte("GET"), []byte("dir")}},
		{"CONFIG GET dbfilename", "CONFIG", [][]byte{[]byte("GET"), []byte("dbfilename")}},
		{"CONFIG GET *", "CONFIG", [][]byte{[]byte("GET"), []byte("*")}},
		{"CONFIG SET maxmemory 100mb", "CONFIG", [][]byte{[]byte("SET"), []byte("maxmemory"), []byte("100mb")}},
		{"CONFIG SET maxclients 1000", "CONFIG", [][]byte{[]byte("SET"), []byte("maxclients"), []byte("1000")}},
		{"CONFIG SET timeout 300", "CONFIG", [][]byte{[]byte("SET"), []byte("timeout"), []byte("300")}},
		{"CONFIG RESETSTAT", "CONFIG", [][]byte{[]byte("RESETSTAT")}},
		{"CONFIG REWRITE", "CONFIG", [][]byte{[]byte("REWRITE")}},
		{"CONFIG no args", "CONFIG", nil},
		{"CONFIG unknown subcommand", "CONFIG", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsClientFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLIENT LIST", "CLIENT", [][]byte{[]byte("LIST")}},
		{"CLIENT SETNAME name", "CLIENT", [][]byte{[]byte("SETNAME"), []byte("test-client")}},
		{"CLIENT GETNAME", "CLIENT", [][]byte{[]byte("GETNAME")}},
		{"CLIENT KILL ip:port", "CLIENT", [][]byte{[]byte("KILL"), []byte("127.0.0.1:6379")}},
		{"CLIENT PAUSE 100", "CLIENT", [][]byte{[]byte("PAUSE"), []byte("100")}},
		{"CLIENT REPLY ON", "CLIENT", [][]byte{[]byte("REPLY"), []byte("ON")}},
		{"CLIENT REPLY OFF", "CLIENT", [][]byte{[]byte("REPLY"), []byte("OFF")}},
		{"CLIENT REPLY SKIP", "CLIENT", [][]byte{[]byte("REPLY"), []byte("SKIP")}},
		{"CLIENT no args", "CLIENT", nil},
		{"CLIENT unknown", "CLIENT", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsSlowlogFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SLOWLOG LEN", "SLOWLOG", [][]byte{[]byte("LEN")}},
		{"SLOWLOG RESET", "SLOWLOG", [][]byte{[]byte("RESET")}},
		{"SLOWLOG no args", "SLOWLOG", nil},
		{"SLOWLOG unknown", "SLOWLOG", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsCommandFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COMMAND", "COMMAND", nil},
		{"COMMAND COUNT", "COMMAND", [][]byte{[]byte("COUNT")}},
		{"COMMAND INFO GET", "COMMAND", [][]byte{[]byte("INFO"), []byte("GET")}},
		{"COMMAND INFO SET GET", "COMMAND", [][]byte{[]byte("INFO"), []byte("SET"), []byte("GET")}},
		{"COMMAND GETKEYS", "COMMAND", [][]byte{[]byte("GETKEYS"), []byte("SET"), []byte("key"), []byte("value")}},
		{"COMMAND no args", "COMMAND", nil},
		{"COMMAND unknown", "COMMAND", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsDebugFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEBUG OBJECT key1", "DEBUG", [][]byte{[]byte("OBJECT"), []byte("key1")}},
		{"DEBUG OBJECT notfound", "DEBUG", [][]byte{[]byte("OBJECT"), []byte("notfound")}},
		{"DEBUG SLEEP 100", "DEBUG", [][]byte{[]byte("SLEEP"), []byte("100")}},
		{"DEBUG SLEEP no time", "DEBUG", [][]byte{[]byte("SLEEP")}},
		{"DEBUG SEGFAULT", "DEBUG", [][]byte{[]byte("SEGFAULT")}},
		{"DEBUG no args", "DEBUG", nil},
		{"DEBUG unknown", "DEBUG", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsACLFinal(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ACL LIST", "ACL", [][]byte{[]byte("LIST")}},
		{"ACL USERS", "ACL", [][]byte{[]byte("USERS")}},
		{"ACL GETUSER default", "ACL", [][]byte{[]byte("GETUSER"), []byte("default")}},
		{"ACL SETUSER", "ACL", [][]byte{[]byte("SETUSER"), []byte("newuser")}},
		{"ACL DELUSER", "ACL", [][]byte{[]byte("DELUSER"), []byte("newuser")}},
		{"ACL CAT", "ACL", [][]byte{[]byte("CAT")}},
		{"ACL CAT string", "ACL", [][]byte{[]byte("CAT"), []byte("string")}},
		{"ACL GENPASS", "ACL", [][]byte{[]byte("GENPASS")}},
		{"ACL GENPASS 32", "ACL", [][]byte{[]byte("GENPASS"), []byte("32")}},
		{"ACL WHOAMI", "ACL", [][]byte{[]byte("WHOAMI")}},
		{"ACL LOG", "ACL", [][]byte{[]byte("LOG")}},
		{"ACL HELP", "ACL", [][]byte{[]byte("HELP")}},
		{"ACL no args", "ACL", nil},
		{"ACL unknown", "ACL", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
