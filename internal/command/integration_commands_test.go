package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestIntegrationCommandsCIRCUITBREAKERFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUITBREAKER.CREATE", "CIRCUITBREAKER.CREATE", [][]byte{[]byte("cb1"), []byte("5"), []byte("60000")}},
		{"CIRCUITBREAKER.CREATE no args", "CIRCUITBREAKER.CREATE", nil},
		{"CIRCUITBREAKER.STATE", "CIRCUITBREAKER.STATE", [][]byte{[]byte("cb1")}},
		{"CIRCUITBREAKER.STATE not found", "CIRCUITBREAKER.STATE", [][]byte{[]byte("notfound")}},
		{"CIRCUITBREAKER.STATE no args", "CIRCUITBREAKER.STATE", nil},
		{"CIRCUITBREAKER.TRIP", "CIRCUITBREAKER.TRIP", [][]byte{[]byte("cb1")}},
		{"CIRCUITBREAKER.TRIP no args", "CIRCUITBREAKER.TRIP", nil},
		{"CIRCUITBREAKER.RESET", "CIRCUITBREAKER.RESET", [][]byte{[]byte("cb1")}},
		{"CIRCUITBREAKER.RESET no args", "CIRCUITBREAKER.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsRATELIMITFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RATELIMIT.CREATE", "RATELIMIT.CREATE", [][]byte{[]byte("rl1"), []byte("100"), []byte("60000")}},
		{"RATELIMIT.CREATE no args", "RATELIMIT.CREATE", nil},
		{"RATELIMIT.CHECK", "RATELIMIT.CHECK", [][]byte{[]byte("rl1")}},
		{"RATELIMIT.CHECK not found", "RATELIMIT.CHECK", [][]byte{[]byte("notfound")}},
		{"RATELIMIT.CHECK no args", "RATELIMIT.CHECK", nil},
		{"RATELIMIT.ALLOW", "RATELIMIT.ALLOW", [][]byte{[]byte("rl1")}},
		{"RATELIMIT.ALLOW no args", "RATELIMIT.ALLOW", nil},
		{"RATELIMIT.INFO", "RATELIMIT.INFO", [][]byte{[]byte("rl1")}},
		{"RATELIMIT.INFO no args", "RATELIMIT.INFO", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsCACHELOCKFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.LOCK", "CACHE.LOCK", [][]byte{[]byte("key1"), []byte("1000")}},
		{"CACHE.LOCK no args", "CACHE.LOCK", nil},
		{"CACHE.UNLOCK", "CACHE.UNLOCK", [][]byte{[]byte("key1")}},
		{"CACHE.UNLOCK not found", "CACHE.UNLOCK", [][]byte{[]byte("notfound")}},
		{"CACHE.UNLOCK no args", "CACHE.UNLOCK", nil},
		{"CACHE.ISLOCKED", "CACHE.ISLOCKED", [][]byte{[]byte("key1")}},
		{"CACHE.ISLOCKED no args", "CACHE.ISLOCKED", nil},
		{"CACHE.REFRESH", "CACHE.REFRESH", [][]byte{[]byte("key1"), []byte("60000")}},
		{"CACHE.REFRESH no args", "CACHE.REFRESH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsOBJECTFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBJECT.FROMENTRIES", "OBJECT.FROMENTRIES", [][]byte{[]byte(`[["key1","value1"],["key2","value2"]]`)}},
		{"OBJECT.FROMENTRIES no args", "OBJECT.FROMENTRIES", nil},
		{"OBJECT.MERGE", "OBJECT.MERGE", [][]byte{[]byte(`{"a":1}`), []byte(`{"b":2}`)}},
		{"OBJECT.MERGE no args", "OBJECT.MERGE", nil},
		{"OBJECT.GET", "OBJECT.GET", [][]byte{[]byte(`{"key":"value"}`), []byte("key")}},
		{"OBJECT.GET not found", "OBJECT.GET", [][]byte{[]byte(`{"key":"value"}`), []byte("notfound")}},
		{"OBJECT.GET no args", "OBJECT.GET", nil},
		{"OBJECT.SET", "OBJECT.SET", [][]byte{[]byte(`{"key":"value"}`), []byte("newkey"), []byte("newvalue")}},
		{"OBJECT.SET no args", "OBJECT.SET", nil},
		{"OBJECT.DELETE", "OBJECT.DELETE", [][]byte{[]byte(`{"key":"value"}`), []byte("key")}},
		{"OBJECT.DELETE no args", "OBJECT.DELETE", nil},
		{"OBJECT.KEYS", "OBJECT.KEYS", [][]byte{[]byte(`{"a":1,"b":2}`)}},
		{"OBJECT.KEYS no args", "OBJECT.KEYS", nil},
		{"OBJECT.VALUES", "OBJECT.VALUES", [][]byte{[]byte(`{"a":1,"b":2}`)}},
		{"OBJECT.VALUES no args", "OBJECT.VALUES", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsARRAYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ARRAY.MERGE", "ARRAY.MERGE", [][]byte{[]byte(`[1,2]`), []byte(`[3,4]`)}},
		{"ARRAY.MERGE no args", "ARRAY.MERGE", nil},
		{"ARRAY.UNIQUE", "ARRAY.UNIQUE", [][]byte{[]byte(`[1,2,2,3,3,3]`)}},
		{"ARRAY.UNIQUE no args", "ARRAY.UNIQUE", nil},
		{"ARRAY.FLATTEN", "ARRAY.FLATTEN", [][]byte{[]byte(`[[1,2],[3,4]]`)}},
		{"ARRAY.FLATTEN no args", "ARRAY.FLATTEN", nil},
		{"ARRAY.GROUP", "ARRAY.GROUP", [][]byte{[]byte(`[1,2,3,4,5]`), []byte("2")}},
		{"ARRAY.GROUP no args", "ARRAY.GROUP", nil},
		{"ARRAY.SLICE", "ARRAY.SLICE", [][]byte{[]byte(`[1,2,3,4,5]`), []byte("1"), []byte("3")}},
		{"ARRAY.SLICE no args", "ARRAY.SLICE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsPIPELINEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PIPELINE.START", "PIPELINE.START", nil},
		{"PIPELINE.EXEC", "PIPELINE.EXEC", nil},
		{"PIPELINE.DISCARD", "PIPELINE.DISCARD", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsBATCHFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCH.START", "BATCH.START", nil},
		{"BATCH.EXEC", "BATCH.EXEC", nil},
		{"BATCH.DISCARD", "BATCH.DISCARD", nil},
		{"BATCH.STATUS", "BATCH.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsMIGRATEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

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
		{"MIGRATE KEYS", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte(""), []byte("0"), []byte("1000"), []byte("KEYS"), []byte("key1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsREPLICATIONFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLICATION.START", "REPLICATION.START", nil},
		{"REPLICATION.STOP", "REPLICATION.STOP", nil},
		{"REPLICATION.STATUS", "REPLICATION.STATUS", nil},
		{"REPLICATION.SYNC", "REPLICATION.SYNC", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsFEDERATIONFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FEDERATION.JOIN", "FEDERATION.JOIN", [][]byte{[]byte("cluster1")}},
		{"FEDERATION.JOIN no args", "FEDERATION.JOIN", nil},
		{"FEDERATION.LEAVE", "FEDERATION.LEAVE", [][]byte{[]byte("cluster1")}},
		{"FEDERATION.LEAVE no args", "FEDERATION.LEAVE", nil},
		{"FEDERATION.QUERY", "FEDERATION.QUERY", [][]byte{[]byte("key1")}},
		{"FEDERATION.QUERY no args", "FEDERATION.QUERY", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsPROXYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PROXY.ADD", "PROXY.ADD", [][]byte{[]byte("127.0.0.1:6379")}},
		{"PROXY.ADD no args", "PROXY.ADD", nil},
		{"PROXY.REMOVE", "PROXY.REMOVE", [][]byte{[]byte("127.0.0.1:6379")}},
		{"PROXY.REMOVE no args", "PROXY.REMOVE", nil},
		{"PROXY.LIST", "PROXY.LIST", nil},
		{"PROXY.ROUTE", "PROXY.ROUTE", [][]byte{[]byte("key1")}},
		{"PROXY.ROUTE no args", "PROXY.ROUTE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsADAPTERFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ADAPTER.REGISTER", "ADAPTER.REGISTER", [][]byte{[]byte("adapter1"), []byte("type1")}},
		{"ADAPTER.REGISTER no args", "ADAPTER.REGISTER", nil},
		{"ADAPTER.UNREGISTER", "ADAPTER.UNREGISTER", [][]byte{[]byte("adapter1")}},
		{"ADAPTER.UNREGISTER no args", "ADAPTER.UNREGISTER", nil},
		{"ADAPTER.LIST", "ADAPTER.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsCONNECTORFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONNECTOR.CREATE", "CONNECTOR.CREATE", [][]byte{[]byte("conn1"), []byte("type1")}},
		{"CONNECTOR.CREATE no args", "CONNECTOR.CREATE", nil},
		{"CONNECTOR.DELETE", "CONNECTOR.DELETE", [][]byte{[]byte("conn1")}},
		{"CONNECTOR.DELETE no args", "CONNECTOR.DELETE", nil},
		{"CONNECTOR.STATUS", "CONNECTOR.STATUS", [][]byte{[]byte("conn1")}},
		{"CONNECTOR.STATUS no args", "CONNECTOR.STATUS", nil},
		{"CONNECTOR.LIST", "CONNECTOR.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsBRIDGEFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BRIDGE.CREATE", "BRIDGE.CREATE", [][]byte{[]byte("bridge1"), []byte("source1"), []byte("target1")}},
		{"BRIDGE.CREATE no args", "BRIDGE.CREATE", nil},
		{"BRIDGE.DELETE", "BRIDGE.DELETE", [][]byte{[]byte("bridge1")}},
		{"BRIDGE.DELETE no args", "BRIDGE.DELETE", nil},
		{"BRIDGE.SYNC", "BRIDGE.SYNC", [][]byte{[]byte("bridge1")}},
		{"BRIDGE.SYNC no args", "BRIDGE.SYNC", nil},
		{"BRIDGE.LIST", "BRIDGE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
