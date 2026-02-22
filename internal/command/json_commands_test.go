package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllJSONCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterJSONCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"JSON.SET object", "JSON.SET", [][]byte{[]byte("json1"), []byte("$"), []byte(`{"name":"John","age":30}`)}, nil},
		{"JSON.SET nested", "JSON.SET", [][]byte{[]byte("json2"), []byte("$"), []byte(`{"user":{"name":"Alice","contacts":{"email":"alice@example.com"}}}`)}, nil},
		{"JSON.SET array", "JSON.SET", [][]byte{[]byte("json3"), []byte("$"), []byte(`[1,2,3,4,5]`)}, nil},
		{"JSON.GET root", "JSON.GET", [][]byte{[]byte("json4")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"name": "Bob", "age": 25})
			s.Set("json4", jsonVal, store.SetOptions{})
		}},
		{"JSON.GET with path", "JSON.GET", [][]byte{[]byte("json5"), []byte("$.name")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"name": "Charlie", "age": 35})
			s.Set("json5", jsonVal, store.SetOptions{})
		}},
		{"JSON.GET multiple paths", "JSON.GET", [][]byte{[]byte("json6"), []byte("$.name"), []byte("$.age")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"name": "David", "age": 40})
			s.Set("json6", jsonVal, store.SetOptions{})
		}},
		{"JSON.DEL", "JSON.DEL", [][]byte{[]byte("json7")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"test": "data"})
			s.Set("json7", jsonVal, store.SetOptions{})
		}},
		{"JSON.DEL path", "JSON.DEL", [][]byte{[]byte("json8"), []byte("$.name")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"name": "Eve", "age": 28})
			s.Set("json8", jsonVal, store.SetOptions{})
		}},
		{"JSON.TYPE", "JSON.TYPE", [][]byte{[]byte("json9")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"type": "test"})
			s.Set("json9", jsonVal, store.SetOptions{})
		}},
		{"JSON.TYPE with path", "JSON.TYPE", [][]byte{[]byte("json10"), []byte("$.name")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"name": "Frank"})
			s.Set("json10", jsonVal, store.SetOptions{})
		}},
		{"JSON.NUMINCRBY", "JSON.NUMINCRBY", [][]byte{[]byte("json11"), []byte("$.counter"), []byte("5")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"counter": 10})
			s.Set("json11", jsonVal, store.SetOptions{})
		}},
		{"JSON.NUMMULTBY", "JSON.NUMMULTBY", [][]byte{[]byte("json12"), []byte("$.value"), []byte("2")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"value": 5})
			s.Set("json12", jsonVal, store.SetOptions{})
		}},
		{"JSON.STRAPPEND", "JSON.STRAPPEND", [][]byte{[]byte("json13"), []byte("$.text"), []byte(`" World"`)}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"text": "Hello"})
			s.Set("json13", jsonVal, store.SetOptions{})
		}},
		{"JSON.STRLEN", "JSON.STRLEN", [][]byte{[]byte("json14"), []byte("$.text")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"text": "Hello World"})
			s.Set("json14", jsonVal, store.SetOptions{})
		}},
		{"JSON.ARRAPPEND", "JSON.ARRAPPEND", [][]byte{[]byte("json15"), []byte("$.items"), []byte("4"), []byte("5")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"items": []int{1, 2, 3}})
			s.Set("json15", jsonVal, store.SetOptions{})
		}},
		{"JSON.ARRLEN", "JSON.ARRLEN", [][]byte{[]byte("json17"), []byte("$.items")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"items": []int{1, 2, 3, 4, 5}})
			s.Set("json17", jsonVal, store.SetOptions{})
		}},
		{"JSON.OBJKEYS", "JSON.OBJKEYS", [][]byte{[]byte("json16")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"name": "Grace", "age": 32, "city": "NYC"})
			s.Set("json16", jsonVal, store.SetOptions{})
		}},
		{"JSON.OBJLEN", "JSON.OBJLEN", [][]byte{[]byte("json17")}, func() {
			jsonVal, _ := store.NewJSONValue(map[string]interface{}{"a": 1, "b": 2, "c": 3})
			s.Set("json17", jsonVal, store.SetOptions{})
		}},
		{"JSON.MGET", "JSON.MGET", [][]byte{[]byte("json18"), []byte("json19"), []byte("$")}, func() {
			jsonVal1, _ := store.NewJSONValue(map[string]interface{}{"data": "value1"})
			s.Set("json18", jsonVal1, store.SetOptions{})
			jsonVal2, _ := store.NewJSONValue(map[string]interface{}{"data": "value2"})
			s.Set("json19", jsonVal2, store.SetOptions{})
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

func TestJSONValueOperations(t *testing.T) {
	t.Run("JSON Value Creation", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "Test",
			"value": 123,
		}
		jsonVal, err := store.NewJSONValue(data)
		if err != nil {
			t.Errorf("NewJSONValue failed: %v", err)
		}
		if jsonVal == nil {
			t.Fatal("NewJSONValue returned nil")
		}
	})

	t.Run("JSON Value Get", func(t *testing.T) {
		data := map[string]interface{}{"key": "value"}
		jsonVal, _ := store.NewJSONValue(data)

		result, err := jsonVal.Get()
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if result == nil {
			t.Error("Get returned nil")
		}
	})

	t.Run("JSON Value String", func(t *testing.T) {
		data := map[string]interface{}{"name": "Alice"}
		jsonVal, _ := store.NewJSONValue(data)

		str := jsonVal.String()
		if str == "" {
			t.Error("String returned empty")
		}
	})

	t.Run("JSON Value Clone", func(t *testing.T) {
		data := map[string]interface{}{"data": "test"}
		jsonVal, _ := store.NewJSONValue(data)

		cloned := jsonVal.Clone()
		if cloned == nil {
			t.Error("Clone returned nil")
		}
	})
}
