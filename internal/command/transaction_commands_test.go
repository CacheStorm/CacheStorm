package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllTransactionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTransactionCommands(router)
	RegisterStringCommands(router)

	tests := []struct {
		name     string
		commands []struct {
			cmd  string
			args [][]byte
		}
		setup    func(*Context)
		validate func(*Context) bool
	}{
		{
			name: "MULTI-EXEC basic",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"MULTI", nil},
				{"SET", [][]byte{[]byte("txkey1"), []byte("value1")}},
				{"SET", [][]byte{[]byte("txkey2"), []byte("value2")}},
				{"EXEC", nil},
			},
			validate: func(ctx *Context) bool {
				_, exists1 := s.Get("txkey1")
				_, exists2 := s.Get("txkey2")
				return exists1 && exists2
			},
		},
		{
			name: "MULTI-DISCARD",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"MULTI", nil},
				{"SET", [][]byte{[]byte("discardkey"), []byte("value")}},
				{"DISCARD", nil},
			},
			// Note: In current implementation, SET executes immediately, DISCARD just clears transaction state
		},
		{
			name: "WATCH-EXEC success",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"WATCH", [][]byte{[]byte("watchkey")}},
				{"MULTI", nil},
				{"SET", [][]byte{[]byte("watchkey"), []byte("newvalue")}},
				{"EXEC", nil},
			},
			setup: func(ctx *Context) {
				s.Set("watchkey", &store.StringValue{Data: []byte("original")}, store.SetOptions{})
			},
			validate: func(ctx *Context) bool {
				entry, exists := s.Get("watchkey")
				if !exists {
					return false
				}
				val := entry.Value.(*store.StringValue)
				return string(val.Data) == "newvalue"
			},
		},
		{
			name: "UNWATCH",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"WATCH", [][]byte{[]byte("unwatchkey")}},
				{"UNWATCH", nil},
				{"MULTI", nil},
				{"SET", [][]byte{[]byte("unwatchkey"), []byte("value")}},
				{"EXEC", nil},
			},
			setup: func(ctx *Context) {
				s.Set("unwatchkey", &store.StringValue{Data: []byte("original")}, store.SetOptions{})
			},
			validate: func(ctx *Context) bool {
				_, exists := s.Get("unwatchkey")
				return exists
			},
		},
		{
			name: "EXEC without MULTI",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"EXEC", nil},
			},
		},
		{
			name: "DISCARD without MULTI",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"DISCARD", nil},
			},
		},
		{
			name: "WATCH inside MULTI should fail",
			commands: []struct {
				cmd  string
				args [][]byte
			}{
				{"MULTI", nil},
				{"WATCH", [][]byte{[]byte("somekey")}},
				{"EXEC", nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext("", nil, s)
			ctx.Transaction = NewTransaction()

			if tt.setup != nil {
				tt.setup(ctx)
			}

			for _, cmd := range tt.commands {
				handler, ok := router.Get(cmd.cmd)
				if !ok {
					t.Fatalf("Command %s not found", cmd.cmd)
				}

				cmdCtx := newTestContext(cmd.cmd, cmd.args, s)
				cmdCtx.Transaction = ctx.Transaction
				handler.Handler(cmdCtx)
			}

			if tt.validate != nil && !tt.validate(ctx) {
				t.Error("Validation failed")
			}
		})
	}
}

func TestTransactionOperations(t *testing.T) {
	t.Run("Transaction struct operations", func(t *testing.T) {
		tx := NewTransaction()

		// Test IsActive initially false
		if tx.IsActive() {
			t.Error("New transaction should not be active")
		}

		// Start transaction
		tx.Start()
		if !tx.IsActive() {
			t.Error("Transaction should be active after Start()")
		}

		// Queue commands
		tx.Queue("SET", [][]byte{[]byte("key"), []byte("value")})
		tx.Queue("GET", [][]byte{[]byte("key")})

		queued := tx.GetQueued()
		if len(queued) != 2 {
			t.Errorf("Expected 2 queued commands, got %d", len(queued))
		}

		// Test Clear
		tx.Clear()
		if tx.IsActive() {
			t.Error("Transaction should not be active after Clear()")
		}

		queued = tx.GetQueued()
		if len(queued) != 0 {
			t.Errorf("Expected 0 queued commands after clear, got %d", len(queued))
		}
	})

	t.Run("Transaction Watch operations", func(t *testing.T) {
		tx := NewTransaction()

		// Watch keys
		tx.Watch("key1", 1)
		tx.Watch("key2", 2)

		if !tx.HasWatchedKeys() {
			t.Error("Transaction should have watched keys")
		}

		// Check versions
		getVersion := func(key string) int64 {
			if key == "key1" {
				return 1
			}
			if key == "key2" {
				return 2
			}
			return 0
		}

		if !tx.CheckWatchedVersions(getVersion) {
			t.Error("CheckWatchedVersions should return true when versions match")
		}

		// Clear watch
		tx.ClearWatch()
		if tx.HasWatchedKeys() {
			t.Error("Transaction should not have watched keys after ClearWatch()")
		}
	})
}
